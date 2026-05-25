package catalog

// ItemType classifies the kind of best-practice entry in the corpus.
// Mirrors the categories used by FlorianBruniaux/claude-code-ultimate-guide.
type ItemType string

const (
	ItemTypeSkill    ItemType = "skill"
	ItemTypeTemplate ItemType = "template"
	ItemTypeAgent    ItemType = "agent"
	ItemTypeHook     ItemType = "hook"
)

// Item is a single entry in the best-practice corpus.
//
// Contract:
//   - Phase 1 ingest produces Items from the pinned upstream source (see source.go).
//   - Phase 2 recommender references Items by ID via Recommendation.CatalogItemID
//     (see internal/recommender). The ID is the citation handle and MUST be stable
//     across cache refreshes for the same underlying upstream entry.
//
// ID format convention: "<type>/<slug>", e.g. "skill/test-driven-flow".
type Item struct {
	ID            string   // stable identifier; required by Recommendation citation
	Title         string   // human-readable title
	Type          ItemType // skill / template / agent / hook
	Description   string   // 1-3 line summary
	Tags          []string // matching keys for signal-to-item mapping (e.g. "shell", "retry", "orchestration")
	MaturityLevel int      // 1..5 per ultimate-guide context-audit; 0 = unknown
	SourceURL     string   // canonical upstream URL (for attribution + click-through)
}

// IsValid reports whether the Item meets the minimum contract — ID and Title
// non-empty, Type one of the known constants. Callers (ingest, recommender)
// should filter out invalid Items rather than propagate them.
func (i Item) IsValid() bool {
	if i.ID == "" || i.Title == "" {
		return false
	}
	switch i.Type {
	case ItemTypeSkill, ItemTypeTemplate, ItemTypeAgent, ItemTypeHook:
		return true
	}
	return false
}
