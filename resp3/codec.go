// Package resp3 implements encoding and decoding for the RESP3 protocol
// (https://github.com/redis/redis-specifications/blob/master/protocol/RESP3.md),
// the superset of RESP2 used by Redis 6+ in protover 3 mode.
//
// Type mapping:
//
//	respcodec.SimpleString          ↔ +<value>\r\n
//	string                          ↔ $<len>\r\n<data>\r\n   (blob string)
//	error                           ↔ -<message>\r\n
//	BlobError                       ↔ !<len>\r\n<data>\r\n
//	VerbatimString                  ↔ =<len>\r\n<data>\r\n
//	int                             ↔ :<value>\r\n
//	*big.Int                        ↔ (<value>\r\n             (big number)
//	float64 (and InfValue/NaNValue) ↔ ,<value>\r\n             (double)
//	NullValue                       ↔ _\r\n
//	bool                            ↔ #t\r\n / #f\r\n
//	[]any                           ↔ *<len>\r\n<elements>...
//	map[respcodec.SimpleString]any  ↔ %<pairs>\r\n<key><value>...
//	map[any]struct{}                ↔ ~<len>\r\n<elements>...  (set)
//	AttributeType                   ↔ |<pairs>\r\n<key><value>...
//	Push                            ↔ ><len>\r\n<kind><args>...
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

	// Verbatim String - same as Blob String
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

	// Push — server-to-client out-of-band message. Format: ><count>\r\n<kind><args...>.
	// Kind is always encoded first as a simple string; count includes it.
	case Push:
		savedBuf := buf
		start := len(buf)

		buf = append(buf, '>')
		buf = strconv.AppendInt(buf, int64(len(v.Args)+1), 10)
		buf = append(buf, '\r', '\n')

		var err error
		buf, err = appendEncode(buf, v.Kind)
		if err != nil {
			return savedBuf[:start], fmt.Errorf("failed to encode push kind: %w", err)
		}

		for i, item := range v.Args {
			buf, err = appendEncode(buf, item)
			if err != nil {
				return savedBuf[:start], fmt.Errorf("failed to encode push arg at index %d: %w", i, err)
			}
		}

		return buf, nil

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

// Decode parses a single complete RESP3 frame from buf and returns the decoded Go value.
// The caller must supply exactly one complete frame with no trailing bytes.
//
// Type mapping:
//
//	`+` → respcodec.SimpleString
//	`-` → error
//	`:` → int
//	`$` → string
//	`!` → BlobError
//	`=` → VerbatimString
//	`(` → *big.Int
//	`,` → float64
//	`_` → NullValue
//	`#` → bool
//	`*` → []any
//	`%` → map[respcodec.SimpleString]any
//	`~` → map[any]struct{}
//	`|` → AttributeType
//	`>` → Push
//
// Decode is implemented as a recursive-descent parser over a shared, mutable
// cursor (see parser below): aggregate types never pre-compute how many
// bytes a nested element occupies — they simply ask the parser to decode the
// next value N times and let the cursor track how far each call actually
// advanced. This mirrors the approach used by Redis's own client-side reply
// parser (src/resp_parser.c), and is what correctly handles arbitrary
// nesting depth and binary-safe blob data without scanning for newlines
// through it.
func Decode(buf []byte) (any, error) {
	if len(buf) == 0 {
		return nil, fmt.Errorf("empty buffer")
	}
	p := &parser{buf: buf}
	v, err := p.parseValue()
	if err != nil {
		return nil, err
	}
	if p.pos != len(buf) {
		return nil, fmt.Errorf("trailing data after frame: %d unconsumed byte(s)", len(buf)-p.pos)
	}
	return v, nil
}

// parser walks buf left to right, decoding one RESP3 value at a time via
// parseValue. pos always points at the sigil byte of the next value to
// decode (or at len(buf) when exhausted).
type parser struct {
	buf []byte
	pos int
}

// parseValue decodes the single RESP3 value starting at p.pos and advances
// p.pos past it. Aggregate types call parseValue recursively for each of
// their elements, so the cursor naturally lands in the right place
// regardless of how deeply nested or how long any individual element is.
func (p *parser) parseValue() (any, error) {
	if p.pos >= len(p.buf) {
		return nil, fmt.Errorf("unexpected end of buffer")
	}

	switch p.buf[p.pos] {
	case '+', '-', ':':
		return p.parseLineFrame()
	case '$':
		return p.parseBlobFrame()
	case '!':
		s, err := p.parseBlobFrame()
		if err != nil {
			return nil, err
		}
		return BlobError(s.(string)), nil
	case '=':
		s, err := p.parseBlobFrame()
		if err != nil {
			return nil, err
		}
		return VerbatimString(s.(string)), nil
	case '(':
		return p.parseBigNumber()
	case ',':
		return p.parseDouble()
	case '_':
		return p.parseNull()
	case '#':
		return p.parseBoolean()
	case '*':
		return p.parseArray()
	case '%':
		return p.parseMap()
	case '~':
		return p.parseSet()
	case '|':
		return p.parseAttribute()
	case '>':
		return p.parsePush()
	default:
		return nil, fmt.Errorf("unknown RESP3 type sigil: %q", p.buf[p.pos])
	}
}

// frameEnd returns the end index (exclusive) of the CRLF-terminated frame
// starting at p.pos, i.e. the index just past the first "\r\n" found after
// the sigil byte. It does not validate the frame's content; callers that
// need binary safety (blob types) use blobFrameEnd instead, since scanning
// for '\r' would misinterpret a data byte that happens to be one.
func (p *parser) frameEnd() (int, error) {
	for i := p.pos + 1; i < len(p.buf); i++ {
		if p.buf[i] == '\r' {
			if i+1 >= len(p.buf) || p.buf[i+1] != '\n' {
				return 0, fmt.Errorf("frame missing CRLF terminator")
			}
			return i + 2, nil
		}
	}
	return 0, fmt.Errorf("frame missing CRLF terminator")
}

// parseLineFrame bounds a +/-/: frame via frameEnd, then delegates the
// complete frame (sigil through trailing CRLF) to the root respcodec
// decoder, which performs the actual content validation (e.g. rejecting
// embedded CR/LF in simple strings and errors).
func (p *parser) parseLineFrame() (any, error) {
	end, err := p.frameEnd()
	if err != nil {
		return nil, err
	}
	frame := p.buf[p.pos:end]
	p.pos = end
	return respcodec.Decode(frame)
}

// blobFrameEnd locates the length-prefixed frame ("<sigil><len>\r\n<data>\r\n")
// starting at p.pos. Unlike frameEnd, it jumps directly to where the
// declared length says the data should end, rather than scanning for a
// terminator — required for binary-safe data, which may itself contain
// '\r', '\n', or bytes that look like other sigils. Returns the start and
// end (exclusive) indices of the data payload.
func (p *parser) blobFrameEnd() (dataStart, dataEnd int, err error) {
	i := p.pos + 1
	for i < len(p.buf) && p.buf[i] != '\r' {
		i++
	}
	if i+1 >= len(p.buf) || p.buf[i+1] != '\n' {
		return 0, 0, fmt.Errorf("blob frame missing CRLF after length")
	}
	lengthDigits := p.buf[p.pos+1 : i]
	length, convErr := strconv.Atoi(string(lengthDigits))
	if convErr != nil || length < 0 {
		return 0, 0, fmt.Errorf("invalid blob frame length: %q", lengthDigits)
	}
	dataStart = i + 2
	dataEnd = dataStart + length
	if dataEnd+2 > len(p.buf) {
		return 0, 0, fmt.Errorf("blob frame declared length %d exceeds remaining buffer", length)
	}
	if p.buf[dataEnd] != '\r' || p.buf[dataEnd+1] != '\n' {
		return 0, 0, fmt.Errorf("blob frame missing CRLF terminator after data")
	}
	return dataStart, dataEnd, nil
}

// parseBlobFrame decodes a length-prefixed, binary-safe frame ($, !, or =)
// and returns its data payload as a string.
func (p *parser) parseBlobFrame() (any, error) {
	dataStart, dataEnd, err := p.blobFrameEnd()
	if err != nil {
		return nil, err
	}
	data := p.buf[dataStart:dataEnd]
	p.pos = dataEnd + 2
	return string(data), nil
}

func (p *parser) parseBigNumber() (any, error) {
	end, err := p.frameEnd()
	if err != nil {
		return nil, err
	}
	content := p.buf[p.pos+1 : end-2]
	p.pos = end
	num, ok := big.NewInt(0).SetString(string(content), 10)
	if !ok {
		return nil, fmt.Errorf("invalid big number value: %q", content)
	}
	return num, nil
}

func (p *parser) parseDouble() (any, error) {
	end, err := p.frameEnd()
	if err != nil {
		return nil, err
	}
	content := p.buf[p.pos+1 : end-2]
	p.pos = end
	num, err := strconv.ParseFloat(string(content), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid double value: %q", content)
	}
	return num, nil
}

func (p *parser) parseNull() (any, error) {
	end, err := p.frameEnd()
	if err != nil {
		return nil, err
	}
	content := p.buf[p.pos+1 : end-2]
	p.pos = end
	if len(content) != 0 {
		return nil, fmt.Errorf("invalid null frame: %q", content)
	}
	return Null, nil
}

func (p *parser) parseBoolean() (any, error) {
	end, err := p.frameEnd()
	if err != nil {
		return nil, err
	}
	content := p.buf[p.pos+1 : end-2]
	p.pos = end
	switch string(content) {
	case "t":
		return true, nil
	case "f":
		return false, nil
	default:
		return nil, fmt.Errorf("invalid boolean value: %q", content)
	}
}

// readHeaderCount consumes a "<sigil><digits>\r\n" header at p.pos (used by
// all five aggregate types) and returns the declared count.
func (p *parser) readHeaderCount() (int, error) {
	end, err := p.frameEnd()
	if err != nil {
		return 0, err
	}
	digits := p.buf[p.pos+1 : end-2]
	p.pos = end
	if len(digits) == 0 {
		return 0, fmt.Errorf("missing length digits before CRLF")
	}
	n := 0
	for _, c := range digits {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid character in length: %q", c)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// parseArray decodes a RESP3 array (*<count>\r\n<elements>...) by asking the
// parser for exactly count values in sequence; each recursive parseValue
// call advances the shared cursor by exactly as much as it needed,
// regardless of element type or nesting depth.
func (p *parser) parseArray() (any, error) {
	count, err := p.readHeaderCount()
	if err != nil {
		return nil, err
	}
	result := make([]any, 0, count)
	for i := range count {
		v, err := p.parseValue()
		if err != nil {
			return nil, fmt.Errorf("array element %d: %w", i, err)
		}
		result = append(result, v)
	}
	return result, nil
}

func (p *parser) parseMap() (any, error) {
	pairCount, err := p.readHeaderCount()
	if err != nil {
		return nil, err
	}
	result := make(map[respcodec.SimpleString]any, pairCount)
	for i := range pairCount {
		k, err := p.parseValue()
		if err != nil {
			return nil, fmt.Errorf("map key %d: %w", i, err)
		}
		key, ok := k.(respcodec.SimpleString)
		if !ok {
			return nil, fmt.Errorf("invalid map key at pair %d: expected SimpleString, got %T", i, k)
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("duplicate map key at pair %d: %q", i, key)
		}
		v, err := p.parseValue()
		if err != nil {
			return nil, fmt.Errorf("map value %d: %w", i, err)
		}
		result[key] = v
	}
	return result, nil
}

func (p *parser) parseSet() (any, error) {
	count, err := p.readHeaderCount()
	if err != nil {
		return nil, err
	}
	result := make(map[any]struct{}, count)
	for i := range count {
		v, err := p.parseValue()
		if err != nil {
			return nil, fmt.Errorf("set element %d: %w", i, err)
		}
		result[v] = struct{}{}
	}
	return result, nil
}

func (p *parser) parseAttribute() (any, error) {
	pairCount, err := p.readHeaderCount()
	if err != nil {
		return nil, err
	}
	result := make(AttributeType, pairCount)
	for i := range pairCount {
		k, err := p.parseValue()
		if err != nil {
			return nil, fmt.Errorf("attribute key %d: %w", i, err)
		}
		key, ok := k.(respcodec.SimpleString)
		if !ok {
			return nil, fmt.Errorf("invalid attribute key at pair %d: expected SimpleString, got %T", i, k)
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("duplicate attribute key at pair %d: %q", i, key)
		}
		v, err := p.parseValue()
		if err != nil {
			return nil, fmt.Errorf("attribute value %d: %w", i, err)
		}
		result[key] = v
	}
	return result, nil
}

func (p *parser) parsePush() (any, error) {
	count, err := p.readHeaderCount()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, fmt.Errorf("invalid push frame: expected at least one element for kind")
	}
	kindVal, err := p.parseValue()
	if err != nil {
		return nil, fmt.Errorf("push kind: %w", err)
	}
	kind, ok := kindVal.(respcodec.SimpleString)
	if !ok {
		return nil, fmt.Errorf("invalid push kind: expected SimpleString, got %T", kindVal)
	}
	var args []any
	if count > 1 {
		args = make([]any, 0, count-1)
		for i := 1; i < count; i++ {
			v, err := p.parseValue()
			if err != nil {
				return nil, fmt.Errorf("push arg %d: %w", i-1, err)
			}
			args = append(args, v)
		}
	}
	return Push{Kind: kind, Args: args}, nil
}
