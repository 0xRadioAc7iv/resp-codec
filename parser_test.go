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
	buf, err := Encode(3.14) // unknown type → nil, error
	fmt.Printf("%v %v\n", buf, err)

	// Output:
	// "+OK\r\n"
	// "-ERR unknown command\r\n"
	// ":42\r\n"
	// [] unsupported type float64: cannot encode to RESP
}

func BenchmarkEncode(b *testing.B) {
	cases := []struct {
		name  string
		input any
	}{
		{"simple string", SimpleString("OK")},
		{"error", errors.New("ERR unknown command")},
		{"integer", 42},
		{"integer max", math.MaxInt},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			for b.Loop() {
				_, _ = Encode(c.input)
			}
		})
	}
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
