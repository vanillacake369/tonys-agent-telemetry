package recommender

import "github.com/vanillacake369/tonys-agent-telemetry/internal/signal"

// MappingRule defines the candidate catalog tags for a given signal type.
// BaseTags are always included. Refinements are applied in order; the first
// matching EvidenceRefinement appends its ExtraTags and stops (first-match wins).
// This ensures that a single piece of evidence cannot trigger multiple overlapping
// refinement branches.
type MappingRule struct {
	BaseTags    []string
	Refinements []EvidenceRefinement
}

// EvidenceRefinement narrows the tag set based on a specific Evidence key/value
// pair. When the Evidence map contains Key with a string value equal to Value,
// ExtraTags are appended to the candidate tag list. The first matching
// refinement wins; subsequent refinements are not evaluated.
//
// For unused_installed_skill, a special Value of "*" means "any non-empty
// value"; the actual value from Evidence is used as the extra tag itself
// (see applyRefinements for handling).
type EvidenceRefinement struct {
	Key       string   // Evidence key to inspect, e.g. "tool_name"
	Value     string   // Exact match value, or "*" for any non-empty string
	ExtraTags []string // Tags appended when matched. If nil and Value=="*", the
	// matched evidence value itself is used as the sole extra tag.
}

// SignalMappings is the SSoT for all signal→tag rules. This table is the single
// place where signal types are mapped to catalog tag candidates. Phase 3 should
// extend this table when new SignalTypes are added; do not add mapping logic
// elsewhere (DRY + SSoT principles).
//
// Other subsystems (e.g., a debug CLI) may read this table directly to inspect
// the mapping rules without importing the engine.
var SignalMappings = map[signal.SignalType]MappingRule{
	// stalled_node: a leaf span ran too long without delegating to a child.
	// Base tags: performance + responsiveness.
	// Refinements: bash → shell; read/write/edit → file-io.
	signal.SignalStalledNode: {
		BaseTags: []string{"performance", "responsiveness"},
		Refinements: []EvidenceRefinement{
			{Key: "tool_name", Value: "bash", ExtraTags: []string{"shell"}},
			{Key: "tool_name", Value: "read", ExtraTags: []string{"file-io"}},
			{Key: "tool_name", Value: "write", ExtraTags: []string{"file-io"}},
			{Key: "tool_name", Value: "edit", ExtraTags: []string{"file-io"}},
		},
	},

	// duplicate_subagent_work: two sibling subagents performed overlapping work.
	// Base tags: orchestration + fan-out. No evidence-driven refinements needed
	// because the signal itself already captures the orchestration anti-pattern.
	signal.SignalDuplicateSubagentWork: {
		BaseTags: []string{"orchestration", "fan-out"},
	},

	// unused_installed_skill: an installed skill was never invoked.
	// Base tag: skill-utilization.
	// Refinement: the actual skill_name is added as an extra tag so the engine
	// can prefer the specific skill item in the catalog when it exists.
	// Value "*" with nil ExtraTags means: use the evidence value itself.
	signal.SignalUnusedInstalledSkill: {
		BaseTags: []string{"skill-utilization"},
		Refinements: []EvidenceRefinement{
			{Key: "skill_name", Value: "*", ExtraTags: nil},
		},
	},

	// failed_handoff: a tool call errored and was retried at the same depth.
	// Base tags: error-recovery + retry-patterns. No further refinements needed
	// for v0 — both base tags are precise enough to match retry/recovery catalog items.
	signal.SignalFailedHandoff: {
		BaseTags: []string{"error-recovery", "retry-patterns"},
	},
}

// applyRefinements returns the full candidate tag set for a rule given the
// evidence map from a specific signal. It starts with a copy of BaseTags, then
// iterates Refinements in order. The first matching refinement appends its
// ExtraTags (or, for Value=="*" with nil ExtraTags, the actual evidence value)
// and returns immediately (first-match semantics).
//
// This function is exported for use in tests and the debug CLI.
func applyRefinements(rule MappingRule, evidence map[string]any) []string {
	tags := make([]string, len(rule.BaseTags))
	copy(tags, rule.BaseTags)

	for _, ref := range rule.Refinements {
		val, ok := evidence[ref.Key]
		if !ok {
			continue
		}
		strVal, ok := val.(string)
		if !ok || strVal == "" {
			continue
		}

		if ref.Value == "*" {
			// Wildcard: use the actual evidence value as the extra tag.
			if ref.ExtraTags == nil {
				tags = append(tags, strVal)
			} else {
				tags = append(tags, ref.ExtraTags...)
			}
			return tags // first-match wins
		}

		if strVal == ref.Value {
			tags = append(tags, ref.ExtraTags...)
			return tags // first-match wins
		}
	}

	return tags
}
