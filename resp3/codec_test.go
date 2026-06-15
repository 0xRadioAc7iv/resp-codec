package resp3

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
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
	fmt.Printf("%q\n", buf)

	// Output:
	// "+OK\r\n-ERR unknown command\r\n:42\r\n$5\r\nhello\r\n!8\r\nERR oops\r\n=9\r\ntxt:hello\r\n(9999999999999999999\r\n,3.14\r\n,inf\r\n,-inf\r\n,nan\r\n_\r\n#t\r\n"
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
		{"big number", func() *big.Int { n, _ := new(big.Int).SetString("123456789012345678901234567890", 10); return n }()},
		{"null", Null},
		{"boolean true", true},
		{"boolean false", false},
		{"double float64", 3.14},
		{"double float32", float32(1.5)},
		{"double inf", Inf},
		{"double neg inf", NegInf},
		{"double nan", NaN},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			for b.Loop() {
				_, _ = Encode(c.input)
			}
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
