package resp3

import (
	"fmt"
	"math/big"
	"strconv"

	respcodec "github.com/0xRadioAc7iv/resp-codec"
	"github.com/0xRadioAc7iv/resp-codec/internal/wire"
)

// Encode serializes a Go value into its RESP3 byte representation.
func Encode(data any) ([]byte, error) {
	buf, err := appendEncode(make([]byte, 0, 64), data)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// AppendEncode appends the RESP3 encoding of data into buf and returns the extended slice.
func AppendEncode(buf []byte, data any) ([]byte, error) {
	return appendEncode(buf, data)
}

func appendEncode(buf []byte, data any) ([]byte, error) {
	switch v := data.(type) {

	// Simple string — cannot contain CR or LF
	case respcodec.SimpleString:
		return wire.AppendSimpleString(buf, string(v))

	// Verbatim String - same as Blog String
	case VerbatimString:
		return wire.AppendVerbatimString(buf, string(v)), nil

	// Blob string (RESP3 name for bulk string) — binary safe
	case string:
		return wire.AppendBlobString(buf, v), nil

	// Simple error — cannot contain CR or LF
	case error:
		return wire.AppendSimpleError(buf, v.Error())

	// Blob error — binary-safe error, unlike simple error allows CR/LF
	case BlobError:
		return wire.AppendBlobError(buf, string(v)), nil

	// Integer
	case int:
		return wire.AppendInteger(buf, v), nil

	// Big number — arbitrary-precision integer. Format: (<decimal>\r\n.
	case *big.Int:
		buf = append(buf, '(')
		buf = append(buf, v.String()...)
		buf = append(buf, '\r', '\n')
		return buf, nil

	// Double — float32 is promoted to float64 before encoding. Format: ,<value>\r\n.
	case float32, float64:
		var f float64
		if f32, ok := data.(float32); ok {
			f = float64(f32)
		} else {
			f = data.(float64)
		}
		buf = append(buf, ',')
		buf = strconv.AppendFloat(buf, f, 'g', -1, 64)
		buf = append(buf, '\r', '\n')
		return buf, nil

	// Double special values — use sentinel types instead of math.Inf / math.NaN
	// to avoid ambiguity with regular float64 values.
	case InfValue:
		return append(buf, ',', 'i', 'n', 'f', '\r', '\n'), nil
	case NegInfValue:
		return append(buf, ',', '-', 'i', 'n', 'f', '\r', '\n'), nil
	case NaNValue:
		return append(buf, ',', 'n', 'a', 'n', '\r', '\n'), nil

	// Null — unified null replacing RESP2's $-1 and *-1 variants
	case NullValue:
		buf = append(buf, '_', '\r', '\n')
		return buf, nil

	// Boolean
	case bool:
		buf = append(buf, '#')
		if v {
			buf = append(buf, 't')
		} else {
			buf = append(buf, 'f')
		}
		buf = append(buf, '\r', '\n')
		return buf, nil

	// Array — ordered sequence of heterogeneous RESP3 values. Format: *<count>\r\n<elements>...
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

	// Map — key-value pairs with SimpleString keys. Format: %<pairs>\r\n<key><value>...
	case map[respcodec.SimpleString]any:
		return appendMapData(buf, v, '%')

	// Set — unordered collection of unique RESP3 values. Format: ~<count>\r\n<elements>...
	case map[any]struct{}:
		savedBuf := buf
		start := len(buf)

		buf = append(buf, '~')
		buf = strconv.AppendInt(buf, int64(len(v)), 10)
		buf = append(buf, '\r', '\n')

		for item := range v {
			var err error
			buf, err = appendEncode(buf, item)
			if err != nil {
				return savedBuf[:start], fmt.Errorf("failed to encode set item %q: %w", item, err)
			}
		}

		return buf, nil

	// Attribute — out-of-band metadata map preceding a reply. Same structure as Map but sigil |.
	case AttributeType:
		return appendMapData(buf, map[respcodec.SimpleString]any(v), '|')

	default:
		return buf, fmt.Errorf("unsupported type %T: cannot encode to RESP3", data)
	}
}

// appendMapData encodes a SimpleString-keyed map into buf using sigil as the
// first byte. Used by Map (%) and Attribute (|).
func appendMapData(buf []byte, m map[respcodec.SimpleString]any, sigil byte) ([]byte, error) {
	savedBuf := buf
	start := len(buf)

	buf = append(buf, sigil)
	buf = strconv.AppendInt(buf, int64(len(m)), 10)
	buf = append(buf, '\r', '\n')

	for k, v := range m {
		var err error
		buf, err = appendEncode(buf, k)
		if err != nil {
			return savedBuf[:start], fmt.Errorf("failed to encode map key %q: %w", k, err)
		}
		buf, err = appendEncode(buf, v)
		if err != nil {
			return savedBuf[:start], fmt.Errorf("failed to encode map value %q: %w", v, err)
		}
	}

	return buf, nil
}
