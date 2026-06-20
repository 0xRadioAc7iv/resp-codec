# resp-codec

A Go library for encoding and decoding the [Redis Serialization Protocol (RESP2)](https://redis.io/docs/latest/develop/reference/protocol-spec/).

[![CI](https://github.com/0xRadioAc7iv/resp-codec/actions/workflows/ci.yml/badge.svg)](https://github.com/0xRadioAc7iv/resp-codec/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/0xRadioAc7iv/resp-codec.svg)](https://pkg.go.dev/github.com/0xRadioAc7iv/resp-codec)

## Installation

```sh
go get github.com/0xRadioAc7iv/resp-codec
```

## Type Mapping

| Go type        | RESP type        | Wire format            |
| -------------- | ---------------- | ---------------------- |
| `SimpleString` | Simple string    | `+<value>\r\n`         |
| `string`       | Bulk string      | `$<len>\r\n<data>\r\n` |
| `error`        | Error            | `-<message>\r\n`       |
| `int`          | Integer          | `:<value>\r\n`         |
| `[]any`        | Array            | `*<len>\r\n<elements>` |
| `Null`         | Null bulk string | `$-1\r\n`              |
| `NullArr`      | Null array       | `*-1\r\n`              |

## Usage

### Encode

```go
buf, err := respcodec.Encode(respcodec.SimpleString("OK"))  // "+OK\r\n"
buf, err := respcodec.Encode(errors.New("ERR unknown"))     // "-ERR unknown\r\n"
buf, err := respcodec.Encode(42)                            // ":42\r\n"
buf, err := respcodec.Encode("hello")                       // "$5\r\nhello\r\n"
buf, err := respcodec.Encode([]any{"GET", "key"})           // "*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n"
buf, err := respcodec.Encode(respcodec.Null)                // "$-1\r\n"
buf, err := respcodec.Encode(respcodec.NullArr)             // "*-1\r\n"
```

### AppendEncode

`AppendEncode` writes into a caller-supplied buffer, enabling buffer reuse and avoiding extra allocations — useful when writing to a `net.Conn` with a pooled buffer:

```go
buf := make([]byte, 0, 64)
buf, err := respcodec.AppendEncode(buf, respcodec.SimpleString("OK"))
buf, err  = respcodec.AppendEncode(buf, 42)
```

### Decode

`Decode` parses a single complete RESP frame and returns the decoded Go value, dispatched by wire-format prefix:

```go
ss,  err := respcodec.Decode([]byte("+OK\r\n"))                          // SimpleString("OK")
msg, err := respcodec.Decode([]byte("-ERR unknown\r\n"))                 // error
n,   err := respcodec.Decode([]byte(":42\r\n"))                          // 42
s,   err := respcodec.Decode([]byte("$5\r\nhello\r\n"))                  // "hello"
arr, err := respcodec.Decode([]byte("*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n")) // []any{"GET", "key"}
null, err := respcodec.Decode([]byte("$-1\r\n"))                         // nil
nullArr, err := respcodec.Decode([]byte("*-1\r\n"))                      // nil
```

For the `-` (error) type, `Decode` returns an `error` value, not a plain string.

## RESP3

The `resp3` subpackage implements [RESP3](https://github.com/redis/redis-specifications/blob/master/protocol/RESP3.md), the protocol used by Redis 6+ in protover 3 mode. It adds new types on top of RESP2 (big numbers, doubles, booleans, blob errors, verbatim strings, maps, sets, attributes, and push messages) and reinterprets RESP2's null as a single unified null type.

`resp3.Decode` is a recursive-descent parser over a shared cursor, the same approach Redis's own client-side reply parser uses: aggregate types (arrays, maps, sets, attributes, push) don't pre-compute how many bytes a nested element occupies, they just decode the next value and let the cursor advance by exactly as much as that value needed. This means arbitrarily deep nesting and binary-safe blob/verbatim/error strings (which may contain `\r`, `\n`, or bytes that look like other type sigils) decode correctly without special-casing.

```go
import "github.com/0xRadioAc7iv/resp-codec/resp3"
```

| Go type                              | RESP3 type    | Wire format                    |
| ------------------------------------ | ------------- | ------------------------------- |
| `respcodec.SimpleString`             | Simple string | `+<value>\r\n`                  |
| `string`                             | Blob string   | `$<len>\r\n<data>\r\n`          |
| `error`                              | Simple error  | `-<message>\r\n`                |
| `resp3.BlobError`                    | Blob error    | `!<len>\r\n<data>\r\n`          |
| `resp3.VerbatimString`               | Verbatim string | `=<len>\r\n<data>\r\n`        |
| `int`                                | Integer       | `:<value>\r\n`                  |
| `*big.Int`                           | Big number    | `(<value>\r\n`                  |
| `float64` / `resp3.Inf` / `resp3.NaN`| Double        | `,<value>\r\n`                  |
| `resp3.Null`                         | Null          | `_\r\n`                         |
| `bool`                               | Boolean       | `#t\r\n` / `#f\r\n`             |
| `[]any`                              | Array         | `*<len>\r\n<elements>`          |
| `map[respcodec.SimpleString]any`     | Map           | `%<pairs>\r\n<key><value>...`   |
| `map[any]struct{}`                   | Set           | `~<len>\r\n<elements>`          |
| `resp3.AttributeType`                | Attribute     | `\|<pairs>\r\n<key><value>...`  |
| `resp3.Push`                         | Push          | `><len>\r\n<kind><args>...`     |

```go
buf, err := resp3.Encode([]any{"GET", "key"})                         // "*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n"
v,   err := resp3.Decode([]byte("*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n"))   // []any{"GET", "key"}
```

## Testing

Run the test suite:

```sh
make test
```

with coverage:

```sh
make test-cov
```

### Integration tests

`resp3` also has integration tests that run against a real Redis 6+ server, verifying that `Encode`/`Decode` round-trip correctly with what an actual server sends and accepts on the wire (not just synthetic frames). They're excluded from the default test run via a build tag, so a Redis instance isn't required for normal development:

```sh
make test-integration
```

This connects to `localhost:6379` by default; set `REDIS_ADDR` to point elsewhere. Tests skip automatically if no server is reachable.

## Benchmarks

Run benchmarks:

```sh
make bench
```

## License

This project is available under the [MIT License](LICENSE).
