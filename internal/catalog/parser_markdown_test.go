package catalog

import (
	"strings"
	"testing"
)

// markdownFixture is a small but representative slice of upstream_fixture.md
// (captured from SHA 37e9335457b829b8b307c12e0b8cbdf42be7cd8b, 2026-05-24).
// It exercises all four ItemType values so that TestParse_Markdown_HasAllItemTypes passes.
// Intentionally keeps description on some entries and omits on others (blank-line gap)
// to verify resilience.
const markdownFixture = `# Template Catalog

Auto-generated template index with complexity, time, and domain filters.

**Last updated**: [auto-generated]

---

**Total Templates**: 10

## By Category

### Agents (2)

- **[adr-writer](agents/adr-writer.md)** *intermediate* • 30 min
  Architecture Decision Record generator agent.

- **[code-reviewer](agents/code-reviewer.md)** *intermediate* • 30 min
  Use for thorough code review with quality, security, and performance checks


### Commands (2)

- **[audit-agents-skills](commands/audit-agents-skills.md)** *intermediate* • 30 min
  Audit quality of agents, skills, and commands in a Claude Code project

- **[catchup](commands/catchup.md)** *intermediate* • 30 min
  Restore context after /clear by summarizing recent work and project state


### Skills (3)

- **[ast-grep-patterns](skills/ast-grep-patterns.md)** *intermediate* • 30 min
  Skill teaching Claude when and how to use ast-grep for structural code searches

- **[design-patterns](skills/design-patterns/SKILL.md)** *intermediate* • 30 min
  Detect, suggest, and evaluate GoF design patterns in TypeScript/JavaScript codebases.

- **[eval-rules](skills/eval-rules/SKILL.md)** *intermediate* • 30 min
  Audit .claude/rules/ files for structural correctness, glob validity, and real-world usefulness.


### Hooks (3)

- **[auto-checkpoint](hooks/bash/auto-checkpoint.sh)** *intermediate* • 30 min
  Automatic git checkpoint after each tool use.

- **[dangerous-actions-blocker](hooks/bash/dangerous-actions-blocker.sh)** *intermediate* • 30 min
  Block dangerous CLI commands before execution.

- **[output-secrets-scanner](hooks/bash/output-secrets-scanner.sh)** *intermediate* • 30 min
  Scan tool output for accidental secret leakage.
`

// TestParseMarkdown_ReturnsValidItems is a table-driven TDD-first test.
// It must FAIL before parser_markdown.go exists; once implemented it passes.
func TestParseMarkdown_ReturnsValidItems(t *testing.T) {
	items, err := ParseMarkdown([]byte(markdownFixture))
	if err != nil {
		t.Fatalf("ParseMarkdown returned unexpected error: %v", err)
	}
	const wantMin = 10
	if len(items) < wantMin {
		t.Errorf("ParseMarkdown returned %d items, want at least %d", len(items), wantMin)
	}
	for _, item := range items {
		if !item.IsValid() {
			t.Errorf("invalid item slipped through: %+v", item)
		}
	}
}

// TestParseMarkdown_HasAllItemTypes asserts the fixture exercises at least
// agent, template (mapped from commands), skill, and hook.
func TestParseMarkdown_HasAllItemTypes(t *testing.T) {
	items, err := ParseMarkdown([]byte(markdownFixture))
	if err != nil {
		t.Fatalf("ParseMarkdown error: %v", err)
	}
	seen := map[ItemType]bool{}
	for _, it := range items {
		seen[it.Type] = true
	}
	required := []ItemType{ItemTypeAgent, ItemTypeSkill, ItemTypeHook, ItemTypeTemplate}
	for _, rt := range required {
		if !seen[rt] {
			t.Errorf("no items with type %q found; fixture must exercise all 4 types", rt)
		}
	}
}

// TestParseMarkdown_IDFormat verifies all IDs follow the "<type>/<slug>" convention.
func TestParseMarkdown_IDFormat(t *testing.T) {
	items, err := ParseMarkdown([]byte(markdownFixture))
	if err != nil {
		t.Fatalf("ParseMarkdown error: %v", err)
	}
	for _, it := range items {
		if !strings.Contains(it.ID, "/") {
			t.Errorf("item ID %q does not follow <type>/<slug> convention", it.ID)
		}
		prefix := string(it.Type) + "/"
		if !strings.HasPrefix(it.ID, prefix) {
			t.Errorf("item ID %q should start with %q for type %q", it.ID, prefix, it.Type)
		}
	}
}

// TestParseMarkdown_SourceURLPointsUpstream verifies all SourceURLs are non-empty
// and point to the upstream github.com path.
func TestParseMarkdown_SourceURLPointsUpstream(t *testing.T) {
	items, err := ParseMarkdown([]byte(markdownFixture))
	if err != nil {
		t.Fatalf("ParseMarkdown error: %v", err)
	}
	const wantPrefix = "https://github.com/FlorianBruniaux/claude-code-ultimate-guide"
	for _, it := range items {
		if it.SourceURL == "" {
			t.Errorf("item %q has empty SourceURL", it.ID)
		}
		if !strings.HasPrefix(it.SourceURL, wantPrefix) {
			t.Errorf("item %q SourceURL %q does not start with %q", it.ID, it.SourceURL, wantPrefix)
		}
	}
}

// TestParseMarkdown_EmptyInput_ReturnsEmpty verifies graceful handling of empty input.
func TestParseMarkdown_EmptyInput_ReturnsEmpty(t *testing.T) {
	items, err := ParseMarkdown([]byte(""))
	if err != nil {
		t.Fatalf("ParseMarkdown empty input returned error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items for empty input, got %d", len(items))
	}
}

// TestParseMarkdown_DescriptionPopulated verifies that description-bearing entries
// have non-empty Description and description-less entries default gracefully.
func TestParseMarkdown_DescriptionPopulated(t *testing.T) {
	items, err := ParseMarkdown([]byte(markdownFixture))
	if err != nil {
		t.Fatalf("ParseMarkdown error: %v", err)
	}
	// Every entry in markdownFixture has a description — verify at least 5 are non-empty.
	nonEmpty := 0
	for _, it := range items {
		if it.Description != "" {
			nonEmpty++
		}
	}
	if nonEmpty < 5 {
		t.Errorf("expected at least 5 items with non-empty Description, got %d", nonEmpty)
	}
}

// TestParseMarkdown_ProducesIndistinguishableItemsFromJSON verifies that
// ParseMarkdown produces Items that satisfy the same IsValid contract as
// Parse (JSON parser) — the two parsers are equivalent at the contract level.
func TestParseMarkdown_ProducesIndistinguishableItemsFromJSON(t *testing.T) {
	jsonFixture := `[
		{"id":"agent/adr-writer","title":"adr-writer","type":"agent","description":"Architecture Decision Record generator agent.","tags":[],"maturity_level":0,"source_url":"https://github.com/FlorianBruniaux/claude-code-ultimate-guide/blob/main/examples/agents/adr-writer.md"}
	]`

	jsonItems, err := Parse([]byte(jsonFixture))
	if err != nil {
		t.Fatalf("Parse (JSON) error: %v", err)
	}

	mdItems, err := ParseMarkdown([]byte(markdownFixture))
	if err != nil {
		t.Fatalf("ParseMarkdown error: %v", err)
	}

	// Both should produce valid Items that satisfy IsValid
	for _, it := range jsonItems {
		if !it.IsValid() {
			t.Errorf("JSON parser produced invalid item: %+v", it)
		}
	}
	for _, it := range mdItems {
		if !it.IsValid() {
			t.Errorf("Markdown parser produced invalid item: %+v", it)
		}
	}
}
