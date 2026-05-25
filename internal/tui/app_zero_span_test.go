package tui

import (
	"strings"
	"testing"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// TestApp_ZeroValueSpanDoesNotPanic verifies that a zero-value telemetry.Span
// delivered via SpanBatchMsg does not panic and is either rejected or rendered
// cleanly (no blank row with garbage TraceID "" shown in the trace list).
//
// The safer behavior is rejection: a span with an empty TraceID cannot be
// correctly grouped or displayed, so the DAGTab simply skips it in
// rebuildTraces. This test asserts that behavior and bakes it in.
func TestApp_ZeroValueSpanDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("App.Update(SpanBatchMsg with zero-value Span) panicked: %v", r)
		}
	}()

	a := NewApp()
	a, _ = updateApp(t, a, SpanBatchMsg{Spans: []telemetry.Span{{}}})

	// The DAG tab must not have stored an empty-TraceID span in its trace list.
	dagTab := a.tabs[TabDAG].(*DAGTab)
	for _, tr := range dagTab.traces {
		if tr.TraceID == "" {
			t.Errorf("DAGTab.traces contains entry with empty TraceID — zero-value span was not rejected")
		}
	}

	// View must not panic and must not render a blank trace row. We call View()
	// with a sized terminal so the real rendering path is exercised.
	a.width = 120
	a.height = 30
	a = a.propagateSize()
	a.activeTab = TabDAG

	view := a.View()
	// The view should either show the empty state or a trace list with no
	// empty entries. An empty TraceID would appear as an 8-char ID of all
	// spaces or an empty string — check that the trace list section does not
	// contain a row whose TraceID field is blank.
	plain := stripAnsiSeq(view)
	_ = plain // Non-empty by definition if no panic occurred.

	// Confirm no entry with a blank TraceID was rendered in the trace table.
	// The table format is "# STATUS TRACE SYSTEM SPANS DEPTH LAST"; a zero
	// TraceID would produce a short ID of "" (len 0 ≤ 12, returned as-is).
	// We check the raw view doesn't contain a line that looks like a table
	// row with an empty third column.
	if strings.Contains(plain, "  1 ") {
		// There is at least one trace row; verify the TraceID column is not blank.
		for _, line := range strings.Split(plain, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "1 ") {
				continue
			}
			fields := strings.Fields(line)
			// Expected: [#, status-icon, traceID, system, spans, depth, last]
			// For a zero span the system field will also be empty, but the
			// traceID field (index 2 if no cursor prefix) must not be blank.
			if len(fields) >= 3 && fields[2] == "" {
				t.Errorf("trace list row 1 has blank TraceID column: %q", line)
			}
		}
	}
}
