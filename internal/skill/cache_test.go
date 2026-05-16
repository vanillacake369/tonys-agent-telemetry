package skill

import (
	"testing"
	"time"
)

func makeTestSkills(names ...string) []Skill {
	skills := make([]Skill, 0, len(names))
	for _, n := range names {
		skills = append(skills, Skill{Name: n, Source: SourceGitHub})
	}
	return skills
}

func TestCache_GetOnEmpty_Miss(t *testing.T) {
	c := NewCache(10, time.Minute)
	_, ok := c.Get("nonexistent")
	if ok {
		t.Error("expected cache miss for empty cache, got hit")
	}
}

func TestCache_SetThenGet_Hit(t *testing.T) {
	c := NewCache(10, time.Minute)
	skills := makeTestSkills("skill-a", "skill-b")
	c.Set("key1", skills)

	got, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected cache hit, got miss")
	}
	if len(got) != 2 {
		t.Errorf("got %d skills, want 2", len(got))
	}
	if got[0].Name != "skill-a" {
		t.Errorf("got[0].Name = %q, want %q", got[0].Name, "skill-a")
	}
}

func TestCache_TTLExpiration_Miss(t *testing.T) {
	c := NewCache(10, 50*time.Millisecond)
	c.Set("key1", makeTestSkills("skill-a"))

	// Should be a hit immediately.
	if _, ok := c.Get("key1"); !ok {
		t.Fatal("expected hit before TTL expiry")
	}

	time.Sleep(60 * time.Millisecond)

	// Should be a miss after TTL.
	if _, ok := c.Get("key1"); ok {
		t.Error("expected cache miss after TTL, got hit")
	}
}

func TestCache_LRU_EvictsOldestWhenFull(t *testing.T) {
	c := NewCache(3, time.Minute)

	c.Set("key1", makeTestSkills("a"))
	c.Set("key2", makeTestSkills("b"))
	c.Set("key3", makeTestSkills("c"))

	// Adding a 4th entry should evict key1 (oldest).
	c.Set("key4", makeTestSkills("d"))

	if _, ok := c.Get("key1"); ok {
		t.Error("key1 should have been evicted (LRU), but it was found")
	}
	if _, ok := c.Get("key2"); !ok {
		t.Error("key2 should still be present after eviction")
	}
	if _, ok := c.Get("key3"); !ok {
		t.Error("key3 should still be present after eviction")
	}
	if _, ok := c.Get("key4"); !ok {
		t.Error("key4 should be present after insertion")
	}
}

func TestCache_SetSameKey_DoesNotGrow(t *testing.T) {
	c := NewCache(2, time.Minute)
	c.Set("key1", makeTestSkills("a"))
	c.Set("key1", makeTestSkills("b")) // overwrite

	// Should not have grown beyond 1 unique key.
	if len(c.items) != 1 {
		t.Errorf("cache size = %d, want 1 after overwriting same key", len(c.items))
	}
	got, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected hit after overwrite")
	}
	if got[0].Name != "b" {
		t.Errorf("overwritten value = %q, want %q", got[0].Name, "b")
	}
}
