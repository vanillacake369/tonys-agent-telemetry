.PHONY: build test test-race vet lint clean install ci release-dry help

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

# Force pure-Go builds — no transitive cgo, simpler cross-compilation,
# smaller static binaries. Matches .goreleaser.yml + ci.yml.
export CGO_ENABLED := 0

# Default target
help:
	@echo "tonys-agent-telemetry — make targets"
	@echo ""
	@echo "  build       Build the tonys-agent-telemetry binary with version stamp"
	@echo "  test        Run tests (no race detector)"
	@echo "  test-race   Run tests with -race -count=1"
	@echo "  vet         go vet ./..."
	@echo "  lint        Alias for vet"
	@echo "  ci          vet + race tests (used by CI)"
	@echo "  install     go install the binary to GOPATH/bin"
	@echo "  clean       Remove bin/, dist/, result"
	@echo "  release-dry GoReleaser snapshot build"
	@echo ""
	@echo "VERSION=$(VERSION)"

build:
	go build -ldflags "$(LDFLAGS)" -o bin/tonys-agent-telemetry .

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

clean:
	rm -rf bin/ dist/ result

release-dry:
	goreleaser release --snapshot --clean
