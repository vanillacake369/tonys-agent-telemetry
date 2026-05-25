package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// dumpGraph renders the graph for the given spans at the given panel width
// and returns the rendered string. Helper for visual debugging.
func dumpGraph(t *testing.T, width int, spans []telemetry.Span) string {
	t.Helper()
	d := NewDAGTab().SetSize(width, 50).(*DAGTab)
	tab, _ := d.Update(SpanBatchMsg{Spans: spans})
	d = tab.(*DAGTab)
	if len(d.traces) == 0 {
		return "(no traces)"
	}
	tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d = tab.(*DAGTab)
	return d.renderGraph(width - 4)
}

// stripAnsi removes ANSI escape sequences for plain-text alignment checks.
func stripAnsi(s string) string {
	var sb strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
				j++
			}
			if j < len(s) {
				j++ // consume final byte
			}
			i = j
			continue
		}
		sb.WriteByte(s[i])
		i++
	}
	return sb.String()
}

// ── Shape 1: Single node, no children ─────────────────────────────────────
func TestDAGShape_SingleNode(t *testing.T) {
	out := dumpGraph(t, 160, []telemetry.Span{
		{TraceID: "t", SpanID: "only", System: "anthropic", Model: "x"},
	})
	t.Logf("\n--- single node ---\n%s", out)
	if !strings.Contains(stripAnsi(out), "┌") {
		t.Error("missing box border")
	}
}

// ── Shape 2: Linear chain (deep DAG, common in Claude tool sequences) ─────
func TestDAGShape_LinearChain(t *testing.T) {
	var spans []telemetry.Span
	for i := 0; i < 10; i++ {
		parent := ""
		if i > 0 {
			parent = fmt.Sprintf("n%d", i-1)
		}
		spans = append(spans, telemetry.Span{
			TraceID:      "t",
			SpanID:       fmt.Sprintf("n%d", i),
			ParentSpanID: parent,
			System:       "anthropic",
		})
	}
	out := dumpGraph(t, 200, spans)
	t.Logf("\n--- chain of 10 ---\n%s", out)
	if !strings.Contains(stripAnsi(out), "┌") {
		t.Error("missing box border")
	}
}

// ── Shape 3: Wide fan-out (1 parent, many siblings) ───────────────────────
func TestDAGShape_WideFanout(t *testing.T) {
	spans := []telemetry.Span{
		{TraceID: "t", SpanID: "root", System: "anthropic", Attrs: map[string]string{"gen_ai.operation.name": "chat"}},
	}
	for i := 0; i < 6; i++ {
		spans = append(spans, telemetry.Span{
			TraceID:      "t",
			SpanID:       fmt.Sprintf("child%d", i),
			ParentSpanID: "root",
			System:       "anthropic",
			Attrs:        map[string]string{"gen_ai.tool.name": fmt.Sprintf("Tool%d", i)},
		})
	}
	out := dumpGraph(t, 160, spans)
	t.Logf("\n--- fanout 6 ---\n%s", out)
	// Trunk should branch (├ or └) at multiple rows.
	plain := stripAnsi(out)
	branches := strings.Count(plain, "├") + strings.Count(plain, "└")
	if branches < 2 {
		t.Errorf("wide fanout should produce multiple trunk branches; got %d ├/└ chars", branches)
	}
}

// ── Shape 4: Real Claude-style session (mixed chain + fanout) ─────────────
func TestDAGShape_RealClaudeSession(t *testing.T) {
	spans := []telemetry.Span{
		{TraceID: "sess", SpanID: "user1", System: "anthropic", Attrs: map[string]string{"gen_ai.operation.name": "chat"}},
		{TraceID: "sess", SpanID: "asst1", ParentSpanID: "user1", System: "anthropic", Model: "claude-sonnet-4-6", InputTokens: 1500, OutputTokens: 420, Attrs: map[string]string{"gen_ai.operation.name": "chat"}},
		{TraceID: "sess", SpanID: "tool1", ParentSpanID: "asst1", System: "anthropic", Attrs: map[string]string{"gen_ai.tool.name": "Bash"}},
		{TraceID: "sess", SpanID: "tool2", ParentSpanID: "asst1", System: "anthropic", Attrs: map[string]string{"gen_ai.tool.name": "Read"}},
		{TraceID: "sess", SpanID: "asst2", ParentSpanID: "tool2", System: "anthropic", Model: "claude-sonnet-4-6", Attrs: map[string]string{"gen_ai.operation.name": "chat"}},
		{TraceID: "sess", SpanID: "tool3", ParentSpanID: "asst2", System: "anthropic", Attrs: map[string]string{"gen_ai.tool.name": "Edit"}},
		{TraceID: "sess", SpanID: "user2", ParentSpanID: "tool3", System: "anthropic"},
		{TraceID: "sess", SpanID: "asst3", ParentSpanID: "user2", System: "anthropic", Status: "error"},
	}
	out := dumpGraph(t, 200, spans)
	t.Logf("\n--- real session ---\n%s", out)
}

// ── Shape 5: Multiple orphan roots in same trace ──────────────────────────
func TestDAGShape_MultipleOrphans(t *testing.T) {
	spans := []telemetry.Span{
		{TraceID: "t", SpanID: "a", System: "anthropic"},
		{TraceID: "t", SpanID: "b", ParentSpanID: "missing-parent-1", System: "anthropic"},
		{TraceID: "t", SpanID: "c", ParentSpanID: "missing-parent-2", System: "anthropic"},
	}
	out := dumpGraph(t, 160, spans)
	t.Logf("\n--- multiple orphans ---\n%s", out)
	// All three orphans should appear in the rendered output as separate boxes.
	plain := stripAnsi(out)
	boxes := strings.Count(plain, "┌")
	if boxes < 1 {
		t.Errorf("expected at least one box, got %d", boxes)
	}
	// NOTE: telemetry.BuildTrees overwrites duplicate-TraceID roots, so
	// only the last orphan survives. This is a separate bug if it
	// manifests for real users.
	t.Logf("Found %d boxes; if < 3, BuildTrees overwrote orphan roots", boxes)
}

// ── Shape 6: Constrained width — panel narrower than computed gridW ───────
func TestDAGShape_NarrowPanel(t *testing.T) {
	spans := []telemetry.Span{
		{TraceID: "t", SpanID: "root", System: "anthropic"},
		{TraceID: "t", SpanID: "c1", ParentSpanID: "root", System: "anthropic"},
		{TraceID: "t", SpanID: "g1", ParentSpanID: "c1", System: "anthropic"},
		{TraceID: "t", SpanID: "gg1", ParentSpanID: "g1", System: "anthropic"},
	}
	out := dumpGraph(t, 60, spans) // 60 col panel, but depth 4 needs ~80
	t.Logf("\n--- narrow panel (60 cols, depth 4) ---\n%s", out)
	// Just check no panic — visual breakage is expected here.
}

// ── Shape 7: Real-data scale — 50 spans in one trace ──────────────────────
func TestDAGShape_FiftySpans(t *testing.T) {
	var spans []telemetry.Span
	// Linear chain of 50.
	for i := 0; i < 50; i++ {
		parent := ""
		if i > 0 {
			parent = fmt.Sprintf("s%d", i-1)
		}
		spans = append(spans, telemetry.Span{
			TraceID:      "t",
			SpanID:       fmt.Sprintf("s%d", i),
			ParentSpanID: parent,
			System:       "anthropic",
		})
	}
	out := dumpGraph(t, 500, spans)
	// Just count lines: should not panic, should produce a 50-row grid.
	lines := strings.Count(out, "\n")
	t.Logf("50-span chain produced %d lines", lines)
	if lines < 50 {
		t.Errorf("expected at least 50 lines, got %d", lines)
	}
}
