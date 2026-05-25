package tui

import (
	"os"
	"strconv"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

const (
	defaultSpanCap = 50_000
	// spanCapEnvVar is the environment variable that overrides the default cap.
	spanCapEnvVar = "TONYS_MAX_SPANS"
)

// resolveSpanCap returns the maximum number of spans to retain in TUI state.
// Default: 50_000. Override via env var TONYS_MAX_SPANS (positive integer).
// 0 or negative env value → use default (do not allow unbounded; that's the OOM
// we're preventing). Non-integer or unset values also fall back to default.
func resolveSpanCap() int {
	raw := os.Getenv(spanCapEnvVar)
	if raw == "" {
		return defaultSpanCap
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultSpanCap
	}
	return n
}

// capSpans returns spans pruned to at most n, keeping the most recent by EndTime.
// If a span has zero EndTime (in-progress), it is treated as "newest" so it
// always survives eviction.
//
// The providers emit spans in approximate time order, so the input slice is
// already roughly sorted. A simple tail-slice of the last n elements is
// therefore sufficient — no sort is required. If a full sort were needed it
// would be O(N log N) per batch arrival, which defeats the purpose of the cap
// on startup with tens of thousands of spans.
//
// In-progress spans (zero EndTime) are logically "now" and would naturally
// appear at the tail, so the tail-slice strategy preserves them without special
// handling in the common path. An explicit rescue pass below handles the rare
// case where an in-progress span arrived early (e.g. out-of-order backfill).
func capSpans(spans []telemetry.Span, n int) []telemetry.Span {
	if n <= 0 || len(spans) <= n {
		return spans
	}

	// Keep the most recent n spans (tail of the slice).
	retained := spans[len(spans)-n:]

	// Rescue any in-progress spans (zero EndTime) that were evicted from the
	// head. These are active spans that must not disappear from the UI.
	// Build a set of SpanIDs already in retained to avoid duplicates.
	inRetained := make(map[string]struct{}, len(retained))
	for _, s := range retained {
		inRetained[s.SpanID] = struct{}{}
	}

	var rescued []telemetry.Span
	evicted := spans[:len(spans)-n]
	for _, s := range evicted {
		if s.EndTime.IsZero() {
			if _, already := inRetained[s.SpanID]; !already {
				rescued = append(rescued, s)
			}
		}
	}

	if len(rescued) == 0 {
		// Fast path: no rescue needed; return a copy to avoid aliasing the
		// original backing array.
		out := make([]telemetry.Span, n)
		copy(out, retained)
		return out
	}

	// Prepend rescued in-progress spans ahead of the retained tail so the
	// caller sees them. The combined slice may exceed n by the rescue count,
	// which is intentional — in-progress spans are small in number and must
	// not be silently dropped.
	out := make([]telemetry.Span, 0, len(rescued)+len(retained))
	out = append(out, rescued...)
	out = append(out, retained...)
	return out
}
