package recommender

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// EnforceEvidence tests
// ---------------------------------------------------------------------------

func TestEnforceEvidence_RejectsMissingSignalID(t *testing.T) {
	recs := []Recommendation{
		{
			SignalID:      "", // missing
			CatalogItemID: "cat-item-001",
			Title:         "Use bash skill",
			Score:         0.8,
			CreatedAt:     time.Now(),
		},
	}
	if err := EnforceEvidence(recs); err == nil {
		t.Error("EnforceEvidence: expected error for missing SignalID, got nil")
	}
}

func TestEnforceEvidence_RejectsMissingCatalogItemID(t *testing.T) {
	recs := []Recommendation{
		{
			SignalID:      "sig-stalled-001",
			CatalogItemID: "", // missing
			Title:         "Use bash skill",
			Score:         0.8,
			CreatedAt:     time.Now(),
		},
	}
	if err := EnforceEvidence(recs); err == nil {
		t.Error("EnforceEvidence: expected error for missing CatalogItemID, got nil")
	}
}

func TestEnforceEvidence_RejectsBothMissing(t *testing.T) {
	recs := []Recommendation{
		{
			SignalID:      "", // missing
			CatalogItemID: "", // missing
			Title:         "Use bash skill",
			Score:         0.5,
			CreatedAt:     time.Now(),
		},
	}
	err := EnforceEvidence(recs)
	if err == nil {
		t.Error("EnforceEvidence: expected error when both citation fields are missing, got nil")
	}
	// Error message should mention both fields.
	msg := err.Error()
	if !strings.Contains(msg, "SignalID") && !strings.Contains(msg, "CatalogItemID") {
		t.Errorf("error message %q should reference the missing citation fields", msg)
	}
}

func TestEnforceEvidence_AcceptsValid(t *testing.T) {
	recs := []Recommendation{
		{
			SignalID:      "sig-stalled-001",
			CatalogItemID: "cat-item-001",
			Title:         "Adopt shell skill",
			Reasoning:     "stalled_node detected on bash spans",
			Score:         0.9,
			CreatedAt:     time.Now(),
		},
		{
			SignalID:      "sig-handoff-002",
			CatalogItemID: "cat-item-042",
			Title:         "Add retry decorator",
			Reasoning:     "failed_handoff pattern found",
			Score:         0.7,
			CreatedAt:     time.Now(),
		},
	}
	if err := EnforceEvidence(recs); err != nil {
		t.Errorf("EnforceEvidence: expected nil for fully-cited recommendations, got %v", err)
	}
}

func TestEnforceEvidence_AcceptsEmptySlice(t *testing.T) {
	if err := EnforceEvidence([]Recommendation{}); err != nil {
		t.Errorf("EnforceEvidence: expected nil for empty slice, got %v", err)
	}
	if err := EnforceEvidence(nil); err != nil {
		t.Errorf("EnforceEvidence: expected nil for nil slice, got %v", err)
	}
}

func TestEnforceEvidence_RejectsFirstViolatorInMultiple(t *testing.T) {
	recs := []Recommendation{
		{
			SignalID:      "sig-001",
			CatalogItemID: "cat-001",
			Title:         "Valid",
			Score:         0.9,
			CreatedAt:     time.Now(),
		},
		{
			SignalID:      "sig-002",
			CatalogItemID: "", // violation
			Title:         "Invalid",
			Score:         0.6,
			CreatedAt:     time.Now(),
		},
	}
	if err := EnforceEvidence(recs); err == nil {
		t.Error("EnforceEvidence: expected error when any recommendation is missing a citation")
	}
}

// ---------------------------------------------------------------------------
// FilterEvidenced tests
// ---------------------------------------------------------------------------

func TestFilterEvidenced_StripsViolators(t *testing.T) {
	valid := Recommendation{
		SignalID:      "sig-001",
		CatalogItemID: "cat-001",
		Title:         "Valid",
		Score:         0.9,
		CreatedAt:     time.Now(),
	}
	invalid := Recommendation{
		SignalID:      "", // missing
		CatalogItemID: "cat-002",
		Title:         "No signal",
		Score:         0.5,
		CreatedAt:     time.Now(),
	}
	alsoInvalid := Recommendation{
		SignalID:      "sig-003",
		CatalogItemID: "", // missing
		Title:         "No catalog",
		Score:         0.4,
		CreatedAt:     time.Now(),
	}

	got := FilterEvidenced([]Recommendation{valid, invalid, alsoInvalid})
	if len(got) != 1 {
		t.Fatalf("FilterEvidenced: got %d results, want 1", len(got))
	}
	if got[0].SignalID != valid.SignalID {
		t.Errorf("FilterEvidenced: returned wrong recommendation; got SignalID=%q", got[0].SignalID)
	}
}

func TestFilterEvidenced_KeepsValid(t *testing.T) {
	recs := []Recommendation{
		{SignalID: "sig-a", CatalogItemID: "cat-a", Title: "A", Score: 0.9, CreatedAt: time.Now()},
		{SignalID: "sig-b", CatalogItemID: "cat-b", Title: "B", Score: 0.8, CreatedAt: time.Now()},
	}
	got := FilterEvidenced(recs)
	if len(got) != 2 {
		t.Fatalf("FilterEvidenced: got %d results, want 2", len(got))
	}
}

func TestFilterEvidenced_EmptyInput(t *testing.T) {
	if got := FilterEvidenced(nil); len(got) != 0 {
		t.Errorf("FilterEvidenced(nil): got %d results, want 0", len(got))
	}
	if got := FilterEvidenced([]Recommendation{}); len(got) != 0 {
		t.Errorf("FilterEvidenced([]): got %d results, want 0", len(got))
	}
}
