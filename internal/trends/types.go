// Package trends provides aggregation over persisted Signal snapshots
// for longitudinal ("month-over-month") trend display.
//
// This package is the contract surface for Phase κ (time-series MVP).
// The Bucket type defines what the aggregator produces and what the
// Trends tab UI renders. Implementation lives in aggregate.go.
package trends

import (
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
)

// Bucket is a single time-window summary of signals observed during
// [Start, Start+Duration). One Bucket per displayed time window.
//
// Counts is keyed by SignalType (per SIGNALS_SPEC) so downstream code can
// render "stalled_node: 12 → 8 ▼" style trend lines.
type Bucket struct {
	Start    time.Time
	Duration time.Duration
	Counts   map[signal.SignalType]int

	// Sessions is the number of distinct session IDs that contributed to this bucket.
	// Used by the UI to label sparsity ("3 sessions in this period").
	Sessions int
}

// IsEmpty reports whether the bucket has no contributing data.
func (b Bucket) IsEmpty() bool {
	return b.Sessions == 0 && len(b.Counts) == 0
}

// Total returns the sum of all signal counts in this bucket.
func (b Bucket) Total() int {
	n := 0
	for _, c := range b.Counts {
		n += c
	}
	return n
}
