package signal

import (
	"sort"
	"time"
)

// Extract is the sole entry point for the Signal Extractor v0.
//
// It is a pure function: no I/O, no global state, no goroutines. All signals
// share the same EmittedAt timestamp, captured once at the start of the call
// (SIGNALS_SPEC §5 determinism contract).
//
// Output is sorted deterministically by (TraceID, SpanIDs[0], Type) ascending.
// Signals with empty TraceID (unused_installed_skill) sort after all trace-scoped
// signals, then by Evidence["skill_name"] (SIGNALS_SPEC §5).
func Extract(forest Forest, opts ExtractOpts) []Signal {
	now := time.Now()

	var signals []Signal

	// Run per-trace detectors in sorted key order (SIGNALS_SPEC §5 no-randomness).
	for _, traceID := range sortedTraceIDs(forest) {
		roots := forest[traceID]
		signals = append(signals, detectStalledNode(traceID, roots, opts, now)...)
		signals = append(signals, detectDuplicateSubagentWork(traceID, roots, opts, now)...)
		signals = append(signals, detectFailedHandoff(traceID, roots, opts, now)...)
	}

	// Run forest-wide detector once (requires global view).
	signals = append(signals, detectUnusedInstalledSkill(forest, opts, now)...)

	sortSignals(signals)
	return signals
}

// sortSignals sorts the signal slice in place per SIGNALS_SPEC §5:
//
//  1. Trace-scoped signals (TraceID != "") sorted by (TraceID, SpanIDs[0], Type).
//  2. Cross-trace signals (TraceID == "") sorted by Evidence["skill_name"],
//     and they always sort after all trace-scoped signals.
func sortSignals(signals []Signal) {
	sort.SliceStable(signals, func(i, j int) bool {
		a, b := signals[i], signals[j]

		// Cross-trace signals sort after trace-scoped.
		aEmpty := a.TraceID == ""
		bEmpty := b.TraceID == ""
		if aEmpty != bEmpty {
			return !aEmpty // trace-scoped first
		}

		// Both cross-trace: sort by skill_name.
		if aEmpty && bEmpty {
			aSkill, _ := a.Evidence["skill_name"].(string)
			bSkill, _ := b.Evidence["skill_name"].(string)
			return aSkill < bSkill
		}

		// Both trace-scoped: sort by (TraceID, SpanIDs[0], Type).
		if a.TraceID != b.TraceID {
			return a.TraceID < b.TraceID
		}
		aSpan0 := firstSpanID(a)
		bSpan0 := firstSpanID(b)
		if aSpan0 != bSpan0 {
			return aSpan0 < bSpan0
		}
		return string(a.Type) < string(b.Type)
	})
}

// firstSpanID returns SpanIDs[0] or "" if the slice is empty.
func firstSpanID(s Signal) string {
	if len(s.SpanIDs) == 0 {
		return ""
	}
	return s.SpanIDs[0]
}
