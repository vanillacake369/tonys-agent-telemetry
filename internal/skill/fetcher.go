package skill

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"golang.org/x/time/rate"
)

// Fetcher orchestrates local + remote skill searches with caching and rate limiting.
type Fetcher struct {
	cache   *Cache
	limiter *rate.Limiter
}

// NewFetcher creates a Fetcher with a 10-entry, 5-minute cache and a
// rate limiter of 3-burst then 1 request per 2 seconds.
func NewFetcher() *Fetcher {
	return &Fetcher{
		cache:   NewCache(10, 5*time.Minute),
		limiter: rate.NewLimiter(rate.Every(2*time.Second), 3),
	}
}

// SearchLocal returns local skills immediately (no network).
func (f *Fetcher) SearchLocal() ([]Skill, error) {
	return ScanLocal()
}

// SearchRemote fetches GitHub, npm, and awesome-list results concurrently.
// Returns (nil, nil) when query is shorter than 3 characters.
// Uses cache + rate limiter. One source failing does not block the others.
func (f *Fetcher) SearchRemote(ctx context.Context, query string, sortBy SortBy) ([]Skill, error) {
	if len(query) < 3 {
		return nil, nil
	}

	cacheKey := cacheKeyFor(query, sortBy)
	if cached, ok := f.cache.Get(cacheKey); ok {
		return cached, nil
	}

	if err := f.limiter.Wait(ctx); err != nil {
		if ctx.Err() != nil {
			return nil, nil
		}
		return nil, nil
	}

	// Fetch from all remote sources concurrently.
	type result struct {
		skills []Skill
		err    error
	}

	const numSources = 4
	ch := make(chan result, numSources)

	// GitHub (primary + fallback).
	go func() {
		s, e := SearchGitHub(ctx, query, sortByToGHSort(sortBy), 30)
		ch <- result{s, e}
	}()

	// npm registry.
	go func() {
		s, e := SearchNPM(ctx, query, 10)
		ch <- result{s, e}
	}()

	// Awesome lists.
	go func() {
		s, e := SearchAwesome(ctx, query)
		ch <- result{s, e}
	}()

	// Skill registries (monorepos like mattpocock/skills).
	go func() {
		s, e := SearchRegistries(ctx, query)
		ch <- result{s, e}
	}()

	var all []Skill
	for i := 0; i < numSources; i++ {
		select {
		case r := <-ch:
			if r.err == nil {
				all = append(all, r.skills...)
			} else {
				log.Printf("skill: SearchRemote source error: %v", r.err)
			}
		case <-ctx.Done():
			// Context cancelled — return what we have so far.
			sortSkills(all, sortBy)
			return all, nil
		}
	}

	sortSkills(all, sortBy)
	f.cache.Set(cacheKey, all)
	return all, nil
}

// Search combines local + remote results. Checks cache first.
// ctx is used for cancellation — tab switches cancel in-flight requests.
// When offline or ctx is cancelled after local results are ready, local skills
// are returned rather than an error.
func (f *Fetcher) Search(ctx context.Context, query string, sortBy SortBy) ([]Skill, error) {
	cacheKey := cacheKeyFor(query, sortBy)

	// Cache hit.
	if cached, ok := f.cache.Get(cacheKey); ok {
		return cached, nil
	}

	// Always scan local (fast, no network).
	local, err := ScanLocal()
	if err != nil {
		log.Printf("skill: ScanLocal error: %v", err)
		local = nil
	}

	var remote []Skill

	// Only query remote sources when there is a search term.
	if query != "" {
		// Apply rate limiting — wait unless context is cancelled first.
		if err := f.limiter.Wait(ctx); err != nil {
			// Context cancelled: return local results only.
			if ctx.Err() != nil {
				return local, nil
			}
			return local, nil
		}

		remoteResults, remoteErr := f.fetchRemoteAll(ctx, query, sortBy)
		if remoteErr != nil {
			if ctx.Err() != nil {
				// Cancelled mid-fetch — return what we have.
				return local, nil
			}
			log.Printf("skill: remote fetch error: %v", remoteErr)
		} else {
			remote = remoteResults
		}
	}

	merged := mergeSkills(local, remote, sortBy)
	f.cache.Set(cacheKey, merged)
	return merged, nil
}

// fetchRemoteAll fetches from GitHub, npm, awesome-lists, and skill registries concurrently.
// A single source failure does not block the others.
func (f *Fetcher) fetchRemoteAll(ctx context.Context, query string, sortBy SortBy) ([]Skill, error) {
	type result struct {
		skills []Skill
		err    error
	}

	const numSources = 4
	ch := make(chan result, numSources)

	go func() {
		s, e := SearchGitHub(ctx, query, sortByToGHSort(sortBy), 30)
		ch <- result{s, e}
	}()

	go func() {
		s, e := SearchNPM(ctx, query, 10)
		ch <- result{s, e}
	}()

	go func() {
		s, e := SearchAwesome(ctx, query)
		ch <- result{s, e}
	}()

	go func() {
		s, e := SearchRegistries(ctx, query)
		ch <- result{s, e}
	}()

	var all []Skill
	for i := 0; i < numSources; i++ {
		select {
		case r := <-ch:
			if r.err == nil {
				all = append(all, r.skills...)
			} else {
				log.Printf("skill: fetchRemoteAll source error: %v", r.err)
			}
		case <-ctx.Done():
			return all, ctx.Err()
		}
	}

	return all, nil
}

// cacheKeyFor builds the cache key from query and sort mode.
func cacheKeyFor(query string, sortBy SortBy) string {
	return fmt.Sprintf("%s:%d", query, int(sortBy))
}

// sortByToGHSort maps SortBy to the string accepted by gh search repos --sort.
func sortByToGHSort(s SortBy) string {
	switch s {
	case SortByCreated:
		return "created"
	case SortByUpdated:
		return "updated"
	default:
		return "stars"
	}
}

// mergeSkills places local results first, then remote results ordered by sortBy.
func mergeSkills(local, remote []Skill, sortBy SortBy) []Skill {
	// Sort remote slice.
	sortSkills(remote, sortBy)

	result := make([]Skill, 0, len(local)+len(remote))
	result = append(result, local...)
	result = append(result, remote...)
	return result
}

// sortSkills sorts a slice of skills in-place by the given criterion (descending).
func sortSkills(skills []Skill, sortBy SortBy) {
	sort.SliceStable(skills, func(i, j int) bool {
		switch sortBy {
		case SortByCreated:
			return skills[i].CreatedAt.After(skills[j].CreatedAt)
		case SortByUpdated:
			return skills[i].UpdatedAt.After(skills[j].UpdatedAt)
		default: // SortByStars
			return skills[i].Stars > skills[j].Stars
		}
	})
}
