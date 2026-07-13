.PHONY: build build-linux clean run-tui run-web test fmt lint

BINARY := llm-api-uptime
HOST_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe)
OUTPUT := $(BINARY)$(HOST_EXT)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILDINFO_PKG := llm-api-uptime/internal/buildinfo
LDFLAGS := -s -w -X $(BUILDINFO_PKG).Version=$(VERSION) -X $(BUILDINFO_PKG).Commit=$(COMMIT) -X $(BUILDINFO_PKG).BuildDate=$(BUILD_DATE)

build:
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(OUTPUT) .

build-linux:
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) .

clean:
	rm -f $(BINARY) $(BINARY).exe

run-tui: build
	./$(OUTPUT)

run-web: build
	WEB_ENABLED=true ./$(OUTPUT)

test:
	go test ./...

fmt:
	go fmt ./...

lint:
	golangci-lint run
