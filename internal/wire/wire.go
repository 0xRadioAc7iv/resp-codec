package wire

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// AppendBlobString appends the RESP bulk/blob string encoding of s into buf
// and returns the extended slice. Format: $<len>\r\n<data>\r\n.
func AppendBlobString(buf []byte, s string) []byte {
	return appendBlobData(buf, s, '$')
}

// AppendVerbatimString appends the RESP3 verbatim string encoding of s into buf
// and returns the extended slice. Format: =<len>\r\n<data>\r\n.
// The caller is responsible for the encoding prefix (e.g. "txt:" or "mkd:") in s.
func AppendVerbatimString(buf []byte, s string) []byte {
	return appendBlobData(buf, s, '=')
}

// AppendBlobError appends the RESP3 blob error encoding of s into buf
// and returns the extended slice. Format: !<len>\r\n<data>\r\n.
func AppendBlobError(buf []byte, s string) []byte {
	return appendBlobData(buf, s, '!')
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

// DecodeLineFrame validates and extracts the payload from a line-based frame (+/-).
// Returns the raw payload bytes without the sigil or \r\n. Returns an error if
// the sigil is wrong, the CRLF terminator is absent, or the payload contains CR or LF.
func DecodeLineFrame(buf []byte, sigil byte) ([]byte, error) {
	n := len(buf)
	if n < 3 {
		return nil, fmt.Errorf("line frame too short: need at least 3 bytes, got %d", n)
	}
	if buf[0] != sigil {
		return nil, fmt.Errorf("wrong sigil: expected %q, got %q", sigil, buf[0])
	}
	if buf[n-2] != '\r' || buf[n-1] != '\n' {
		return nil, fmt.Errorf("line frame missing CRLF terminator")
	}
	payload := buf[1 : n-2]
	if bytes.ContainsAny(payload, "\r\n") {
		return nil, fmt.Errorf("line frame payload must not contain CR or LF")
	}
	return payload, nil
}

// DecodeBlobFrame validates and extracts data from a length-prefixed frame ($, !, =).
// Returns the data string. Returns an error if the sigil is wrong, the declared
// length is invalid, or it does not match the actual payload size.
func DecodeBlobFrame(buf []byte, sigil byte) (string, error) {
	n := len(buf)
	if n < 5 {
		return "", fmt.Errorf("blob frame too short: need at least 5 bytes, got %d", n)
	}
	if buf[0] != sigil {
		return "", fmt.Errorf("wrong sigil: expected %q, got %q", sigil, buf[0])
	}
	if buf[n-2] != '\r' || buf[n-1] != '\n' {
		return "", fmt.Errorf("blob frame missing CRLF terminator")
	}
	p := bytes.IndexByte(buf[1:], '\r')
	if p < 0 {
		return "", fmt.Errorf("blob frame missing CRLF after length")
	}
	length, err := strconv.Atoi(string(buf[1 : p+1]))
	if err != nil || length < 0 {
		return "", fmt.Errorf("invalid blob frame length: %q", buf[1:p+1])
	}
	dataStart := p + 3
	if dataStart+length+2 != n {
		return "", fmt.Errorf("blob frame length mismatch: declared %d, frame has %d data bytes", length, n-dataStart-2)
	}
	return string(buf[dataStart : dataStart+length]), nil
}

// appendBlobData writes a length-prefixed RESP frame into buf using firstByte as the sigil.
// Format: <sigil><len>\r\n<data>\r\n. Used by AppendBlobString ($), AppendBlobError (!), and AppendVerbatimString (=).
func appendBlobData(buf []byte, s string, firstByte byte) []byte {
	buf = append(buf, firstByte)
	buf = strconv.AppendInt(buf, int64(len(s)), 10)
	buf = append(buf, '\r', '\n')
	buf = append(buf, s...)
	buf = append(buf, '\r', '\n')
	return buf
}
