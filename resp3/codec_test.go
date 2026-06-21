package resp3

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"testing"

	respcodec "github.com/0xRadioAc7iv/resp-codec/v2"
)

func ExampleEncode() {
	buf, _ := Encode(respcodec.SimpleString("OK"))
	fmt.Printf("%q\n", buf)
	buf, _ = Encode(errors.New("ERR unknown command"))
	fmt.Printf("%q\n", buf)
	buf, _ = Encode(42)
	fmt.Printf("%q\n", buf)
	buf, _ = Encode("hello")
	fmt.Printf("%q\n", buf)
	buf, _ = Encode(BlobError("ERR multi\r\nline error"))
	fmt.Printf("%q\n", buf)
	buf, _ = Encode(VerbatimString("txt:hello"))
	fmt.Printf("%q\n", buf)
	n, _ := new(big.Int).SetString("123456789012345678901234567890", 10)
	buf, _ = Encode(n)
	fmt.Printf("%q\n", buf)
	buf, _ = Encode(3.14)
	fmt.Printf("%q\n", buf)
	buf, _ = Encode(Inf)
	fmt.Printf("%q\n", buf)
	buf, _ = Encode(NegInf)
	fmt.Printf("%q\n", buf)
	buf, _ = Encode(NaN)
	fmt.Printf("%q\n", buf)
	buf, _ = Encode(Null)
	fmt.Printf("%q\n", buf)
	buf, _ = Encode(true)
	fmt.Printf("%q\n", buf)
	buf, _ = Encode(false)
	fmt.Printf("%q\n", buf)
	buf, _ = Encode([]any{42, "hello"})
	fmt.Printf("%q\n", buf)
	buf, _ = Encode(map[respcodec.SimpleString]any{respcodec.SimpleString("key"): 42})
	fmt.Printf("%q\n", buf)
	buf, _ = Encode(map[any]struct{}{42: {}})
	fmt.Printf("%q\n", buf)
	buf, _ = Encode(AttributeType{respcodec.SimpleString("ttl"): 100})
	fmt.Printf("%q\n", buf)
	buf, _ = Encode(Push{Kind: respcodec.SimpleString("message"), Args: []any{respcodec.SimpleString("ch"), "hello"}})
	fmt.Printf("%q\n", buf)
	buf, err := Encode(uint(42))
	fmt.Printf("%v %v\n", buf, err)

	// Output:
	// "+OK\r\n"
	// "-ERR unknown command\r\n"
	// ":42\r\n"
	// "$5\r\nhello\r\n"
	// "!21\r\nERR multi\r\nline error\r\n"
	// "=9\r\ntxt:hello\r\n"
	// "(123456789012345678901234567890\r\n"
	// ",3.14\r\n"
	// ",inf\r\n"
	// ",-inf\r\n"
	// ",nan\r\n"
	// "_\r\n"
	// "#t\r\n"
	// "#f\r\n"
	// "*2\r\n:42\r\n$5\r\nhello\r\n"
	// "%1\r\n+key\r\n:42\r\n"
	// "~1\r\n:42\r\n"
	// "|1\r\n+ttl\r\n:100\r\n"
	// ">3\r\n+message\r\n+ch\r\n$5\r\nhello\r\n"
	// [] unsupported type uint: cannot encode to RESP3
}

func ExampleAppendEncode() {
	buf := make([]byte, 0, 128)
	buf, _ = AppendEncode(buf, respcodec.SimpleString("OK"))
	buf, _ = AppendEncode(buf, errors.New("ERR unknown command"))
	buf, _ = AppendEncode(buf, 42)
	buf, _ = AppendEncode(buf, "hello")
	buf, _ = AppendEncode(buf, BlobError("ERR oops"))
	buf, _ = AppendEncode(buf, VerbatimString("txt:hello"))
	n, _ := new(big.Int).SetString("9999999999999999999", 10)
	buf, _ = AppendEncode(buf, n)
	buf, _ = AppendEncode(buf, 3.14)
	buf, _ = AppendEncode(buf, Inf)
	buf, _ = AppendEncode(buf, NegInf)
	buf, _ = AppendEncode(buf, NaN)
	buf, _ = AppendEncode(buf, Null)
	buf, _ = AppendEncode(buf, true)
	buf, _ = AppendEncode(buf, []any{1, "x"})
	fmt.Printf("%q\n", buf)

	// Output:
	// "+OK\r\n-ERR unknown command\r\n:42\r\n$5\r\nhello\r\n!8\r\nERR oops\r\n=9\r\ntxt:hello\r\n(9999999999999999999\r\n,3.14\r\n,inf\r\n,-inf\r\n,nan\r\n_\r\n#t\r\n*2\r\n:1\r\n$1\r\nx\r\n"
}

func TestEncode(t *testing.T) {
	t.Run("simple string - dispatches correctly", func(t *testing.T) {
		got, err := Encode(respcodec.SimpleString("OK"))
		assertNoError(t, err)
		assertBytes(t, got, []byte("+OK\r\n"))
	})

	t.Run("simple string - CRLF rejected", func(t *testing.T) {
		_, err := Encode(respcodec.SimpleString("bad\r\ninput"))
		if err == nil {
			t.Error("expected error for CRLF in simple string, got nil")
		}
	})

	t.Run("simple error - dispatches correctly", func(t *testing.T) {
		got, err := Encode(errors.New("ERR unknown command"))
		assertNoError(t, err)
		assertBytes(t, got, []byte("-ERR unknown command\r\n"))
	})

	t.Run("simple error - CRLF rejected", func(t *testing.T) {
		_, err := Encode(errors.New("bad\r\ninput"))
		if err == nil {
			t.Error("expected error for CRLF in simple error, got nil")
		}
	})

	t.Run("blob string - dispatches correctly", func(t *testing.T) {
		got, err := Encode("hello")
		assertNoError(t, err)
		assertBytes(t, got, []byte("$5\r\nhello\r\n"))
	})

	t.Run("integer - dispatches correctly", func(t *testing.T) {
		got, err := Encode(42)
		assertNoError(t, err)
		assertBytes(t, got, []byte(":42\r\n"))
	})

	t.Run("blob error - dispatches correctly", func(t *testing.T) {
		got, err := Encode(BlobError("ERR unknown command"))
		assertNoError(t, err)
		assertBytes(t, got, []byte("!19\r\nERR unknown command\r\n"))
	})

	t.Run("blob error - allows CRLF", func(t *testing.T) {
		got, err := Encode(BlobError("ERR multi\r\nline"))
		assertNoError(t, err)
		assertBytes(t, got, []byte("!15\r\nERR multi\r\nline\r\n"))
	})

	t.Run("verbatim string - dispatches correctly", func(t *testing.T) {
		got, err := Encode(VerbatimString("txt:hello"))
		assertNoError(t, err)
		assertBytes(t, got, []byte("=9\r\ntxt:hello\r\n"))
	})

	t.Run("verbatim string - binary safe", func(t *testing.T) {
		got, err := Encode(VerbatimString("txt:line1\r\nline2"))
		assertNoError(t, err)
		assertBytes(t, got, []byte("=16\r\ntxt:line1\r\nline2\r\n"))
	})

	t.Run("big number - positive", func(t *testing.T) {
		n, _ := new(big.Int).SetString("123456789012345678901234567890", 10)
		got, err := Encode(n)
		assertNoError(t, err)
		assertBytes(t, got, []byte("(123456789012345678901234567890\r\n"))
	})

	t.Run("big number - negative", func(t *testing.T) {
		n, _ := new(big.Int).SetString("-123456789012345678901234567890", 10)
		got, err := Encode(n)
		assertNoError(t, err)
		assertBytes(t, got, []byte("(-123456789012345678901234567890\r\n"))
	})

	t.Run("big number - zero", func(t *testing.T) {
		got, err := Encode(new(big.Int))
		assertNoError(t, err)
		assertBytes(t, got, []byte("(0\r\n"))
	})

	t.Run("double float64 - positive", func(t *testing.T) {
		got, err := Encode(3.14)
		assertNoError(t, err)
		assertBytes(t, got, []byte(",3.14\r\n"))
	})

	t.Run("double float64 - negative", func(t *testing.T) {
		got, err := Encode(-1.5)
		assertNoError(t, err)
		assertBytes(t, got, []byte(",-1.5\r\n"))
	})

	t.Run("double float64 - zero", func(t *testing.T) {
		got, err := Encode(float64(0))
		assertNoError(t, err)
		assertBytes(t, got, []byte(",0\r\n"))
	})

	t.Run("double float32 - dispatches correctly", func(t *testing.T) {
		got, err := Encode(float32(1.5))
		assertNoError(t, err)
		assertBytes(t, got, []byte(",1.5\r\n"))
	})

	t.Run("double inf - encodes correctly", func(t *testing.T) {
		got, err := Encode(Inf)
		assertNoError(t, err)
		assertBytes(t, got, []byte(",inf\r\n"))
	})

	t.Run("double neg inf - encodes correctly", func(t *testing.T) {
		got, err := Encode(NegInf)
		assertNoError(t, err)
		assertBytes(t, got, []byte(",-inf\r\n"))
	})

	t.Run("double nan - encodes correctly", func(t *testing.T) {
		got, err := Encode(NaN)
		assertNoError(t, err)
		assertBytes(t, got, []byte(",nan\r\n"))
	})

	t.Run("null - encodes correctly", func(t *testing.T) {
		got, err := Encode(Null)
		assertNoError(t, err)
		assertBytes(t, got, []byte("_\r\n"))
	})

	t.Run("boolean - true", func(t *testing.T) {
		got, err := Encode(true)
		assertNoError(t, err)
		assertBytes(t, got, []byte("#t\r\n"))
	})

	t.Run("boolean - false", func(t *testing.T) {
		got, err := Encode(false)
		assertNoError(t, err)
		assertBytes(t, got, []byte("#f\r\n"))
	})

	t.Run("array - empty", func(t *testing.T) {
		got, err := Encode([]any{})
		assertNoError(t, err)
		assertBytes(t, got, []byte("*0\r\n"))
	})

	t.Run("array - single element", func(t *testing.T) {
		got, err := Encode([]any{42})
		assertNoError(t, err)
		assertBytes(t, got, []byte("*1\r\n:42\r\n"))
	})

	t.Run("array - mixed types", func(t *testing.T) {
		got, err := Encode([]any{1, "hello", true})
		assertNoError(t, err)
		assertBytes(t, got, []byte("*3\r\n:1\r\n$5\r\nhello\r\n#t\r\n"))
	})

	t.Run("array - nested", func(t *testing.T) {
		got, err := Encode([]any{[]any{1, 2}})
		assertNoError(t, err)
		assertBytes(t, got, []byte("*1\r\n*2\r\n:1\r\n:2\r\n"))
	})

	t.Run("array - unsupported element rolls back", func(t *testing.T) {
		prefix := []byte("+OK\r\n")
		buf := make([]byte, len(prefix), 64)
		copy(buf, prefix)
		result, err := AppendEncode(buf, []any{42, uint(1)})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		assertBytes(t, result, prefix)
	})

	t.Run("map - empty", func(t *testing.T) {
		got, err := Encode(map[respcodec.SimpleString]any{})
		assertNoError(t, err)
		assertBytes(t, got, []byte("%0\r\n"))
	})

	t.Run("map - single key", func(t *testing.T) {
		got, err := Encode(map[respcodec.SimpleString]any{respcodec.SimpleString("key"): 42})
		assertNoError(t, err)
		assertBytes(t, got, []byte("%1\r\n+key\r\n:42\r\n"))
	})

	t.Run("map - multiple keys contain all pairs", func(t *testing.T) {
		got, err := Encode(map[respcodec.SimpleString]any{
			respcodec.SimpleString("a"): 1,
			respcodec.SimpleString("b"): 2,
		})
		assertNoError(t, err)
		if !bytes.HasPrefix(got, []byte("%2\r\n")) {
			t.Errorf("expected %%2\\r\\n header, got %q", got[:min(len(got), 8)])
		}
		if !bytes.Contains(got, []byte("+a\r\n:1\r\n")) {
			t.Errorf("missing pair a→1 in %q", got)
		}
		if !bytes.Contains(got, []byte("+b\r\n:2\r\n")) {
			t.Errorf("missing pair b→2 in %q", got)
		}
	})

	t.Run("map - unsupported value rolls back", func(t *testing.T) {
		prefix := []byte("+OK\r\n")
		buf := make([]byte, len(prefix), 64)
		copy(buf, prefix)
		result, err := AppendEncode(buf, map[respcodec.SimpleString]any{respcodec.SimpleString("k"): uint(1)})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		assertBytes(t, result, prefix)
	})

	t.Run("set - empty", func(t *testing.T) {
		got, err := Encode(map[any]struct{}{})
		assertNoError(t, err)
		assertBytes(t, got, []byte("~0\r\n"))
	})

	t.Run("set - single element", func(t *testing.T) {
		got, err := Encode(map[any]struct{}{42: {}})
		assertNoError(t, err)
		assertBytes(t, got, []byte("~1\r\n:42\r\n"))
	})

	t.Run("set - multiple elements contain all members", func(t *testing.T) {
		got, err := Encode(map[any]struct{}{1: {}, 2: {}})
		assertNoError(t, err)
		if !bytes.HasPrefix(got, []byte("~2\r\n")) {
			t.Errorf("expected ~2\\r\\n header, got %q", got[:min(len(got), 8)])
		}
		if !bytes.Contains(got, []byte(":1\r\n")) {
			t.Errorf("missing element 1 in %q", got)
		}
		if !bytes.Contains(got, []byte(":2\r\n")) {
			t.Errorf("missing element 2 in %q", got)
		}
	})

	t.Run("set - unsupported element rolls back", func(t *testing.T) {
		prefix := []byte("+OK\r\n")
		buf := make([]byte, len(prefix), 64)
		copy(buf, prefix)
		result, err := AppendEncode(buf, map[any]struct{}{uint(1): {}})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		assertBytes(t, result, prefix)
	})

	t.Run("attribute - empty", func(t *testing.T) {
		got, err := Encode(AttributeType{})
		assertNoError(t, err)
		assertBytes(t, got, []byte("|0\r\n"))
	})

	t.Run("attribute - single key", func(t *testing.T) {
		got, err := Encode(AttributeType{respcodec.SimpleString("ttl"): 100})
		assertNoError(t, err)
		assertBytes(t, got, []byte("|1\r\n+ttl\r\n:100\r\n"))
	})

	t.Run("attribute - multiple keys contain all pairs", func(t *testing.T) {
		got, err := Encode(AttributeType{
			respcodec.SimpleString("ttl"):  100,
			respcodec.SimpleString("hits"): 42,
		})
		assertNoError(t, err)
		if !bytes.HasPrefix(got, []byte("|2\r\n")) {
			t.Errorf("expected |2\\r\\n header, got %q", got[:min(len(got), 8)])
		}
		if !bytes.Contains(got, []byte("+ttl\r\n:100\r\n")) {
			t.Errorf("missing pair ttl→100 in %q", got)
		}
		if !bytes.Contains(got, []byte("+hits\r\n:42\r\n")) {
			t.Errorf("missing pair hits→42 in %q", got)
		}
	})

	t.Run("attribute - unsupported value rolls back", func(t *testing.T) {
		prefix := []byte("+OK\r\n")
		buf := make([]byte, len(prefix), 64)
		copy(buf, prefix)
		result, err := AppendEncode(buf, AttributeType{respcodec.SimpleString("k"): uint(1)})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		assertBytes(t, result, prefix)
	})

	t.Run("push - no args", func(t *testing.T) {
		got, err := Encode(Push{Kind: respcodec.SimpleString("subscribe")})
		assertNoError(t, err)
		assertBytes(t, got, []byte(">1\r\n+subscribe\r\n"))
	})

	t.Run("push - with args", func(t *testing.T) {
		got, err := Encode(Push{Kind: respcodec.SimpleString("message"), Args: []any{respcodec.SimpleString("ch"), "payload"}})
		assertNoError(t, err)
		assertBytes(t, got, []byte(">3\r\n+message\r\n+ch\r\n$7\r\npayload\r\n"))
	})

	t.Run("push - unsupported arg rolls back", func(t *testing.T) {
		prefix := []byte("+OK\r\n")
		buf := make([]byte, len(prefix), 64)
		copy(buf, prefix)
		result, err := AppendEncode(buf, Push{Kind: respcodec.SimpleString("message"), Args: []any{uint(1)}})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		assertBytes(t, result, prefix)
	})

	t.Run("push - invalid kind rolls back", func(t *testing.T) {
		prefix := []byte("+OK\r\n")
		buf := make([]byte, len(prefix), 64)
		copy(buf, prefix)
		result, err := AppendEncode(buf, Push{Kind: respcodec.SimpleString("bad\r\nkind")})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		assertBytes(t, result, prefix)
	})

	t.Run("map - invalid key rolls back", func(t *testing.T) {
		prefix := []byte("+OK\r\n")
		buf := make([]byte, len(prefix), 64)
		copy(buf, prefix)
		result, err := AppendEncode(buf, map[respcodec.SimpleString]any{respcodec.SimpleString("bad\r\nkey"): 1})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		assertBytes(t, result, prefix)
	})

	t.Run("unsupported type returns nil and error", func(t *testing.T) {
		got, err := Encode(uint(42))
		if got != nil {
			t.Errorf("expected nil bytes, got %q", got)
		}
		if err == nil {
			t.Error("expected error for unsupported type, got nil")
		}
	})
}

func TestAppendEncode(t *testing.T) {
	t.Run("appends to existing content", func(t *testing.T) {
		buf := []byte("+OK\r\n")
		buf, err := AppendEncode(buf, "world")
		assertNoError(t, err)
		assertBytes(t, buf, []byte("+OK\r\n$5\r\nworld\r\n"))
	})

	t.Run("nil buf behaves like empty buf", func(t *testing.T) {
		got, err := AppendEncode(nil, respcodec.SimpleString("OK"))
		assertNoError(t, err)
		assertBytes(t, got, []byte("+OK\r\n"))
	})

	t.Run("unsupported type returns error", func(t *testing.T) {
		_, err := AppendEncode(nil, uint(42))
		if err == nil {
			t.Error("expected error for unsupported type, got nil")
		}
	})
}

func BenchmarkEncode(b *testing.B) {
	cases := []struct {
		name  string
		input any
	}{
		{"verbatim string", VerbatimString("txt:hello world")},
		{"array 3 elements", []any{1, "hello", true}},
		{"map 1 key", map[respcodec.SimpleString]any{respcodec.SimpleString("key"): 42}},
		{"set 1 element", map[any]struct{}{42: {}}},
		{"attribute 1 key", AttributeType{respcodec.SimpleString("ttl"): 100}},
		{"big number", func() *big.Int { n, _ := new(big.Int).SetString("123456789012345678901234567890", 10); return n }()},
		{"null", Null},
		{"boolean true", true},
		{"boolean false", false},
		{"double float64", 3.14},
		{"double float32", float32(1.5)},
		{"double inf", Inf},
		{"double neg inf", NegInf},
		{"double nan", NaN},
		{"push 2 args", Push{Kind: respcodec.SimpleString("message"), Args: []any{respcodec.SimpleString("ch"), "hello"}}},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			for b.Loop() {
				_, _ = Encode(c.input)
			}
		})
	}
}

func ExampleDecode() {
	v, _ := Decode([]byte("+OK\r\n"))
	fmt.Println(v)
	v, _ = Decode([]byte("-ERR unknown\r\n"))
	fmt.Println(v)
	v, _ = Decode([]byte(":42\r\n"))
	fmt.Println(v)
	v, _ = Decode([]byte("$5\r\nhello\r\n"))
	fmt.Println(v)
	v, _ = Decode([]byte("!8\r\nERR oops\r\n"))
	fmt.Println(v)
	v, _ = Decode([]byte("=9\r\ntxt:hello\r\n"))
	fmt.Println(v)
	v, _ = Decode([]byte("(123456789012345678901234567890\r\n"))
	fmt.Println(v)
	v, _ = Decode([]byte(",3.14\r\n"))
	fmt.Println(v)
	v, _ = Decode([]byte("_\r\n"))
	fmt.Println(v == Null)
	v, _ = Decode([]byte("#t\r\n"))
	fmt.Println(v)
	v, _ = Decode([]byte("#f\r\n"))
	fmt.Println(v)
	_, err := Decode([]byte("?unknown\r\n"))
	fmt.Println(err)

	// Output:
	// OK
	// ERR unknown
	// 42
	// hello
	// ERR oops
	// txt:hello
	// 123456789012345678901234567890
	// 3.14
	// true
	// true
	// false
	// unknown RESP3 type sigil: '?'
}

func ExampleDecode_array() {
	v, _ := Decode([]byte("*2\r\n:1\r\n:2\r\n"))
	fmt.Println(v)

	// Output:
	// [1 2]
}

func ExampleDecode_map() {
	v, _ := Decode([]byte("%1\r\n+key\r\n:42\r\n"))
	fmt.Println(v)

	// Output:
	// map[key:42]
}

func ExampleDecode_set() {
	v, _ := Decode([]byte("~1\r\n:42\r\n"))
	fmt.Println(v)

	// Output:
	// map[42:{}]
}

func ExampleDecode_attribute() {
	v, _ := Decode([]byte("|1\r\n+ttl\r\n:100\r\n"))
	fmt.Println(v)

	// Output:
	// map[ttl:100]
}

func ExampleDecode_push() {
	v, _ := Decode([]byte(">3\r\n+message\r\n+ch\r\n$5\r\nhello\r\n"))
	fmt.Println(v)

	// Output:
	// {message [ch hello]}
}

func TestDecode(t *testing.T) {
	t.Run("simple string - dispatches correctly", func(t *testing.T) {
		got, err := Decode([]byte("+OK\r\n"))
		assertNoError(t, err)
		if got != respcodec.SimpleString("OK") {
			t.Errorf("expected SimpleString(\"OK\"), got %v (%T)", got, got)
		}
	})

	t.Run("simple string - CRLF rejected", func(t *testing.T) {
		_, err := Decode([]byte("+bad\r\ninput\r\n"))
		if err == nil {
			t.Error("expected error for CRLF in simple string, got nil")
		}
	})

	t.Run("simple error - dispatches correctly", func(t *testing.T) {
		got, err := Decode([]byte("-ERR unknown\r\n"))
		assertNoError(t, err)
		e, ok := got.(error)
		if !ok {
			t.Fatalf("expected error type, got %T", got)
		}
		if e.Error() != "ERR unknown" {
			t.Errorf("expected %q, got %q", "ERR unknown", e.Error())
		}
	})

	t.Run("simple error - CRLF rejected", func(t *testing.T) {
		_, err := Decode([]byte("-ERR\r\nbad\r\n"))
		if err == nil {
			t.Error("expected error for CRLF in simple error, got nil")
		}
	})

	t.Run("integer - dispatches correctly", func(t *testing.T) {
		got, err := Decode([]byte(":42\r\n"))
		assertNoError(t, err)
		if got != 42 {
			t.Errorf("expected 42, got %v", got)
		}
	})

	t.Run("blob string - dispatches correctly", func(t *testing.T) {
		got, err := Decode([]byte("$5\r\nhello\r\n"))
		assertNoError(t, err)
		if got != "hello" {
			t.Errorf("expected \"hello\", got %v", got)
		}
	})

	t.Run("blob string - missing CRLF after length returns error", func(t *testing.T) {
		_, err := Decode([]byte("$3"))
		if err == nil {
			t.Error("expected error for missing CRLF after length, got nil")
		}
	})

	t.Run("blob error - dispatches correctly", func(t *testing.T) {
		got, err := Decode([]byte("!19\r\nERR unknown command\r\n"))
		assertNoError(t, err)
		if got != BlobError("ERR unknown command") {
			t.Errorf("expected BlobError(\"ERR unknown command\"), got %v", got)
		}
	})

	t.Run("blob error - allows CRLF", func(t *testing.T) {
		got, err := Decode([]byte("!15\r\nERR multi\r\nline\r\n"))
		assertNoError(t, err)
		if got != BlobError("ERR multi\r\nline") {
			t.Errorf("expected BlobError with embedded CRLF, got %v", got)
		}
	})

	t.Run("blob error - invalid frame returns error", func(t *testing.T) {
		_, err := Decode([]byte("!5\r\nhi\r\n"))
		if err == nil {
			t.Error("expected error for blob error length mismatch, got nil")
		}
	})

	t.Run("verbatim string - dispatches correctly", func(t *testing.T) {
		got, err := Decode([]byte("=9\r\ntxt:hello\r\n"))
		assertNoError(t, err)
		if got != VerbatimString("txt:hello") {
			t.Errorf("expected VerbatimString(\"txt:hello\"), got %v", got)
		}
	})

	t.Run("verbatim string - invalid frame returns error", func(t *testing.T) {
		_, err := Decode([]byte("=5\r\nhi\r\n"))
		if err == nil {
			t.Error("expected error for verbatim string length mismatch, got nil")
		}
	})

	t.Run("verbatim string - binary safe", func(t *testing.T) {
		got, err := Decode([]byte("=16\r\ntxt:line1\r\nline2\r\n"))
		assertNoError(t, err)
		if got != VerbatimString("txt:line1\r\nline2") {
			t.Errorf("expected VerbatimString with embedded CRLF, got %v", got)
		}
	})

	t.Run("big number - positive", func(t *testing.T) {
		got, err := Decode([]byte("(123456789012345678901234567890\r\n"))
		assertNoError(t, err)
		n, ok := got.(*big.Int)
		if !ok {
			t.Fatalf("expected *big.Int, got %T", got)
		}
		want, _ := new(big.Int).SetString("123456789012345678901234567890", 10)
		if n.Cmp(want) != 0 {
			t.Errorf("expected %s, got %s", want, n)
		}
	})

	t.Run("big number - negative", func(t *testing.T) {
		got, err := Decode([]byte("(-123456789012345678901234567890\r\n"))
		assertNoError(t, err)
		n, ok := got.(*big.Int)
		if !ok {
			t.Fatalf("expected *big.Int, got %T", got)
		}
		want, _ := new(big.Int).SetString("-123456789012345678901234567890", 10)
		if n.Cmp(want) != 0 {
			t.Errorf("expected %s, got %s", want, n)
		}
	})

	t.Run("big number - zero", func(t *testing.T) {
		got, err := Decode([]byte("(0\r\n"))
		assertNoError(t, err)
		n, ok := got.(*big.Int)
		if !ok {
			t.Fatalf("expected *big.Int, got %T", got)
		}
		if n.Sign() != 0 {
			t.Errorf("expected 0, got %s", n)
		}
	})

	t.Run("big number - invalid value returns error", func(t *testing.T) {
		_, err := Decode([]byte("(12x34\r\n"))
		if err == nil {
			t.Error("expected error for invalid big number value, got nil")
		}
	})

	t.Run("big number - missing CRLF returns error", func(t *testing.T) {
		_, err := Decode([]byte("(123"))
		if err == nil {
			t.Error("expected error for missing CRLF, got nil")
		}
	})

	t.Run("big number - too short returns error", func(t *testing.T) {
		_, err := Decode([]byte("("))
		if err == nil {
			t.Error("expected error for too-short frame, got nil")
		}
	})

	t.Run("double - positive", func(t *testing.T) {
		got, err := Decode([]byte(",3.14\r\n"))
		assertNoError(t, err)
		f, ok := got.(float64)
		if !ok {
			t.Fatalf("expected float64, got %T", got)
		}
		if f != 3.14 {
			t.Errorf("expected 3.14, got %v", f)
		}
	})

	t.Run("double - negative", func(t *testing.T) {
		got, err := Decode([]byte(",-1.5\r\n"))
		assertNoError(t, err)
		f, ok := got.(float64)
		if !ok {
			t.Fatalf("expected float64, got %T", got)
		}
		if f != -1.5 {
			t.Errorf("expected -1.5, got %v", f)
		}
	})

	t.Run("double - zero", func(t *testing.T) {
		got, err := Decode([]byte(",0\r\n"))
		assertNoError(t, err)
		f, ok := got.(float64)
		if !ok {
			t.Fatalf("expected float64, got %T", got)
		}
		if f != 0 {
			t.Errorf("expected 0, got %v", f)
		}
	})

	t.Run("double - inf", func(t *testing.T) {
		got, err := Decode([]byte(",inf\r\n"))
		assertNoError(t, err)
		f, ok := got.(float64)
		if !ok {
			t.Fatalf("expected float64, got %T", got)
		}
		if !math.IsInf(f, 1) {
			t.Errorf("expected +Inf, got %v", f)
		}
	})

	t.Run("double - negative inf", func(t *testing.T) {
		got, err := Decode([]byte(",-inf\r\n"))
		assertNoError(t, err)
		f, ok := got.(float64)
		if !ok {
			t.Fatalf("expected float64, got %T", got)
		}
		if !math.IsInf(f, -1) {
			t.Errorf("expected -Inf, got %v", f)
		}
	})

	t.Run("double - nan", func(t *testing.T) {
		got, err := Decode([]byte(",nan\r\n"))
		assertNoError(t, err)
		f, ok := got.(float64)
		if !ok {
			t.Fatalf("expected float64, got %T", got)
		}
		if !math.IsNaN(f) {
			t.Errorf("expected NaN, got %v", f)
		}
	})

	t.Run("double - invalid value returns error", func(t *testing.T) {
		_, err := Decode([]byte(",12x34\r\n"))
		if err == nil {
			t.Error("expected error for invalid double value, got nil")
		}
	})

	t.Run("double - missing CRLF returns error", func(t *testing.T) {
		_, err := Decode([]byte(",3.14"))
		if err == nil {
			t.Error("expected error for missing CRLF, got nil")
		}
	})

	t.Run("double - too short returns error", func(t *testing.T) {
		_, err := Decode([]byte(","))
		if err == nil {
			t.Error("expected error for too-short frame, got nil")
		}
	})

	t.Run("null - decodes correctly", func(t *testing.T) {
		got, err := Decode([]byte("_\r\n"))
		assertNoError(t, err)
		if got != Null {
			t.Errorf("expected Null, got %v (%T)", got, got)
		}
	})

	t.Run("null - invalid frame returns error", func(t *testing.T) {
		_, err := Decode([]byte("_junk\r\n"))
		if err == nil {
			t.Error("expected error for invalid null frame, got nil")
		}
	})

	t.Run("null - missing CRLF returns error", func(t *testing.T) {
		_, err := Decode([]byte("_"))
		if err == nil {
			t.Error("expected error for missing CRLF, got nil")
		}
	})

	t.Run("boolean - true", func(t *testing.T) {
		got, err := Decode([]byte("#t\r\n"))
		assertNoError(t, err)
		if got != true {
			t.Errorf("expected true, got %v", got)
		}
	})

	t.Run("boolean - false", func(t *testing.T) {
		got, err := Decode([]byte("#f\r\n"))
		assertNoError(t, err)
		if got != false {
			t.Errorf("expected false, got %v", got)
		}
	})

	t.Run("boolean - invalid value returns error", func(t *testing.T) {
		_, err := Decode([]byte("#x\r\n"))
		if err == nil {
			t.Error("expected error for invalid boolean value, got nil")
		}
	})

	t.Run("boolean - invalid frame length returns error", func(t *testing.T) {
		_, err := Decode([]byte("#tt\r\n"))
		if err == nil {
			t.Error("expected error for invalid boolean frame length, got nil")
		}
	})

	t.Run("boolean - missing CRLF returns error", func(t *testing.T) {
		_, err := Decode([]byte("#"))
		if err == nil {
			t.Error("expected error for missing CRLF, got nil")
		}
	})

	t.Run("unknown sigil returns error", func(t *testing.T) {
		_, err := Decode([]byte("?unknown\r\n"))
		if err == nil {
			t.Error("expected error for unknown sigil, got nil")
		}
	})

	t.Run("empty buffer returns error", func(t *testing.T) {
		_, err := Decode([]byte{})
		if err == nil {
			t.Error("expected error for empty buffer, got nil")
		}
	})

	t.Run("array - empty", func(t *testing.T) {
		got, err := Decode([]byte("*0\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, []any{})
	})

	t.Run("array - single element", func(t *testing.T) {
		got, err := Decode([]byte("*1\r\n:42\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, []any{42})
	})

	t.Run("array - mixed types", func(t *testing.T) {
		got, err := Decode([]byte("*3\r\n:1\r\n$5\r\nhello\r\n#t\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, []any{1, "hello", true})
	})

	t.Run("array - nested", func(t *testing.T) {
		got, err := Decode([]byte("*1\r\n*2\r\n:1\r\n:2\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, []any{[]any{1, 2}})
	})

	t.Run("array - truncated returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n"))
		if err == nil {
			t.Error("expected error for truncated array, got nil")
		}
	})

	t.Run("array - invalid element returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n?bad\r\n"))
		if err == nil {
			t.Error("expected error for invalid element sigil, got nil")
		}
	})

	t.Run("array - too short returns error", func(t *testing.T) {
		_, err := Decode([]byte("*"))
		if err == nil {
			t.Error("expected error for too-short array buffer, got nil")
		}
	})

	t.Run("array - non-numeric count returns error", func(t *testing.T) {
		_, err := Decode([]byte("*x\r\n"))
		if err == nil {
			t.Error("expected error for non-numeric count, got nil")
		}
	})

	t.Run("array - missing LF after CR in count returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\rX"))
		if err == nil {
			t.Error("expected error for missing LF after CR, got nil")
		}
	})

	t.Run("array - missing length digits returns error", func(t *testing.T) {
		_, err := Decode([]byte("*\r\nX"))
		if err == nil {
			t.Error("expected error for missing length digits, got nil")
		}
	})

	t.Run("array - blob string element with non-numeric length returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n$x\r\nhello\r\n"))
		if err == nil {
			t.Error("expected error for blob string element with non-numeric length, got nil")
		}
	})

	t.Run("array - invalid blob string element returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n$3\r\nabcd\r\n"))
		if err == nil {
			t.Error("expected error for malformed blob string element, got nil")
		}
	})

	t.Run("array - valid error element", func(t *testing.T) {
		got, err := Decode([]byte("*1\r\n-ERR oops\r\n"))
		assertNoError(t, err)
		arr, ok := got.([]any)
		if !ok || len(arr) != 1 {
			t.Fatalf("expected single-element []any, got %#v", got)
		}
		e, ok := arr[0].(error)
		if !ok || e.Error() != "ERR oops" {
			t.Errorf("expected error \"ERR oops\", got %#v", arr[0])
		}
	})

	t.Run("array - valid big number element", func(t *testing.T) {
		got, err := Decode([]byte("*1\r\n(123\r\n"))
		assertNoError(t, err)
		arr, ok := got.([]any)
		if !ok || len(arr) != 1 {
			t.Fatalf("expected single-element []any, got %#v", got)
		}
		n, ok := arr[0].(*big.Int)
		if !ok || n.String() != "123" {
			t.Errorf("expected big.Int 123, got %#v", arr[0])
		}
	})

	t.Run("array - valid double element", func(t *testing.T) {
		got, err := Decode([]byte("*1\r\n,3.14\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, []any{3.14})
	})

	t.Run("array - valid null element", func(t *testing.T) {
		got, err := Decode([]byte("*1\r\n_\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, []any{Null})
	})

	t.Run("array - nested array with non-numeric count returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n*x\r\n"))
		if err == nil {
			t.Error("expected error for nested array non-numeric count, got nil")
		}
	})

	t.Run("array - nested array element count mismatch returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n*3\r\n:1\r\n:2\r\n"))
		if err == nil {
			t.Error("expected error for nested array element count mismatch, got nil")
		}
	})

	t.Run("array - invalid integer element returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n:bad\r\n"))
		if err == nil {
			t.Error("expected error for invalid integer element, got nil")
		}
	})

	t.Run("array - invalid big number element returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n(bad\r\n"))
		if err == nil {
			t.Error("expected error for invalid big number element, got nil")
		}
	})

	t.Run("array - invalid double element returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n,bad\r\n"))
		if err == nil {
			t.Error("expected error for invalid double element, got nil")
		}
	})

	t.Run("array - invalid boolean element returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n#bad\r\n"))
		if err == nil {
			t.Error("expected error for invalid boolean element, got nil")
		}
	})

	t.Run("array - invalid null element returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n_bad\r\n"))
		if err == nil {
			t.Error("expected error for invalid null element, got nil")
		}
	})

	t.Run("array - invalid error element returns error", func(t *testing.T) {
		_, err := Decode([]byte("*2\r\n:1\r\n-bad\rerr\r\n"))
		if err == nil {
			t.Error("expected error for invalid error element, got nil")
		}
	})

	t.Run("array - multi-digit count", func(t *testing.T) {
		got, err := Decode([]byte("*10\r\n:0\r\n:1\r\n:2\r\n:3\r\n:4\r\n:5\r\n:6\r\n:7\r\n:8\r\n:9\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, []any{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	})

	t.Run("array - nested set element", func(t *testing.T) {
		got, err := Decode([]byte("*1\r\n~2\r\n:1\r\n:2\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, []any{map[any]struct{}{1: {}, 2: {}}})
	})

	t.Run("array - nested set element with non-numeric count returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n~x\r\n"))
		if err == nil {
			t.Error("expected error for nested set non-numeric count, got nil")
		}
	})

	t.Run("array - nested set element with malformed item returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n~1\r\n?bad\r\n"))
		if err == nil {
			t.Error("expected error for nested set malformed item, got nil")
		}
	})

	t.Run("array - nested push element", func(t *testing.T) {
		got, err := Decode([]byte("*1\r\n>1\r\n+k\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, []any{Push{Kind: respcodec.SimpleString("k")}})
	})

	t.Run("array - nested push element with args", func(t *testing.T) {
		got, err := Decode([]byte("*1\r\n>2\r\n+k\r\n:1\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, []any{Push{Kind: respcodec.SimpleString("k"), Args: []any{1}}})
	})

	t.Run("array - nested push element with non-numeric count returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n>x\r\n"))
		if err == nil {
			t.Error("expected error for nested push non-numeric count, got nil")
		}
	})

	t.Run("array - nested push element with malformed kind frame returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n>1\r\n$x\r\n"))
		if err == nil {
			t.Error("expected error for nested push malformed kind frame, got nil")
		}
	})

	t.Run("array - nested push element count mismatch returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n>2\r\n+k\r\n"))
		if err == nil {
			t.Error("expected error for nested push element count mismatch, got nil")
		}
	})

	t.Run("array - nested push element with malformed kind returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n>1\r\n?bad\r\n"))
		if err == nil {
			t.Error("expected error for nested push malformed kind, got nil")
		}
	})

	t.Run("array - nested push element with no kind returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n>0\r\n"))
		if err == nil {
			t.Error("expected error for nested push with no kind, got nil")
		}
	})

	t.Run("array - nested push element with non-SimpleString kind returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n>1\r\n:1\r\n"))
		if err == nil {
			t.Error("expected error for nested push non-SimpleString kind, got nil")
		}
	})

	t.Run("array - nested map element with non-SimpleString key returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n%1\r\n:1\r\n:2\r\n"))
		if err == nil {
			t.Error("expected error for nested map non-SimpleString key, got nil")
		}
	})

	t.Run("array - nested map element with trailing extra data returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n%1\r\n+a\r\n:1\r\n:2\r\n"))
		if err == nil {
			t.Error("expected error for trailing data after declared pair count, got nil")
		}
	})

	t.Run("array - nested map element with malformed item returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n%1\r\n?bad\r\n:1\r\n"))
		if err == nil {
			t.Error("expected error for nested map malformed item, got nil")
		}
	})

	t.Run("array - nested map element with non-numeric pair count returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n%x\r\n"))
		if err == nil {
			t.Error("expected error for nested map non-numeric pair count, got nil")
		}
	})

	t.Run("array - nested map element with duplicate key returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n%2\r\n+k\r\n:1\r\n+k\r\n:2\r\n"))
		if err == nil {
			t.Error("expected error for nested map duplicate key, got nil")
		}
	})

	t.Run("array - nested attribute element with non-SimpleString key returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n|1\r\n:1\r\n:2\r\n"))
		if err == nil {
			t.Error("expected error for nested attribute non-SimpleString key, got nil")
		}
	})

	t.Run("array - nested attribute element with trailing extra data returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n|1\r\n+a\r\n:1\r\n:2\r\n"))
		if err == nil {
			t.Error("expected error for trailing data after declared pair count, got nil")
		}
	})

	t.Run("array - nested attribute element with malformed item returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n|1\r\n?bad\r\n:1\r\n"))
		if err == nil {
			t.Error("expected error for nested attribute malformed item, got nil")
		}
	})

	t.Run("array - nested attribute element with non-numeric pair count returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n|x\r\n"))
		if err == nil {
			t.Error("expected error for nested attribute non-numeric pair count, got nil")
		}
	})

	t.Run("array - nested attribute element with duplicate key returns error", func(t *testing.T) {
		_, err := Decode([]byte("*1\r\n|2\r\n+k\r\n:1\r\n+k\r\n:2\r\n"))
		if err == nil {
			t.Error("expected error for nested attribute duplicate key, got nil")
		}
	})

	t.Run("array - nested map element", func(t *testing.T) {
		got, err := Decode([]byte("*1\r\n%1\r\n+k\r\n:1\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, []any{map[respcodec.SimpleString]any{respcodec.SimpleString("k"): 1}})
	})

	t.Run("array - nested attribute element", func(t *testing.T) {
		got, err := Decode([]byte("*1\r\n|1\r\n+k\r\n:1\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, []any{AttributeType{respcodec.SimpleString("k"): 1}})
	})

	t.Run("array - nested array followed by trailing sibling", func(t *testing.T) {
		got, err := Decode([]byte("*2\r\n*2\r\n:1\r\n:2\r\n:5\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, []any{[]any{1, 2}, 5})
	})

	t.Run("array - nested set followed by trailing sibling", func(t *testing.T) {
		got, err := Decode([]byte("*2\r\n~2\r\n:1\r\n:2\r\n:5\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, []any{map[any]struct{}{1: {}, 2: {}}, 5})
	})

	t.Run("array - nested attribute followed by trailing sibling", func(t *testing.T) {
		got, err := Decode([]byte("*2\r\n|1\r\n+k\r\n:1\r\n:5\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, []any{AttributeType{respcodec.SimpleString("k"): 1}, 5})
	})

	t.Run("array - nested push followed by trailing sibling", func(t *testing.T) {
		got, err := Decode([]byte("*2\r\n>1\r\n+k\r\n:5\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, []any{Push{Kind: respcodec.SimpleString("k")}, 5})
	})

	t.Run("array - nested map followed by trailing sibling", func(t *testing.T) {
		got, err := Decode([]byte("*2\r\n%1\r\n+k\r\n:1\r\n:5\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, []any{map[respcodec.SimpleString]any{respcodec.SimpleString("k"): 1}, 5})
	})

	t.Run("map - too short returns error", func(t *testing.T) {
		_, err := Decode([]byte("%"))
		if err == nil {
			t.Error("expected error for too-short map buffer, got nil")
		}
	})

	t.Run("map - empty", func(t *testing.T) {
		got, err := Decode([]byte("%0\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, map[respcodec.SimpleString]any{})
	})

	t.Run("map - single pair", func(t *testing.T) {
		got, err := Decode([]byte("%1\r\n+key\r\n:42\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, map[respcodec.SimpleString]any{respcodec.SimpleString("key"): 42})
	})

	t.Run("map - multiple pairs", func(t *testing.T) {
		got, err := Decode([]byte("%2\r\n+a\r\n:1\r\n+b\r\n:2\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, map[respcodec.SimpleString]any{
			respcodec.SimpleString("a"): 1,
			respcodec.SimpleString("b"): 2,
		})
	})

	t.Run("map - non-SimpleString key returns error", func(t *testing.T) {
		_, err := Decode([]byte("%1\r\n:1\r\n:2\r\n"))
		if err == nil {
			t.Error("expected error for non-SimpleString map key, got nil")
		}
	})

	t.Run("map - malformed value returns error", func(t *testing.T) {
		_, err := Decode([]byte("%1\r\n+k\r\n?bad\r\n"))
		if err == nil {
			t.Error("expected error for malformed map value, got nil")
		}
	})

	t.Run("map - truncated returns error", func(t *testing.T) {
		_, err := Decode([]byte("%1\r\n"))
		if err == nil {
			t.Error("expected error for truncated map, got nil")
		}
	})

	t.Run("map - trailing extra data returns error", func(t *testing.T) {
		_, err := Decode([]byte("%1\r\n+a\r\n:1\r\n:2\r\n"))
		if err == nil {
			t.Error("expected error for trailing data after declared pair count, got nil")
		}
	})

	t.Run("set - too short returns error", func(t *testing.T) {
		_, err := Decode([]byte("~"))
		if err == nil {
			t.Error("expected error for too-short set buffer, got nil")
		}
	})

	t.Run("set - empty", func(t *testing.T) {
		got, err := Decode([]byte("~0\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, map[any]struct{}{})
	})

	t.Run("set - single element", func(t *testing.T) {
		got, err := Decode([]byte("~1\r\n:42\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, map[any]struct{}{42: {}})
	})

	t.Run("set - multiple elements", func(t *testing.T) {
		got, err := Decode([]byte("~2\r\n:1\r\n:2\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, map[any]struct{}{1: {}, 2: {}})
	})

	t.Run("set - malformed element returns error", func(t *testing.T) {
		_, err := Decode([]byte("~1\r\n?bad\r\n"))
		if err == nil {
			t.Error("expected error for malformed set element, got nil")
		}
	})

	t.Run("attribute - too short returns error", func(t *testing.T) {
		_, err := Decode([]byte("|"))
		if err == nil {
			t.Error("expected error for too-short attribute buffer, got nil")
		}
	})

	t.Run("attribute - empty", func(t *testing.T) {
		got, err := Decode([]byte("|0\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, AttributeType{})
	})

	t.Run("attribute - single pair", func(t *testing.T) {
		got, err := Decode([]byte("|1\r\n+ttl\r\n:100\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, AttributeType{respcodec.SimpleString("ttl"): 100})
	})

	t.Run("attribute - non-SimpleString key returns error", func(t *testing.T) {
		_, err := Decode([]byte("|1\r\n:1\r\n:2\r\n"))
		if err == nil {
			t.Error("expected error for non-SimpleString attribute key, got nil")
		}
	})

	t.Run("attribute - malformed value returns error", func(t *testing.T) {
		_, err := Decode([]byte("|1\r\n+k\r\n?bad\r\n"))
		if err == nil {
			t.Error("expected error for malformed attribute value, got nil")
		}
	})

	t.Run("attribute - trailing extra data returns error", func(t *testing.T) {
		_, err := Decode([]byte("|1\r\n+a\r\n:1\r\n:2\r\n"))
		if err == nil {
			t.Error("expected error for trailing data after declared pair count, got nil")
		}
	})

	t.Run("attribute - malformed leading element returns error", func(t *testing.T) {
		_, err := Decode([]byte("|1\r\n?bad\r\n:1\r\n"))
		if err == nil {
			t.Error("expected error for malformed attribute element, got nil")
		}
	})

	t.Run("push - too short returns error", func(t *testing.T) {
		_, err := Decode([]byte(">"))
		if err == nil {
			t.Error("expected error for too-short push buffer, got nil")
		}
	})

	t.Run("push - non-numeric count returns error", func(t *testing.T) {
		_, err := Decode([]byte(">x\r\n"))
		if err == nil {
			t.Error("expected error for non-numeric push count, got nil")
		}
	})

	t.Run("push - no args", func(t *testing.T) {
		got, err := Decode([]byte(">1\r\n+subscribe\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, Push{Kind: respcodec.SimpleString("subscribe")})
	})

	t.Run("push - with args", func(t *testing.T) {
		got, err := Decode([]byte(">3\r\n+message\r\n+ch\r\n$7\r\npayload\r\n"))
		assertNoError(t, err)
		assertDeepEqual(t, got, Push{
			Kind: respcodec.SimpleString("message"),
			Args: []any{respcodec.SimpleString("ch"), "payload"},
		})
	})

	t.Run("push - non-SimpleString kind returns error", func(t *testing.T) {
		_, err := Decode([]byte(">1\r\n:1\r\n"))
		if err == nil {
			t.Error("expected error for non-SimpleString push kind, got nil")
		}
	})

	t.Run("push - malformed kind element returns error", func(t *testing.T) {
		_, err := Decode([]byte(">1\r\n?bad\r\n"))
		if err == nil {
			t.Error("expected error for malformed push kind element, got nil")
		}
	})

	t.Run("push - empty returns error", func(t *testing.T) {
		_, err := Decode([]byte(">0\r\n"))
		if err == nil {
			t.Error("expected error for push with no kind, got nil")
		}
	})
}

func BenchmarkDecode(b *testing.B) {
	cases := []struct {
		name  string
		input []byte
	}{
		{"simple string", []byte("+OK\r\n")},
		{"simple error", []byte("-ERR unknown\r\n")},
		{"integer", []byte(":42\r\n")},
		{"blob string", []byte("$5\r\nhello\r\n")},
		{"blob error", []byte("!8\r\nERR oops\r\n")},
		{"verbatim string", []byte("=9\r\ntxt:hello\r\n")},
		{"big number", []byte("(123456789012345678901234567890\r\n")},
		{"double", []byte(",3.14\r\n")},
		{"null", []byte("_\r\n")},
		{"boolean true", []byte("#t\r\n")},
		{"boolean false", []byte("#f\r\n")},
		{"array 3 elements", []byte("*3\r\n:1\r\n$5\r\nhello\r\n#t\r\n")},
		{"map 1 pair", []byte("%1\r\n+key\r\n:42\r\n")},
		{"set 1 element", []byte("~1\r\n:42\r\n")},
		{"attribute 1 pair", []byte("|1\r\n+ttl\r\n:100\r\n")},
		{"push 2 args", []byte(">3\r\n+message\r\n+ch\r\n$5\r\nhello\r\n")},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			for b.Loop() {
				_, _ = Decode(c.input)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	cases := []struct {
		name  string
		input any
	}{
		{"simple string", respcodec.SimpleString("OK")},
		{"integer", 42},
		{"blob string", "hello"},
		{"double", 3.14},
		{"null", Null},
		{"boolean", true},
		{"array", []any{1, "hello", true}},
		{"map", map[respcodec.SimpleString]any{respcodec.SimpleString("key"): 42}},
		{"set", map[any]struct{}{42: {}}},
		{"attribute", AttributeType{respcodec.SimpleString("ttl"): 100}},
		{"push", Push{Kind: respcodec.SimpleString("message"), Args: []any{respcodec.SimpleString("ch"), "hello"}}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			buf, err := Encode(c.input)
			assertNoError(t, err)
			got, err := Decode(buf)
			assertNoError(t, err)
			assertDeepEqual(t, got, c.input)
		})
	}
}

func assertNoError(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertBytes(t testing.TB, got, want []byte) {
	t.Helper()
	if !bytes.Equal(got, want) {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func assertDeepEqual(t testing.TB, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %#v, got %#v", want, got)
	}
}
