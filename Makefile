VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.Version=$(VERSION)
PREFIX  ?= /usr/local

.PHONY: build test cover lint mod-tidy snapshot release install clean help

## build: compile screenscope
build:
	CGO_ENABLED=1 go build -ldflags "$(LDFLAGS)" -o screenscope ./cmd/screenscope

## test: run unit tests
test:
	go test ./...

## cover: run tests with coverage report
cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## mod-tidy: tidy go.mod
mod-tidy:
	go mod tidy

## snapshot: build a local snapshot release via goreleaser (no publish)
snapshot:
	goreleaser release --snapshot --clean

## release: publish a release via goreleaser (requires a clean git tag)
release:
	goreleaser release --clean

## install: install pre-built binary to PREFIX/bin (run make build first)
install:
	@test -f screenscope || { echo "error: screenscope binary not found. Run 'make build' first."; exit 1; }
	install -Dm755 screenscope $(DESTDIR)$(PREFIX)/bin/screenscope

## clean: remove all generated build and test artifacts
clean:
	rm -f screenscope
	rm -rf dist/
	find . -name '*.test' -delete
	find . -name 'coverage.out' -o -name 'coverage.*' -o -name '*.coverprofile' -o -name 'profile.cov' | xargs rm -f

## help: list available targets
help:
	@grep -E '^##' Makefile | sed 's/^## /  /'
