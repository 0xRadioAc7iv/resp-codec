package respcodec

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"testing"
)

func ExampleEncode() {
	buf, _ := Encode(SimpleString("OK"))
	fmt.Printf("%q\n", buf)
	buf, _ = Encode(errors.New("ERR unknown command"))
	fmt.Printf("%q\n", buf)
	buf, _ = Encode(42)
	fmt.Printf("%q\n", buf)
	buf, _ = Encode("hello")
	fmt.Printf("%q\n", buf)
	buf, _ = Encode([]any{"GET", "key"})
	fmt.Printf("%q\n", buf)
	buf, _ = Encode(Null)
	fmt.Printf("%q\n", buf)
	buf, _ = Encode(NullArr)
	fmt.Printf("%q\n", buf)
	buf, err := Encode(3.14) // unknown type → nil, error
	fmt.Printf("%v %v\n", buf, err)

	// Output:
	// "+OK\r\n"
	// "-ERR unknown command\r\n"
	// ":42\r\n"
	// "$5\r\nhello\r\n"
	// "*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n"
	// "$-1\r\n"
	// "*-1\r\n"
	// [] unsupported type float64: cannot encode to RESP
}

func BenchmarkEncode(b *testing.B) {
	cases := []struct {
		name  string
		input any
	}{
		{"simple string", SimpleString("OK")},
		{"bulk string", "hello world"},
		{"error", errors.New("ERR unknown command")},
		{"integer", 42},
		{"integer max", math.MaxInt},
		{"array", []any{"SET", "key", "value"}},
		{"null bulk string", Null},
		{"null array", NullArr},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			for b.Loop() {
				_, _ = Encode(c.input)
			}
		})
	}
}

func ExampleAppendEncode() {
	// Reuse a single buffer across multiple encodes — zero additional allocations
	// when capacity is sufficient.
	buf := make([]byte, 0, 128)

	buf, _ = AppendEncode(buf, SimpleString("OK"))
	buf, _ = AppendEncode(buf, errors.New("ERR unknown command"))
	buf, _ = AppendEncode(buf, 42)
	buf, _ = AppendEncode(buf, "hello")
	fmt.Printf("%q\n", buf)

	// Output:
	// "+OK\r\n-ERR unknown command\r\n:42\r\n$5\r\nhello\r\n"
}

func BenchmarkAppendEncode(b *testing.B) {
	cases := []struct {
		name  string
		input any
	}{
		{"simple string", SimpleString("OK")},
		{"bulk string", "hello world"},
		{"error", errors.New("ERR unknown command")},
		{"integer", 42},
		{"integer max", math.MaxInt},
		{"array", []any{"SET", "key", "value"}},
		{"null bulk string", Null},
		{"null array", NullArr},
	}

	// Pre-allocate a buffer large enough to avoid any reallocs during the benchmark.
	buf := make([]byte, 0, 256)

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			for b.Loop() {
				buf, _ = AppendEncode(buf[:0], c.input)
			}
		})
	}
}

func Example_decode() {
	ss, _ := DecodeSimpleString([]byte("+OK\r\n"))
	fmt.Printf("%q\n", ss)
	e, _ := DecodeErrorString([]byte("-ERR unknown command\r\n"))
	fmt.Printf("%q\n", e)
	n, _ := DecodeInteger([]byte(":42\r\n"))
	fmt.Printf("%d\n", n)
	s, _ := DecodeBulkString([]byte("$5\r\nhello\r\n"))
	fmt.Printf("%q\n", s)
	err := DecodeNullBulkString([]byte("$-1\r\n"))
	fmt.Printf("%v\n", err == nil)
	arr, _ := DecodeArray([]byte("*3\r\n:1\r\n:2\r\n:3\r\n"))
	fmt.Printf("%v\n", arr)
	err = DecodeNullArray([]byte("*-1\r\n"))
	fmt.Printf("%v\n", err == nil)

	// Output:
	// "OK"
	// "ERR unknown command"
	// 42
	// "hello"
	// true
	// [1 2 3]
	// true
}

func BenchmarkDecode(b *testing.B) {
	cases := []struct {
		name  string
		input []byte
		fn    func([]byte)
	}{
		{"simple string", []byte("+OK\r\n"), func(b []byte) { _, _ = DecodeSimpleString(b) }},
		{"error", []byte("-ERR unknown command\r\n"), func(b []byte) { _, _ = DecodeErrorString(b) }},
		{"integer", []byte(":42\r\n"), func(b []byte) { _, _ = DecodeInteger(b) }},
		{"bulk string", []byte("$5\r\nhello\r\n"), func(b []byte) { _, _ = DecodeBulkString(b) }},
		{"null bulk string", []byte("$-1\r\n"), func(b []byte) { _ = DecodeNullBulkString(b) }},
		{"null array", []byte("*-1\r\n"), func(b []byte) { _ = DecodeNullArray(b) }},
		{"array integers", []byte("*3\r\n:1\r\n:2\r\n:3\r\n"), func(b []byte) { _, _ = DecodeArray(b) }},
		{"array bulk strings", []byte("*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"), func(b []byte) { _, _ = DecodeArray(b) }},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			for b.Loop() {
				c.fn(c.input)
			}
		})
	}
}

func TestAppendEncode(t *testing.T) {
	t.Run("appends to existing content", func(t *testing.T) {
		buf := []byte("+OK\r\n")
		buf, err := AppendEncode(buf, 42)
		assertNoError(t, err)
		assertCorrectMessage(t, buf, []byte("+OK\r\n:42\r\n"))
	})

	t.Run("nil buf behaves like empty buf", func(t *testing.T) {
		buf, err := AppendEncode(nil, SimpleString("OK"))
		assertNoError(t, err)
		assertCorrectMessage(t, buf, []byte("+OK\r\n"))
	})

	t.Run("reused buf produces correct output", func(t *testing.T) {
		buf := make([]byte, 0, 64)
		buf, err := AppendEncode(buf, "hello")
		assertNoError(t, err)
		assertCorrectMessage(t, buf, []byte("$5\r\nhello\r\n"))

		buf, err = AppendEncode(buf[:0], 42)
		assertNoError(t, err)
		assertCorrectMessage(t, buf, []byte(":42\r\n"))
	})

	t.Run("unsupported type returns error", func(t *testing.T) {
		_, err := AppendEncode(nil, 3.14)
		if err == nil {
			t.Error("expected error for unsupported type, got nil")
		}
	})

	t.Run("top-level unsupported type preserves existing content", func(t *testing.T) {
		existing := []byte("+OK\r\n")
		buf := make([]byte, len(existing), 128)
		copy(buf, existing)
		result, err := AppendEncode(buf, 3.14)
		if err == nil {
			t.Fatal("expected error for unsupported type, got nil")
		}
		assertCorrectMessage(t, result, existing)
	})

	t.Run("failed array encoding rolls back partial bytes", func(t *testing.T) {
		existing := []byte("+OK\r\n")
		buf := make([]byte, len(existing), 128)
		copy(buf, existing)

		result, err := AppendEncode(buf, []any{"good", 3.14})
		if err == nil {
			t.Fatal("expected error for unsupported element type, got nil")
		}
		// buf should be restored to its pre-call state
		assertCorrectMessage(t, result, existing)
	})
}

func TestEncode(t *testing.T) {

	t.Run("simple string - OK", func(t *testing.T) {
		encoded, err := Encode(SimpleString("OK"))
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("+OK\r\n"))
	})

	t.Run("simple string - PONG", func(t *testing.T) {
		encoded, err := Encode(SimpleString("PONG"))
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("+PONG\r\n"))
	})

	t.Run("simple string - empty", func(t *testing.T) {
		encoded, err := Encode(SimpleString(""))
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("+\r\n"))
	})

	t.Run("simple string - multi word", func(t *testing.T) {
		encoded, err := Encode(SimpleString("hello world"))
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("+hello world\r\n"))
	})

	t.Run("simple string - contains CR returns nil and error", func(t *testing.T) {
		encoded, err := Encode(SimpleString("OK\rHACKED"))
		if encoded != nil {
			t.Errorf("expected nil bytes, got %q", encoded)
		}
		if err == nil {
			t.Error("expected error for CR in simple string, got nil")
		}
	})

	t.Run("simple string - contains LF returns nil and error", func(t *testing.T) {
		encoded, err := Encode(SimpleString("OK\nHACKED"))
		if encoded != nil {
			t.Errorf("expected nil bytes, got %q", encoded)
		}
		if err == nil {
			t.Error("expected error for LF in simple string, got nil")
		}
	})

	t.Run("simple string - contains CRLF returns nil and error", func(t *testing.T) {
		encoded, err := Encode(SimpleString("OK\r\nHACKED"))
		if encoded != nil {
			t.Errorf("expected nil bytes, got %q", encoded)
		}
		if err == nil {
			t.Error("expected error for CRLF in simple string, got nil")
		}
	})

	t.Run("error - basic message", func(t *testing.T) {
		encoded, err := Encode(errors.New("key not found"))
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("-key not found\r\n"))
	})

	t.Run("error - empty", func(t *testing.T) {
		encoded, err := Encode(errors.New(""))
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("-\r\n"))
	})

	t.Run("error - ERR prefix convention", func(t *testing.T) {
		encoded, err := Encode(errors.New("ERR unknown command"))
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("-ERR unknown command\r\n"))
	})

	t.Run("error - WRONGTYPE prefix convention", func(t *testing.T) {
		encoded, err := Encode(errors.New("WRONGTYPE Operation against a key holding the wrong kind of value"))
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"))
	})

	t.Run("error - prefix only, no message", func(t *testing.T) {
		encoded, err := Encode(errors.New("ERR"))
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("-ERR\r\n"))
	})

	t.Run("error - contains CR returns nil and error", func(t *testing.T) {
		encoded, err := Encode(errors.New("ERR bad\rinput"))
		if encoded != nil {
			t.Errorf("expected nil bytes, got %q", encoded)
		}
		if err == nil {
			t.Error("expected error for CR in error message, got nil")
		}
	})

	t.Run("error - contains LF returns nil and error", func(t *testing.T) {
		encoded, err := Encode(errors.New("ERR bad\ninput"))
		if encoded != nil {
			t.Errorf("expected nil bytes, got %q", encoded)
		}
		if err == nil {
			t.Error("expected error for LF in error message, got nil")
		}
	})

	t.Run("error - contains CRLF returns nil and error", func(t *testing.T) {
		encoded, err := Encode(errors.New("ERR bad\r\ninput"))
		if encoded != nil {
			t.Errorf("expected nil bytes, got %q", encoded)
		}
		if err == nil {
			t.Error("expected error for CRLF in error message, got nil")
		}
	})

	t.Run("integer - positive", func(t *testing.T) {
		encoded, err := Encode(42)
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte(":42\r\n"))
	})

	t.Run("integer - zero", func(t *testing.T) {
		encoded, err := Encode(0)
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte(":0\r\n"))
	})

	t.Run("integer - negative", func(t *testing.T) {
		encoded, err := Encode(-1)
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte(":-1\r\n"))
	})

	t.Run("integer - large value", func(t *testing.T) {
		encoded, err := Encode(1000000)
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte(":1000000\r\n"))
	})

	t.Run("integer - max int", func(t *testing.T) {
		encoded, err := Encode(math.MaxInt)
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, fmt.Appendf(nil, ":%d\r\n", math.MaxInt))
	})

	t.Run("integer - min int", func(t *testing.T) {
		encoded, err := Encode(math.MinInt)
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, fmt.Appendf(nil, ":%d\r\n", math.MinInt))
	})

	t.Run("bulk string - normal", func(t *testing.T) {
		encoded, err := Encode("hello")
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("$5\r\nhello\r\n"))
	})

	t.Run("bulk string - with spaces", func(t *testing.T) {
		encoded, err := Encode("hello world")
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("$11\r\nhello world\r\n"))
	})

	t.Run("bulk string - binary safe with CRLF", func(t *testing.T) {
		encoded, err := Encode("hello\r\nworld")
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("$12\r\nhello\r\nworld\r\n"))
	})

	t.Run("bulk string - zero length", func(t *testing.T) {
		encoded, err := Encode("")
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("$0\r\n\r\n"))
	})

	t.Run("bulk string - null", func(t *testing.T) {
		encoded, err := Encode(Null)
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("$-1\r\n"))
	})

	t.Run("array - empty", func(t *testing.T) {
		encoded, err := Encode([]any{})
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("*0\r\n"))
	})

	t.Run("array - nil slice", func(t *testing.T) {
		encoded, err := Encode([]any(nil))
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("*0\r\n"))
	})

	t.Run("array - single string element", func(t *testing.T) {
		encoded, err := Encode([]any{"hello"})
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("*1\r\n$5\r\nhello\r\n"))
	})

	t.Run("array - typical command", func(t *testing.T) {
		encoded, err := Encode([]any{"GET", "key"})
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n"))
	})

	t.Run("array - mixed types", func(t *testing.T) {
		encoded, err := Encode([]any{"SET", "key", 1})
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n:1\r\n"))
	})

	t.Run("array - nested", func(t *testing.T) {
		encoded, err := Encode([]any{[]any{"foo"}})
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("*1\r\n*1\r\n$3\r\nfoo\r\n"))
	})

	t.Run("array - null", func(t *testing.T) {
		encoded, err := Encode(NullArr)
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("*-1\r\n"))
	})

	t.Run("array - integers only", func(t *testing.T) {
		encoded, err := Encode([]any{1, 2, 3})
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("*3\r\n:1\r\n:2\r\n:3\r\n"))
	})

	t.Run("array - null element", func(t *testing.T) {
		encoded, err := Encode([]any{"foo", Null, "bar"})
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("*3\r\n$3\r\nfoo\r\n$-1\r\n$3\r\nbar\r\n"))
	})

	t.Run("array - nested with simple string and error", func(t *testing.T) {
		encoded, err := Encode([]any{
			[]any{1, 2, 3},
			[]any{SimpleString("Foo"), errors.New("Bar")},
		})
		assertNoError(t, err)
		assertCorrectMessage(t, encoded, []byte("*2\r\n*3\r\n:1\r\n:2\r\n:3\r\n*2\r\n+Foo\r\n-Bar\r\n"))
	})

	t.Run("array - unsupported element returns nil and error", func(t *testing.T) {
		encoded, err := Encode([]any{"ok", 3.14})
		if encoded != nil {
			t.Errorf("expected nil bytes, got %q", encoded)
		}
		if err == nil {
			t.Error("expected error for unsupported element type, got nil")
		}
	})

	t.Run("unknown type - returns nil and error", func(t *testing.T) {
		encoded, err := Encode(3.14)
		if encoded != nil {
			t.Errorf("expected nil bytes, got %q", encoded)
		}
		if err == nil {
			t.Error("expected error for unsupported type, got nil")
		}
	})
}

func TestDecode(t *testing.T) {
	t.Run("simple string - ok", func(t *testing.T) {
		got, err := DecodeSimpleString([]byte("+OK\r\n"))
		assertNoError(t, err)
		if got != SimpleString("OK") {
			t.Errorf("expected %q, got %q", SimpleString("OK"), got)
		}
	})

	t.Run("simple string - empty", func(t *testing.T) {
		got, err := DecodeSimpleString([]byte("+\r\n"))
		assertNoError(t, err)
		if got != SimpleString("") {
			t.Errorf("expected empty SimpleString, got %q", got)
		}
	})

	t.Run("simple string - multi word", func(t *testing.T) {
		got, err := DecodeSimpleString([]byte("+hello world\r\n"))
		assertNoError(t, err)
		if got != SimpleString("hello world") {
			t.Errorf("expected %q, got %q", SimpleString("hello world"), got)
		}
	})

	t.Run("simple string - wrong prefix returns error", func(t *testing.T) {
		_, err := DecodeSimpleString([]byte("-ERR\r\n"))
		if err == nil {
			t.Error("expected error for wrong prefix, got nil")
		}
	})

	t.Run("simple string - empty buffer returns error", func(t *testing.T) {
		_, err := DecodeSimpleString([]byte{})
		if err == nil {
			t.Error("expected error for empty buffer, got nil")
		}
	})

	t.Run("error - basic message", func(t *testing.T) {
		got, err := DecodeErrorString([]byte("-ERR unknown command\r\n"))
		assertNoError(t, err)
		if got != "ERR unknown command" {
			t.Errorf("expected %q, got %q", "ERR unknown command", got)
		}
	})

	t.Run("error - empty message", func(t *testing.T) {
		got, err := DecodeErrorString([]byte("-\r\n"))
		assertNoError(t, err)
		if got != "" {
			t.Errorf("expected empty error message, got %q", got)
		}
	})

	t.Run("error - WRONGTYPE prefix", func(t *testing.T) {
		got, err := DecodeErrorString([]byte("-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"))
		assertNoError(t, err)
		if got != "WRONGTYPE Operation against a key holding the wrong kind of value" {
			t.Errorf("unexpected error message: %q", got)
		}
	})

	t.Run("error - wrong prefix returns error", func(t *testing.T) {
		_, err := DecodeErrorString([]byte("+OK\r\n"))
		if err == nil {
			t.Error("expected error for wrong prefix, got nil")
		}
	})

	t.Run("error - empty buffer returns error", func(t *testing.T) {
		_, err := DecodeErrorString([]byte{})
		if err == nil {
			t.Error("expected error for empty buffer, got nil")
		}
	})

	t.Run("integer - positive", func(t *testing.T) {
		got, err := DecodeInteger([]byte(":42\r\n"))
		assertNoError(t, err)
		if got != 42 {
			t.Errorf("expected 42, got %d", got)
		}
	})

	t.Run("integer - zero", func(t *testing.T) {
		got, err := DecodeInteger([]byte(":0\r\n"))
		assertNoError(t, err)
		if got != 0 {
			t.Errorf("expected 0, got %d", got)
		}
	})

	t.Run("integer - negative", func(t *testing.T) {
		got, err := DecodeInteger([]byte(":-1\r\n"))
		assertNoError(t, err)
		if got != -1 {
			t.Errorf("expected -1, got %d", got)
		}
	})

	t.Run("integer - large value", func(t *testing.T) {
		got, err := DecodeInteger([]byte(":1000000\r\n"))
		assertNoError(t, err)
		if got != 1000000 {
			t.Errorf("expected 1000000, got %d", got)
		}
	})

	t.Run("integer - max int", func(t *testing.T) {
		buf := fmt.Appendf(nil, ":%d\r\n", math.MaxInt)
		got, err := DecodeInteger(buf)
		assertNoError(t, err)
		if got != math.MaxInt {
			t.Errorf("expected %d, got %d", math.MaxInt, got)
		}
	})

	t.Run("integer - min int", func(t *testing.T) {
		buf := fmt.Appendf(nil, ":%d\r\n", math.MinInt)
		got, err := DecodeInteger(buf)
		assertNoError(t, err)
		if got != math.MinInt {
			t.Errorf("expected %d, got %d", math.MinInt, got)
		}
	})

	t.Run("integer - invalid character returns error", func(t *testing.T) {
		_, err := DecodeInteger([]byte(":abc\r\n"))
		if err == nil {
			t.Error("expected error for non-digit character, got nil")
		}
	})

	t.Run("integer - wrong prefix returns error", func(t *testing.T) {
		_, err := DecodeInteger([]byte("+OK\r\n"))
		if err == nil {
			t.Error("expected error for wrong prefix, got nil")
		}
	})

	t.Run("integer - empty buffer returns error", func(t *testing.T) {
		_, err := DecodeInteger([]byte{})
		if err == nil {
			t.Error("expected error for empty buffer, got nil")
		}
	})

	t.Run("bulk string - normal", func(t *testing.T) {
		got, err := DecodeBulkString([]byte("$6\r\nfoobar\r\n"))
		assertNoError(t, err)
		if got != "foobar" {
			t.Errorf("expected %q, got %q", "foobar", got)
		}
	})

	t.Run("bulk string - empty", func(t *testing.T) {
		got, err := DecodeBulkString([]byte("$0\r\n\r\n"))
		assertNoError(t, err)
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("bulk string - with spaces", func(t *testing.T) {
		got, err := DecodeBulkString([]byte("$11\r\nhello world\r\n"))
		assertNoError(t, err)
		if got != "hello world" {
			t.Errorf("expected %q, got %q", "hello world", got)
		}
	})

	t.Run("bulk string - binary safe with CRLF in content", func(t *testing.T) {
		got, err := DecodeBulkString([]byte("$12\r\nhello\r\nworld\r\n"))
		assertNoError(t, err)
		if got != "hello\r\nworld" {
			t.Errorf("expected %q, got %q", "hello\r\nworld", got)
		}
	})

	t.Run("bulk string - length mismatch returns error", func(t *testing.T) {
		_, err := DecodeBulkString([]byte("$3\r\nhello\r\n"))
		if err == nil {
			t.Error("expected error for length mismatch, got nil")
		}
	})

	t.Run("bulk string - wrong prefix returns error", func(t *testing.T) {
		_, err := DecodeBulkString([]byte("+OK\r\n"))
		if err == nil {
			t.Error("expected error for wrong prefix, got nil")
		}
	})

	t.Run("bulk string - empty buffer returns error", func(t *testing.T) {
		_, err := DecodeBulkString([]byte{})
		if err == nil {
			t.Error("expected error for empty buffer, got nil")
		}
	})

	t.Run("null bulk string - decodes correctly", func(t *testing.T) {
		err := DecodeNullBulkString([]byte("$-1\r\n"))
		assertNoError(t, err)
	})

	t.Run("null bulk string - wrong prefix returns error", func(t *testing.T) {
		err := DecodeNullBulkString([]byte("+OK\r\n"))
		if err == nil {
			t.Error("expected error for wrong prefix, got nil")
		}
	})

	t.Run("null bulk string - non-null payload returns error", func(t *testing.T) {
		err := DecodeNullBulkString([]byte("$6\r\nfoobar\r\n"))
		if err == nil {
			t.Error("expected error for non-null bulk string, got nil")
		}
	})

	t.Run("null array - decodes correctly", func(t *testing.T) {
		err := DecodeNullArray([]byte("*-1\r\n"))
		assertNoError(t, err)
	})

	t.Run("null array - wrong prefix returns error", func(t *testing.T) {
		err := DecodeNullArray([]byte("+OK\r\n"))
		if err == nil {
			t.Error("expected error for wrong prefix, got nil")
		}
	})

	t.Run("null array - non-null payload returns error", func(t *testing.T) {
		err := DecodeNullArray([]byte("*2\r\n+OK\r\n+OK\r\n"))
		if err == nil {
			t.Error("expected error for non-null array, got nil")
		}
	})

	// *0\r\n
	t.Run("array - empty", func(t *testing.T) {
		got, err := DecodeArray([]byte("*0\r\n"))
		assertNoError(t, err)
		if len(got) != 0 {
			t.Errorf("expected empty slice, got %v", got)
		}
	})

	// *3\r\n:1\r\n:2\r\n:3\r\n
	t.Run("array - integers", func(t *testing.T) {
		got, err := DecodeArray([]byte("*3\r\n:1\r\n:2\r\n:3\r\n"))
		assertNoError(t, err)
		if len(got) != 3 {
			t.Fatalf("expected 3 elements, got %d", len(got))
		}
		if got[0] != 1 || got[1] != 2 || got[2] != 3 {
			t.Errorf("expected [1 2 3], got %v", got)
		}
	})

	// *2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n
	t.Run("array - bulk strings", func(t *testing.T) {
		got, err := DecodeArray([]byte("*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"))
		assertNoError(t, err)
		if len(got) != 2 {
			t.Fatalf("expected 2 elements, got %d", len(got))
		}
		if got[0] != "foo" || got[1] != "bar" {
			t.Errorf("expected [foo bar], got %v", got)
		}
	})

	// *5\r\n:1\r\n:2\r\n:3\r\n:4\r\n$6\r\nfoobar\r\n
	t.Run("array - mixed types", func(t *testing.T) {
		got, err := DecodeArray([]byte("*5\r\n:1\r\n:2\r\n:3\r\n:4\r\n$6\r\nfoobar\r\n"))
		assertNoError(t, err)
		if len(got) != 5 {
			t.Fatalf("expected 5 elements, got %d", len(got))
		}
		if got[0] != 1 || got[1] != 2 || got[2] != 3 || got[3] != 4 {
			t.Errorf("expected integers [1 2 3 4], got %v", got[:4])
		}
		if got[4] != "foobar" {
			t.Errorf("expected bulk string \"foobar\", got %v", got[4])
		}
	})

	// *3\r\n+OK\r\n-ERR\r\n:42\r\n
	t.Run("array - simple string, error, and int", func(t *testing.T) {
		got, err := DecodeArray([]byte("*3\r\n+OK\r\n-ERR\r\n:42\r\n"))
		assertNoError(t, err)
		if len(got) != 3 {
			t.Fatalf("expected 3 elements, got %d", len(got))
		}
		if got[0] != SimpleString("OK") {
			t.Errorf("element 0: expected SimpleString(\"OK\"), got %v", got[0])
		}
		if got[1].(error).Error() != "ERR" {
			t.Errorf("element 1: expected error \"ERR\", got %v", got[1])
		}
		if got[2] != 42 {
			t.Errorf("element 2: expected 42, got %v", got[2])
		}
	})

	// *2\r\n*3\r\n:1\r\n:2\r\n:3\r\n*2\r\n+Foo\r\n-Bar\r\n
	t.Run("array - nested arrays", func(t *testing.T) {
		got, err := DecodeArray([]byte("*2\r\n*3\r\n:1\r\n:2\r\n:3\r\n*2\r\n+Foo\r\n-Bar\r\n"))
		assertNoError(t, err)
		if len(got) != 2 {
			t.Fatalf("expected 2 elements, got %d", len(got))
		}
		inner1, ok := got[0].([]any)
		if !ok {
			t.Fatalf("element 0: expected []any, got %T", got[0])
		}
		if len(inner1) != 3 || inner1[0] != 1 || inner1[1] != 2 || inner1[2] != 3 {
			t.Errorf("inner array 0: expected [1 2 3], got %v", inner1)
		}
		inner2, ok := got[1].([]any)
		if !ok {
			t.Fatalf("element 1: expected []any, got %T", got[1])
		}
		if len(inner2) != 2 || inner2[0] != SimpleString("Foo") || inner2[1].(error).Error() != "Bar" {
			t.Errorf("inner array 1: expected [Foo Bar], got %v", inner2)
		}
	})

	// *2\r\n*-1\r\n$3\r\nfoo\r\n — null array element decodes as nil
	t.Run("array - null array element", func(t *testing.T) {
		got, err := DecodeArray([]byte("*2\r\n*-1\r\n$3\r\nfoo\r\n"))
		assertNoError(t, err)
		if len(got) != 2 {
			t.Fatalf("expected 2 elements, got %d", len(got))
		}
		if got[0] != nil {
			t.Errorf("element 0: expected nil for null array, got %v", got[0])
		}
		if got[1] != "foo" {
			t.Errorf("element 1: expected \"foo\", got %v", got[1])
		}
	})

	// *3\r\n$3\r\nfoo\r\n$-1\r\n$3\r\nbar\r\n — null element in array decodes as nil
	t.Run("array - null element", func(t *testing.T) {
		got, err := DecodeArray([]byte("*3\r\n$3\r\nfoo\r\n$-1\r\n$3\r\nbar\r\n"))
		assertNoError(t, err)
		if len(got) != 3 {
			t.Fatalf("expected 3 elements, got %d", len(got))
		}
		if got[0] != "foo" {
			t.Errorf("element 0: expected \"foo\", got %v", got[0])
		}
		if got[1] != nil {
			t.Errorf("element 1: expected nil for null bulk string, got %v", got[1])
		}
		if got[2] != "bar" {
			t.Errorf("element 2: expected \"bar\", got %v", got[2])
		}
	})

	t.Run("array - wrong prefix returns error", func(t *testing.T) {
		_, err := DecodeArray([]byte("+OK\r\n"))
		if err == nil {
			t.Error("expected error for wrong prefix, got nil")
		}
	})

	t.Run("array - empty buffer returns error", func(t *testing.T) {
		_, err := DecodeArray([]byte{})
		if err == nil {
			t.Error("expected error for empty buffer, got nil")
		}
	})
}

func TestDecodeMalformedFrames(t *testing.T) {
	t.Run("simple string rejects malformed terminator", func(t *testing.T) {
		cases := map[string][]byte{
			"missing terminator": []byte("+OK"),
			"bare CR":            []byte("+OK\rjunk"),
			"trailing bytes":     []byte("+OK\r\njunk"),
			"embedded LF":        []byte("+O\nK\r\n"),
			"embedded CR":        []byte("+O\rK\r\n"),
		}

		for name, input := range cases {
			t.Run(name, func(t *testing.T) {
				_, err := DecodeSimpleString(input)
				if err == nil {
					t.Fatalf("expected error for malformed simple string %q", input)
				}
			})
		}
	})

	t.Run("error rejects malformed terminator", func(t *testing.T) {
		cases := map[string][]byte{
			"missing terminator": []byte("-ERR"),
			"bare CR":            []byte("-ERR\rjunk"),
			"trailing bytes":     []byte("-ERR\r\njunk"),
			"embedded LF":        []byte("-E\nRR\r\n"),
			"embedded CR":        []byte("-E\rRR\r\n"),
		}

		for name, input := range cases {
			t.Run(name, func(t *testing.T) {
				_, err := DecodeErrorString(input)
				if err == nil {
					t.Fatalf("expected error for malformed error %q", input)
				}
			})
		}
	})

	t.Run("integer rejects malformed payload", func(t *testing.T) {
		cases := map[string][]byte{
			"no digits":          []byte(":\r\n"),
			"negative no digits": []byte(":-\r\n"),
			"missing terminator": []byte(":42"),
			"bare CR":            []byte(":42\rjunk"),
			"trailing bytes":     []byte(":42\r\njunk"),
			"embedded CR":        []byte(":4\r2\r\n"),
		}

		for name, input := range cases {
			t.Run(name, func(t *testing.T) {
				_, err := DecodeInteger(input)
				if err == nil {
					t.Fatalf("expected error for malformed integer %q", input)
				}
			})
		}
	})

	t.Run("bulk string rejects malformed frames", func(t *testing.T) {
		cases := map[string][]byte{
			"missing terminator":        []byte("$6\r\nfoobar"),
			"length mismatch low":       []byte("$3\r\nfoobar\r\n"),
			"length mismatch high":      []byte("$9\r\nfoobar\r\n"),
			"non-numeric length":        []byte("$abc\r\nfoobar\r\n"),
			"wrong prefix":              []byte("+foobar\r\n"),
			"empty buf":                 {},
			"empty string missing CRLF": []byte("$0\r\nXX\r\n"),
		}

		for name, input := range cases {
			t.Run(name, func(t *testing.T) {
				_, err := DecodeBulkString(input)
				if err == nil {
					t.Fatalf("expected error for malformed bulk string %q", input)
				}
			})
		}
	})

	t.Run("null bulk string rejects malformed frames", func(t *testing.T) {
		cases := map[string][]byte{
			"missing terminator": []byte("$-1"),
			"wrong count":        []byte("$-2\r\n"),
			"positive count":     []byte("$5\r\nhello\r\n"),
			"wrong prefix":       []byte("*-1\r\n"),
			"empty buf":          {},
		}

		for name, input := range cases {
			t.Run(name, func(t *testing.T) {
				err := DecodeNullBulkString(input)
				if err == nil {
					t.Fatalf("expected error for malformed null bulk string %q", input)
				}
			})
		}
	})

	t.Run("null array rejects malformed frames", func(t *testing.T) {
		cases := map[string][]byte{
			"missing terminator": []byte("*-1"),
			"wrong count":        []byte("*-2\r\n"),
			"empty array":        []byte("*0\r\n"),
			"wrong prefix":       []byte("$-1\r\n"),
			"empty buf":          {},
		}

		for name, input := range cases {
			t.Run(name, func(t *testing.T) {
				err := DecodeNullArray(input)
				if err == nil {
					t.Fatalf("expected error for malformed null array %q", input)
				}
			})
		}
	})

	t.Run("array rejects malformed frames", func(t *testing.T) {
		cases := map[string][]byte{
			"wrong prefix":                    []byte("+OK\r\n"),
			"empty buf":                       {},
			"missing header terminator":       []byte("*3"),
			"non-numeric count":               []byte("*abc\r\n"),
			"fewer elements than count":       []byte("*3\r\n:1\r\n:2\r\n"),
			"nested array non-numeric count":  []byte("*1\r\n*X\r\n"),
		}

		for name, input := range cases {
			t.Run(name, func(t *testing.T) {
				_, err := DecodeArray(input)
				if err == nil {
					t.Fatalf("expected error for malformed array %q", input)
				}
			})
		}
	})

	t.Run("array rejects malformed elements", func(t *testing.T) {
		cases := map[string][]byte{
			// embedded CR in simple string payload
			"malformed simple string": []byte("*1\r\n+O\rK\r\n"),
			// embedded CR in error payload
			"malformed error": []byte("*1\r\n-E\rRR\r\n"),
			// non-digit character in integer payload
			"malformed integer": []byte("*1\r\n:abc\r\n"),
			// null bulk string with wrong count (-2 instead of -1)
			"malformed null bulk string": []byte("*1\r\n$-2\r\n"),
			// bulk string declares length 3 but payload is 6 bytes
			"malformed bulk string": []byte("*1\r\n$3\r\nfoobar\r\n"),
			// null array with wrong count (-2 instead of -1)
			"malformed null array": []byte("*1\r\n*-2\r\n"),
			// nested array declares 2 elements but only contains 1
			"malformed nested array": []byte("*1\r\n*2\r\n:1\r\n"),
		}

		for name, input := range cases {
			t.Run(name, func(t *testing.T) {
				_, err := DecodeArray(input)
				if err == nil {
					t.Fatalf("expected error for malformed element in array %q", input)
				}
			})
		}
	})
}

func assertNoError(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertCorrectMessage(t testing.TB, encoded, expected []byte) {
	t.Helper()
	if !bytes.Equal(encoded, expected) {
		t.Errorf("expected %q but got %q", expected, encoded)
	}
}
