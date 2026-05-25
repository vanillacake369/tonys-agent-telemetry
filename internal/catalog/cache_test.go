package catalog

import (
	"path/filepath"
	"testing"
	"time"
)

func makeTestItems() []Item {
	return []Item{
		{
			ID:          "skill/test-driven-flow",
			Title:       "Test Driven Flow",
			Type:        ItemTypeSkill,
			Description: "Write tests first.",
			Tags:        []string{"tdd", "testing"},
		},
		{
			ID:          "template/go-microservice",
			Title:       "Go Microservice Template",
			Type:        ItemTypeTemplate,
			Description: "Scaffolds a Go service.",
			Tags:        []string{"go"},
		},
		{
			ID:          "agent/reviewer",
			Title:       "Reviewer Agent",
			Type:        ItemTypeAgent,
			Description: "Automated code review.",
			Tags:        []string{"review"},
		},
	}
}

func TestCache_WriteAndRead_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := &Cache{Path: filepath.Join(dir, "items.json")}

	want := makeTestItems()
	if err := c.Write(want); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got, mtime, err := c.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("Read returned %d items, want %d", len(got), len(want))
	}
	for i, item := range got {
		if item.ID != want[i].ID {
			t.Errorf("item[%d].ID = %q, want %q", i, item.ID, want[i].ID)
		}
		if item.Title != want[i].Title {
			t.Errorf("item[%d].Title = %q, want %q", i, item.Title, want[i].Title)
		}
	}
	// mtime should be recent.
	age := time.Since(mtime)
	if age < 0 || age > 5*time.Second {
		t.Errorf("mtime age %v is out of expected range (0..5s)", age)
	}
}

func TestCache_Age_WithinTolerance(t *testing.T) {
	dir := t.TempDir()
	c := &Cache{Path: filepath.Join(dir, "items.json")}

	if err := c.Write(makeTestItems()); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	age := c.Age()
	if age < 0 || age > 5*time.Second {
		t.Errorf("Age() = %v, want within 0..5s", age)
	}
}

func TestCache_IsStale_ZeroTTL_IsTrue(t *testing.T) {
	dir := t.TempDir()
	c := &Cache{Path: filepath.Join(dir, "items.json")}

	if err := c.Write(makeTestItems()); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// TTL=0 means any age is stale.
	if !c.IsStale(0) {
		t.Error("IsStale(0) should be true for any non-negative age")
	}
}

func TestCache_IsStale_LongTTL_IsFalse(t *testing.T) {
	dir := t.TempDir()
	c := &Cache{Path: filepath.Join(dir, "items.json")}

	if err := c.Write(makeTestItems()); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// TTL=1h: just-written cache should NOT be stale.
	if c.IsStale(time.Hour) {
		t.Error("IsStale(1h) should be false for a just-written cache")
	}
}

func TestCache_Read_MissingFile_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	c := &Cache{Path: filepath.Join(dir, "nonexistent.json")}

	_, _, err := c.Read()
	if err == nil {
		t.Error("Read on missing file should return error, got nil")
	}
}

func TestCache_Write_CreatesIntermediateDirs(t *testing.T) {
	dir := t.TempDir()
	c := &Cache{Path: filepath.Join(dir, "subdir", "nested", "items.json")}

	if err := c.Write(makeTestItems()); err != nil {
		t.Fatalf("Write with nested path failed: %v", err)
	}

	got, _, err := c.Read()
	if err != nil {
		t.Fatalf("Read after nested write failed: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("Read returned %d items, want 3", len(got))
	}
}

func TestResolveCachePath_EnvOverride(t *testing.T) {
	const testPath = "/tmp/tonys-test-catalog.json"
	t.Setenv("TONYS_CATALOG_PATH", testPath)

	got := ResolveCachePath()
	if got != testPath {
		t.Errorf("ResolveCachePath() = %q, want %q when env is set", got, testPath)
	}
}

func TestResolveCachePath_DefaultContainsTonys(t *testing.T) {
	t.Setenv("TONYS_CATALOG_PATH", "")

	got := ResolveCachePath()
	if got == "" {
		t.Error("ResolveCachePath() returned empty string")
	}
	// The default path should reference our app name in some form.
	if !contains(got, "tonys") && !contains(got, "catalog") {
		t.Errorf("ResolveCachePath() = %q; want path containing 'tonys' or 'catalog'", got)
	}
}

// contains is a simple substring helper for test assertions.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}())
}
