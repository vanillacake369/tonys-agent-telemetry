package telemetry

import "context"

// ProviderIngestor is implemented by each adapter (claudecode, otlp, vllm, ...).
// Detect MUST be fast (<200ms total) and side-effect-free.
// Ingest is started in its own goroutine by Registry.StartAll; it owns its
// reconnect/retry policy and writes Spans to out until ctx is cancelled.
type ProviderIngestor interface {
	ProviderID() string
	Detect(ctx context.Context) bool
	Ingest(ctx context.Context, out chan<- Span) error
}
