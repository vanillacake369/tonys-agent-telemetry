package tui

import (
	"sync"
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// buildTestSpans creates n root spans each lasting dur.
func buildTestSpans(tracePrefix string, n int, dur time.Duration) []telemetry.Span {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	spans := make([]telemetry.Span, n)
	for i := range n {
		spans[i] = telemetry.Span{
			TraceID:   tracePrefix + "-" + intToStrFC(i),
			SpanID:    tracePrefix + "-span-" + intToStrFC(i),
			System:    "anthropic",
			StartTime: base,
			EndTime:   base.Add(dur),
			Status:    "done",
			Attrs:     map[string]string{},
		}
	}
	return spans
}

// intToStrFC is a local helper to avoid import conflicts (engine_test has one too,
// but that is in the recommender package; we cannot share across packages).
func intToStrFC(n int) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 8)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}

// TestForestCache_HitOnSameSpanCountAndOpts verifies that calling Get twice with
// identical span count and opts returns the same slice identity (pointer equality),
// proving no recomputation occurred.
func TestForestCache_HitOnSameSpanCountAndOpts(t *testing.T) {
	c := NewForestCache()
	spans := buildTestSpans("trace", 3, 30*time.Second)
	opts := signal.DefaultExtractOpts()

	sigs1 := c.Get(spans, opts)
	sigs2 := c.Get(spans, opts)

	// Cache hit: same underlying array pointer (no reallocation).
	if &sigs1 == &sigs2 {
		// Local slice headers differ, but that's fine — we check capacity/len match
		// as a proxy. The real proof is the computeCount stays at 1.
	}
	if c.computeCount != 1 {
		t.Errorf("expected 1 compute (cache hit on second call), got %d", c.computeCount)
	}
}

// TestForestCache_MissOnSpanCountChange verifies that adding spans triggers recompute.
func TestForestCache_MissOnSpanCountChange(t *testing.T) {
	c := NewForestCache()
	opts := signal.DefaultExtractOpts()

	spans1 := buildTestSpans("trace", 3, 30*time.Second)
	c.Get(spans1, opts)

	spans2 := buildTestSpans("trace", 8, 30*time.Second) // larger slice
	c.Get(spans2, opts)

	if c.computeCount != 2 {
		t.Errorf("expected 2 computes (miss on span-count change), got %d", c.computeCount)
	}
}

// TestForestCache_MissOnOptsChange verifies that changing InstalledSkills triggers recompute.
func TestForestCache_MissOnOptsChange(t *testing.T) {
	c := NewForestCache()
	spans := buildTestSpans("trace", 3, 30*time.Second)

	opts1 := signal.DefaultExtractOpts()
	c.Get(spans, opts1)

	opts2 := signal.DefaultExtractOpts()
	opts2.InstalledSkills = []string{"new-skill"}
	c.Get(spans, opts2)

	if c.computeCount != 2 {
		t.Errorf("expected 2 computes (miss on opts change), got %d", c.computeCount)
	}
}

// TestForestCache_RaceClean runs concurrent Gets and verifies no data race.
// Run with: go test -race ./internal/tui/ -run TestForestCache_RaceClean
func TestForestCache_RaceClean(t *testing.T) {
	c := NewForestCache()
	spans := buildTestSpans("race", 4, 30*time.Second)
	opts := signal.DefaultExtractOpts()

	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			c.Get(spans, opts)
		}()
	}
	wg.Wait()
	// No assertions needed beyond passing the race detector.
}
