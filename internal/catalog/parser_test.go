package catalog

import (
	"testing"
)

// fixture is a small JSON catalog used across parser tests.
// One entry is intentionally invalid (missing Title) to verify graceful skipping.
const parserFixture = `[
  {
    "id": "skill/test-driven-flow",
    "title": "Test Driven Flow",
    "type": "skill",
    "description": "Write tests before implementation to drive design.",
    "tags": ["tdd", "testing", "quality"],
    "maturity_level": 4,
    "source_url": "https://github.com/FlorianBruniaux/claude-code-ultimate-guide"
  },
  {
    "id": "template/go-microservice",
    "title": "Go Microservice Template",
    "type": "template",
    "description": "Scaffolds a production-ready Go service.",
    "tags": ["go", "microservice"],
    "maturity_level": 3,
    "source_url": "https://github.com/FlorianBruniaux/claude-code-ultimate-guide"
  },
  {
    "id": "agent/reviewer",
    "title": "Reviewer Agent",
    "type": "agent",
    "description": "Automated code review agent.",
    "tags": ["review", "agent"],
    "maturity_level": 2,
    "source_url": "https://github.com/FlorianBruniaux/claude-code-ultimate-guide"
  },
  {
    "id": "skill/no-title",
    "title": "",
    "type": "skill",
    "description": "This entry is invalid because title is empty.",
    "tags": [],
    "maturity_level": 0,
    "source_url": ""
  }
]`

func TestParse_ReturnsValidItems(t *testing.T) {
	items, err := Parse([]byte(parserFixture))
	if err != nil {
		t.Fatalf("Parse returned unexpected error: %v", err)
	}
	const wantCount = 3
	if len(items) != wantCount {
		t.Errorf("Parse returned %d items, want %d (1 invalid should be skipped)", len(items), wantCount)
	}
}

func TestParse_SkipsBadEntryWithoutPoisoningSlice(t *testing.T) {
	items, err := Parse([]byte(parserFixture))
	if err != nil {
		t.Fatalf("Parse returned unexpected error: %v", err)
	}
	for _, item := range items {
		if !item.IsValid() {
			t.Errorf("invalid item %+v made it past Parse filter", item)
		}
		if item.ID == "skill/no-title" {
			t.Errorf("item with empty title should have been skipped, but was returned")
		}
	}
}

func TestParse_FieldsArePopulated(t *testing.T) {
	items, err := Parse([]byte(parserFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least 1 item")
	}
	first := items[0]
	if first.ID != "skill/test-driven-flow" {
		t.Errorf("ID = %q, want %q", first.ID, "skill/test-driven-flow")
	}
	if first.Title != "Test Driven Flow" {
		t.Errorf("Title = %q, want %q", first.Title, "Test Driven Flow")
	}
	if first.Type != ItemTypeSkill {
		t.Errorf("Type = %q, want %q", first.Type, ItemTypeSkill)
	}
	if len(first.Tags) != 3 {
		t.Errorf("Tags len = %d, want 3", len(first.Tags))
	}
	if first.MaturityLevel != 4 {
		t.Errorf("MaturityLevel = %d, want 4", first.MaturityLevel)
	}
}

func TestParse_EmptyArray_ReturnsEmpty(t *testing.T) {
	items, err := Parse([]byte(`[]`))
	if err != nil {
		t.Fatalf("unexpected error on empty array: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items for empty array, got %d", len(items))
	}
}

func TestParse_InvalidJSON_ReturnsError(t *testing.T) {
	_, err := Parse([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}
