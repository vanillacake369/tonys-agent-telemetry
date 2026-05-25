package tui

import (
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// TestDAGFreeze_BigTraceRenderTime measures how long renderGraph takes
// for traces of various sizes. Regression guard for the user-reported
// "Enter freezes screen" issue.
func TestDAGFreeze_BigTraceRenderTime(t *testing.T) {
	for _, N := range []int{100, 500, 2000, 5000} {
		spans := generateLinearChain(N)
		d := NewDAGTab().SetSize(140, 40).(*DAGTab)
		tab, _ := d.Update(SpanBatchMsg{Spans: spans})
		d = tab.(*DAGTab)
		tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
		d = tab.(*DAGTab)

		start := time.Now()
		_ = d.renderGraph(136)
		elapsed := time.Since(start)
		t.Logf("N=%d: renderGraph took %v", N, elapsed)
		// Sentinel: if rendering >100ms, the TUI will feel frozen on j/k.
		if elapsed > 200*time.Millisecond {
			t.Errorf("renderGraph for N=%d took %v (>200ms — this freezes the UI)", N, elapsed)
		}
	}
}

// BenchmarkRenderGraph_5000Spans measures renderGraph cost for a 5000-span
// linear chain with cache-miss forced on every iteration. Without the
// cache-miss reset, the bench would report nanoseconds (cache hit path) and
// hide regressions in the actual render logic.
func BenchmarkRenderGraph_5000Spans(b *testing.B) {
	spans := generateLinearChain(5000)
	d := NewDAGTab().SetSize(140, 40).(*DAGTab)
	tab, _ := d.Update(SpanBatchMsg{Spans: spans})
	d = tab.(*DAGTab)
	tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d = tab.(*DAGTab)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Force cache miss so every iteration measures the full render path,
		// not the string-copy cache-hit path. graphCache and graphCacheKey are
		// the two fields that guard the cache in renderGraph (tab_dag.go).
		d.graphCache = ""
		d.graphCacheKey = ""
		_ = d.renderGraph(136)
	}
}

// BenchmarkRebuildTraces_5000Spans measures rebuildTraces cost for 5000 spans.
// Cache-miss discipline: rebuildTraces builds the trace list from scratch on
// every call (no internal cache), so no reset is required. The bench confirms
// that adding spans does not cause unbounded rebuild time.
func BenchmarkRebuildTraces_5000Spans(b *testing.B) {
	spans := generateLinearChain(5000)
	d := NewDAGTab().SetSize(140, 40).(*DAGTab)
	tab, _ := d.Update(SpanBatchMsg{Spans: spans})
	d = tab.(*DAGTab)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// rebuildTraces has no result cache — each call rebuilds d.traces from
		// d.spans in O(N). Force-clearing would have no effect; calling it
		// directly exercises the full computation.
		d.rebuildTraces()
	}
}

// generateLinearChain creates N spans forming a single chain (worst case
// for vertical layout because every span becomes a row).
func generateLinearChain(N int) []telemetry.Span {
	spans := make([]telemetry.Span, N)
	for i := 0; i < N; i++ {
		parent := ""
		if i > 0 {
			parent = fmt.Sprintf("s%d", i-1)
		}
		spans[i] = telemetry.Span{
			TraceID:      "t",
			SpanID:       fmt.Sprintf("s%d", i),
			ParentSpanID: parent,
			System:       "anthropic",
		}
	}
	return spans
}
