.PHONY: build test check

build:
	go build -o bin/spare ./cmd/spare

test:
	go test ./...

check: test
	go vet ./...
