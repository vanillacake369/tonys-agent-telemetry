package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Cache manages a disk-backed JSON snapshot of the catalog.
// Each Cache instance owns exactly one file path; it has no global state.
// SRP: this file only handles read/write/staleness of the on-disk JSON.
// It does not fetch from the network (see fetcher.go).
type Cache struct {
	// Path is the absolute path to the JSON file.
	// Inject a TempDir path in tests; use ResolveCachePath() for production.
	Path string
}

// Read loads the cached items and returns the file's modification time.
// Returns an error if the file does not exist or cannot be decoded.
func (c *Cache) Read() ([]Item, time.Time, error) {
	info, err := os.Stat(c.Path)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("catalog cache: stat %q: %w", c.Path, err)
	}
	mtime := info.ModTime()

	data, err := os.ReadFile(c.Path)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("catalog cache: read %q: %w", c.Path, err)
	}

	var items []Item
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, time.Time{}, fmt.Errorf("catalog cache: decode %q: %w", c.Path, err)
	}
	return items, mtime, nil
}

// Write atomically persists items to disk by writing to a temp file then
// renaming into place. This prevents a half-written file from being read.
func (c *Cache) Write(items []Item) error {
	dir := filepath.Dir(c.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("catalog cache: mkdir %q: %w", dir, err)
	}

	data, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("catalog cache: marshal: %w", err)
	}

	tmp := c.Path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("catalog cache: write tmp %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, c.Path); err != nil {
		_ = os.Remove(tmp) // best-effort cleanup on rename failure
		return fmt.Errorf("catalog cache: rename %q → %q: %w", tmp, c.Path, err)
	}
	return nil
}

// Age returns how long ago the cache file was last written.
// Returns a very large duration if the file does not exist.
func (c *Cache) Age() time.Duration {
	info, err := os.Stat(c.Path)
	if err != nil {
		return 1<<63 - 1 // max duration = always stale
	}
	return time.Since(info.ModTime())
}

// IsStale reports whether the cache is older than ttl.
func (c *Cache) IsStale(ttl time.Duration) bool {
	return c.Age() >= ttl
}
