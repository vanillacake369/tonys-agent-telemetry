// Package recommender defines the canonical output type and evidence-enforcement
// policy for the Phase 2 recommender pipeline.
//
// The "no recommendation without evidence" rule is a GA gate assertion defined
// in PIVOT_PLAN.md (GA — B-layer MVP gate). Every [Recommendation] produced by
// any caller must carry a non-empty SignalID (which DAG signal triggered it) and
// a non-empty CatalogItemID (which catalog item is recommended). The
// [EnforceEvidence] function is the single enforcement point for this rule.
//
// This package is intentionally kept stub-sized for Phase 0 pre-staging: it
// defines only types and policy, not any actual recommendation logic. Phase 2
// will implement the recommender engine on top of these contracts.
package recommender
