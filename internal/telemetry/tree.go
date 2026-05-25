package telemetry

// SpanNode is one node in a per-trace span tree, used by DAG renderers.
type SpanNode struct {
	Span     Span
	Children []*SpanNode
}

// BuildTrees groups spans by TraceID and reconstructs parent-child trees
// using ParentSpanID. Orphan spans (ParentSpanID points to a span not in
// the input slice) become additional roots within their trace.
//
// Returns one tree per unique TraceID. The legacy single-root return form
// (kept for backward compat) keeps the FIRST root encountered per trace.
// Callers that need all orphans should use BuildForests.
func BuildTrees(spans []Span) map[string]*SpanNode {
	forests := BuildForests(spans)
	out := make(map[string]*SpanNode, len(forests))
	for k, roots := range forests {
		if len(roots) > 0 {
			out[k] = roots[0]
		}
	}
	return out
}

// BuildForests is like BuildTrees but preserves EVERY root within a trace.
// A trace with N orphans returns a slice of N roots (in first-seen order).
// This is the correct view for DAG renderers; BuildTrees was losing data
// when sessions had multiple disconnected roots (e.g. system messages
// without parent links, queue-operation spans, partially-truncated logs).
func BuildForests(spans []Span) map[string][]*SpanNode {
	nodes := make(map[string]*SpanNode, len(spans))
	for i := range spans {
		nodes[spans[i].SpanID] = &SpanNode{Span: spans[i]}
	}

	roots := make(map[string][]*SpanNode)
	for i := range spans {
		s := spans[i]
		node := nodes[s.SpanID]
		if s.ParentSpanID == "" {
			roots[s.TraceID] = append(roots[s.TraceID], node)
			continue
		}
		parent, ok := nodes[s.ParentSpanID]
		if !ok {
			// Parent not present in input — also a root.
			roots[s.TraceID] = append(roots[s.TraceID], node)
			continue
		}
		parent.Children = append(parent.Children, node)
	}
	return roots
}
