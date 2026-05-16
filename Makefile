.PHONY: build test clean lint

build:
	go build -ldflags "-s -w -X main.version=dev" -o bin/tonys-agent-telemetry .
	go build -ldflags "-s -w" -o bin/tonys-agent-telemetry-hook ./cmd/hook-handler

test:
	go test -count=1 ./...

clean:
	rm -rf bin/ dist/

lint:
	go vet ./...
