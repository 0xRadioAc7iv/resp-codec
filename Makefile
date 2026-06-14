.PHONY: test test-cov bench bench-root bench-wire bench-resp3

test:
	go test ./... -v

test-cov:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

bench-root:
	go test -bench=. -benchmem .

bench-wire:
	go test -bench=. -benchmem ./internal/wire

bench-resp3:
	go test -bench=. -benchmem ./resp3

bench: bench-root bench-wire bench-resp3
