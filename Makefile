.PHONY: build test test-race vet lint clean install ci release-dry help

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

# Default target
help:
	@echo "tonys-agent-telemetry — make targets"
	@echo ""
	@echo "  build       Build both binaries with version stamp"
	@echo "  test        Run tests (no race detector)"
	@echo "  test-race   Run tests with -race -count=1"
	@echo "  vet         go vet ./..."
	@echo "  lint        Alias for vet"
	@echo "  ci          vet + race tests (used by CI)"
	@echo "  install     go install both binaries to GOPATH/bin"
	@echo "  clean       Remove bin/, dist/, result"
	@echo "  release-dry GoReleaser snapshot build"
	@echo ""
	@echo "VERSION=$(VERSION)"

build:
	go build -ldflags "$(LDFLAGS)" -o bin/tonys-agent-telemetry .
	go build -ldflags "-s -w" -o bin/tonys-agent-telemetry-hook ./cmd/hook-handler

test:
	go test -count=1 ./...

test-race:
	go test -race -count=1 ./...

vet:
	go vet ./...

lint: vet

ci: vet test-race

install:
	go install -ldflags "$(LDFLAGS)" .
	go install -ldflags "-s -w" ./cmd/hook-handler

clean:
	rm -rf bin/ dist/ result

release-dry:
	goreleaser release --snapshot --clean
