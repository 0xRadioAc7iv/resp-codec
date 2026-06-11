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

```go
s, err   := respcodec.Decode[respcodec.SimpleString]([]byte("+OK\r\n"))
e, err   := respcodec.Decode[error]([]byte("-ERR unknown\r\n"))
n, err   := respcodec.Decode[int]([]byte(":42\r\n"))
str, err := respcodec.Decode[string]([]byte("$5\r\nhello\r\n"))
arr, err := respcodec.Decode[[]any]([]byte("*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n"))
```

## Testing

Run the test suite:

```sh
go test ./...
```

## Benchmarks

Run benchmarks:

```sh
go test -bench=. -benchmem ./...
```

## License

This project is available under the [MIT License](LICENSE).
