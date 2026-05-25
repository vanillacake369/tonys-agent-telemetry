package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/catalog"
)

// makeCatalogItems builds a slice of catalog.Item with the given count.
// All items are valid per Item.IsValid().
func makeCatalogItems(n int) []catalog.Item {
	items := make([]catalog.Item, n)
	types := []catalog.ItemType{catalog.ItemTypeSkill, catalog.ItemTypeTemplate, catalog.ItemTypeAgent, catalog.ItemTypeHook}
	for i := range items {
		items[i] = catalog.Item{
			ID:          "skill/item-" + string(rune('a'+i%26)),
			Title:       "Catalog Item " + string(rune('A'+i%26)),
			Type:        types[i%len(types)],
			Description: "A best-practice entry.",
			Tags:        []string{"tag1", "tag2"},
		}
	}
	return items
}

// TestSkillsTab_RendersLicenseAttribution is the GA gate test.
// It injects a CatalogLoadedMsg with >= MinViableEntries items and asserts
// that the rendered View contains the catalog.Attribution string.
func TestSkillsTab_RendersLicenseAttribution(t *testing.T) {
	s := NewSkillsTab()
	s = s.SetSize(120, 40).(SkillsTab)

	items := makeCatalogItems(catalog.MinViableEntries)
	s, _ = updateSkillsTab(t, s, CatalogLoadedMsg{
		Items:     items,
		FetchedAt: time.Now(),
	})

	view := s.View()
	if !strings.Contains(view, catalog.Attribution) {
		t.Errorf("View() does not contain license attribution string.\nWant: %q\nView excerpt: %.500s",
			catalog.Attribution, view)
	}
}

// TestSkillsTab_RendersStaleWarningWhenCacheOld asserts that when FetchedAt
// is older than DefaultTTL, the title row prefixes with "(stale)".
func TestSkillsTab_RendersStaleWarningWhenCacheOld(t *testing.T) {
	s := NewSkillsTab()
	s = s.SetSize(120, 40).(SkillsTab)

	items := makeCatalogItems(catalog.MinViableEntries)
	s, _ = updateSkillsTab(t, s, CatalogLoadedMsg{
		Items:     items,
		FetchedAt: time.Now().Add(-48 * time.Hour), // 48h ago — clearly stale
	})

	view := s.View()
	if !strings.Contains(view, "(stale)") {
		t.Errorf("View() does not contain '(stale)' when cache is 48h old.\nView excerpt: %.500s", view)
	}
}

// TestSkillsTab_RendersPartialWarningBelowMinViable asserts that when fewer than
// MinViableEntries items are loaded, a "partial" warning is shown and no
// individual items are listed.
func TestSkillsTab_RendersPartialWarningBelowMinViable(t *testing.T) {
	s := NewSkillsTab()
	s = s.SetSize(120, 40).(SkillsTab)

	const partialCount = 5 // well below MinViableEntries (100)
	items := makeCatalogItems(partialCount)
	s, _ = updateSkillsTab(t, s, CatalogLoadedMsg{
		Items:     items,
		FetchedAt: time.Now(),
	})

	view := s.View()
	if !strings.Contains(view, "partial") {
		t.Errorf("View() does not contain 'partial' warning for %d/%d items.\nView excerpt: %.500s",
			partialCount, catalog.MinViableEntries, view)
	}
	// Individual catalog item titles must NOT appear when partial.
	for _, item := range items {
		if strings.Contains(view, item.Title) {
			t.Errorf("View() shows item %q title in partial state — should only show warning", item.Title)
		}
	}
}

// TestSkillsTab_CatalogLoadingState asserts that before any CatalogLoadedMsg
// arrives, the catalog section shows "Loading catalog...".
func TestSkillsTab_CatalogLoadingState(t *testing.T) {
	s := NewSkillsTab()
	s = s.SetSize(120, 40).(SkillsTab)

	view := s.View()
	if !strings.Contains(view, "Loading catalog") {
		t.Errorf("View() does not show 'Loading catalog' before catalog data arrives.\nView excerpt: %.500s", view)
	}
}

// TestApp_RoutesCatalogLoadedMsg verifies that CatalogLoadedMsg is routed to
// TabSkills by App.Update regardless of the active tab.
func TestApp_RoutesCatalogLoadedMsg(t *testing.T) {
	a := NewApp()
	a.width, a.height = 120, 40
	a = a.propagateSize()

	// Active tab is Sessions at startup.
	if a.activeTab != TabSessions {
		t.Fatalf("initial active tab = %d, want TabSessions", a.activeTab)
	}

	items := makeCatalogItems(catalog.MinViableEntries)
	updated, _ := a.Update(CatalogLoadedMsg{Items: items, FetchedAt: time.Now()})
	got := updated.(App)

	st := got.tabs[TabSkills].(SkillsTab)
	if len(st.catalogItems) != catalog.MinViableEntries {
		t.Errorf("SkillsTab.catalogItems len = %d after routing, want %d",
			len(st.catalogItems), catalog.MinViableEntries)
	}
}
