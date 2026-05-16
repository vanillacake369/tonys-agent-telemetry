package skill

import (
	"sync"
	"time"
)

// cacheEntry holds a cached result and its creation time.
type cacheEntry struct {
	value     []Skill
	createdAt time.Time
}

// Cache is a simple in-memory LRU+TTL cache for skill search results.
// When maxSize is exceeded, the oldest entry (by insertion order) is evicted.
type Cache struct {
	mu      sync.RWMutex
	items   map[string]cacheEntry
	keys    []string // insertion order for LRU eviction
	maxSize int
	ttl     time.Duration
}

// NewCache creates a new Cache with the given capacity and TTL.
func NewCache(maxSize int, ttl time.Duration) *Cache {
	return &Cache{
		items:   make(map[string]cacheEntry),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get returns the cached skills for the given key, or (nil, false) on miss/expiry.
func (c *Cache) Get(key string) ([]Skill, bool) {
	c.mu.RLock()
	entry, ok := c.items[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}
	if time.Since(entry.createdAt) > c.ttl {
		// Expired — remove lazily.
		c.mu.Lock()
		delete(c.items, key)
		c.removeKey(key)
		c.mu.Unlock()
		return nil, false
	}
	return entry.value, true
}

// Set stores skills under key, evicting the oldest entry when at capacity.
func (c *Cache) Set(key string, skills []Skill) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.items[key]; !exists {
		// Evict oldest when full.
		if len(c.items) >= c.maxSize && c.maxSize > 0 {
			oldest := c.keys[0]
			c.keys = c.keys[1:]
			delete(c.items, oldest)
		}
		c.keys = append(c.keys, key)
	}

	c.items[key] = cacheEntry{
		value:     skills,
		createdAt: time.Now(),
	}
}

// removeKey removes a key from the ordered list (called with lock held).
func (c *Cache) removeKey(key string) {
	for i, k := range c.keys {
		if k == key {
			c.keys = append(c.keys[:i], c.keys[i+1:]...)
			return
		}
	}
}
