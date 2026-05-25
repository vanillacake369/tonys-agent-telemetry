package telemetry

import (
	"testing"
)

func TestBuildTrees_EmptyInput(t *testing.T) {
	result := BuildTrees(nil)
	if len(result) != 0 {
		t.Errorf("BuildTrees(nil) = %d trees, want 0", len(result))
	}

	result = BuildTrees([]Span{})
	if len(result) != 0 {
		t.Errorf("BuildTrees([]) = %d trees, want 0", len(result))
	}
}

func TestBuildTrees_SingleRoot(t *testing.T) {
	spans := []Span{
		{TraceID: "trace1", SpanID: "span1", ParentSpanID: ""},
	}
	trees := BuildTrees(spans)

	root, ok := trees["trace1"]
	if !ok {
		t.Fatal("expected root node for trace1")
	}
	if root.Span.SpanID != "span1" {
		t.Errorf("root span ID = %q, want %q", root.Span.SpanID, "span1")
	}
	if len(root.Children) != 0 {
		t.Errorf("root children = %d, want 0", len(root.Children))
	}
}

func TestBuildTrees_TwoLevels(t *testing.T) {
	spans := []Span{
		{TraceID: "t1", SpanID: "root", ParentSpanID: ""},
		{TraceID: "t1", SpanID: "child", ParentSpanID: "root"},
	}
	trees := BuildTrees(spans)

	root, ok := trees["t1"]
	if !ok {
		t.Fatal("expected root node for t1")
	}
	if root.Span.SpanID != "root" {
		t.Errorf("root span ID = %q, want %q", root.Span.SpanID, "root")
	}
	if len(root.Children) != 1 {
		t.Fatalf("root children = %d, want 1", len(root.Children))
	}
	if root.Children[0].Span.SpanID != "child" {
		t.Errorf("child span ID = %q, want %q", root.Children[0].Span.SpanID, "child")
	}
}

func TestBuildTrees_OrphanBecomesRoot(t *testing.T) {
	spans := []Span{
		{TraceID: "t1", SpanID: "orphan", ParentSpanID: "missing-parent"},
	}
	trees := BuildTrees(spans)

	root, ok := trees["t1"]
	if !ok {
		t.Fatal("orphan span should be promoted to root")
	}
	if root.Span.SpanID != "orphan" {
		t.Errorf("orphan root span ID = %q, want %q", root.Span.SpanID, "orphan")
	}
}

func TestBuildTrees_MultipleTraces(t *testing.T) {
	spans := []Span{
		{TraceID: "traceA", SpanID: "a1", ParentSpanID: ""},
		{TraceID: "traceB", SpanID: "b1", ParentSpanID: ""},
	}
	trees := BuildTrees(spans)

	if len(trees) != 2 {
		t.Errorf("got %d trees, want 2", len(trees))
	}
	if _, ok := trees["traceA"]; !ok {
		t.Error("missing tree for traceA")
	}
	if _, ok := trees["traceB"]; !ok {
		t.Error("missing tree for traceB")
	}
}

func TestBuildTrees_DeepTree(t *testing.T) {
	// 5-level chain: root -> l1 -> l2 -> l3 -> l4
	spans := []Span{
		{TraceID: "deep", SpanID: "root", ParentSpanID: ""},
		{TraceID: "deep", SpanID: "l1", ParentSpanID: "root"},
		{TraceID: "deep", SpanID: "l2", ParentSpanID: "l1"},
		{TraceID: "deep", SpanID: "l3", ParentSpanID: "l2"},
		{TraceID: "deep", SpanID: "l4", ParentSpanID: "l3"},
	}
	trees := BuildTrees(spans)

	root, ok := trees["deep"]
	if !ok {
		t.Fatal("missing root for deep trace")
	}

	// Walk the chain.
	ids := []string{"root", "l1", "l2", "l3", "l4"}
	node := root
	for i, want := range ids {
		if node.Span.SpanID != want {
			t.Errorf("level %d: span ID = %q, want %q", i, node.Span.SpanID, want)
		}
		if i < len(ids)-1 {
			if len(node.Children) != 1 {
				t.Fatalf("level %d: children = %d, want 1", i, len(node.Children))
			}
			node = node.Children[0]
		}
	}
}
