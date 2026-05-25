package signal

import (
	"sort"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// detectFailedHandoff implements the failed_handoff detector per SIGNALS_SPEC §3.4.
//
// Detects: a span with Status=="error" that has a sibling span (same parent) with
// the same gen_ai.tool.name AND whose StartTime is after the error span's EndTime.
//
// Clock skew note (OQ-6): the After() check is strict per SIGNALS_SPEC §3.4 E5.
// ClockSkewTolerance is NOT applied here to avoid treating concurrent calls as retries.
//
// Edge cases handled: E1 (in-progress trace; retry span may lack EndTime — still
// detected by StartTime), E2 (orphan roots never siblings), E3 (error span with
// zero EndTime skipped), E4 (single-span trace; no siblings), E5 (strict After
// check; no skew tolerance), E6 (multiple failed attempts each independently
// checked against subsequent siblings), E7 (tool-name-only key; arg discrimination
// deferred to Phase 2).
func detectFailedHandoff(traceID string, roots []*telemetry.SpanNode, opts ExtractOpts, now time.Time) []Signal {
	var signals []Signal

	depthFirstWalk(roots, func(n *telemetry.SpanNode) {
		if len(n.Children) < 2 {
			return
		}

		// Group children by tool name for efficient lookup.
		byTool := make(map[string][]*telemetry.SpanNode)
		for _, child := range n.Children {
			name := toolName(child.Span)
			byTool[name] = append(byTool[name], child)
		}

		// Iterate tool names in sorted order for determinism.
		toolNames := make([]string, 0, len(byTool))
		for tn := range byTool {
			toolNames = append(toolNames, tn)
		}
		sortStrings(toolNames)

		for _, tn := range toolNames {
			group := byTool[tn]
			if len(group) < 2 {
				continue
			}

			for _, errorSpan := range group {
				if errorSpan.Span.Status != "error" {
					continue
				}
				// E3: error span without EndTime — skip.
				if errorSpan.Span.EndTime.IsZero() {
					continue
				}

				// Find the first temporal sibling that starts after errorSpan ends.
				for _, retrySpan := range group {
					if retrySpan == errorSpan {
						continue
					}
					if retrySpan.Span.StartTime.IsZero() {
						continue
					}
					if !retrySpan.Span.StartTime.After(errorSpan.Span.EndTime) {
						continue
					}

					gapMs := retrySpan.Span.StartTime.Sub(errorSpan.Span.EndTime).Milliseconds()
					errType := errorSpan.Span.Attrs["error.type"]
					retryAlsoErrored := retrySpan.Span.Status == "error"

					conf := confidenceFailedHandoff(errType, retryAlsoErrored)

					spanIDs := []string{errorSpan.Span.SpanID, retrySpan.Span.SpanID}
					sig := Signal{
						Type:    SignalFailedHandoff,
						TraceID: traceID,
						SpanIDs: spanIDs,
						Evidence: map[string]any{
							"tool_name":          tn,
							"error_span_id":      errorSpan.Span.SpanID,
							"retry_span_id":      retrySpan.Span.SpanID,
							"parent_span_id":     n.Span.SpanID,
							"error_type":         errType,
							"gap_ms":             gapMs,
							"retry_also_errored": retryAlsoErrored,
						},
						Confidence:   conf,
						EmittedAt:    now,
						ProviderTier: ProviderTierFull,
					}
					sig.ID = deriveID(sig.Type, sig.TraceID, sig.SpanIDs)
					signals = append(signals, sig)
					break // one retry signal per error span; take first temporal match
				}
			}
		}
	})

	return signals
}

// confidenceFailedHandoff computes the confidence score per SIGNALS_SPEC §3.4.
//
//	Base: 0.7
//	+0.2 if error_type is non-empty
//	+0.1 if retry_also_errored
//	Cap at 1.0
func confidenceFailedHandoff(errorType string, retryAlsoErrored bool) float64 {
	conf := failedHandoffBaseConfidence
	if errorType != "" {
		conf += failedHandoffErrorTypeBoost
	}
	if retryAlsoErrored {
		conf += failedHandoffRetryAlsoErroredBoost
	}
	return minFloat64(1.0, conf)
}

// sortStrings sorts a string slice in place.
func sortStrings(s []string) {
	sort.Strings(s)
}
