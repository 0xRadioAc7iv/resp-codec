package resp3

import (
	"fmt"

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

	// Blob string (RESP3 name for bulk string) — binary safe
	case string:
		return wire.AppendBlobString(buf, v), nil

	// Simple error — cannot contain CR or LF
	case error:
		return wire.AppendSimpleError(buf, v.Error())

	// Integer
	case int:
		return wire.AppendInteger(buf, v), nil

	default:
		return buf, fmt.Errorf("unsupported type %T: cannot encode to RESP3", data)
	}
}
