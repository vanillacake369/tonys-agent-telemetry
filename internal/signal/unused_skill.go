package signal

import (
	"sort"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// detectUnusedInstalledSkill implements the unused_installed_skill detector
// per SIGNALS_SPEC §3.3.
//
// The detector is a pure function: it does no filesystem I/O. The caller
// must populate opts.InstalledSkills before calling Extract (e.g. by calling
// skill.ScanLocal() and extracting the Name fields).
//
// Edge cases handled: E1 (below MinSessions gate), E2 (in-progress spans
// still contribute tool names), E3 (orphan roots walked normally), E4
// (single-span traces included), E5 (duplicate skill names in InstalledSkills
// de-duplicated before iterating), E7 (empty InstalledSkills → return early).
func detectUnusedInstalledSkill(forest Forest, opts ExtractOpts, now time.Time) []Signal {
	// E7: nothing to check.
	if len(opts.InstalledSkills) == 0 {
		return nil
	}
	// E1: insufficient sample.
	if len(forest) < opts.MinSessionsForUnusedSkill {
		return nil
	}

	// Build the set of all tool names actually invoked across ALL traces.
	invoked := make(map[string]struct{})
	for _, traceID := range sortedTraceIDs(forest) {
		roots := forest[traceID]
		depthFirstWalk(roots, func(n *telemetry.SpanNode) {
			if name := toolName(n.Span); name != "" {
				invoked[name] = struct{}{}
			}
		})
	}

	// Collect sorted invoked tool names for the evidence payload.
	invokedList := make([]string, 0, len(invoked))
	for name := range invoked {
		invokedList = append(invokedList, name)
	}
	sort.Strings(invokedList)

	// E5: de-duplicate InstalledSkills before iterating.
	seen := make(map[string]struct{}, len(opts.InstalledSkills))
	unique := make([]string, 0, len(opts.InstalledSkills))
	for _, skill := range opts.InstalledSkills {
		if _, ok := seen[skill]; !ok {
			seen[skill] = struct{}{}
			unique = append(unique, skill)
		}
	}
	sort.Strings(unique) // deterministic emission order

	var signals []Signal
	sessionsChecked := len(forest)
	minSessions := opts.MinSessionsForUnusedSkill

	for _, skillName := range unique {
		if _, ok := invoked[skillName]; ok {
			continue
		}
		conf := minFloat64(1.0, float64(sessionsChecked)/float64(3*minSessions))
		sig := Signal{
			Type:    SignalUnusedInstalledSkill,
			TraceID: "", // cross-trace
			SpanIDs: []string{},
			Evidence: map[string]any{
				"skill_name":         skillName,
				"sessions_checked":   sessionsChecked,
				"min_sessions":       minSessions,
				"invoked_tool_names": invokedList,
			},
			Confidence:   conf,
			EmittedAt:    now,
			ProviderTier: ProviderTierFull,
		}
		sig.ID = deriveID(sig.Type, sig.TraceID, []string{skillName})
		signals = append(signals, sig)
	}

	return signals
}
