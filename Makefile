.PHONY: build test test-race vet lint lint-strict fmt fmt-check hooks-install clean install ci release-dry help

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

# Force pure-Go builds — no transitive cgo, simpler cross-compilation,
# smaller static binaries. Matches .goreleaser.yml + ci.yml.
export CGO_ENABLED := 0

# Default target
help:
	@echo "tonys-agent-telemetry — make targets"
	@echo ""
	@echo "  build         Build the tonys-agent-telemetry binary with version stamp"
	@echo "  test          Run tests (no race detector)"
	@echo "  test-race     Run tests with -race -count=1"
	@echo "  fmt           gofmt -w on all .go files"
	@echo "  fmt-check     gofmt -l — fails if any file is unformatted"
	@echo "  vet           go vet ./..."
	@echo "  lint          go vet (always available)"
	@echo "  lint-strict   golangci-lint run (requires the binary on PATH)"
	@echo "  hooks-install Wire .githooks/ as the repo hooks directory"
	@echo "  lint-new      golangci-lint run --new-from-rev=origin/main (PR gate)"
	@echo "  ci            fmt-check + vet + race tests (matches CI workflow)"
	@echo "  install       go install the binary to GOPATH/bin"
	@echo "  clean         Remove bin/, dist/, result"
	@echo "  release-dry   GoReleaser snapshot build"
	@echo ""
	@echo "VERSION=$(VERSION)"

build:
	go build -ldflags "$(LDFLAGS)" -o bin/tonys-agent-telemetry .

test:
	go test -count=1 ./...

test-race:
	go test -race -count=1 ./...

fmt:
	gofmt -w .

fmt-check:
	@out=$$(gofmt -l . 2>&1); \
	if [ -n "$$out" ]; then \
		echo "The following files are not gofmt-clean:"; \
		echo "$$out"; \
		echo "Run: make fmt"; \
		exit 1; \
	fi

vet:
	go vet ./...

lint: vet

lint-strict:
	@if command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed — skipping strict lint."; \
		echo "Install: https://golangci-lint.run/usage/install/"; \
	fi

# lint-new only flags NEW issues introduced since main. Use this in PRs to
# avoid being blocked by the 179 pre-existing accumulated issues. Once those
# are paid down, swap make ci over to lint-strict.
lint-new:
	@if command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run --new-from-rev=origin/main; \
	else \
		echo "golangci-lint not installed — skipping diff lint."; \
	fi

hooks-install:
	@git config core.hooksPath .githooks
	@echo "Git hooks installed (core.hooksPath = .githooks)"
	@echo "pre-commit: gofmt + vet + short tests"
	@echo "commit-msg: Conventional Commits enforcement"

ci: fmt-check vet test-race
	@echo "ci: fmt + vet + race tests OK. Run 'make lint-new' before each PR."

install:
	go install -ldflags "$(LDFLAGS)" .

clean:
	rm -rf bin/ dist/ result

release-dry:
	@# Skip sbom (needs syft) and sign (needs cosign + OIDC) for local
	@# dry-runs. CI installs both via the workflow.
	goreleaser release --snapshot --clean --skip=sbom,sign,publish
