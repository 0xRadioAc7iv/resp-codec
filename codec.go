package respcodec

import (
	"bytes"
	"errors"
	"fmt"
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
//	Decode[SimpleString] — expects '+' prefix; 1 alloc ([]byte→string copy)
//	Decode[error]        — expects '-' prefix; 2 allocs (string copy + errors.New)
//	Decode[int]          — expects ':' prefix; zero-alloc, handles negative values
//
// Returns (zero, error) on a missing or wrong type prefix, a buffer that is
// too short, or an invalid character in the data. Panics for unsupported
// type parameters.
//
// All cases validate the "\r\n" terminator at the end of the buffer. SimpleString
// and error additionally reject embedded CR or LF in the payload.
func Decode[T any](buf []byte) (T, error) {
	var t T

	switch any((*T)(nil)).(type) {
	case *SimpleString:
		if len(buf) < 3 {
			return t, fmt.Errorf("buffer too short for SimpleString: need at least 3 bytes, got %d", len(buf))
		}
		if buf[0] != '+' {
			return t, fmt.Errorf("invalid type prefix for SimpleString: expected '+', got %q", buf[0])
		}
		if buf[len(buf)-2] != '\r' || buf[len(buf)-1] != '\n' {
			return t, fmt.Errorf("simple string missing CRLF terminator")
		}

		payload := buf[1 : len(buf)-2]
		if bytes.ContainsAny(payload, "\r\n") {
			return t, fmt.Errorf("simple string must not contain CR or LF characters")
		}
		return any(SimpleString(payload)).(T), nil

	case *error:
		if len(buf) < 3 {
			return t, fmt.Errorf("buffer too short for error: need at least 3 bytes, got %d", len(buf))
		}
		if buf[0] != '-' {
			return t, fmt.Errorf("invalid type prefix for error: expected '-', got %q", buf[0])
		}
		if buf[len(buf)-2] != '\r' || buf[len(buf)-1] != '\n' {
			return t, fmt.Errorf("error string missing CRLF terminator")
		}

		payload := buf[1 : len(buf)-2]
		if bytes.ContainsAny(payload, "\r\n") {
			return t, fmt.Errorf("error string must not contain CR or LF characters")
		}
		return any(errors.New(string(payload))).(T), nil

	case *int:
		if len(buf) < 4 {
			return t, fmt.Errorf("buffer too short for int: need at least 4 bytes, got %d", len(buf))
		}
		if buf[0] != ':' {
			return t, fmt.Errorf("invalid type prefix for int: expected ':', got %q", buf[0])
		}
		if buf[len(buf)-2] != '\r' || buf[len(buf)-1] != '\n' {
			return t, fmt.Errorf("integer frame missing CRLF terminator")
		}

		// isNeg is a sign multiplier: -1 for negative numbers, 1 for positive
		isNeg := 1
		num := 0
		payload := buf[1 : len(buf)-2]

		if len(payload) == 0 {
			return t, fmt.Errorf("integer has no digit characters")
		}
		if payload[0] == '-' {
			isNeg = -1
			payload = payload[1:]
		}
		if len(payload) == 0 {
			return t, fmt.Errorf("integer has no digit characters")
		}

		for _, b := range payload {
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
