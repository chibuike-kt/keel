BINARY  := keel
PKG     := github.com/chibuike-kt/keel
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X $(PKG)/internal/buildinfo.Version=$(VERSION)

.PHONY: build test lint fmt tidy clean

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/keel

test:
	go test -race ./...

lint:
	golangci-lint run

fmt:
	golangci-lint fmt

tidy:
	go mod tidy

clean:
	rm -rf bin
