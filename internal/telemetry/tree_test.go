package telemetry

import "testing"

func TestBuildTrees_EmptyInput(t *testing.T) {
	got := BuildTrees(nil)
	if len(got) != 0 {
		t.Errorf("BuildTrees(nil) = %v, want empty", got)
	}
}

func TestBuildTrees_SingleRoot(t *testing.T) {
	spans := []Span{{TraceID: "t1", SpanID: "a"}}
	trees := BuildTrees(spans)
	if len(trees) != 1 {
		t.Fatalf("len(trees) = %d, want 1", len(trees))
	}
	root := trees["t1"]
	if root == nil || root.Span.SpanID != "a" {
		t.Errorf("root = %+v, want SpanID=a", root)
	}
	if len(root.Children) != 0 {
		t.Errorf("Children = %d, want 0", len(root.Children))
	}
}

func TestBuildTrees_TwoLevels(t *testing.T) {
	spans := []Span{
		{TraceID: "t1", SpanID: "root"},
		{TraceID: "t1", SpanID: "child", ParentSpanID: "root"},
	}
	trees := BuildTrees(spans)
	root := trees["t1"]
	if root == nil {
		t.Fatal("root nil")
	}
	if root.Span.SpanID != "root" {
		t.Errorf("root SpanID = %s, want root", root.Span.SpanID)
	}
	if len(root.Children) != 1 || root.Children[0].Span.SpanID != "child" {
		t.Errorf("Children = %v, want [child]", root.Children)
	}
}

func TestBuildTrees_OrphanBecomesRoot(t *testing.T) {
	spans := []Span{
		{TraceID: "t1", SpanID: "child", ParentSpanID: "missing-parent"},
	}
	trees := BuildTrees(spans)
	if root := trees["t1"]; root == nil || root.Span.SpanID != "child" {
		t.Errorf("orphan should be promoted to root, got %+v", root)
	}
}

func TestBuildTrees_MultipleTraces(t *testing.T) {
	spans := []Span{
		{TraceID: "t1", SpanID: "a"},
		{TraceID: "t2", SpanID: "b"},
	}
	trees := BuildTrees(spans)
	if len(trees) != 2 {
		t.Errorf("len(trees) = %d, want 2", len(trees))
	}
	if trees["t1"] == nil || trees["t2"] == nil {
		t.Errorf("missing trace: t1=%v t2=%v", trees["t1"], trees["t2"])
	}
}

func TestBuildTrees_DeepChain(t *testing.T) {
	spans := []Span{
		{TraceID: "t1", SpanID: "1"},
		{TraceID: "t1", SpanID: "2", ParentSpanID: "1"},
		{TraceID: "t1", SpanID: "3", ParentSpanID: "2"},
		{TraceID: "t1", SpanID: "4", ParentSpanID: "3"},
		{TraceID: "t1", SpanID: "5", ParentSpanID: "4"},
	}
	root := BuildTrees(spans)["t1"]
	depth := 1
	cur := root
	for len(cur.Children) > 0 {
		cur = cur.Children[0]
		depth++
	}
	if depth != 5 {
		t.Errorf("chain depth = %d, want 5", depth)
	}
}

func TestBuildTrees_SiblingChildren(t *testing.T) {
	spans := []Span{
		{TraceID: "t1", SpanID: "root"},
		{TraceID: "t1", SpanID: "c1", ParentSpanID: "root"},
		{TraceID: "t1", SpanID: "c2", ParentSpanID: "root"},
		{TraceID: "t1", SpanID: "c3", ParentSpanID: "root"},
	}
	root := BuildTrees(spans)["t1"]
	if len(root.Children) != 3 {
		t.Errorf("Children = %d, want 3", len(root.Children))
	}
}
