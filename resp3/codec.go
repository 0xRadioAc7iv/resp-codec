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
func Decode(buf []byte) (any, error) {
	if len(buf) == 0 {
		return nil, fmt.Errorf("empty buffer")
	}

	switch buf[0] {
	case '+', '-', ':', '$':
		return respcodec.Decode(buf)

	case '!':
		s, err := wire.DecodeBlobFrame(buf, '!')
		if err != nil {
			return nil, err
		}
		return BlobError(s), nil

	case '=':
		s, err := wire.DecodeBlobFrame(buf, '=')
		if err != nil {
			return nil, err
		}
		return VerbatimString(s), nil

	case '(':
		bufLen := len(buf)
		if bufLen < 4 || buf[bufLen-2] != '\r' || buf[bufLen-1] != '\n' {
			return nil, fmt.Errorf("invalid big number frame: %q", buf)
		}
		numBytes := buf[1 : bufLen-2]
		num, ok := big.NewInt(0).SetString(string(numBytes), 10)
		if !ok {
			return nil, fmt.Errorf("invalid big number value: %q", numBytes)
		}
		return num, nil

	case ',':
		bufLen := len(buf)
		if bufLen < 4 || buf[bufLen-2] != '\r' || buf[bufLen-1] != '\n' {
			return nil, fmt.Errorf("invalid big number frame: %q", buf)
		}
		numBytes := buf[1 : bufLen-2]
		num, err := strconv.ParseFloat(string(numBytes), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid double number value: %q", numBytes)
		}
		return num, nil

	case '_':
		if len(buf) != 3 || buf[1] != '\r' || buf[2] != '\n' {
			return nil, fmt.Errorf("invalid null frame: %q", buf)
		}
		return Null, nil

	case '#':
		if len(buf) != 4 || buf[2] != '\r' || buf[3] != '\n' {
			return nil, fmt.Errorf("invalid boolean frame: %q", buf)
		}
		switch buf[1] {
		case 't':
			return true, nil
		case 'f':
			return false, nil
		default:
			return nil, fmt.Errorf("invalid boolean value: %q", buf[1])
		}

	case '*':
		bufLen := len(buf)
		if bufLen < 4 {
			return []any{}, fmt.Errorf("buffer too short for array: need at least 4 bytes, got %d", bufLen)
		}
		arr, err := decodeArray(buf)
		if err != nil {
			return nil, err
		}
		return arr, nil

	case '%':
		bufLen := len(buf)
		if bufLen < 4 {
			return []any{}, fmt.Errorf("buffer too short for array: need at least 4 bytes, got %d", bufLen)
		}
		arr, err := decodeArray(buf)
		if err != nil {
			return nil, err
		}
		if len(arr)%2 == 1 {
			return nil, fmt.Errorf("invalid map: expected an even number of elements, got %d", len(arr))
		}
		mapObject := make(map[respcodec.SimpleString]any, len(arr)/2)
		for i := 0; i < len(arr)-1; {
			ele, ok := arr[i].(respcodec.SimpleString)
			if !ok {
				return nil, fmt.Errorf("invalid map key at index %d: expected SimpleString, got %T", i, arr[i])
			}
			mapObject[ele] = arr[i+1]
			i += 2
		}
		return mapObject, nil

	case '~':
		bufLen := len(buf)
		if bufLen < 4 {
			return []any{}, fmt.Errorf("buffer too short for array: need at least 4 bytes, got %d", bufLen)
		}
		arr, err := decodeArray(buf)
		if err != nil {
			return nil, err
		}
		set := make(map[any]struct{}, len(arr))
		for _, v := range arr {
			set[v] = struct{}{}
		}
		return set, nil

	case '|':
		bufLen := len(buf)
		if bufLen < 4 {
			return []any{}, fmt.Errorf("buffer too short for array: need at least 4 bytes, got %d", bufLen)
		}
		arr, err := decodeArray(buf)
		if err != nil {
			return nil, err
		}
		if len(arr)%2 == 1 {
			return nil, fmt.Errorf("invalid attribute: expected an even number of elements, got %d", len(arr))
		}
		attributes := make(AttributeType, len(arr)/2)
		for i := 0; i < len(arr)-1; {
			ele, ok := arr[i].(respcodec.SimpleString)
			if !ok {
				return nil, fmt.Errorf("invalid attribute key at index %d: expected SimpleString, got %T", i, arr[i])
			}
			attributes[ele] = arr[i+1]
			i += 2
		}
		return attributes, nil

	case '>':
		bufLen := len(buf)
		if bufLen < 4 {
			return []any{}, fmt.Errorf("buffer too short for array: need at least 4 bytes, got %d", bufLen)
		}
		arr, err := decodeArray(buf)
		if err != nil {
			return nil, err
		}
		if len(arr) == 0 {
			return nil, fmt.Errorf("invalid push frame: expected at least one element for kind")
		}
		kind, ok := arr[0].(respcodec.SimpleString)
		if !ok {
			return nil, fmt.Errorf("invalid push kind: expected SimpleString, got %T", arr[0])
		}
		if len(arr) == 1 {
			return Push{Kind: kind, Args: nil}, nil
		}
		return Push{Kind: kind, Args: arr[1:]}, nil

	default:
		return nil, fmt.Errorf("unknown RESP3 type sigil: %q", buf[0])
	}
}

// decodeArray parses a RESP3 array-like frame (*<count>\r\n<elements>...), recursing
// into nested arrays, sets, and pushes. It is also used to decode the underlying
// array structure of Map (%) and Attribute (|) frames, which the caller then
// reinterprets as key/value pairs.
func decodeArray(buf []byte) ([]any, error) {
	size, numLength, err := getSizeAndNumberLength(buf[1:])
	if err != nil {
		return nil, err
	}
	if buf[numLength+1] != '\r' || buf[numLength+2] != '\n' {
		return nil, fmt.Errorf("invalid array frame: missing CRLF after length: %q", buf)
	}
	if size == 0 {
		return []any{}, nil
	}

	array := make([]any, 0)
	payload := buf[numLength+3:]
	maxIdx := len(buf) - numLength - 3
	pos := 0

	for pos < maxIdx {
		fIdx := pos
		lIdx := pos

		// checks for whether the element of the array is of a particular type
		IsaKindOfBlobString := payload[pos] == '=' || payload[pos] == '$' || payload[pos] == '!'
		isArray := payload[pos] == '*'
		isMap := payload[pos] == '%'
		isSet := payload[pos] == '~'
		isAttribute := payload[pos] == '|'
		isPush := payload[pos] == '>'

		if IsaKindOfBlobString {
			blobStringSize, numLen, err := getSizeAndNumberLength(payload[pos+1:])
			if err != nil {
				return nil, err
			}
			itemBytes := payload[pos : pos+numLen+blobStringSize+5]
			blobString, err := Decode(itemBytes)
			if err != nil {
				return nil, err
			}
			pos = pos + numLen + blobStringSize + 2
			array = append(array, blobString)
		} else if isArray {
			arrayLen, headerNumLen, err := getSizeAndNumberLength(payload[pos+1:])
			if err != nil {
				return nil, err
			}

			// Skip past this array's own header line ("*<digits>\r\n") before
			// counting data-terminating newlines: the header's own \n is not
			// a data terminator.
			lIdx = pos + 1 + headerNumLen + 2
			needed := arrayLen
			for lIdx < maxIdx && needed > 0 {
				if payload[lIdx] == '\n' {
					needed--
				}
				lIdx++
			}

			itemBytes := payload[pos:lIdx]
			nestedArr, err := decodeArray(itemBytes)
			if err != nil {
				return nil, err
			}
			pos = lIdx
			array = append(array, nestedArr)
		} else if isMap {
			pairCount, headerNumLen, err := getSizeAndNumberLength(payload[pos+1:])
			if err != nil {
				return nil, err
			}

			// Skip past this map's own header line ("%<digits>\r\n") before
			// counting data-terminating newlines: the header's own \n is not
			// a data terminator. Each declared pair is 2 raw values (key+value).
			lIdx = pos + 1 + headerNumLen + 2
			needed := pairCount * 2
			for lIdx < maxIdx && needed > 0 {
				if payload[lIdx] == '\n' {
					needed--
				}
				lIdx++
			}

			itemBytes := payload[pos:lIdx]
			arr, err := decodeArray(itemBytes)
			if err != nil {
				return nil, err
			}

			mapObject := make(map[respcodec.SimpleString]any, len(arr)/2)
			for i := 0; i < len(arr)-1; i += 2 {
				ele, ok := arr[i].(respcodec.SimpleString)
				if !ok {
					return nil, fmt.Errorf("invalid map key at index %d: expected SimpleString, got %T", i, arr[i])
				}
				mapObject[ele] = arr[i+1]
			}
			if len(arr)/2 != len(mapObject) {
				return nil, fmt.Errorf("array element count mismatch: declared %d, got %d", len(arr)/2, len(mapObject))
			}
			pos = lIdx
			array = append(array, mapObject)
		} else if isSet {
			arrayLen, headerNumLen, err := getSizeAndNumberLength(payload[pos+1:])
			if err != nil {
				return nil, err
			}

			// Skip past this set's own header line ("~<digits>\r\n") before
			// counting data-terminating newlines: the header's own \n is not
			// a data terminator.
			lIdx = pos + 1 + headerNumLen + 2
			needed := arrayLen
			for lIdx < maxIdx && needed > 0 {
				if payload[lIdx] == '\n' {
					needed--
				}
				lIdx++
			}

			itemBytes := payload[pos:lIdx]
			nestedArr, err := decodeArray(itemBytes)
			if err != nil {
				return nil, err
			}
			set := make(map[any]struct{}, len(nestedArr))
			for _, v := range nestedArr {
				set[v] = struct{}{}
			}
			pos = lIdx
			array = append(array, set)
		} else if isAttribute {
			pairCount, headerNumLen, err := getSizeAndNumberLength(payload[pos+1:])
			if err != nil {
				return nil, err
			}

			// Skip past this attribute's own header line ("|<digits>\r\n")
			// before counting data-terminating newlines: the header's own \n
			// is not a data terminator. Each declared pair is 2 raw values
			// (key+value).
			lIdx = pos + 1 + headerNumLen + 2
			needed := pairCount * 2
			for lIdx < maxIdx && needed > 0 {
				if payload[lIdx] == '\n' {
					needed--
				}
				lIdx++
			}

			itemBytes := payload[pos:lIdx]
			arr, err := decodeArray(itemBytes)
			if err != nil {
				return nil, err
			}

			attributes := make(AttributeType, len(arr)/2)
			for i := 0; i < len(arr)-1; i += 2 {
				ele, ok := arr[i].(respcodec.SimpleString)
				if !ok {
					return nil, fmt.Errorf("invalid attribute key at index %d: expected SimpleString, got %T", i, arr[i])
				}
				attributes[ele] = arr[i+1]
			}
			if len(arr)/2 != len(attributes) {
				return nil, fmt.Errorf("array element count mismatch: declared %d, got %d", len(arr)/2, len(attributes))
			}
			pos = lIdx
			array = append(array, attributes)
		} else if isPush {
			arrayLen, headerNumLen, err := getSizeAndNumberLength(payload[pos+1:])
			if err != nil {
				return nil, err
			}
			size := arrayLen

			// Skip past this push's own header line (">digits\r\n") before
			// counting data-terminating newlines: the header's own \n is not
			// a data terminator.
			lIdx = pos + 1 + headerNumLen + 2
			needed := arrayLen
			for lIdx < maxIdx && needed > 0 {
				if payload[lIdx] == '\n' {
					needed--
				}
				lIdx++
			}

			itemBytes := payload[pos:lIdx]
			nestedArr, err := decodeArray(itemBytes)
			if err != nil {
				return nil, err
			}
			if len(nestedArr) == 0 {
				return nil, fmt.Errorf("invalid push frame: expected at least one element for kind")
			}
			if len(nestedArr) != size {
				return []any{}, fmt.Errorf("array element count mismatch: declared %d, got %d", size, len(nestedArr))
			}
			kind, ok := nestedArr[0].(respcodec.SimpleString)
			if !ok {
				return nil, fmt.Errorf("invalid push kind: expected SimpleString, got %T", nestedArr[0])
			}
			pos = lIdx
			if len(nestedArr) == 1 {
				array = append(array, Push{Kind: kind, Args: nil})
				continue
			}
			array = append(array, Push{Kind: kind, Args: nestedArr[1:]})
		} else {
			for lIdx < maxIdx {
				if payload[lIdx] == '\n' {
					lIdx++
					break
				}
				lIdx++
			}

			switch payload[pos] {
			case '+':
				s, _ := Decode(payload[fIdx:lIdx])
				array = append(array, s)
			case '-':
				sErr, err := Decode(payload[fIdx:lIdx])
				if err != nil {
					return nil, err
				}
				array = append(array, sErr)
			case ':':
				integer, err := Decode(payload[fIdx:lIdx])
				if err != nil {
					return nil, err
				}
				array = append(array, integer)
			case '(':
				bigNum, err := Decode(payload[fIdx:lIdx])
				if err != nil {
					return nil, err
				}
				array = append(array, bigNum)
			case ',':
				double, err := Decode(payload[fIdx:lIdx])
				if err != nil {
					return nil, err
				}
				array = append(array, double)
			case '_':
				null, err := Decode(payload[fIdx:lIdx])
				if err != nil {
					return nil, err
				}
				array = append(array, null)
			case '#':
				boolean, err := Decode(payload[fIdx:lIdx])
				if err != nil {
					return nil, err
				}
				array = append(array, boolean)
			}
			pos = lIdx
		}
	}

	switch buf[0] {
	case '*', '~':
		// fmt.Println("buf", string(buf), "array len", len(array), "size", size, "array", array)
		if len(array) != size {
			return nil, fmt.Errorf("array element count mismatch: declared %d, got %d", size, len(array))
		}
	case '%', '|':
		if len(array)/2 != size {
			return nil, fmt.Errorf("array element count mismatch: declared %d, got %d", size, len(array))
		}
	}
	return array, nil
}

func getSizeAndNumberLength(buf []byte) (size, numLength int, err error) {
	for _, v := range buf {
		if v == '\r' {
			break
		}
		if v < '0' || v > '9' {
			return 0, 0, fmt.Errorf("invalid character in bulk string length: %q", v)
		}
		size = (size * 10) + int(v-'0')
		numLength++
	}
	if numLength == 0 {
		return 0, 0, fmt.Errorf("invalid frame: missing length digits before CRLF")
	}
	return size, numLength, nil
}
