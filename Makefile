.PHONY: build test check

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || printf unknown)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT)

build:
	go build -trimpath -buildvcs=false -ldflags '$(LDFLAGS)' -o bin/spare ./cmd/spare

test:
	go test ./...

check: test
	go vet ./...
