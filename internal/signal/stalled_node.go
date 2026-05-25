package signal

import (
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// detectStalledNode implements the stalled_node detector per SIGNALS_SPEC §3.1.
//
// A leaf span (no children) whose wall-clock duration, adjusted for clock skew,
// exceeds opts.StallThreshold is considered stalled. The containing trace must
// have at least one completed span (traceEndTime non-zero); otherwise the entire
// trace is skipped (edge case E1).
//
// Edge cases handled: E1 (in-progress trace), E2 (orphan roots), E3 (zero/neg
// duration), E4 (single-span trace), E5 (clock skew produces negative adjusted),
// E6 (multiple orphan roots), E7 (zero StartTime).
func detectStalledNode(traceID string, roots []*telemetry.SpanNode, opts ExtractOpts, now time.Time) []Signal {
	// Note A: if no span in the trace has completed, skip all stall checks.
	traceEnd := maxEndTime(roots)
	if traceEnd.IsZero() {
		return nil
	}

	var signals []Signal

	depthFirstWalk(roots, func(n *telemetry.SpanNode) {
		// Not a leaf.
		if len(n.Children) > 0 {
			return
		}
		s := n.Span

		// E1 guard: span still running.
		if s.EndTime.IsZero() {
			return
		}
		// E7: invalid StartTime.
		if s.StartTime.IsZero() {
			return
		}
		// E3: zero or negative raw duration.
		rawDuration := s.EndTime.Sub(s.StartTime)
		if rawDuration <= 0 {
			return
		}
		// E5: clock skew adjustment.
		adjustedDuration := rawDuration - opts.ClockSkewTolerance
		if adjustedDuration <= opts.StallThreshold {
			return
		}

		// confidence = min(1.0, adjustedDuration / (2 * StallThreshold))
		conf := minFloat64(1.0, float64(adjustedDuration)/float64(2*opts.StallThreshold))

		spanIDs := []string{s.SpanID}
		sig := Signal{
			Type:    SignalStalledNode,
			TraceID: traceID,
			SpanIDs: spanIDs,
			Evidence: map[string]any{
				"stall_duration_ms": adjustedDuration.Milliseconds(),
				"threshold_ms":      opts.StallThreshold.Milliseconds(),
				"parent_span_id":    s.ParentSpanID,
				"tool_name":         toolName(s),
				"system":            s.System,
			},
			Confidence:   conf,
			EmittedAt:    now,
			ProviderTier: ProviderTierFull,
		}
		sig.ID = deriveID(sig.Type, sig.TraceID, sig.SpanIDs)
		signals = append(signals, sig)
	})

	return signals
}
