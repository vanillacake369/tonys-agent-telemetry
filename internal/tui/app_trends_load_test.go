package tui

import (
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signalstore"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/trends"
)

// TestLoadTrendsCmd_UsesLiveSpansWhenStoreEmpty verifies that loadTrendsCmd
// produces non-empty buckets from live span data when the persisted store
// contains no entries. This is the regression test for the "9 sessions exist
// on disk but Trends tab shows nothing" bug: the live path must supply data
// before the 5-minute flush tick fires.
func TestLoadTrendsCmd_UsesLiveSpansWhenStoreEmpty(t *testing.T) {
	// Create a fresh, empty signal store (nothing persisted yet).
	dir := t.TempDir()
	store := signalstore.NewStoreAt(dir)

	a := NewApp()
	a.width, a.height = 80, 24
	a = a.propagateSize()
	// Wire in the empty store so loadTrendsCmd can reach it.
	a.trendsPersistence = NewTrendsPersistence(store)

	// Inject synthetic stalled-node spans into the DAG tab via the normal
	// routing path (SpanBatchMsg). Each span is a single-span trace lasting
	// 30s — well above the 10s stall threshold — so Extract emits a
	// stalled_node signal. Use times within the last 30 days so they land
	// inside the DefaultLookbackDays window.
	now := time.Now().UTC()
	makeStallSpan := func(traceID, spanID string, daysAgo int) telemetry.Span {
		end := now.AddDate(0, 0, -daysAgo)
		return telemetry.Span{
			TraceID:   traceID,
			SpanID:    spanID,
			System:    "anthropic",
			StartTime: end.Add(-30 * time.Second),
			EndTime:   end,
			Status:    "done",
			Attrs:     map[string]string{},
		}
	}

	// Nine traces to mirror the "9 sessions on disk" scenario.
	var spans []telemetry.Span
	for i := 1; i <= 9; i++ {
		traceID := "trace-live-" + string(rune('A'-1+i))
		spanID := "span-" + string(rune('A'-1+i))
		spans = append(spans, makeStallSpan(traceID, spanID, i))
	}

	updated, _ := a.Update(SpanBatchMsg{Spans: spans})
	a = updated.(App)

	// Execute loadTrendsCmd — this is what Tab6 ('6' key) triggers.
	cmd := a.loadTrendsCmd()
	if cmd == nil {
		t.Fatal("loadTrendsCmd returned nil cmd")
	}
	msg := cmd()

	loaded, ok := msg.(TrendsLoadedMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want TrendsLoadedMsg", msg)
	}

	// Assert at least one non-empty bucket so the Trends tab renders signal data.
	var nonEmpty int
	for _, b := range loaded.Buckets {
		if !b.IsEmpty() {
			nonEmpty++
		}
	}
	if nonEmpty == 0 {
		t.Errorf("expected at least one non-empty bucket from live spans, got 0 (all %d buckets empty); "+
			"this indicates live span data was not folded into the aggregation",
			len(loaded.Buckets))
	}

	// Sanity: the total bucket count should match the DefaultLookbackDays window.
	wantBuckets := trends.DefaultLookbackDays + 1 // ceiling division for rounded-now boundary
	if len(loaded.Buckets) < trends.DefaultLookbackDays || len(loaded.Buckets) > wantBuckets {
		t.Errorf("expected ~%d buckets for %d-day lookback, got %d",
			trends.DefaultLookbackDays, trends.DefaultLookbackDays, len(loaded.Buckets))
	}
}
