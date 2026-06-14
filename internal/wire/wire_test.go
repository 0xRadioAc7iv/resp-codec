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
