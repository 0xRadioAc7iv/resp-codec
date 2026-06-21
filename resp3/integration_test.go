//go:build integration

// Integration tests against a real Redis instance, verifying that this
// package's Encode/Decode round-trip correctly with what an actual Redis
// server sends and accepts on the wire — not just against synthetic frames.
//
// These do not run as part of the normal test suite. Run them with:
//
//	go test -tags integration ./resp3/...
//
// against a Redis 6+ server (defaults to localhost:6379; override with the
// REDIS_ADDR environment variable). Tests skip if no server is reachable.
package resp3_test

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	respcodec "github.com/0xRadioAc7iv/resp-codec/v2"
	"github.com/0xRadioAc7iv/resp-codec/v2/resp3"
)

func redisAddr() string {
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		return addr
	}
	return "localhost:6379"
}

// dial opens a connection to Redis and switches it to RESP3 via HELLO 3,
// skipping the test if no server is reachable.
func dial(t *testing.T) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", redisAddr(), 2*time.Second)
	if err != nil {
		t.Skipf("no Redis server reachable at %s: %v", redisAddr(), err)
	}
	t.Cleanup(func() { conn.Close() })
	r := bufio.NewReader(conn)

	v := sendCmd(t, conn, r, "HELLO", "3")
	if _, ok := v.(map[respcodec.SimpleString]any); !ok {
		t.Fatalf("expected HELLO 3 reply to be a map, got %T (%v)", v, v)
	}
	return conn, r
}

// sendCmd encodes args as a RESP3 command array (Redis always expects
// requests in this form, regardless of negotiated reply protocol), writes
// it, reads exactly one reply frame off the wire, and decodes it. Non-string
// arguments are stringified, since Redis command arguments are always bulk
// strings on the wire.
func sendCmd(t *testing.T, conn net.Conn, r *bufio.Reader, args ...any) any {
	t.Helper()
	cmd := make([]any, len(args))
	for i, a := range args {
		if s, ok := a.(string); ok {
			cmd[i] = s
		} else {
			cmd[i] = fmt.Sprint(a)
		}
	}

	buf, err := resp3.Encode(cmd)
	if err != nil {
		t.Fatalf("encode command %v: %v", args, err)
	}
	if _, err := conn.Write(buf); err != nil {
		t.Fatalf("write command %v: %v", args, err)
	}

	frame, err := readFrame(r)
	if err != nil {
		t.Fatalf("read reply for %v: %v", args, err)
	}
	v, err := resp3.Decode(frame)
	if err != nil {
		t.Fatalf("decode reply for %v (raw %q): %v", args, frame, err)
	}
	return v
}

// readFrame reads exactly one complete RESP3 frame from r and returns its
// raw bytes. It mirrors the same per-type framing rules resp3.Decode itself
// uses, but operates on a live stream instead of a pre-sliced buffer, since
// we don't know frame lengths ahead of time when reading off the wire.
func readFrame(r *bufio.Reader) ([]byte, error) {
	sigil, err := r.ReadByte()
	if err != nil {
		return nil, err
	}

	switch sigil {
	case '+', '-', ':', '(', ',', '_', '#':
		line, err := r.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		return append([]byte{sigil}, line...), nil

	case '$', '!', '=':
		lengthLine, err := r.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		length, err := strconv.Atoi(strings.TrimSpace(string(lengthLine)))
		if err != nil {
			return nil, fmt.Errorf("invalid blob length %q: %w", lengthLine, err)
		}
		data := make([]byte, length+2) // +2 for trailing \r\n
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, err
		}
		frame := append([]byte{sigil}, lengthLine...)
		return append(frame, data...), nil

	case '*', '~', '>':
		countLine, err := r.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		count, err := strconv.Atoi(strings.TrimSpace(string(countLine)))
		if err != nil {
			return nil, fmt.Errorf("invalid count %q: %w", countLine, err)
		}
		frame := append([]byte{sigil}, countLine...)
		for range count {
			elem, err := readFrame(r)
			if err != nil {
				return nil, err
			}
			frame = append(frame, elem...)
		}
		return frame, nil

	case '%', '|':
		countLine, err := r.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		pairs, err := strconv.Atoi(strings.TrimSpace(string(countLine)))
		if err != nil {
			return nil, fmt.Errorf("invalid pair count %q: %w", countLine, err)
		}
		frame := append([]byte{sigil}, countLine...)
		for range pairs * 2 {
			elem, err := readFrame(r)
			if err != nil {
				return nil, err
			}
			frame = append(frame, elem...)
		}
		return frame, nil

	default:
		return nil, fmt.Errorf("unknown RESP3 sigil while reading frame: %q", sigil)
	}
}

// testKey returns a unique key for the running test and registers its
// cleanup (DEL) so integration runs don't leave state behind.
func testKey(t *testing.T, conn net.Conn, r *bufio.Reader) string {
	t.Helper()
	key := fmt.Sprintf("resp3-integration:%s:%d", t.Name(), time.Now().UnixNano())
	t.Cleanup(func() { sendCmd(t, conn, r, "DEL", key) })
	return key
}

func TestIntegrationPing(t *testing.T) {
	conn, r := dial(t)
	got := sendCmd(t, conn, r, "PING")
	if got != respcodec.SimpleString("PONG") {
		t.Errorf("expected SimpleString(\"PONG\"), got %#v", got)
	}
}

func TestIntegrationSetGet(t *testing.T) {
	conn, r := dial(t)
	key := testKey(t, conn, r)

	got := sendCmd(t, conn, r, "SET", key, "hello")
	if got != respcodec.SimpleString("OK") {
		t.Fatalf("expected SimpleString(\"OK\"), got %#v", got)
	}

	got = sendCmd(t, conn, r, "GET", key)
	if got != "hello" {
		t.Errorf("expected \"hello\", got %#v", got)
	}
}

func TestIntegrationGetMissingKeyReturnsNull(t *testing.T) {
	conn, r := dial(t)
	key := testKey(t, conn, r) // never set, only used for a unique never-existing name

	got := sendCmd(t, conn, r, "GET", key)
	if got != resp3.Null {
		t.Errorf("expected resp3.Null, got %#v", got)
	}
}

func TestIntegrationIncr(t *testing.T) {
	conn, r := dial(t)
	key := testKey(t, conn, r)

	got := sendCmd(t, conn, r, "INCR", key)
	if got != 1 {
		t.Errorf("expected 1, got %#v", got)
	}
	got = sendCmd(t, conn, r, "INCR", key)
	if got != 2 {
		t.Errorf("expected 2, got %#v", got)
	}
}

func TestIntegrationHashAsMap(t *testing.T) {
	conn, r := dial(t)
	key := testKey(t, conn, r)

	sendCmd(t, conn, r, "HSET", key, "a", "1", "b", "2")

	got := sendCmd(t, conn, r, "HGETALL", key)
	m, ok := got.(map[respcodec.SimpleString]any)
	if !ok {
		t.Fatalf("expected map[respcodec.SimpleString]any, got %T (%#v)", got, got)
	}
	if m[respcodec.SimpleString("a")] != "1" || m[respcodec.SimpleString("b")] != "2" {
		t.Errorf("expected {a:1 b:2}, got %#v", m)
	}
}

func TestIntegrationSetAsSet(t *testing.T) {
	conn, r := dial(t)
	key := testKey(t, conn, r)

	sendCmd(t, conn, r, "SADD", key, "x", "y")

	got := sendCmd(t, conn, r, "SMEMBERS", key)
	s, ok := got.(map[any]struct{})
	if !ok {
		t.Fatalf("expected map[any]struct{}, got %T (%#v)", got, got)
	}
	if _, ok := s["x"]; !ok {
		t.Errorf("expected member \"x\" in set, got %#v", s)
	}
	if _, ok := s["y"]; !ok {
		t.Errorf("expected member \"y\" in set, got %#v", s)
	}
}

func TestIntegrationZScoreAsDouble(t *testing.T) {
	conn, r := dial(t)
	key := testKey(t, conn, r)

	sendCmd(t, conn, r, "ZADD", key, "1.5", "member")

	got := sendCmd(t, conn, r, "ZSCORE", key, "member")
	f, ok := got.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T (%#v)", got, got)
	}
	if f != 1.5 {
		t.Errorf("expected 1.5, got %v", f)
	}
}

func TestIntegrationLolwutVerbatimString(t *testing.T) {
	conn, r := dial(t)

	got := sendCmd(t, conn, r, "LOLWUT")
	if _, ok := got.(resp3.VerbatimString); !ok {
		t.Errorf("expected resp3.VerbatimString, got %T (%#v)", got, got)
	}
}

func TestIntegrationWrongTypeError(t *testing.T) {
	conn, r := dial(t)
	key := testKey(t, conn, r)

	sendCmd(t, conn, r, "SET", key, "not-a-list")

	got := sendCmd(t, conn, r, "LPUSH", key, "x")
	e, ok := got.(error)
	if !ok {
		t.Fatalf("expected error, got %T (%#v)", got, got)
	}
	if !strings.Contains(e.Error(), "WRONGTYPE") {
		t.Errorf("expected WRONGTYPE error, got %q", e.Error())
	}
}

func TestIntegrationPush(t *testing.T) {
	sub, subReader := dial(t)
	pub, pubReader := dial(t)
	channel := fmt.Sprintf("resp3-integration-channel:%d", time.Now().UnixNano())

	got := sendCmd(t, sub, subReader, "SUBSCRIBE", channel)
	push, ok := got.(resp3.Push)
	if !ok || push.Kind != respcodec.SimpleString("subscribe") {
		t.Fatalf("expected subscribe confirmation push, got %T (%#v)", got, got)
	}

	got = sendCmd(t, pub, pubReader, "PUBLISH", channel, "hello")
	if got != 1 {
		t.Fatalf("expected 1 subscriber to receive the message, got %#v", got)
	}

	frame, err := readFrame(subReader)
	if err != nil {
		t.Fatalf("read push message: %v", err)
	}
	v, err := resp3.Decode(frame)
	if err != nil {
		t.Fatalf("decode push message (raw %q): %v", frame, err)
	}
	push, ok = v.(resp3.Push)
	if !ok {
		t.Fatalf("expected resp3.Push, got %T (%#v)", v, v)
	}
	if push.Kind != respcodec.SimpleString("message") {
		t.Errorf("expected push kind \"message\", got %q", push.Kind)
	}
	if len(push.Args) != 2 || push.Args[0] != channel || push.Args[1] != "hello" {
		t.Errorf("expected push args [%q hello], got %#v", channel, push.Args)
	}
}
