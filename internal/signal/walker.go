package signal

import (
	"sort"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// depthFirstWalk visits every node in the forest rooted at roots using
// DFS pre-order. It calls fn for each node. Order within siblings is
// preserved (children are visited in the order they appear in node.Children).
func depthFirstWalk(roots []*telemetry.SpanNode, fn func(*telemetry.SpanNode)) {
	var walk func(n *telemetry.SpanNode)
	walk = func(n *telemetry.SpanNode) {
		fn(n)
		for _, child := range n.Children {
			walk(child)
		}
	}
	for _, r := range roots {
		walk(r)
	}
}

// maxEndTime returns the latest non-zero EndTime across all nodes reachable
// from roots. If all EndTimes are zero (trace fully in-progress), returns
// the zero time value.
//
// This is "Note A" from SIGNALS_SPEC §3.1: a zero return means no span has
// completed — the whole trace is in-progress; stall checks must be skipped.
func maxEndTime(roots []*telemetry.SpanNode) time.Time {
	var max time.Time
	depthFirstWalk(roots, func(n *telemetry.SpanNode) {
		if !n.Span.EndTime.IsZero() && n.Span.EndTime.After(max) {
			max = n.Span.EndTime
		}
	})
	return max
}

// toolName extracts the gen_ai.tool.name attribute from a span, returning
// an empty string if the attribute is absent. This is the canonical accessor
// used by all detectors (DRY).
func toolName(s telemetry.Span) string {
	return s.Attrs["gen_ai.tool.name"]
}

// sortedTraceIDs returns the map keys sorted lexicographically. All detectors
// call this before ranging over the forest to guarantee deterministic ordering
// regardless of Go map iteration (SIGNALS_SPEC §5, no-randomness guarantee).
func sortedTraceIDs(forest Forest) []string {
	ids := make([]string, 0, len(forest))
	for id := range forest {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// toolSequence returns the list of gen_ai.tool.name values in DFS pre-order
// for the entire subtree rooted at node. Spans with no tool name contribute
// an empty string (SIGNALS_SPEC §3.2 algorithm).
func toolSequence(node *telemetry.SpanNode) []string {
	var seq []string
	depthFirstWalk([]*telemetry.SpanNode{node}, func(n *telemetry.SpanNode) {
		seq = append(seq, toolName(n.Span))
	})
	return seq
}

// minFloat64 returns the smaller of a and b.
func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
