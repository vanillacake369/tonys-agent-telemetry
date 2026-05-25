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
// Returns one tree per unique TraceID, keyed by TraceID. If a trace has
// multiple root candidates (e.g., one true root + several orphans), only
// the last-encountered root is kept under that key; orphans are still
// reachable as direct children of nothing in the input but the function
// surfaces the most-recent root for that trace.
func BuildTrees(spans []Span) map[string]*SpanNode {
	nodes := make(map[string]*SpanNode, len(spans))
	for i := range spans {
		nodes[spans[i].SpanID] = &SpanNode{Span: spans[i]}
	}

	roots := make(map[string]*SpanNode)
	for i := range spans {
		s := spans[i]
		node := nodes[s.SpanID]
		if s.ParentSpanID == "" {
			roots[s.TraceID] = node
			continue
		}
		parent, ok := nodes[s.ParentSpanID]
		if !ok {
			// Parent not present in input — treat as a root within its trace.
			roots[s.TraceID] = node
			continue
		}
		parent.Children = append(parent.Children, node)
	}
	return roots
}
