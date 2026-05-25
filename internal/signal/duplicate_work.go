package signal

import (
	"sort"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// detectDuplicateSubagentWork implements the duplicate_subagent_work detector
// per SIGNALS_SPEC §3.2.
//
// Complexity: O(K·M) where K is the number of siblings under each parent and
// M is the max tool-list length, achieved by:
//   1. Precomputing a multiset count-map for each sibling's tool sequence once: O(K·M).
//   2. For each pair (i,j), computing intersection/union from the precomputed maps
//      in O(min(|seqA|,|seqB|)) using one map-walk per pair: total O(K²·T) where
//      T = distinct tool count. In practice K²·T << K·M for realistic fan-outs.
//
// This avoids the naïve O(K²·M) approach of re-walking both subtrees per pair.
//
// Edge cases handled: E2 (orphan roots not siblings), E3 (empty sequences),
// E5 (fan-out N>2 — every pair evaluated independently).
func detectDuplicateSubagentWork(traceID string, roots []*telemetry.SpanNode, opts ExtractOpts, now time.Time) []Signal {
	var signals []Signal

	depthFirstWalk(roots, func(n *telemetry.SpanNode) {
		if len(n.Children) < 2 {
			return
		}
		children := n.Children

		// Phase 1: precompute tool multisets for each child — O(K·M).
		// Only non-empty tool names are included in the sequence and multiset.
		// Spans without gen_ai.tool.name are structural containers; they
		// contribute no signal to the duplicate-work comparison.
		type childData struct {
			node     *telemetry.SpanNode
			seq      []string // non-empty tool names only (DFS pre-order)
			multiset map[string]int
		}
		data := make([]childData, len(children))
		for i, child := range children {
			rawSeq := toolSequence(child)
			// Filter out empty strings — spans with no tool name do not
			// contribute to the Jaccard comparison (spec E3: "no tool calls").
			filtered := make([]string, 0, len(rawSeq))
			for _, s := range rawSeq {
				if s != "" {
					filtered = append(filtered, s)
				}
			}
			data[i] = childData{
				node:     child,
				seq:      filtered,
				multiset: buildMultiset(filtered),
			}
		}

		// Phase 2: evaluate each pair — O(K²·T), T = distinct tool count.
		for i := 0; i < len(data); i++ {
			for j := i + 1; j < len(data); j++ {
				a, b := data[i], data[j]

				// E3: both empty → skip.
				if len(a.seq) == 0 && len(b.seq) == 0 {
					continue
				}

				sim := jaccardSimilarity(a.multiset, b.multiset)
				if sim < opts.DupOverlapThreshold {
					continue
				}

				overlap := intersectionKeys(a.multiset, b.multiset)
				sort.Strings(overlap)

				spanIDs := []string{a.node.Span.SpanID, b.node.Span.SpanID}
				sig := Signal{
					Type:    SignalDuplicateSubagentWork,
					TraceID: traceID,
					SpanIDs: spanIDs,
					Evidence: map[string]any{
						"jaccard_similarity": sim,
						"threshold":          opts.DupOverlapThreshold,
						"tools_a":            a.seq,
						"tools_b":            b.seq,
						"parent_span_id":     n.Span.SpanID,
						"overlap_tool_names": overlap,
					},
					Confidence:   sim,
					EmittedAt:    now,
					ProviderTier: ProviderTierFull,
				}
				sig.ID = deriveID(sig.Type, sig.TraceID, sig.SpanIDs)
				signals = append(signals, sig)
			}
		}
	})

	return signals
}

// buildMultiset builds a frequency map from a tool-name sequence.
// O(M) per call.
func buildMultiset(seq []string) map[string]int {
	m := make(map[string]int, len(seq))
	for _, t := range seq {
		m[t]++
	}
	return m
}

// jaccardSimilarity computes multiset Jaccard: |intersection| / |union|.
// Uses the precomputed frequency maps so no sequence re-walk is needed.
//
// Multiset intersection count for key k: min(countA[k], countB[k]).
// Multiset union count for key k: max(countA[k], countB[k]).
// Complexity: O(|A| + |B|) where |A|,|B| are the number of distinct keys.
func jaccardSimilarity(a, b map[string]int) float64 {
	var intersection, union int

	// Walk a; for each key, compute contribution.
	for k, ca := range a {
		cb := b[k]
		if ca < cb {
			intersection += ca
		} else {
			intersection += cb
		}
		if ca > cb {
			union += ca
		} else {
			union += cb
		}
	}
	// Walk b for keys not in a.
	for k, cb := range b {
		if _, ok := a[k]; !ok {
			union += cb
		}
	}

	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// intersectionKeys returns a deduplicated list of tool names present in both
// multisets (SIGNALS_SPEC §3.2 evidence field "overlap_tool_names").
func intersectionKeys(a, b map[string]int) []string {
	var overlap []string
	for k := range a {
		if b[k] > 0 {
			overlap = append(overlap, k)
		}
	}
	return overlap
}
