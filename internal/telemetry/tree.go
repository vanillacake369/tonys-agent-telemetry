package telemetry

// SpanNode is one node in a per-trace tree.
type SpanNode struct {
	Span     Span
	Children []*SpanNode
}

// BuildTrees groups spans by TraceID and reconstructs parent-child trees
// using ParentSpanID. Orphan spans (ParentSpanID points to a span not in the
// input) become roots. Returns one tree per unique TraceID.
func BuildTrees(spans []Span) map[string]*SpanNode {
	// by-id index
	nodes := make(map[string]*SpanNode, len(spans))
	for _, s := range spans {
		nodes[s.SpanID] = &SpanNode{Span: s}
	}

	roots := make(map[string]*SpanNode) // keyed by TraceID
	for _, s := range spans {
		node := nodes[s.SpanID]
		if s.ParentSpanID == "" {
			roots[s.TraceID] = node
			continue
		}
		parent, ok := nodes[s.ParentSpanID]
		if !ok {
			// Orphan: treat as root within its trace
			roots[s.TraceID] = node
			continue
		}
		parent.Children = append(parent.Children, node)
	}
	return roots
}
