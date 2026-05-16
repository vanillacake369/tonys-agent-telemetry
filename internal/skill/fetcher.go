package skill

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"golang.org/x/time/rate"
)

// Fetcher orchestrates local + GitHub skill searches with caching and rate limiting.
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

// SearchRemote fetches GitHub results asynchronously.
// Returns (nil, nil) when query is shorter than 3 characters.
// Uses cache + rate limiter.
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

	ghSortStr := sortByToGHSort(sortBy)
	results, err := SearchGitHub(ctx, query, ghSortStr, 30)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil
		}
		log.Printf("skill: SearchRemote GitHub error: %v", err)
		return nil, err
	}

	sortSkills(results, sortBy)
	f.cache.Set(cacheKey, results)
	return results, nil
}

// Search combines local + GitHub results. Checks cache first.
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

	var github []Skill

	// Only query GitHub when there is a search term.
	if query != "" {
		// Apply rate limiting — wait unless context is cancelled first.
		if err := f.limiter.Wait(ctx); err != nil {
			// Context cancelled: return local results only.
			if ctx.Err() != nil {
				return local, nil
			}
			return local, nil
		}

		ghSortStr := sortByToGHSort(sortBy)
		ghResults, ghErr := SearchGitHub(ctx, query, ghSortStr, 30)
		if ghErr != nil {
			if ctx.Err() != nil {
				// Cancelled mid-fetch — return what we have.
				return local, nil
			}
			log.Printf("skill: SearchGitHub error: %v", ghErr)
		} else {
			github = ghResults
		}
	}

	merged := mergeSkills(local, github, sortBy)
	f.cache.Set(cacheKey, merged)
	return merged, nil
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

// mergeSkills places local results first, then GitHub results ordered by sortBy.
func mergeSkills(local, github []Skill, sortBy SortBy) []Skill {
	// Sort GitHub slice.
	sortSkills(github, sortBy)

	result := make([]Skill, 0, len(local)+len(github))
	result = append(result, local...)
	result = append(result, github...)
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
