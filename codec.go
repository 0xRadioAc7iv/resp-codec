package respcodec

import (
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
// Returns (nil, error) for unsupported types or invalid input.
// For arrays, encoding stops at the first invalid element and returns (nil, error)
// wrapping the element's error with its index.
func Encode(data any) ([]byte, error) {
	var buf []byte

	switch v := data.(type) {

	// Used for simple strings (cannot have CR or LF) like "OK", "PONG" etc.
	case SimpleString:
		if strings.ContainsAny(string(v), "\r\n") {
			return nil, fmt.Errorf("simple string must not contain CR or LF characters: %q", string(v))
		}

		buf = make([]byte, 0, 3+len(v))
		buf = append(buf, '+')
		buf = append(buf, v...)

	// Used for strings (can have CR and LF), Binary Safe
	case string:
		vLength := len(v)

		buf = make([]byte, 0, 6+vLength)
		buf = append(buf, '$')
		buf = strconv.AppendInt(buf, int64(vLength), 10)
		buf = append(buf, '\r', '\n')
		buf = append(buf, v...)

	// Used for sending error messages (cannot have CR or LF characters)
	case error:
		msg := v.Error()

		if strings.ContainsAny(msg, "\r\n") {
			return nil, fmt.Errorf("error message must not contain CR or LF characters: %q", msg)
		}

		buf = make([]byte, 0, 3+len(msg))
		buf = append(buf, '-')
		buf = append(buf, msg...)

	// Used for numbers
	case int:
		buf = make([]byte, 0, 35)
		buf = append(buf, ':')
		buf = strconv.AppendInt(buf, int64(v), 10)

	// Used for arrays; elements are encoded recursively and may be of mixed types
	case []any:
		vLength := len(v)

		buf = make([]byte, 0, 4+vLength*16)
		buf = append(buf, '*')
		buf = strconv.AppendInt(buf, int64(vLength), 10)
		buf = append(buf, '\r', '\n')

		for i, item := range v {
			encodedItem, err := Encode(item)
			if err != nil {
				return nil, fmt.Errorf("failed to encode array element at index %d: %w", i, err)
			}
			buf = append(buf, encodedItem...)
		}

		return buf, nil

	// Used for null bulk string; signals absence of a value ($-1\r\n)
	case NullBulkString:
		return []byte("$-1\r\n"), nil
		// buf = make([]byte, 0, 4)
		// buf = append(buf, []byte("$-1")...)

	// Used for null array; alternative null representation used by commands like BLPOP on timeout (*-1\r\n)
	case NullArray:
		return []byte("*-1\r\n"), nil

	// When none of the type matches, return an error
	default:
		return nil, fmt.Errorf("unsupported type %T: cannot encode to RESP", data)
	}

	buf = append(buf, '\r', '\n')
	return buf, nil
}
