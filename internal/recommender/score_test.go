package recommender

import (
	"math"
	"testing"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/catalog"
)

func TestMatchScore_EmptyBoth(t *testing.T) {
	item := catalog.Item{ID: "i1", Title: "T", Type: catalog.ItemTypeSkill}
	got := matchScore(item, nil)
	if got != 0 {
		t.Errorf("matchScore(empty item tags, nil candidates) = %v, want 0", got)
	}
}

func TestMatchScore_EmptyItemTags(t *testing.T) {
	item := catalog.Item{ID: "i1", Title: "T", Type: catalog.ItemTypeSkill, Tags: nil}
	got := matchScore(item, []string{"shell", "performance"})
	if got != 0 {
		t.Errorf("matchScore(nil item.Tags, non-empty candidates) = %v, want 0", got)
	}
}

func TestMatchScore_EmptyCandidates(t *testing.T) {
	item := catalog.Item{ID: "i1", Title: "T", Type: catalog.ItemTypeSkill, Tags: []string{"shell"}}
	got := matchScore(item, []string{})
	if got != 0 {
		t.Errorf("matchScore(non-empty item.Tags, empty candidates) = %v, want 0", got)
	}
}

func TestMatchScore_NoOverlap(t *testing.T) {
	item := catalog.Item{ID: "i1", Title: "T", Type: catalog.ItemTypeSkill, Tags: []string{"orchestration"}}
	got := matchScore(item, []string{"shell", "performance"})
	if got != 0 {
		t.Errorf("matchScore(no overlap) = %v, want 0", got)
	}
}

func TestMatchScore_PerfectOverlap(t *testing.T) {
	tags := []string{"shell", "performance"}
	item := catalog.Item{ID: "i1", Title: "T", Type: catalog.ItemTypeSkill, Tags: tags}
	got := matchScore(item, tags)
	if math.Abs(got-1.0) > 1e-9 {
		t.Errorf("matchScore(perfect overlap) = %v, want 1.0", got)
	}
}

func TestMatchScore_PartialOverlap(t *testing.T) {
	// item has {shell, performance, orchestration}, candidates = {shell, performance}
	// intersect = {shell, performance} = 2
	// union = {shell, performance, orchestration} = 3
	// Jaccard = 2/3 ≈ 0.6667
	item := catalog.Item{
		ID:    "i1",
		Title: "T",
		Type:  catalog.ItemTypeSkill,
		Tags:  []string{"shell", "performance", "orchestration"},
	}
	got := matchScore(item, []string{"shell", "performance"})
	want := 2.0 / 3.0
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("matchScore(partial overlap) = %v, want %v", got, want)
	}
}

func TestMatchScore_MaturityBoost_Applied(t *testing.T) {
	// Perfect overlap, maturity >= 4 → boosted score = min(1.0, 1.0 * 1.1) = 1.0
	tags := []string{"shell"}
	item := catalog.Item{
		ID:            "i1",
		Title:         "T",
		Type:          catalog.ItemTypeSkill,
		Tags:          tags,
		MaturityLevel: 4,
	}
	got := matchScore(item, tags)
	if math.Abs(got-1.0) > 1e-9 {
		t.Errorf("matchScore(perfect + maturity4) = %v, want 1.0 (capped)", got)
	}
}

func TestMatchScore_MaturityBoost_PartialOverlap(t *testing.T) {
	// Partial overlap: Jaccard = 1/2 = 0.5; maturity=4 → 0.5*1.1 = 0.55
	item := catalog.Item{
		ID:            "i1",
		Title:         "T",
		Type:          catalog.ItemTypeSkill,
		Tags:          []string{"shell", "other"},
		MaturityLevel: 4,
	}
	got := matchScore(item, []string{"shell"})
	want := 0.5 * 1.1
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("matchScore(partial + maturity4) = %v, want %v", got, want)
	}
}

func TestMatchScore_MaturityBoost_BelowThreshold(t *testing.T) {
	// Maturity < 4 → no boost; Jaccard = 1/2 = 0.5
	item := catalog.Item{
		ID:            "i1",
		Title:         "T",
		Type:          catalog.ItemTypeSkill,
		Tags:          []string{"shell", "other"},
		MaturityLevel: 3,
	}
	got := matchScore(item, []string{"shell"})
	want := 0.5
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("matchScore(partial + maturity3) = %v, want %v (no boost)", got, want)
	}
}

func TestMatchScore_MaturityBoost_Zero(t *testing.T) {
	// MaturityLevel = 0 (unknown) → no boost
	item := catalog.Item{
		ID:            "i1",
		Title:         "T",
		Type:          catalog.ItemTypeSkill,
		Tags:          []string{"shell"},
		MaturityLevel: 0,
	}
	got := matchScore(item, []string{"shell"})
	want := 1.0
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("matchScore(perfect + maturity0) = %v, want %v (no boost)", got, want)
	}
}

func TestMatchScore_ScoreRange(t *testing.T) {
	// Ensure score is always in [0, 1] regardless of inputs.
	cases := []struct {
		itemTags      []string
		candidateTags []string
		maturity      int
	}{
		{nil, nil, 0},
		{[]string{"a"}, []string{"b"}, 5},
		{[]string{"a", "b"}, []string{"a", "b", "c"}, 4},
		{[]string{"x", "y", "z"}, []string{"x"}, 5},
	}
	for _, tc := range cases {
		item := catalog.Item{
			ID:            "test",
			Title:         "T",
			Type:          catalog.ItemTypeSkill,
			Tags:          tc.itemTags,
			MaturityLevel: tc.maturity,
		}
		got := matchScore(item, tc.candidateTags)
		if got < 0 || got > 1.0+1e-9 {
			t.Errorf("matchScore out of [0,1]: got %v for tags=%v candidates=%v maturity=%d",
				got, tc.itemTags, tc.candidateTags, tc.maturity)
		}
	}
}
