// Package respcodec implements encoding for the Redis Serialization Protocol (RESP).
// It supports RESP2 types: simple strings, errors, integers, bulk strings, arrays,
// null bulk strings, and null arrays.
//
// Use Encode to serialize a single value into a fresh buffer. Use AppendEncode to
// write into a caller-supplied buffer, enabling buffer reuse and avoiding extra allocations.
package respcodec

// SimpleString represents a RESP simple string (prefix '+').
// Simple strings are for short status messages like "OK" or "PONG".
// They must not contain CR (\r) or LF (\n) characters.
// Use the plain string type for binary-safe bulk string encoding instead.
type SimpleString string

// nullBulkString is the unexported backing type for the Null sentinel.
type nullBulkString struct{}

// Null is the sentinel value for encoding a RESP null bulk string ($-1\r\n).
// It signals the absence of a value, distinct from an empty string.
var Null = nullBulkString{}

// nullArray is the unexported backing type for the NullArr sentinel.
type nullArray struct{}

// NullArr is the sentinel value for encoding a RESP null array (*-1\r\n).
// It is an alternative null representation used by commands like BLPOP on timeout.
// Prefer Null for general null values; use NullArr only when the protocol specifically requires it.
var NullArr = nullArray{}
