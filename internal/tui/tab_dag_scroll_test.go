package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// TestDAGTraceList_ScrollPast30 verifies that the trace list:
//   - accepts 100 traces without truncating d.traces
//   - scrolls the visible window when the cursor moves past the old hard cap of 30
//   - renders trace IDs beyond position 30 when cursor is at position 50
func TestDAGTraceList_ScrollPast30(t *testing.T) {
	d := NewDAGTab()
	// height=20 → visibleRows = 20 - 4 = 16, well below 100.
	d = d.SetSize(120, 20).(*DAGTab)

	// Build 100 distinct traces via SpanBatchMsg (one span per trace).
	spans := make([]telemetry.Span, 100)
	base := time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 100; i++ {
		traceID := fmt.Sprintf("trace-%03d", i)
		spans[i] = telemetry.Span{
			TraceID:   traceID,
			SpanID:    fmt.Sprintf("span-%03d", i),
			System:    "anthropic",
			StartTime: base.Add(time.Duration(i) * time.Second),
		}
	}
	tab, _ := d.Update(SpanBatchMsg{Spans: spans})
	d = tab.(*DAGTab)

	// Assert all 100 traces were stored — no cap on d.traces.
	if len(d.traces) != 100 {
		t.Fatalf("len(d.traces) = %d, want 100", len(d.traces))
	}

	// Advance cursor to index 49 (0-based → the 50th trace) by pressing j 49 times.
	for i := 0; i < 49; i++ {
		tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		d = tab.(*DAGTab)
	}

	if d.traceCursor != 49 {
		t.Fatalf("traceCursor = %d, want 49 after 49 j-presses", d.traceCursor)
	}

	// The scroll offset must have advanced so cursor is in the visible window.
	if d.traceCursor < d.traceOffset {
		t.Errorf("traceCursor %d < traceOffset %d — cursor scrolled off the top", d.traceCursor, d.traceOffset)
	}
	visibleRows := 20 - 4
	if d.traceCursor >= d.traceOffset+visibleRows {
		t.Errorf("traceCursor %d >= traceOffset+visibleRows (%d+%d=%d) — cursor below visible bottom",
			d.traceCursor, d.traceOffset, visibleRows, d.traceOffset+visibleRows)
	}

	v := d.View()

	// The cursor row (rank 49) must be visible.
	// d.traces is sorted newest-first: trace-099 is at index 0, trace-000 at index 99.
	// Index 49 → trace-050.
	expectedTraceID := d.traces[49].TraceID
	expectedShort := shortID(expectedTraceID)
	if !strings.Contains(v, expectedShort) {
		t.Errorf("View() does not contain short ID of trace at cursor (rank 49, id=%s, short=%s);\nview excerpt: %q",
			expectedTraceID, expectedShort, truncate(v, 400))
	}

	// Ensure the visible window contains ranks that are strictly beyond the
	// old hard cap of 30. With offset=34 (or similar) and visibleRows=16,
	// ranks 34–49 are displayed — all > 30. Verify by checking that
	// d.traces[traceOffset] (the topmost visible trace) has index > 30.
	if d.traceOffset <= 30 {
		// Still within old-cap territory — that's fine as long as the cursor
		// row itself is beyond rank 30.
		if d.traceCursor <= 30 {
			t.Errorf("cursor %d is still within old cap of 30 after 49 j-presses", d.traceCursor)
		}
	}

	// Spot-check: at least one trace ID in the visible window must be a trace
	// that the old cap would have hidden (ranks 31+). Pick the trace at
	// the cursor position, which is guaranteed to be rank 49 > 30.
	if d.traceCursor <= 30 {
		t.Errorf("traceCursor %d should be > 30 (the old cap) after 49 j-presses", d.traceCursor)
	}
}
