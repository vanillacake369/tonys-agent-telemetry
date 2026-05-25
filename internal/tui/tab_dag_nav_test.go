package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

func newDAGWith(t *testing.T, spans ...telemetry.Span) *DAGTab {
	t.Helper()
	d := NewDAGTab().SetSize(120, 30).(*DAGTab)
	tab, _ := d.Update(SpanBatchMsg{Spans: spans})
	return tab.(*DAGTab)
}

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func TestDAGTab_StartsInTracesMode(t *testing.T) {
	d := newDAGWith(t, telemetry.Span{TraceID: "t1", SpanID: "a"})
	if d.mode != dagViewTraces {
		t.Errorf("mode = %d, want dagViewTraces", d.mode)
	}
}

func TestDAGTab_JKMovesCursorInTracesList(t *testing.T) {
	d := newDAGWith(t,
		telemetry.Span{TraceID: "t1", SpanID: "a"},
		telemetry.Span{TraceID: "t2", SpanID: "b"},
		telemetry.Span{TraceID: "t3", SpanID: "c"},
	)
	if d.traceCursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", d.traceCursor)
	}
	tab, _ := d.Update(keyMsg("j"))
	d = tab.(*DAGTab)
	if d.traceCursor != 1 {
		t.Errorf("after j: cursor = %d, want 1", d.traceCursor)
	}
	tab, _ = d.Update(keyMsg("k"))
	d = tab.(*DAGTab)
	if d.traceCursor != 0 {
		t.Errorf("after k: cursor = %d, want 0", d.traceCursor)
	}
}

func TestDAGTab_EnterDrillsIntoGraph(t *testing.T) {
	d := newDAGWith(t,
		telemetry.Span{TraceID: "t1", SpanID: "a", System: "anthropic"},
		telemetry.Span{TraceID: "t1", SpanID: "b", ParentSpanID: "a"},
	)
	tab, _ := d.Update(keyMsg("enter"))
	d = tab.(*DAGTab)
	if d.mode != dagViewGraph {
		t.Errorf("mode = %d, want dagViewGraph", d.mode)
	}
	if d.activeTrace != "t1" {
		t.Errorf("activeTrace = %q, want t1", d.activeTrace)
	}
	if len(d.flatNodes) != 2 {
		t.Errorf("flatNodes = %d, want 2", len(d.flatNodes))
	}
}

func TestDAGTab_EnterFromGraphOpensSpanDetail(t *testing.T) {
	d := newDAGWith(t,
		telemetry.Span{TraceID: "t1", SpanID: "a", System: "anthropic"},
	)
	d.Update(keyMsg("enter"))                  // → graph
	tab, _ := d.Update(keyMsg("enter"))        // → span detail
	d = tab.(*DAGTab)
	if d.mode != dagViewSpan {
		t.Errorf("mode = %d, want dagViewSpan", d.mode)
	}
}

func TestDAGTab_EscReturnsToParentMode(t *testing.T) {
	d := newDAGWith(t,
		telemetry.Span{TraceID: "t1", SpanID: "a", System: "anthropic"},
	)
	d.Update(keyMsg("enter")) // → graph
	d.Update(keyMsg("enter")) // → span
	tab, _ := d.Update(keyMsg("esc"))
	d = tab.(*DAGTab)
	if d.mode != dagViewGraph {
		t.Errorf("first esc: mode = %d, want dagViewGraph", d.mode)
	}
	tab, _ = d.Update(keyMsg("esc"))
	d = tab.(*DAGTab)
	if d.mode != dagViewTraces {
		t.Errorf("second esc: mode = %d, want dagViewTraces", d.mode)
	}
}

func TestDAGTab_GraphViewShowsSelectedSpanCompact(t *testing.T) {
	d := newDAGWith(t,
		telemetry.Span{TraceID: "t1", SpanID: "alpha-beta-gamma", System: "anthropic", Model: "claude-x"},
	)
	d.Update(keyMsg("enter")) // → graph
	v := d.View()
	if !strings.Contains(v, "claude-x") {
		t.Errorf("graph view should show selected span model; got: %q", truncate(v, 400))
	}
}

func TestDAGTab_YankInvokesClipboard(t *testing.T) {
	// Stub the clipboard so we don't shell out.
	var captured string
	orig := clipboardCopy
	clipboardCopy = func(text string) error { captured = text; return nil }
	t.Cleanup(func() { clipboardCopy = orig })

	d := newDAGWith(t,
		telemetry.Span{TraceID: "t1", SpanID: "alpha", System: "anthropic", Model: "m"},
	)
	d.Update(keyMsg("enter")) // → graph mode where 'y' is wired
	tab, cmd := d.Update(keyMsg("y"))
	d = tab.(*DAGTab)
	if cmd == nil {
		t.Fatal("expected yank cmd, got nil")
	}
	msg := cmd()
	if _, ok := msg.(flashMsg); !ok {
		t.Errorf("expected flashMsg from yank, got %T", msg)
	}
	if !strings.Contains(captured, `"SpanID": "alpha"`) {
		t.Errorf("clipboard content missing span id: %q", captured)
	}
}
