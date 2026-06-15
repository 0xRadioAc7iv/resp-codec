package resp3

// NullValue is the backing type for the Null sentinel.
// RESP3 uses a single unified null (_\r\n) replacing RESP2's $-1\r\n and *-1\r\n.
type NullValue struct{}

// Null is the sentinel value for encoding a RESP3 null (_\r\n).
var Null = NullValue{}

// BlobError is a binary-safe error type for RESP3.
// Unlike SimpleError, it can contain CR and LF bytes.
// Wire format: !<len>\r\n<data>\r\n.
type BlobError string

// VerbatimString is a RESP3 verbatim string. It is binary-safe like BlobString
// but uses the = sigil to signal to clients that the encoding type is embedded in
// the payload. The caller is responsible for including the encoding prefix
// (e.g. "txt:hello" or "mkd:**bold**"). Wire format: =<len>\r\n<data>\r\n.
type VerbatimString string

// InfValue is the backing type for the Inf sentinel.
// Encodes as ,inf\r\n.
type InfValue struct{}

// NegInfValue is the backing type for the NegInf sentinel.
// Encodes as ,-inf\r\n.
type NegInfValue struct{}

// NaNValue is the backing type for the NaN sentinel.
// Encodes as ,nan\r\n.
type NaNValue struct{}

// Inf encodes a RESP3 double positive infinity (,inf\r\n).
var Inf = InfValue{}

// NegInf encodes a RESP3 double negative infinity (,-inf\r\n).
var NegInf = NegInfValue{}

// NaN encodes a RESP3 double not-a-number (,nan\r\n).
var NaN = NaNValue{}
