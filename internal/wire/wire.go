package wire

import (
	"fmt"
	"strconv"
	"strings"
)

// AppendBlobString appends the RESP bulk/blob string encoding of s into buf
// and returns the extended slice. Format: $<len>\r\n<data>\r\n.
func AppendBlobString(buf []byte, s string) []byte {
	buf = append(buf, '$')
	buf = strconv.AppendInt(buf, int64(len(s)), 10)
	buf = append(buf, '\r', '\n')
	buf = append(buf, s...)
	buf = append(buf, '\r', '\n')
	return buf
}

// AppendInteger appends the RESP integer encoding of n into buf
// and returns the extended slice. Format: :<n>\r\n.
func AppendInteger(buf []byte, n int) []byte {
	buf = append(buf, ':')
	buf = strconv.AppendInt(buf, int64(n), 10)
	buf = append(buf, '\r', '\n')
	return buf
}

// AppendSimpleString appends the RESP simple string encoding of s into buf
// and returns the extended slice. Format: +<s>\r\n.
// Returns an error if s contains CR or LF.
func AppendSimpleString(buf []byte, s string) ([]byte, error) {
	if strings.ContainsAny(s, "\r\n") {
		return buf, fmt.Errorf("simple string must not contain CR or LF characters: %q", s)
	}
	buf = append(buf, '+')
	buf = append(buf, s...)
	buf = append(buf, '\r', '\n')
	return buf, nil
}

// AppendSimpleError appends the RESP simple error encoding of msg into buf
// and returns the extended slice. Format: -<msg>\r\n.
// Returns an error if msg contains CR or LF.
func AppendSimpleError(buf []byte, msg string) ([]byte, error) {
	if strings.ContainsAny(msg, "\r\n") {
		return buf, fmt.Errorf("simple error must not contain CR or LF characters: %q", msg)
	}
	buf = append(buf, '-')
	buf = append(buf, msg...)
	buf = append(buf, '\r', '\n')
	return buf, nil
}
