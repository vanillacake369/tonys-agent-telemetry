// Package telemetry defines provider-agnostic telemetry primitives shaped
// after OpenTelemetry GenAI semantic conventions. Individual provider
// adapters (Claude Code, vLLM, Ollama, LangGraph, etc.) translate their
// native event/metric formats into Span values consumed uniformly by the
// TUI and exporters.
package telemetry

import "time"

// Span is the canonical, provider-agnostic telemetry record. Field names
// follow OTel GenAI semconv where stable; provider-specific or experimental
// data lives in Attrs to keep this struct stable as the spec evolves.
type Span struct {
	// OTel identity
	TraceID      string // groups all spans in one logical session/agent run
	SpanID       string // unique per LLM call or tool invocation
	ParentSpanID string // empty for root spans

	// OTel GenAI semconv (gen_ai.*)
	System       string // "anthropic" | "openai" | "ollama" | "vllm" | "langgraph" | ...
	Model        string // gen_ai.request.model
	InputTokens  int    // gen_ai.usage.input_tokens
	OutputTokens int    // gen_ai.usage.output_tokens

	// Timing
	StartTime time.Time
	EndTime   time.Time

	// Lifecycle
	Status string // "running" | "done" | "error"

	// Extension. Use OTel semconv keys where applicable, e.g.:
	//   "code.filepath", "vcs.ref.head.name",
	//   "gen_ai.operation.name", "gen_ai.tool.name", "error.type".
	Attrs map[string]string
}

// Duration returns EndTime.Sub(StartTime). Returns 0 for spans that have not
// completed (EndTime is zero).
func (s Span) Duration() time.Duration {
	if s.EndTime.IsZero() {
		return 0
	}
	return s.EndTime.Sub(s.StartTime)
}
