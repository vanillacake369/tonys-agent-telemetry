.PHONY: build test clean lint release-dry

VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo dev)

build:
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o bin/tonys-agent-telemetry .
	go build -ldflags "-s -w" -o bin/tonys-agent-telemetry-hook ./cmd/hook-handler

test:
	go test -count=1 -race ./...

clean:
	rm -rf bin/ dist/ result

lint:
	go vet ./...

release-dry:
	goreleaser release --snapshot --clean
