package resp3

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	respcodec "github.com/0xRadioAc7iv/resp-codec"
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
	buf, err := Encode(3.14)
	fmt.Printf("%v %v\n", buf, err)

	// Output:
	// "+OK\r\n"
	// "-ERR unknown command\r\n"
	// ":42\r\n"
	// "$5\r\nhello\r\n"
	// [] unsupported type float64: cannot encode to RESP3
}

func ExampleAppendEncode() {
	buf := make([]byte, 0, 128)
	buf, _ = AppendEncode(buf, respcodec.SimpleString("OK"))
	buf, _ = AppendEncode(buf, errors.New("ERR unknown command"))
	buf, _ = AppendEncode(buf, 42)
	buf, _ = AppendEncode(buf, "hello")
	fmt.Printf("%q\n", buf)

	// Output:
	// "+OK\r\n-ERR unknown command\r\n:42\r\n$5\r\nhello\r\n"
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

	t.Run("unsupported type returns nil and error", func(t *testing.T) {
		got, err := Encode(3.14)
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
		_, err := AppendEncode(nil, 3.14)
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

func assertBytes(t testing.TB, got, want []byte) {
	t.Helper()
	if !bytes.Equal(got, want) {
		t.Errorf("expected %q, got %q", want, got)
	}
}
