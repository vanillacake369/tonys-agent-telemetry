package recommender

import "fmt"

// EnforceEvidence returns a non-nil error if any Recommendation in recs is
// missing SignalID or CatalogItemID. This is the runtime and test guard for
// the GA gate "no recommendation without evidence" (PIVOT_PLAN.md, GA section).
//
// Use this in any code path that produces recommendations before they are
// stored, rendered, or returned to callers. A single violation causes the
// entire batch to be rejected so that partial-citation batches cannot silently
// reach production.
func EnforceEvidence(recs []Recommendation) error {
	for i, r := range recs {
		if r.SignalID == "" && r.CatalogItemID == "" {
			return fmt.Errorf(
				"recommendation[%d] %q: missing both SignalID and CatalogItemID — "+
					"every recommendation must cite a DAG signal and a catalog item (PIVOT_PLAN GA gate)",
				i, r.Title,
			)
		}
		if r.SignalID == "" {
			return fmt.Errorf(
				"recommendation[%d] %q: missing SignalID — "+
					"every recommendation must cite the DAG signal that triggered it (PIVOT_PLAN GA gate)",
				i, r.Title,
			)
		}
		if r.CatalogItemID == "" {
			return fmt.Errorf(
				"recommendation[%d] %q: missing CatalogItemID — "+
					"every recommendation must cite the catalog item it points to (PIVOT_PLAN GA gate)",
				i, r.Title,
			)
		}
	}
	return nil
}

// FilterEvidenced returns only the Recommendations that have both SignalID and
// CatalogItemID populated. Use this in production paths where partial output is
// acceptable (e.g., rendering the best available results while logging the
// stripped violators for investigation). The returned slice preserves the
// original order.
func FilterEvidenced(recs []Recommendation) []Recommendation {
	out := make([]Recommendation, 0, len(recs))
	for _, r := range recs {
		if r.SignalID != "" && r.CatalogItemID != "" {
			out = append(out, r)
		}
	}
	return out
}
