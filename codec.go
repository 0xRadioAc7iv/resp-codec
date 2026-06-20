package respcodec

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/0xRadioAc7iv/resp-codec/internal/wire"
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

// appendEncode appends the RESP encoding of data into buf and returns the extended slice.
// It is the shared append-style core used by Encode and AppendEncode, and called recursively for arrays.
func appendEncode(buf []byte, data any) ([]byte, error) {
	switch v := data.(type) {

	// Used for simple strings (cannot have CR or LF) like "OK", "PONG" etc.
	case SimpleString:
		return wire.AppendSimpleString(buf, string(v))

	// Used for strings (can have CR and LF), Binary Safe
	case string:
		return wire.AppendBlobString(buf, v), nil

	// Used for sending error messages (cannot have CR or LF characters)
	case error:
		return wire.AppendSimpleError(buf, v.Error())

	// Used for numbers
	case int:
		return wire.AppendInteger(buf, v), nil

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

// Decode parses a single complete RESP frame from buf and returns the decoded Go value.
// The caller must supply exactly one complete frame with no trailing bytes.
//
// Type mapping:
//
//	`+` → SimpleString
//	`-` → error
//	`:` → int
//	`$` → string (nil for the null bulk string, "$-1\r\n")
//	`*` → []any (nil for the null array, "*-1\r\n")
func Decode(buf []byte) (any, error) {
	if len(buf) == 0 {
		return nil, fmt.Errorf("empty buffer")
	}

	switch buf[0] {
	case '+':
		return decodeSimpleString(buf)

	case '-':
		s, err := decodeErrorString(buf)
		if err != nil {
			return nil, err
		}
		return errors.New(s), nil

	case ':':
		return decodeInteger(buf)

	case '$':
		if len(buf) > 1 && buf[1] == '-' {
			if err := decodeNullBulkString(buf); err != nil {
				return nil, err
			}
			return nil, nil
		}
		return decodeBulkString(buf)

	case '*':
		if len(buf) > 1 && buf[1] == '-' {
			if err := decodeNullArray(buf); err != nil {
				return nil, err
			}
			return nil, nil
		}
		return decodeArray(buf)

	default:
		return nil, fmt.Errorf("unknown RESP type sigil: %q", buf[0])
	}
}

// decodeSimpleString parses a RESP simple string frame ('+' prefix) from buf and
// returns the payload as a SimpleString. Returns an error if the prefix is wrong,
// the frame is too short, the CRLF terminator is missing, or the payload contains
// CR or LF.
func decodeSimpleString(buf []byte) (SimpleString, error) {
	payload, err := wire.DecodeLineFrame(buf, '+')
	if err != nil {
		return "", err
	}
	return SimpleString(payload), nil
}

// decodeErrorString parses a RESP error frame ('-' prefix) from buf and returns
// the error message text as a plain string. Wrap the result with errors.New if
// you need an error value. Returns an error if the prefix is wrong, the frame is
// too short, the CRLF terminator is missing, or the payload contains CR or LF.
func decodeErrorString(buf []byte) (string, error) {
	payload, err := wire.DecodeLineFrame(buf, '-')
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

// decodeInteger parses a RESP integer frame (':' prefix) from buf and returns the
// value as an int. Handles negative values. Returns an error if the prefix is
// wrong, the frame is too short, the CRLF terminator is missing, or the payload
// contains non-digit characters.
func decodeInteger(buf []byte) (int, error) {
	bufLen := len(buf)
	if bufLen < 4 {
		return 0, fmt.Errorf("buffer too short for int: need at least 4 bytes, got %d", bufLen)
	}
	if buf[0] != ':' {
		return 0, fmt.Errorf("invalid type prefix for int: expected ':', got %q", buf[0])
	}
	if buf[bufLen-2] != '\r' || buf[bufLen-1] != '\n' {
		return 0, fmt.Errorf("integer frame missing CRLF terminator")
	}

	// isNeg is a sign multiplier: -1 for negative numbers, 1 for positive
	isNeg := 1
	num := 0
	payload := buf[1 : bufLen-2]

	if payload[0] == '-' {
		isNeg = -1
		payload = payload[1:]
	}
	if len(payload) == 0 {
		return 0, fmt.Errorf("integer has no digit characters")
	}

	for _, b := range payload {
		if b < '0' || b > '9' {
			return 0, fmt.Errorf("invalid character in int: %q", b)
		}
		num = (num * 10) + int(b-'0')
	}
	return isNeg * num, nil
}

// decodeBulkString parses a RESP bulk string frame ('$' prefix) from buf and
// returns the payload as a string. Returns an error if the prefix is wrong, the
// frame is too short, the CRLF terminator is missing, or the declared length does
// not match the actual payload.
func decodeBulkString(buf []byte) (string, error) {
	return wire.DecodeBlobFrame(buf, '$')
}

// decodeNullBulkString validates that buf is exactly the null bulk string frame
// ("$-1\r\n"). Returns an error if the length or contents do not match.
func decodeNullBulkString(buf []byte) error {
	bufLen := len(buf)
	if bufLen != 5 {
		return fmt.Errorf("invalid null bulk string: expected exactly 5 bytes, got %d", bufLen)
	}
	if buf[0] != '$' || buf[1] != '-' || buf[2] != '1' || buf[3] != '\r' || buf[4] != '\n' {
		return fmt.Errorf("invalid null bulk string: expected \"$-1\\r\\n\", got %q", buf)
	}
	return nil
}

// decodeArray parses a RESP array frame ('*' prefix) from buf and returns its
// elements as []any. Elements are decoded recursively and may be of mixed types;
// null elements ($-1\r\n, *-1\r\n) are decoded as nil. Returns an error if the
// prefix is wrong, the count is invalid, or any element fails to decode.
func decodeArray(buf []byte) ([]any, error) {
	bufLen := len(buf)
	if bufLen < 4 {
		return []any{}, fmt.Errorf("buffer too short for array: need at least 4 bytes, got %d", bufLen)
	}
	if buf[0] != '*' {
		return []any{}, fmt.Errorf("invalid type prefix for array: expected '*', got %q", buf[0])
	}

	digits, size, err := calculateDigitsAndSize(buf[1:])
	if err != nil {
		return []any{}, err
	}
	if size == 0 {
		return []any{}, nil
	}

	array := make([]any, 0, size)
	payload := buf[digits+3:]
	pos := 0
	maxIdx := bufLen - digits - 3

	for pos < maxIdx {
		fIdx := pos
		lIdx := pos
		arrayLen := 0

		// the second conditions excludes null bulk strings and null arrays
		isBulkString := payload[pos] == '$' && payload[pos+1] != '-'
		isArray := payload[pos] == '*' && payload[pos+1] != '-'

		if isArray {
			_, arrayLen, err = calculateDigitsAndSize(payload[1:])
			if err != nil {
				return []any{}, err
			}
		}

		// NOTE
		// skips the first '\n' if '$' is detected for bulk strings
		// skips the first n '\n' if '*' is detected for arrays
		for lIdx < maxIdx {
			if payload[lIdx] == '\n' {
				if isArray {
					if arrayLen == 0 {
						break
					} else {
						lIdx++
						arrayLen--
						continue
					}
				}
				if isBulkString {
					isBulkString = !isBulkString
				} else {
					break
				}
			}
			lIdx++
		}

		lastIndex := lIdx + 1
		if isArray {
			lastIndex--
		}

		itemBytes := payload[fIdx:lastIndex]

		switch itemBytes[0] {
		case '+':
			s, err := decodeSimpleString(itemBytes)
			if err != nil {
				return []any{}, err
			}
			array = append(array, s)
		case '-':
			errString, err := decodeErrorString(itemBytes)
			if err != nil {
				return []any{}, err
			}
			array = append(array, errors.New(errString))
		case ':':
			integer, err := decodeInteger(itemBytes)
			if err != nil {
				return []any{}, err
			}
			array = append(array, integer)
		case '$':
			if itemBytes[1] == '-' {
				err := decodeNullBulkString(itemBytes)
				if err != nil {
					return []any{}, err
				}
				array = append(array, nil)
			} else {
				bulkString, err := decodeBulkString(itemBytes)
				if err != nil {
					return []any{}, err
				}
				array = append(array, bulkString)
			}
		case '*':
			if itemBytes[1] == '-' {
				err := decodeNullArray(itemBytes)
				if err != nil {
					return []any{}, err
				}
				array = append(array, nil)
			} else {
				arr, err := decodeArray(itemBytes)
				if err != nil {
					return []any{}, err
				}
				array = append(array, arr)
			}
		}
		pos = lIdx + 1
	}

	if len(array) != size {
		return []any{}, fmt.Errorf("array element count mismatch: declared %d, got %d", size, len(array))
	}

	return array, nil
}

// decodeNullArray validates that buf is exactly the null array frame ("*-1\r\n").
// Returns an error if the length or contents do not match.
func decodeNullArray(buf []byte) error {
	bufLen := len(buf)
	if bufLen != 5 {
		return fmt.Errorf("invalid null array: expected exactly 5 bytes, got %d", bufLen)
	}
	if buf[0] != '*' || buf[1] != '-' || buf[2] != '1' || buf[3] != '\r' || buf[4] != '\n' {
		return fmt.Errorf("invalid null array: expected \"*-1\\r\\n\", got %q", buf)
	}
	return nil
}

func calculateDigitsAndSize(buf []byte) (digits, size int, err error) {
	for _, v := range buf {
		if v == '\r' {
			break
		}
		if v < '0' || v > '9' {
			return 0, 0, fmt.Errorf("invalid character in bulk string length: %q", v)
		}
		size = (size * 10) + int(v-'0')
		digits++
	}
	return digits, size, nil
}
