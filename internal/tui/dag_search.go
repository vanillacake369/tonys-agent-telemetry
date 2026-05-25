// Package tui — dag_search.go: pure-function helpers for DAG node search.
// Wiring (search mode state + key handling) lives in tab_dag.go.
package tui

import (
	"strings"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// MatchNode reports whether node contains query (case-insensitive substring).
// Searched fields: gen_ai.tool.name, gen_ai.operation.name, span.Model, span.Status.
// An empty query matches everything. A nil node never matches.
func MatchNode(node *telemetry.SpanNode, query string) bool {
	if node == nil {
		return false
	}
	if query == "" {
		return true
	}
	q := strings.ToLower(query)

	haystack := strings.ToLower(strings.Join([]string{
		node.Span.Attrs["gen_ai.tool.name"],
		node.Span.Attrs["gen_ai.operation.name"],
		node.Span.Model,
		node.Span.Status,
	}, " "))

	return strings.Contains(haystack, q)
}
