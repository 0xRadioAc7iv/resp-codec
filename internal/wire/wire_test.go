package wire

import (
	"bytes"
	"fmt"
	"math"
	"testing"
)

// ---- AppendInteger ----

func ExampleAppendInteger() {
	fmt.Printf("%q\n", AppendInteger(nil, 42))
	fmt.Printf("%q\n", AppendInteger(nil, 0))
	fmt.Printf("%q\n", AppendInteger(nil, -1))

	// Output:
	// ":42\r\n"
	// ":0\r\n"
	// ":-1\r\n"
}

func BenchmarkAppendInteger(b *testing.B) {
	for b.Loop() {
		_ = AppendInteger(nil, 42)
	}
}

func TestAppendInteger(t *testing.T) {
	cases := []struct {
		name  string
		input int
		want  []byte
	}{
		{"positive", 42, []byte(":42\r\n")},
		{"zero", 0, []byte(":0\r\n")},
		{"negative", -1, []byte(":-1\r\n")},
		{"large", 1000000, []byte(":1000000\r\n")},
		{"max int", math.MaxInt, fmt.Appendf(nil, ":%d\r\n", math.MaxInt)},
		{"min int", math.MinInt, fmt.Appendf(nil, ":%d\r\n", math.MinInt)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := AppendInteger(nil, c.input)
			if !bytes.Equal(got, c.want) {
				t.Errorf("expected %q, got %q", c.want, got)
			}
		})
	}
}

// ---- AppendSimpleString ----

func ExampleAppendSimpleString() {
	buf, _ := AppendSimpleString(nil, "OK")
	fmt.Printf("%q\n", buf)
	buf, _ = AppendSimpleString(nil, "")
	fmt.Printf("%q\n", buf)
	_, err := AppendSimpleString(nil, "bad\r\ninput")
	fmt.Printf("%v\n", err != nil)

	// Output:
	// "+OK\r\n"
	// "+\r\n"
	// true
}

func BenchmarkAppendSimpleString(b *testing.B) {
	for b.Loop() {
		_, _ = AppendSimpleString(nil, "OK")
	}
}

func TestAppendSimpleString(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []byte
	}{
		{"ok", "OK", []byte("+OK\r\n")},
		{"pong", "PONG", []byte("+PONG\r\n")},
		{"empty", "", []byte("+\r\n")},
		{"multi word", "hello world", []byte("+hello world\r\n")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := AppendSimpleString(nil, c.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !bytes.Equal(got, c.want) {
				t.Errorf("expected %q, got %q", c.want, got)
			}
		})
	}
}

func TestAppendSimpleStringRejectsCRLF(t *testing.T) {
	cases := []string{"has\rnewline", "has\nnewline", "has\r\nboth"}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			_, err := AppendSimpleString(nil, input)
			if err == nil {
				t.Errorf("expected error for input %q, got nil", input)
			}
		})
	}
}

func TestAppendSimpleStringPreservesBufOnError(t *testing.T) {
	existing := []byte("+OK\r\n")
	buf := make([]byte, len(existing), 64)
	copy(buf, existing)
	result, err := AppendSimpleString(buf, "bad\r\ninput")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !bytes.Equal(result, existing) {
		t.Errorf("buf modified on error: expected %q, got %q", existing, result)
	}
}

// ---- AppendSimpleError ----

func ExampleAppendSimpleError() {
	buf, _ := AppendSimpleError(nil, "ERR unknown command")
	fmt.Printf("%q\n", buf)
	buf, _ = AppendSimpleError(nil, "")
	fmt.Printf("%q\n", buf)
	_, err := AppendSimpleError(nil, "bad\r\ninput")
	fmt.Printf("%v\n", err != nil)

	// Output:
	// "-ERR unknown command\r\n"
	// "-\r\n"
	// true
}

func BenchmarkAppendSimpleError(b *testing.B) {
	for b.Loop() {
		_, _ = AppendSimpleError(nil, "ERR unknown command")
	}
}

func TestAppendSimpleError(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []byte
	}{
		{"basic", "ERR unknown command", []byte("-ERR unknown command\r\n")},
		{"empty", "", []byte("-\r\n")},
		{"wrongtype prefix", "WRONGTYPE Operation against wrong kind", []byte("-WRONGTYPE Operation against wrong kind\r\n")},
		{"prefix only", "ERR", []byte("-ERR\r\n")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := AppendSimpleError(nil, c.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !bytes.Equal(got, c.want) {
				t.Errorf("expected %q, got %q", c.want, got)
			}
		})
	}
}

func TestAppendSimpleErrorRejectsCRLF(t *testing.T) {
	cases := []string{"ERR bad\rinput", "ERR bad\ninput", "ERR bad\r\ninput"}
	for _, input := range cases {
		t.Run(input, func(t *testing.T) {
			_, err := AppendSimpleError(nil, input)
			if err == nil {
				t.Errorf("expected error for input %q, got nil", input)
			}
		})
	}
}

func TestAppendSimpleErrorPreservesBufOnError(t *testing.T) {
	existing := []byte("-ERR\r\n")
	buf := make([]byte, len(existing), 64)
	copy(buf, existing)
	result, err := AppendSimpleError(buf, "bad\r\ninput")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !bytes.Equal(result, existing) {
		t.Errorf("buf modified on error: expected %q, got %q", existing, result)
	}
}

// ---- AppendBlobString ----

func ExampleAppendBlobString() {
	buf := AppendBlobString(nil, "hello")
	fmt.Printf("%q\n", buf)
	buf = AppendBlobString(nil, "")
	fmt.Printf("%q\n", buf)
	buf = AppendBlobString(nil, "binary\r\nsafe")
	fmt.Printf("%q\n", buf)

	// Output:
	// "$5\r\nhello\r\n"
	// "$0\r\n\r\n"
	// "$12\r\nbinary\r\nsafe\r\n"
}

func BenchmarkAppendBlobString(b *testing.B) {
	for b.Loop() {
		_ = AppendBlobString(nil, "hello world")
	}
}

func TestAppendBlobString(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []byte
	}{
		{"normal", "hello", []byte("$5\r\nhello\r\n")},
		{"empty", "", []byte("$0\r\n\r\n")},
		{"with spaces", "hello world", []byte("$11\r\nhello world\r\n")},
		{"binary safe with CRLF", "hello\r\nworld", []byte("$12\r\nhello\r\nworld\r\n")},
		{"single char", "x", []byte("$1\r\nx\r\n")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := AppendBlobString(nil, c.input)
			if !bytes.Equal(got, c.want) {
				t.Errorf("expected %q, got %q", c.want, got)
			}
		})
	}
}

func TestAppendBlobStringAppendsToExisting(t *testing.T) {
	buf := []byte(":42\r\n")
	got := AppendBlobString(buf, "hello")
	want := []byte(":42\r\n$5\r\nhello\r\n")
	if !bytes.Equal(got, want) {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// ---- AppendBlobError ----

func ExampleAppendBlobError() {
	buf := AppendBlobError(nil, "ERR unknown command")
	fmt.Printf("%q\n", buf)
	buf = AppendBlobError(nil, "")
	fmt.Printf("%q\n", buf)
	buf = AppendBlobError(nil, "ERR multi\r\nline error")
	fmt.Printf("%q\n", buf)

	// Output:
	// "!19\r\nERR unknown command\r\n"
	// "!0\r\n\r\n"
	// "!21\r\nERR multi\r\nline error\r\n"
}

func BenchmarkAppendBlobError(b *testing.B) {
	for b.Loop() {
		_ = AppendBlobError(nil, "ERR unknown command")
	}
}

func TestAppendBlobError(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []byte
	}{
		{"basic", "ERR unknown command", []byte("!19\r\nERR unknown command\r\n")},
		{"empty", "", []byte("!0\r\n\r\n")},
		{"binary safe with CR", "ERR bad\rvalue", []byte("!13\r\nERR bad\rvalue\r\n")},
		{"binary safe with CRLF", "ERR multi\r\nline", []byte("!15\r\nERR multi\r\nline\r\n")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := AppendBlobError(nil, c.input)
			if !bytes.Equal(got, c.want) {
				t.Errorf("expected %q, got %q", c.want, got)
			}
		})
	}
}

func TestAppendBlobErrorAppendsToExisting(t *testing.T) {
	buf := []byte(":42\r\n")
	got := AppendBlobError(buf, "ERR oops")
	want := []byte(":42\r\n!8\r\nERR oops\r\n")
	if !bytes.Equal(got, want) {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// ---- AppendVerbatimString ----

func ExampleAppendVerbatimString() {
	buf := AppendVerbatimString(nil, "txt:hello")
	fmt.Printf("%q\n", buf)
	buf = AppendVerbatimString(nil, "mkd:**bold**")
	fmt.Printf("%q\n", buf)
	buf = AppendVerbatimString(nil, "")
	fmt.Printf("%q\n", buf)

	// Output:
	// "=9\r\ntxt:hello\r\n"
	// "=12\r\nmkd:**bold**\r\n"
	// "=0\r\n\r\n"
}

func BenchmarkAppendVerbatimString(b *testing.B) {
	for b.Loop() {
		_ = AppendVerbatimString(nil, "txt:hello world")
	}
}

func TestAppendVerbatimString(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []byte
	}{
		{"text encoding", "txt:hello", []byte("=9\r\ntxt:hello\r\n")},
		{"markdown encoding", "mkd:**bold**", []byte("=12\r\nmkd:**bold**\r\n")},
		{"empty", "", []byte("=0\r\n\r\n")},
		{"binary safe with CRLF", "txt:line1\r\nline2", []byte("=16\r\ntxt:line1\r\nline2\r\n")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := AppendVerbatimString(nil, c.input)
			if !bytes.Equal(got, c.want) {
				t.Errorf("expected %q, got %q", c.want, got)
			}
		})
	}
}

func TestAppendVerbatimStringAppendsToExisting(t *testing.T) {
	buf := []byte(":42\r\n")
	got := AppendVerbatimString(buf, "txt:hello")
	want := []byte(":42\r\n=9\r\ntxt:hello\r\n")
	if !bytes.Equal(got, want) {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// ---- DecodeLineFrame ----

func ExampleDecodeLineFrame() {
	payload, _ := DecodeLineFrame([]byte("+OK\r\n"), '+')
	fmt.Printf("%q\n", payload)
	_, err := DecodeLineFrame([]byte("+bad\rinput\r\n"), '+')
	fmt.Println(err)

	// Output:
	// "OK"
	// line frame payload must not contain CR or LF
}

func BenchmarkDecodeLineFrame(b *testing.B) {
	buf := []byte("+OK\r\n")
	for b.Loop() {
		_, _ = DecodeLineFrame(buf, '+')
	}
}

func TestDecodeLineFrame(t *testing.T) {
	t.Run("valid simple string", func(t *testing.T) {
		got, err := DecodeLineFrame([]byte("+OK\r\n"), '+')
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !bytes.Equal(got, []byte("OK")) {
			t.Errorf("expected %q, got %q", "OK", got)
		}
	})

	t.Run("valid empty payload", func(t *testing.T) {
		got, err := DecodeLineFrame([]byte("+\r\n"), '+')
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !bytes.Equal(got, []byte("")) {
			t.Errorf("expected empty payload, got %q", got)
		}
	})

	t.Run("valid simple error", func(t *testing.T) {
		got, err := DecodeLineFrame([]byte("-ERR oops\r\n"), '-')
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !bytes.Equal(got, []byte("ERR oops")) {
			t.Errorf("expected %q, got %q", "ERR oops", got)
		}
	})

	t.Run("too short returns error", func(t *testing.T) {
		_, err := DecodeLineFrame([]byte("+\r"), '+')
		if err == nil {
			t.Error("expected error for too-short frame, got nil")
		}
	})

	t.Run("empty buf returns error", func(t *testing.T) {
		_, err := DecodeLineFrame([]byte{}, '+')
		if err == nil {
			t.Error("expected error for empty buf, got nil")
		}
	})

	t.Run("wrong sigil returns error", func(t *testing.T) {
		_, err := DecodeLineFrame([]byte("+OK\r\n"), '-')
		if err == nil {
			t.Error("expected error for wrong sigil, got nil")
		}
	})

	t.Run("missing CRLF terminator returns error", func(t *testing.T) {
		_, err := DecodeLineFrame([]byte("+OKxx"), '+')
		if err == nil {
			t.Error("expected error for missing CRLF terminator, got nil")
		}
	})

	t.Run("payload containing CR returns error", func(t *testing.T) {
		_, err := DecodeLineFrame([]byte("+a\rb\r\n"), '+')
		if err == nil {
			t.Error("expected error for CR in payload, got nil")
		}
	})

	t.Run("payload containing LF returns error", func(t *testing.T) {
		_, err := DecodeLineFrame([]byte("+a\nb\r\n"), '+')
		if err == nil {
			t.Error("expected error for LF in payload, got nil")
		}
	})
}

// ---- DecodeBlobFrame ----

func ExampleDecodeBlobFrame() {
	data, _ := DecodeBlobFrame([]byte("$5\r\nhello\r\n"), '$')
	fmt.Println(data)
	_, err := DecodeBlobFrame([]byte("$5\r\nhi\r\n"), '$')
	fmt.Println(err)

	// Output:
	// hello
	// blob frame length mismatch: declared 5, frame has 2 data bytes
}

func BenchmarkDecodeBlobFrame(b *testing.B) {
	buf := []byte("$5\r\nhello\r\n")
	for b.Loop() {
		_, _ = DecodeBlobFrame(buf, '$')
	}
}

func TestDecodeBlobFrame(t *testing.T) {
	t.Run("valid blob string", func(t *testing.T) {
		got, err := DecodeBlobFrame([]byte("$5\r\nhello\r\n"), '$')
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "hello" {
			t.Errorf("expected %q, got %q", "hello", got)
		}
	})

	t.Run("valid empty blob", func(t *testing.T) {
		got, err := DecodeBlobFrame([]byte("$0\r\n\r\n"), '$')
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("valid blob error", func(t *testing.T) {
		got, err := DecodeBlobFrame([]byte("!8\r\nERR oops\r\n"), '!')
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "ERR oops" {
			t.Errorf("expected %q, got %q", "ERR oops", got)
		}
	})

	t.Run("binary safe with embedded CRLF", func(t *testing.T) {
		got, err := DecodeBlobFrame([]byte("$12\r\nhello\r\nworld\r\n"), '$')
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "hello\r\nworld" {
			t.Errorf("expected %q, got %q", "hello\r\nworld", got)
		}
	})

	t.Run("too short returns error", func(t *testing.T) {
		_, err := DecodeBlobFrame([]byte("$1\r\n"), '$')
		if err == nil {
			t.Error("expected error for too-short frame, got nil")
		}
	})

	t.Run("empty buf returns error", func(t *testing.T) {
		_, err := DecodeBlobFrame([]byte{}, '$')
		if err == nil {
			t.Error("expected error for empty buf, got nil")
		}
	})

	t.Run("wrong sigil returns error", func(t *testing.T) {
		_, err := DecodeBlobFrame([]byte("$5\r\nhello\r\n"), '!')
		if err == nil {
			t.Error("expected error for wrong sigil, got nil")
		}
	})

	t.Run("missing CRLF terminator returns error", func(t *testing.T) {
		_, err := DecodeBlobFrame([]byte("$5\r\nhelloXX"), '$')
		if err == nil {
			t.Error("expected error for missing CRLF terminator, got nil")
		}
	})

	t.Run("non-numeric length returns error", func(t *testing.T) {
		_, err := DecodeBlobFrame([]byte("$x\r\nhello\r\n"), '$')
		if err == nil {
			t.Error("expected error for non-numeric length, got nil")
		}
	})

	t.Run("negative length returns error", func(t *testing.T) {
		_, err := DecodeBlobFrame([]byte("$-1\r\n\r\n"), '$')
		if err == nil {
			t.Error("expected error for negative length, got nil")
		}
	})

	t.Run("length mismatch returns error", func(t *testing.T) {
		_, err := DecodeBlobFrame([]byte("$5\r\nhi\r\n"), '$')
		if err == nil {
			t.Error("expected error for length mismatch, got nil")
		}
	})
}
