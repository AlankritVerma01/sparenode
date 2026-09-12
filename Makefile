.PHONY: build test check smoke-cpu smoke-gpu

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || printf unknown)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT)

build:
	go build -trimpath -buildvcs=false -ldflags '$(LDFLAGS)' -o bin/spare ./cmd/spare

test:
	go test ./...

check: test
	go vet ./...

smoke-cpu: build
	./scripts/smoke-cpu.sh

smoke-gpu: build
	./scripts/smoke-gpu.sh
