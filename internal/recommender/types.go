package recommender

import "time"

// Recommendation is the canonical output of the Phase 2 recommender.
// Both SignalID and CatalogItemID are MANDATORY: see PIVOT_PLAN GA gate
// "No recommendation without evidence". The EnforceEvidence policy in
// policy.go rejects any Recommendation lacking either citation.
type Recommendation struct {
	// SignalID is the citation identifying which extracted DAG signal triggered
	// this recommendation. Must be non-empty in every valid Recommendation.
	SignalID string

	// CatalogItemID is the citation identifying which catalog item (from the
	// Phase 1 ultimate-guide corpus) is being recommended. Must be non-empty
	// in every valid Recommendation.
	CatalogItemID string

	// Title is a short human-readable name for the recommended action.
	Title string

	// Reasoning is a human-readable explanation of why this recommendation was
	// generated, tying the signal evidence to the catalog item.
	Reasoning string

	// Score is a normalised confidence value in [0, 1].
	Score float64

	// CreatedAt is the wall-clock time at which the recommendation was produced.
	CreatedAt time.Time
}
