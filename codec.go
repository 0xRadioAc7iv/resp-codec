package respcodec

import (
	"fmt"
	"strconv"
	"strings"
)

func Encode(data any) ([]byte, error) {
	var buf []byte

	switch v := data.(type) {

	// Used for simple strings (cannot have CR, LF or CRLF characters) like "OK", "PONG" etc.
	case SimpleString:
		if strings.ContainsAny(string(v), "\r\n") {
			return nil, fmt.Errorf("simple string must not contain CR or LF characters: %q", string(v))
		}

		buf = make([]byte, 0, 1+2+len(v))
		buf = append(buf, '+')
		buf = append(buf, v...)

	// Used for sending error messages (cannot have  CR, LF or CRLF characters)
	case error:
		msg := v.Error()

		if strings.ContainsAny(msg, "\r\n") {
			return nil, fmt.Errorf("error message must not contain CR or LF characters: %q", msg)
		}

		buf = make([]byte, 0, 1+2+len(msg))
		buf = append(buf, '-')
		buf = append(buf, msg...)

	// Used for numbers
	case int:
		buf = make([]byte, 0, 35)
		buf = append(buf, ':')
		buf = strconv.AppendInt(buf, int64(v), 10)

	// When none of the type matches, return an error
	default:
		return nil, fmt.Errorf("unsupported type %T: cannot encode to RESP", data)
	}

	buf = append(buf, '\r', '\n')
	return buf, nil
}
