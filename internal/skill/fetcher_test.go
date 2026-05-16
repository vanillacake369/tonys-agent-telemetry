package skill

import (
	"context"
	"testing"
	"time"
)

// TestFetcher_SearchReturnsLocalSkillsWhenOffline verifies that Search returns
// at least whatever local ScanLocal provides even when GitHub is unavailable.
// This is a best-effort test: if the system has no ~/.claude/skills the result
// may be empty, which is still acceptable.
func TestFetcher_SearchReturnsLocalSkillsWhenOffline(t *testing.T) {
	f := NewFetcher()
	ctx := context.Background()

	// Query empty string → no GitHub call.
	skills, err := f.Search(ctx, "", SortByStars)
	if err != nil {
		t.Errorf("Search returned error: %v", err)
	}
	// All returned skills must be local (no GitHub call was made).
	for _, s := range skills {
		if s.Source != SourceLocal {
			t.Errorf("expected local skill, got source=%q for %q", s.Source, s.Name)
		}
	}
	t.Logf("Search returned %d local skills", len(skills))
}

// TestFetcher_SearchCachesResults verifies that a second call for the same query
// returns a cache hit (no second fetch).
func TestFetcher_SearchCachesResults(t *testing.T) {
	f := NewFetcher()
	ctx := context.Background()

	// Warm cache with a direct Set.
	f.cache.Set(cacheKeyFor("test-query", SortByStars), []Skill{
		{Name: "cached-skill", Source: SourceGitHub, Stars: 42},
	})

	skills, err := f.Search(ctx, "test-query", SortByStars)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1 (cache hit)", len(skills))
	}
	if skills[0].Name != "cached-skill" {
		t.Errorf("skill Name = %q, want %q", skills[0].Name, "cached-skill")
	}
}

// TestFetcher_CacheHit_SkipsFetch verifies that when the cache has a valid entry
// the result is returned immediately without touching the network.
func TestFetcher_CacheHit_SkipsFetch(t *testing.T) {
	f := NewFetcher()

	expected := []Skill{
		{Name: "skill-a", Source: SourceGitHub, Stars: 100},
		{Name: "skill-b", Source: SourceLocal},
	}

	key := cacheKeyFor("myquery", SortByCreated)
	f.cache.Set(key, expected)

	ctx := context.Background()
	got, err := f.Search(ctx, "myquery", SortByCreated)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d skills, want 2", len(got))
	}
	if got[0].Name != "skill-a" {
		t.Errorf("got[0].Name = %q, want %q", got[0].Name, "skill-a")
	}
}

// TestFetcher_MergeSkills_LocalFirst verifies that local skills precede GitHub ones.
func TestFetcher_MergeSkills_LocalFirst(t *testing.T) {
	local := []Skill{
		{Name: "local-a", Source: SourceLocal},
		{Name: "local-b", Source: SourceLocal},
	}
	github := []Skill{
		{Name: "gh-x", Source: SourceGitHub, Stars: 500},
		{Name: "gh-y", Source: SourceGitHub, Stars: 200},
	}

	merged := mergeSkills(local, github, SortByStars)

	if len(merged) != 4 {
		t.Fatalf("merged len = %d, want 4", len(merged))
	}
	if merged[0].Source != SourceLocal {
		t.Errorf("merged[0].Source = %q, want local", merged[0].Source)
	}
	if merged[1].Source != SourceLocal {
		t.Errorf("merged[1].Source = %q, want local", merged[1].Source)
	}
	if merged[2].Name != "gh-x" {
		t.Errorf("merged[2].Name = %q, want gh-x (highest stars)", merged[2].Name)
	}
}

// TestFetcher_SortSkills_ByStars verifies star-descending sort.
func TestFetcher_SortSkills_ByStars(t *testing.T) {
	skills := []Skill{
		{Name: "low", Stars: 10},
		{Name: "high", Stars: 999},
		{Name: "mid", Stars: 50},
	}
	sortSkills(skills, SortByStars)
	if skills[0].Name != "high" {
		t.Errorf("skills[0].Name = %q, want high", skills[0].Name)
	}
	if skills[2].Name != "low" {
		t.Errorf("skills[2].Name = %q, want low", skills[2].Name)
	}
}

// TestFetcher_SortSkills_ByCreated verifies newest-first sort.
func TestFetcher_SortSkills_ByCreated(t *testing.T) {
	now := time.Now()
	skills := []Skill{
		{Name: "old", CreatedAt: now.Add(-24 * time.Hour)},
		{Name: "new", CreatedAt: now},
		{Name: "mid", CreatedAt: now.Add(-1 * time.Hour)},
	}
	sortSkills(skills, SortByCreated)
	if skills[0].Name != "new" {
		t.Errorf("skills[0].Name = %q, want new", skills[0].Name)
	}
}

// TestCacheKeyFor verifies cache key format.
func TestCacheKeyFor(t *testing.T) {
	key := cacheKeyFor("hello", SortByStars)
	if key != "hello:0" {
		t.Errorf("cacheKeyFor = %q, want %q", key, "hello:0")
	}
	key2 := cacheKeyFor("", SortByCreated)
	if key2 != ":1" {
		t.Errorf("cacheKeyFor empty = %q, want %q", key2, ":1")
	}
}
