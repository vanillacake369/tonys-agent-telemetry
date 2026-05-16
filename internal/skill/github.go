package skill

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

// ghSearchRepo is the JSON shape returned by `gh search repos --json`.
type ghSearchRepo struct {
	Name            string    `json:"name"`
	FullName        string    `json:"fullName"`
	Description     string    `json:"description"`
	StargazersCount int       `json:"stargazersCount"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	URL             string    `json:"url"`
}

// ghReadmeResponse is the JSON shape returned by `gh api repos/{owner}/{repo}/readme`.
type ghReadmeResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

// SearchGitHub uses the `gh search repos` CLI to find Claude Code skills.
// query is the search terms, sort is "stars"/"created"/"updated", limit is max results.
// Returns empty slice (not error) when gh is not installed or not authenticated.
func SearchGitHub(ctx context.Context, query string, sort string, limit int) ([]Skill, error) {
	if limit <= 0 {
		limit = 30
	}
	if sort == "" {
		sort = "stars"
	}

	// Check if gh is available.
	if _, err := exec.LookPath("gh"); err != nil {
		log.Printf("skill: gh CLI not found, skipping GitHub search")
		return nil, nil
	}

	args := []string{
		"search", "repos",
		query,
		"--sort", sort,
		"--limit", fmt.Sprintf("%d", limit),
		"--json", "name,fullName,description,stargazersCount,createdAt,updatedAt,url",
	}

	cmd := exec.CommandContext(ctx, "gh", args...)
	out, err := cmd.Output()
	if err != nil {
		// If context was cancelled, propagate.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// gh auth failure or other non-fatal error — log and return empty.
		log.Printf("skill: gh search repos failed: %v", err)
		return nil, nil
	}

	var repos []ghSearchRepo
	if err := json.Unmarshal(out, &repos); err != nil {
		return nil, fmt.Errorf("skill: parsing gh search output: %w", err)
	}

	skills := make([]Skill, 0, len(repos))
	for _, r := range repos {
		readmeURL := ""
		if r.URL != "" {
			readmeURL = r.URL + "#readme"
		}
		skills = append(skills, Skill{
			Name:        r.Name,
			Description: r.Description,
			Source:      SourceGitHub,
			URL:         r.URL,
			Stars:       r.StargazersCount,
			CreatedAt:   r.CreatedAt,
			UpdatedAt:   r.UpdatedAt,
			ReadmeURL:   readmeURL,
		})
	}

	return skills, nil
}

// FetchReadme fetches the README content for a GitHub repo identified by repoFullName
// (e.g. "owner/repo"). Uses `gh api repos/{owner}/{repo}/readme` with base64 decoding.
// Truncates to maxBytes. Returns empty string when gh is unavailable.
func FetchReadme(ctx context.Context, repoFullName string, maxBytes int) (string, error) {
	if maxBytes <= 0 {
		maxBytes = 10240
	}

	if _, err := exec.LookPath("gh"); err != nil {
		return "", nil
	}

	cmd := exec.CommandContext(ctx, "gh", "api", fmt.Sprintf("repos/%s/readme", repoFullName))
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		log.Printf("skill: gh api readme failed for %s: %v", repoFullName, err)
		return "", nil
	}

	var resp ghReadmeResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", fmt.Errorf("skill: parsing readme response: %w", err)
	}

	var content string
	switch resp.Encoding {
	case "base64":
		// GitHub wraps base64 in newlines.
		cleaned := strings.ReplaceAll(resp.Content, "\n", "")
		decoded, err := base64.StdEncoding.DecodeString(cleaned)
		if err != nil {
			return "", fmt.Errorf("skill: decoding readme base64: %w", err)
		}
		content = string(decoded)
	default:
		content = resp.Content
	}

	if len(content) > maxBytes {
		content = content[:maxBytes]
	}

	return content, nil
}

// repoFullNameFromURL extracts "owner/repo" from a GitHub URL.
// e.g. "https://github.com/owner/repo" → "owner/repo"
func repoFullNameFromURL(url string) string {
	// Strip trailing slash.
	url = strings.TrimRight(url, "/")
	// Find last two path segments.
	parts := strings.Split(url, "/")
	if len(parts) < 2 {
		return url
	}
	return parts[len(parts)-2] + "/" + parts[len(parts)-1]
}
