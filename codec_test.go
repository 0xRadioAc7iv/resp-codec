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

func ExampleDecode() {
	ss, _ := Decode[SimpleString]([]byte("+OK\r\n"))
	fmt.Printf("%q\n", ss)
	e, _ := Decode[error]([]byte("-ERR unknown command\r\n"))
	fmt.Printf("%q\n", e.Error())
	n, _ := Decode[int]([]byte(":42\r\n"))
	fmt.Printf("%d\n", n)

	// Output:
	// "OK"
	// "ERR unknown command"
	// 42
}

func BenchmarkDecode(b *testing.B) {
	cases := []struct {
		name string
		fn   func()
	}{
		{"simple string", func() { _, _ = Decode[SimpleString]([]byte("+OK\r\n")) }},
		{"error", func() { _, _ = Decode[error]([]byte("-ERR unknown command\r\n")) }},
		{"integer", func() { _, _ = Decode[int]([]byte(":42\r\n")) }},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			for b.Loop() {
				c.fn()
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
		got, err := Decode[SimpleString]([]byte("+OK\r\n"))
		assertNoError(t, err)
		if got != SimpleString("OK") {
			t.Errorf("expected %q, got %q", SimpleString("OK"), got)
		}
	})

	t.Run("simple string - empty", func(t *testing.T) {
		got, err := Decode[SimpleString]([]byte("+\r\n"))
		assertNoError(t, err)
		if got != SimpleString("") {
			t.Errorf("expected empty SimpleString, got %q", got)
		}
	})

	t.Run("simple string - multi word", func(t *testing.T) {
		got, err := Decode[SimpleString]([]byte("+hello world\r\n"))
		assertNoError(t, err)
		if got != SimpleString("hello world") {
			t.Errorf("expected %q, got %q", SimpleString("hello world"), got)
		}
	})

	t.Run("simple string - wrong prefix returns error", func(t *testing.T) {
		_, err := Decode[SimpleString]([]byte("-ERR\r\n"))
		if err == nil {
			t.Error("expected error for wrong prefix, got nil")
		}
	})

	t.Run("simple string - empty buffer returns error", func(t *testing.T) {
		_, err := Decode[SimpleString]([]byte{})
		if err == nil {
			t.Error("expected error for empty buffer, got nil")
		}
	})

	t.Run("error - basic message", func(t *testing.T) {
		got, err := Decode[error]([]byte("-ERR unknown command\r\n"))
		assertNoError(t, err)
		if got.Error() != "ERR unknown command" {
			t.Errorf("expected %q, got %q", "ERR unknown command", got.Error())
		}
	})

	t.Run("error - empty message", func(t *testing.T) {
		got, err := Decode[error]([]byte("-\r\n"))
		assertNoError(t, err)
		if got.Error() != "" {
			t.Errorf("expected empty error message, got %q", got.Error())
		}
	})

	t.Run("error - WRONGTYPE prefix", func(t *testing.T) {
		got, err := Decode[error]([]byte("-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"))
		assertNoError(t, err)
		if got.Error() != "WRONGTYPE Operation against a key holding the wrong kind of value" {
			t.Errorf("unexpected error message: %q", got.Error())
		}
	})

	t.Run("error - wrong prefix returns error", func(t *testing.T) {
		_, err := Decode[error]([]byte("+OK\r\n"))
		if err == nil {
			t.Error("expected error for wrong prefix, got nil")
		}
	})

	t.Run("error - empty buffer returns error", func(t *testing.T) {
		_, err := Decode[error]([]byte{})
		if err == nil {
			t.Error("expected error for empty buffer, got nil")
		}
	})

	t.Run("integer - positive", func(t *testing.T) {
		got, err := Decode[int]([]byte(":42\r\n"))
		assertNoError(t, err)
		if got != 42 {
			t.Errorf("expected 42, got %d", got)
		}
	})

	t.Run("integer - zero", func(t *testing.T) {
		got, err := Decode[int]([]byte(":0\r\n"))
		assertNoError(t, err)
		if got != 0 {
			t.Errorf("expected 0, got %d", got)
		}
	})

	t.Run("integer - negative", func(t *testing.T) {
		got, err := Decode[int]([]byte(":-1\r\n"))
		assertNoError(t, err)
		if got != -1 {
			t.Errorf("expected -1, got %d", got)
		}
	})

	t.Run("integer - large value", func(t *testing.T) {
		got, err := Decode[int]([]byte(":1000000\r\n"))
		assertNoError(t, err)
		if got != 1000000 {
			t.Errorf("expected 1000000, got %d", got)
		}
	})

	t.Run("integer - max int", func(t *testing.T) {
		buf := fmt.Appendf(nil, ":%d\r\n", math.MaxInt)
		got, err := Decode[int](buf)
		assertNoError(t, err)
		if got != math.MaxInt {
			t.Errorf("expected %d, got %d", math.MaxInt, got)
		}
	})

	t.Run("integer - min int", func(t *testing.T) {
		buf := fmt.Appendf(nil, ":%d\r\n", math.MinInt)
		got, err := Decode[int](buf)
		assertNoError(t, err)
		if got != math.MinInt {
			t.Errorf("expected %d, got %d", math.MinInt, got)
		}
	})

	t.Run("integer - invalid character returns error", func(t *testing.T) {
		_, err := Decode[int]([]byte(":abc\r\n"))
		if err == nil {
			t.Error("expected error for non-digit character, got nil")
		}
	})

	t.Run("integer - wrong prefix returns error", func(t *testing.T) {
		_, err := Decode[int]([]byte("+OK\r\n"))
		if err == nil {
			t.Error("expected error for wrong prefix, got nil")
		}
	})

	t.Run("integer - empty buffer returns error", func(t *testing.T) {
		_, err := Decode[int]([]byte{})
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
		}

		for name, input := range cases {
			t.Run(name, func(t *testing.T) {
				_, err := Decode[SimpleString](input)
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
		}

		for name, input := range cases {
			t.Run(name, func(t *testing.T) {
				_, err := Decode[error](input)
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
				_, err := Decode[int](input)
				if err == nil {
					t.Fatalf("expected error for malformed integer %q", input)
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
