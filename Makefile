.PHONY: test bench

test:
	go test ./... -v

test-cov:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

bench:
	go test -bench=. -benchmem .