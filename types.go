// Package respcodec implements encoding for the Redis Serialization Protocol (RESP).
// It supports RESP2 types: simple strings, errors, integers, bulk strings, arrays, null bulk strings, and null arrays.
package respcodec

// SimpleString represents a RESP simple string (prefix '+').
// Simple strings are for short status messages like "OK" or "PONG".
// They must not contain CR (\r) or LF (\n) characters.
// Use the plain string type for binary-safe bulk string encoding instead.
type SimpleString string

// NullBulkString represents a RESP null bulk string ($-1\r\n).
// It signals the absence of a value, distinct from an empty string.
// Use the package-level Null sentinel rather than constructing this directly.
type NullBulkString struct{}

// Null is the sentinel value for encoding a RESP null bulk string ($-1\r\n).
var Null = NullBulkString{}

// NullArray represents a RESP null array (*-1\r\n).
// It is an alternative way to signal a null value, used by commands like BLPOP on timeout.
// Prefer NullBulkString (Null) for general null values; use NullArray only when the
// protocol specifically requires it.
// Use the package-level NullArr sentinel rather than constructing this directly.
type NullArray struct{}

// NullArr is the sentinel value for encoding a RESP null array (*-1\r\n).
var NullArr = NullArray{}
