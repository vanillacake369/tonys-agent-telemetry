package recommender

import (
	"fmt"
	"sort"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/catalog"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
)

const (
	// defaultThreshold is the minimum matchScore an item must reach to appear
	// in the output. Items below this are filtered out silently.
	defaultThreshold = 0.2

	// defaultMaxPerSignal is the maximum number of catalog items recommended per
	// signal. After scoring, items are sorted by score DESC and top-K are kept.
	defaultMaxPerSignal = 3
)

// Engine is the Phase 2 recommender orchestrator. It is stateless and pure:
// given the same inputs it always produces byte-identical output (determinism
// contract from SIGNALS_SPEC §5, applied to the recommendation layer).
//
// GA gate: Engine.Recommend panics if EnforceEvidence detects a violation.
// The only path that would produce a missing citation is a Signal with an empty
// ID field. The engine guards against this at the top of the signal loop and
// skips such signals — making the EnforceEvidence panic path unreachable in
// practice. This dual-layer defence is documented here so future contributors
// understand the intent: the guard + panic combination means neither a bad
// signal nor a bug in the engine can silently ship a citation-free recommendation.
type Engine struct {
	// Threshold is the minimum matchScore for an item to be included in output.
	// Default: 0.2. Values outside [0, 1] are clamped at construction.
	Threshold float64

	// MaxPerSignal is the maximum number of recommendations emitted per signal.
	// Default: 3. Non-positive values are treated as 1.
	MaxPerSignal int
}

// NewEngine returns an Engine with default configuration.
func NewEngine() *Engine {
	return &Engine{
		Threshold:    defaultThreshold,
		MaxPerSignal: defaultMaxPerSignal,
	}
}

// Recommend takes raw signals and the loaded catalog and returns a deterministic
// slice of Recommendations. Each Recommendation carries both citations:
// SignalID (which signal triggered it) and CatalogItemID (which catalog item is
// recommended). Items that do not pass IsValid() are ignored.
//
// Algorithm (step by step):
//  1. For each signal: skip if Signal.ID is empty (GA gate — see type comment).
//  2. Lookup SignalMappings[signal.Type] and build candidate tags (BaseTags +
//     first matching refinement's ExtraTags from signal.Evidence).
//  3. Score each Item against candidate tags; keep those above Threshold.
//  4. Sort matching Items by score DESC, take top MaxPerSignal.
//  5. For each surviving (signal, item) pair emit a Recommendation.
//  6. Sort output deterministically: score DESC, then CatalogItemID ASC.
//  7. Run EnforceEvidence — any violation panics (programmer error, not runtime).
func (e *Engine) Recommend(signals []signal.Signal, items []catalog.Item) []Recommendation {
	maxK := e.MaxPerSignal
	if maxK <= 0 {
		maxK = 1
	}

	var out []Recommendation
	now := time.Now()

	for _, sig := range signals {
		// GA gate: skip signals with empty IDs. A signal with no ID cannot
		// produce a valid Recommendation (SignalID would be empty, violating the
		// "no recommendation without evidence" policy). Skipping here makes the
		// EnforceEvidence panic path unreachable from valid engine usage.
		if sig.ID == "" {
			continue
		}

		rule, ok := SignalMappings[sig.Type]
		if !ok {
			// Unknown signal type — no mapping defined; produce nothing.
			continue
		}

		candidateTags := applyRefinements(rule, sig.Evidence)

		// Score all valid items.
		type scored struct {
			item  catalog.Item
			score float64
		}
		var candidates []scored
		for _, item := range items {
			if !item.IsValid() {
				continue
			}
			s := matchScore(item, candidateTags)
			if s < e.Threshold {
				continue
			}
			candidates = append(candidates, scored{item, s})
		}

		// Sort: score DESC, then item.ID ASC for tie-breaking determinism.
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].score != candidates[j].score {
				return candidates[i].score > candidates[j].score
			}
			return candidates[i].item.ID < candidates[j].item.ID
		})

		// Take top-K.
		if len(candidates) > maxK {
			candidates = candidates[:maxK]
		}

		for _, c := range candidates {
			rec := Recommendation{
				SignalID:      sig.ID,
				CatalogItemID: c.item.ID,
				Title:         c.item.Title,
				Reasoning:     humanReadable(sig, c.item, candidateTags, c.score),
				Score:         c.score,
				CreatedAt:     now,
			}
			out = append(out, rec)
		}
	}

	// Sort output deterministically: score DESC, then CatalogItemID ASC.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].CatalogItemID < out[j].CatalogItemID
	})

	// EnforceEvidence: a violation here is a programmer error. The guard above
	// (skip empty Signal.ID) makes this path unreachable under correct usage.
	// Panic immediately so the bug surfaces in testing, not silently in production.
	if err := EnforceEvidence(out); err != nil {
		panic(fmt.Sprintf("recommender.Engine.Recommend: EnforceEvidence contract violation — %v", err))
	}

	return out
}

// humanReadable constructs a human-readable reasoning string for a Recommendation
// tying the signal evidence to the catalog item and the matched tags.
func humanReadable(sig signal.Signal, item catalog.Item, candidateTags []string, score float64) string {
	return fmt.Sprintf(
		"Signal %q (%s) matched catalog item %q via tags %v (score=%.3f). "+
			"Consider adopting this pattern to address the detected behavioral issue.",
		sig.ID, sig.Type, item.Title, candidateTags, score,
	)
}
