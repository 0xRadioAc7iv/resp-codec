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

	default:
		return buf, fmt.Errorf("unsupported type %T: cannot encode to RESP3", data)
	}
}
