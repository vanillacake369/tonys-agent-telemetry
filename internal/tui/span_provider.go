package tui

import "github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"

// SpanProvider is implemented by any tab that holds a span buffer the
// advisor and trends pipelines can extract from. Today only DAGTab
// satisfies it; future tabs may opt in.
//
// SRP: this interface exposes only the single Spans() getter. Do not
// add unrelated getters here — they belong on the concrete type or a
// separate interface.
type SpanProvider interface {
	Spans() []telemetry.Span
}

// Compile-time assertion that *DAGTab satisfies SpanProvider.
// If DAGTab is ever refactored to a value receiver, this fails to compile
// before the runtime cast in app.go would panic.
var _ SpanProvider = (*DAGTab)(nil)
