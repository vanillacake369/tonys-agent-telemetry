package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
)

const npmRegistryURL = "https://registry.npmjs.org/-/v1/search"

// npmSearchResult is the JSON shape returned by the npm registry search API.
type npmSearchResult struct {
	Objects []struct {
		Package struct {
			Name        string    `json:"name"`
			Description string    `json:"description"`
			Version     string    `json:"version"`
			Date        time.Time `json:"date"`
			Links       struct {
				Npm        string `json:"npm"`
				Repository string `json:"repository"`
			} `json:"links"`
		} `json:"package"`
		Score struct {
			Detail struct {
				Popularity float64 `json:"popularity"`
			} `json:"detail"`
		} `json:"score"`
	} `json:"objects"`
}

// SearchNPM searches the npm registry for Claude Code skill packages.
// Uses HTTP GET to registry.npmjs.org — no CLI dependency required.
// The query is appended to the fixed "claude-code-skill" prefix.
func SearchNPM(ctx context.Context, query string, limit int) ([]Skill, error) {
	return searchNPMWithURL(ctx, query, limit, npmRegistryURL)
}

// searchNPMWithURL is the testable inner implementation that accepts a base URL.
func searchNPMWithURL(ctx context.Context, query string, limit int, baseURL string) ([]Skill, error) {
	if limit <= 0 {
		limit = 20
	}

	searchQuery := "claude-code-skill"
	if query != "" {
		searchQuery = fmt.Sprintf("claude-code-skill %s", query)
	}

	reqURL := fmt.Sprintf("%s?text=%s&size=%d", baseURL,
		url.QueryEscape(searchQuery), limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("skill: npm search request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		log.Printf("skill: npm search failed: %v", err)
		return nil, nil
	}
	defer resp.Body.Close()

	var result npmSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("skill: parsing npm search response: %w", err)
	}

	skills := make([]Skill, 0, len(result.Objects))
	for _, obj := range result.Objects {
		pkg := obj.Package
		repoURL := pkg.Links.Repository
		if repoURL == "" {
			repoURL = pkg.Links.Npm
		}
		skills = append(skills, Skill{
			Name:        pkg.Name,
			Description: pkg.Description,
			Source:      SourceNPM,
			URL:         repoURL,
			Stars:       int(obj.Score.Detail.Popularity * 1000),
			CreatedAt:   pkg.Date,
			UpdatedAt:   pkg.Date,
		})
	}

	return skills, nil
}
