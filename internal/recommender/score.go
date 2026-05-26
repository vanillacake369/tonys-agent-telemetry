package recommender

import "github.com/vanillacake369/tonys-agent-telemetry/internal/catalog"

// maturityBoostFactor is the score multiplier applied when an item's
// MaturityLevel is >= maturityBoostThreshold. A boosted score is capped at 1.0.
// Rationale: higher-maturity items have been validated more thoroughly in the
// upstream corpus and should be preferred when their tag relevance is otherwise
// equal to a lower-maturity item.
const (
	maturityBoostThreshold = 4
	maturityBoostFactor    = 1.1
)

// matchScore returns a Jaccard-like relevance score in [0, 1] measuring how
// well item.Tags overlaps with candidateTags.
//
// Formula (one line):
//
//	score = |intersect(item.Tags, candidateTags)| / |union(item.Tags, candidateTags)|
//
// Edge cases:
//   - Empty intersection → 0.
//   - Empty union (both slices nil/empty) → 0, not NaN.
//
// Maturity boost: if item.MaturityLevel >= 4, score is multiplied by 1.1 and
// capped at 1.0. This gives a gentle preference to well-validated catalog items
// when their raw Jaccard score is otherwise equal to a lower-maturity item.
func matchScore(item catalog.Item, candidateTags []string) float64 {
	// Build presence sets for both sides.
	itemSet := toSet(item.Tags)
	candidateSet := toSet(candidateTags)

	// Empty union guard — avoids division by zero.
	if len(itemSet) == 0 || len(candidateSet) == 0 {
		return 0
	}

	// Count intersection.
	intersect := 0
	for tag := range itemSet {
		if candidateSet[tag] {
			intersect++
		}
	}
	if intersect == 0 {
		return 0
	}

	// Union = |A| + |B| - |A ∩ B|.
	union := len(itemSet) + len(candidateSet) - intersect
	score := float64(intersect) / float64(union)

	// Apply maturity boost, capped at 1.0.
	if item.MaturityLevel >= maturityBoostThreshold {
		score *= maturityBoostFactor
		if score > 1.0 {
			score = 1.0
		}
	}

	return score
}

// toSet converts a string slice to a presence map. Duplicate values are
// deduplicated (each tag counts once), consistent with Jaccard set semantics.
func toSet(tags []string) map[string]bool {
	if len(tags) == 0 {
		return nil
	}
	m := make(map[string]bool, len(tags))
	for _, t := range tags {
		m[t] = true
	}
	return m
}
