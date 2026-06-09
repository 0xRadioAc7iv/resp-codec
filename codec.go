package respcodec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Encode serializes a Go value into its RESP byte representation.
//
// Supported types and their RESP encoding:
//   - SimpleString → +<value>\r\n        (must not contain CR or LF)
//   - string       → $<len>\r\n<data>\r\n (binary-safe bulk string)
//   - error        → -<message>\r\n      (must not contain CR or LF)
//   - int          → :<value>\r\n
//   - []any        → *<len>\r\n<elements> (each element encoded recursively)
//   - Null         → $-1\r\n             (null bulk string)
//   - NullArr      → *-1\r\n             (null array)
//
// Returns (nil, error) for unsupported types, invalid input, or arrays containing
// an invalid element.
//
// Encode allocates a single initial buffer and grows it as needed; for outputs
// that fit within 64 bytes this is typically one allocation. Array elements are
// written into the same buffer via the internal append-style encode function,
// avoiding per-element allocations. Use AppendEncode to supply your own buffer.
func Encode(data any) ([]byte, error) {
	buf, err := appendEncode(make([]byte, 0, 64), data)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// AppendEncode appends the RESP encoding of data into buf and returns the extended slice.
// It makes zero additional allocations when buf has sufficient capacity, making it
// suitable for callers that manage their own buffer — for example, writing directly
// to a net.Conn using a pooled buffer from sync.Pool.
//
// On error, buf is returned in its original state (no partial bytes are left behind),
// so it is safe to reuse after a failed call.
//
// Supported types are identical to Encode.
func AppendEncode(buf []byte, data any) ([]byte, error) {
	return appendEncode(buf, data)
}

// Decode parses a RESP-encoded byte slice into a Go value of type T.
//
// Supported type parameters and the RESP prefix each expects:
//
//	Decode[SimpleString] — expects '+' prefix
//	Decode[error]        — expects '-' prefix
//	Decode[int]          — expects ':' prefix; handles negative values
//
// Returns (zero, error) on a missing or wrong type prefix, an empty buffer,
// or an invalid character in the data. Panics for unsupported type parameters.
func Decode[T any](buf []byte) (T, error) {
	var t T

	r := bytes.NewReader(buf)

	switch any((*T)(nil)).(type) {
	case *SimpleString:
		b, err := r.ReadByte()
		if err != nil {
			return t, fmt.Errorf("failed to read type prefix: %w", err)
		}
		if b != '+' {
			return t, fmt.Errorf("invalid type prefix for SimpleString: expected '+', got %q", b)
		}

		var simpleString strings.Builder

		for {
			b, err := r.ReadByte()
			if err == io.EOF {
				break
			}
			if err != nil {
				return t, fmt.Errorf("failed to read simple string data: %w", err)
			}
			if b == '\r' {
				break
			}
			simpleString.WriteByte(b)
		}

		return any(SimpleString(simpleString.String())).(T), nil

	case *error:
		b, err := r.ReadByte()
		if err != nil {
			return t, fmt.Errorf("failed to read type prefix: %w", err)
		}
		if b != '-' {
			return t, fmt.Errorf("invalid type prefix for error: expected '-', got %q", b)
		}

		var errorString strings.Builder

		for {
			b, err := r.ReadByte()
			if err == io.EOF {
				break
			}
			if err != nil {
				return t, fmt.Errorf("failed to read error string data: %w", err)
			}
			if b == '\r' {
				break
			}
			errorString.WriteByte(b)
		}

		return any(errors.New(errorString.String())).(T), nil

	case *int:
		b, err := r.ReadByte()
		if err != nil {
			return t, fmt.Errorf("failed to read type prefix: %w", err)
		}
		if b != ':' {
			return t, fmt.Errorf("invalid type prefix for int: expected ':', got %q", b)
		}

		// -1 = yes, 1 = no
		isNeg := 1
		num := 0

		b, err = r.ReadByte()
		if err != nil {
			return t, fmt.Errorf("failed to read type prefix: %w", err)
		}
		if b == '-' {
			isNeg = -1
		} else {
			_ = r.UnreadByte()
		}

		for {
			b, err := r.ReadByte()
			if err == io.EOF {
				break
			}
			if err != nil {
				return t, fmt.Errorf("failed to read int data: %w", err)
			}
			if b == '\r' {
				break
			}
			if b < '0' || b > '9' {
				return t, fmt.Errorf("invalid character in int: %q", b)
			}
			num = (num * 10) + int(b-'0')
		}

		return any(isNeg * num).(T), nil

	default:
		panic("unsupported type")
	}
}

// appendEncode appends the RESP encoding of data into buf and returns the extended slice.
// It is the shared append-style core used by Encode and AppendEncode, and called recursively for arrays.
func appendEncode(buf []byte, data any) ([]byte, error) {
	switch v := data.(type) {

	// Used for simple strings (cannot have CR or LF) like "OK", "PONG" etc.
	case SimpleString:
		if strings.ContainsAny(string(v), "\r\n") {
			return buf, fmt.Errorf("simple string must not contain CR or LF characters: %q", string(v))
		}

		buf = append(buf, '+')
		buf = append(buf, v...)

	// Used for strings (can have CR and LF), Binary Safe
	case string:
		buf = append(buf, '$')
		buf = strconv.AppendInt(buf, int64(len(v)), 10)
		buf = append(buf, '\r', '\n')
		buf = append(buf, v...)

	// Used for sending error messages (cannot have CR or LF characters)
	case error:
		msg := v.Error()

		if strings.ContainsAny(msg, "\r\n") {
			return buf, fmt.Errorf("error message must not contain CR or LF characters: %q", msg)
		}

		buf = append(buf, '-')
		buf = append(buf, msg...)

	// Used for numbers
	case int:
		buf = append(buf, ':')
		buf = strconv.AppendInt(buf, int64(v), 10)

	// Used for arrays; elements are encoded recursively and may be of mixed types
	case []any:
		savedBuf := buf   // preserve reference in case encoding fails
		start := len(buf) // checkpoint before any array bytes are written

		buf = append(buf, '*')
		buf = strconv.AppendInt(buf, int64(len(v)), 10)
		buf = append(buf, '\r', '\n')

		for i, item := range v {
			var err error
			buf, err = appendEncode(buf, item)
			if err != nil {
				return savedBuf[:start], fmt.Errorf("failed to encode array element at index %d: %w", i, err)
			}
		}

		return buf, nil

	// Used for null bulk string; signals absence of a value ($-1\r\n)
	case nullBulkString:
		buf = append(buf, []byte("$-1")...)

	// Used for null array; alternative null representation used by commands like BLPOP on timeout (*-1\r\n)
	case nullArray:
		buf = append(buf, []byte("*-1")...)

	// When none of the type matches, return an error
	default:
		return buf, fmt.Errorf("unsupported type %T: cannot encode to RESP", data)
	}

	buf = append(buf, '\r', '\n')
	return buf, nil
}
