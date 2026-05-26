package trends

import (
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/signalstore"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// BuildLiveSnapshots groups spans by TraceID, extracts signals per trace,
// and returns one SessionSnapshot per trace with CapturedAt = max EndTime.
// Pure function — no I/O. The caller (App.loadTrendsCmd) merges these
// snapshots with the persisted store output before calling Aggregate.
//
// Empty input yields nil. Traces whose spans all have zero EndTime are
// skipped (no meaningful temporal anchor).
func BuildLiveSnapshots(
	spans []telemetry.Span,
	opts signal.ExtractOpts,
) []signalstore.SessionSnapshot {
	if len(spans) == 0 {
		return nil
	}

	// Group spans by TraceID preserving first-seen order for determinism.
	type traceData struct {
		spans  []telemetry.Span
		maxEnd time.Time
	}
	order := make([]string, 0)
	byTrace := make(map[string]*traceData)

	for _, s := range spans {
		td, ok := byTrace[s.TraceID]
		if !ok {
			td = &traceData{}
			byTrace[s.TraceID] = td
			order = append(order, s.TraceID)
		}
		td.spans = append(td.spans, s)
		if !s.EndTime.IsZero() && s.EndTime.After(td.maxEnd) {
			td.maxEnd = s.EndTime
		}
	}

	var snapshots []signalstore.SessionSnapshot

	for _, traceID := range order {
		td := byTrace[traceID]

		// Skip traces with no completed span — no meaningful temporal anchor.
		if td.maxEnd.IsZero() {
			continue
		}

		forest := telemetry.BuildForests(td.spans)
		sigs := signal.Extract(forest, opts)

		entry := signalstore.SnapshotEntry{
			CapturedAt: td.maxEnd,
			Signals:    sigs,
		}
		snapshots = append(snapshots, signalstore.SessionSnapshot{
			SessionID: traceID,
			Entries:   []signalstore.SnapshotEntry{entry},
		})
	}

	if len(snapshots) == 0 {
		return nil
	}
	return snapshots
}
