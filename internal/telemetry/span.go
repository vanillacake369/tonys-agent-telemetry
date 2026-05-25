package telemetry

import "time"

// Span is the canonical, provider-agnostic telemetry record.
// Follows OpenTelemetry GenAI semantic conventions where stable; uses Attrs for
// experimental or provider-specific fields.
type Span struct {
	// OTel identity (REQUIRED)
	TraceID      string
	SpanID       string
	ParentSpanID string // empty for root

	// OTel GenAI semconv (REQUIRED for LLM spans)
	System       string // "anthropic", "openai", "ollama", "vllm", "langgraph", ...
	Model        string
	InputTokens  int
	OutputTokens int

	// Timing
	StartTime time.Time
	EndTime   time.Time

	// Lifecycle
	Status string // "running" | "done" | "error"

	// Provider-agnostic extension. Use OTel semconv key names where applicable.
	// Examples: "code.filepath" (cwd), "vcs.ref.head.name" (git branch),
	// "gen_ai.tool.name", "gen_ai.operation.name", "error.type".
	Attrs map[string]string
}
