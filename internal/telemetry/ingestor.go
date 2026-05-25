package telemetry

import "context"

// ProviderIngestor is implemented by each adapter that knows how to detect
// and harvest telemetry from a specific runtime (Claude Code, vLLM, Ollama,
// LangGraph, OTLP receiver, etc.).
//
// Implementations encapsulate their own detection cascade (port probe,
// process inspection, config-path scan) inside Detect and own their
// reconnect/retry loop inside Ingest.
type ProviderIngestor interface {
	// ProviderID returns a stable, lowercase identifier (e.g. "claudecode",
	// "vllm", "ollama"). Used for logging and provider attribution.
	ProviderID() string

	// Detect reports whether this provider appears to be present in the
	// current environment. Must be fast (< 200 ms total) and side-effect-free.
	// Returning false MUST not block, panic, or leave goroutines behind.
	Detect(ctx context.Context) bool

	// Ingest starts the ingestor and writes Spans to out until ctx is
	// cancelled. Returns a non-nil error only on unrecoverable startup
	// failure; transient failures should be retried internally.
	Ingest(ctx context.Context, out chan<- Span) error
}
