package catalog

import (
	"os"
	"testing"
)

// TestParse_RealUpstreamFixture_ProducesValidItems parses the captured
// upstream fixture (internal/catalog/testdata/upstream_fixture.md) and
// asserts at least 5 valid items with all required fields populated.
//
// The fixture was captured from:
//   - Repo: https://github.com/FlorianBruniaux/claude-code-ultimate-guide
//   - SHA:  37e9335457b829b8b307c12e0b8cbdf42be7cd8b (2026-05-24)
//   - File: examples/CATALOG.md
func TestParse_RealUpstreamFixture_ProducesValidItems(t *testing.T) {
	raw, err := os.ReadFile("testdata/upstream_fixture.md")
	if err != nil {
		t.Fatalf("could not read upstream fixture: %v", err)
	}

	items, err := ParseMarkdown(raw)
	if err != nil {
		t.Fatalf("ParseMarkdown returned unexpected error: %v", err)
	}

	const wantMin = 5
	if len(items) < wantMin {
		t.Errorf("expected at least %d valid items, got %d", wantMin, len(items))
	}

	for i, it := range items {
		if !it.IsValid() {
			t.Errorf("item[%d] is invalid: %+v", i, it)
			continue
		}
		if it.ID == "" {
			t.Errorf("item[%d] has empty ID", i)
		}
		if it.Title == "" {
			t.Errorf("item[%d] has empty Title (ID=%q)", i, it.ID)
		}
		if it.SourceURL == "" {
			t.Errorf("item[%d] has empty SourceURL (ID=%q)", i, it.ID)
		}
	}
}

// TestParse_RealUpstreamFixture_HasAllItemTypes asserts the full upstream
// fixture produces at least 2 distinct ItemType values.
// The CATALOG.md has agents, skills, hooks, and commands — expecting all 4.
func TestParse_RealUpstreamFixture_HasAllItemTypes(t *testing.T) {
	raw, err := os.ReadFile("testdata/upstream_fixture.md")
	if err != nil {
		t.Fatalf("could not read upstream fixture: %v", err)
	}

	items, err := ParseMarkdown(raw)
	if err != nil {
		t.Fatalf("ParseMarkdown error: %v", err)
	}

	seen := map[ItemType]int{}
	for _, it := range items {
		seen[it.Type]++
	}

	const wantDistinctTypes = 2
	if len(seen) < wantDistinctTypes {
		t.Errorf("fixture produced only %d distinct ItemType values, want at least %d; distribution: %v",
			len(seen), wantDistinctTypes, seen)
	}

	// Log the distribution for diagnostic purposes.
	t.Logf("ItemType distribution: agents=%d, skills=%d, hooks=%d, templates=%d",
		seen[ItemTypeAgent], seen[ItemTypeSkill], seen[ItemTypeHook], seen[ItemTypeTemplate])
}

// TestParse_RealUpstreamFixture_CountMeetsMinViable checks that the full
// fixture parses to at least MinViableEntries items (Phase 1 gate).
func TestParse_RealUpstreamFixture_CountMeetsMinViable(t *testing.T) {
	raw, err := os.ReadFile("testdata/upstream_fixture.md")
	if err != nil {
		t.Fatalf("could not read upstream fixture: %v", err)
	}

	items, err := ParseMarkdown(raw)
	if err != nil {
		t.Fatalf("ParseMarkdown error: %v", err)
	}

	if len(items) < MinViableEntries {
		t.Errorf("fixture produced %d items, Phase 1 gate requires at least %d (MinViableEntries)",
			len(items), MinViableEntries)
	}
	t.Logf("fixture produced %d valid items (MinViableEntries=%d)", len(items), MinViableEntries)
}
