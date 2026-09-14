GO ?= go

.PHONY: build test test-race fmt vet
build:
	$(GO) build ./backend/cmd/syslogx

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...
