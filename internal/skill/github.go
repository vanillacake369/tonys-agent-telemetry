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

// ghCodeRepo is the nested repository object returned by `gh search code --json`.
type ghCodeRepo struct {
	FullName    string `json:"fullName"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPrivate   bool   `json:"isPrivate"`
}

// ghCodeSearchResult is one entry from `gh search code --json repository,path`.
type ghCodeSearchResult struct {
	Repository ghCodeRepo `json:"repository"`
	Path       string     `json:"path"`
}

// ghRepoMeta is the shape returned by `gh api repos/{owner}/{repo}`.
type ghRepoMeta struct {
	StargazersCount int       `json:"stargazersCount"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	Description     string    `json:"description"`
	HTMLURL         string    `json:"htmlUrl"`
}

// SearchGitHub finds Claude Code skill repos using two strategies:
//  1. Primary: `gh search repos "claude code skill {query}"` — high quality, stars-sorted.
//  2. Supplement: `gh search repos --topic claude-skills` — topic-tagged repos.
//
// Code search (gh search code --filename SKILL.md) was removed because it returns
// too many false positives (any repo with a SKILL.md file, not just Claude skills).
//
// Returns empty slice (not error) when gh is not installed or not authenticated.
func SearchGitHub(ctx context.Context, query string, sort string, limit int) ([]Skill, error) {
	if limit <= 0 {
		limit = 100
	}
	if sort == "" {
		sort = "stars"
	}

	if _, err := exec.LookPath("gh"); err != nil {
		log.Printf("skill: gh CLI not found, skipping GitHub search")
		return nil, nil
	}

	// Primary: repo search with "claude code skill" prefix.
	repoResults := searchGitHubRepos(ctx, query, sort, limit)

	// Supplement: topic-based search for tagged repos.
	topicResults := searchGitHubByTopic(ctx, query, sort, limit/2)

	all := append(repoResults, topicResults...)
	return deduplicateSkills(all), nil
}

// searchGitHubByTopic searches for repos tagged with claude-skills or claude-code-skills topics.
func searchGitHubByTopic(ctx context.Context, query string, sort string, limit int) []Skill {
	if limit <= 0 {
		return nil
	}

	// Try both common topic names.
	var all []Skill
	for _, topic := range []string{"claude-skills", "claude-code-skills"} {
		args := []string{
			"search", "repos",
			"--topic", topic,
			"--sort", sort,
			"--limit", fmt.Sprintf("%d", limit),
			"--json", "name,fullName,description,stargazersCount,createdAt,updatedAt,url",
		}

		cmd := exec.CommandContext(ctx, "gh", args...)
		out, err := cmd.Output()
		if err != nil {
			continue
		}

		var repos []ghSearchRepo
		if err := json.Unmarshal(out, &repos); err != nil {
			continue
		}

		for _, r := range repos {
			// If query provided, filter by name/description match.
			if query != "" {
				q := strings.ToLower(query)
				if !strings.Contains(strings.ToLower(r.Name), q) &&
					!strings.Contains(strings.ToLower(r.Description), q) {
					continue
				}
			}
			readmeURL := ""
			if r.URL != "" {
				readmeURL = r.URL + "#readme"
			}
			all = append(all, Skill{
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
	}

	return all
}

// searchGitHubRepos uses `gh search repos` with a refined query
// that always prepends "claude code skill".
func searchGitHubRepos(ctx context.Context, query string, sort string, limit int) []Skill {
	if limit <= 0 {
		return nil
	}
	refinedQuery := buildRefinedRepoQuery(query)

	args := []string{
		"search", "repos",
		refinedQuery,
		"--sort", sort,
		"--limit", fmt.Sprintf("%d", limit),
		"--json", "name,fullName,description,stargazersCount,createdAt,updatedAt,url",
	}

	cmd := exec.CommandContext(ctx, "gh", args...)
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		log.Printf("skill: gh search repos (fallback) failed: %v", err)
		return nil
	}

	var repos []ghSearchRepo
	if err := json.Unmarshal(out, &repos); err != nil {
		log.Printf("skill: parsing gh search repos output: %v", err)
		return nil
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

	return skills
}

// buildRefinedRepoQuery prepends "claude skill" to the user query.
// Using "claude skill" instead of "claude code skill" to catch repos like
// obra/superpowers that don't have "code" in their description.
func buildRefinedRepoQuery(query string) string {
	if query == "" {
		return "claude skill"
	}
	return fmt.Sprintf("claude skill %s", query)
}

// ghCodeSearchResultV2 matches the actual output of `gh search code --json repository,path`.
type ghCodeSearchResultV2 struct {
	Path       string `json:"path"`
	Repository struct {
		NameWithOwner string `json:"nameWithOwner"`
		URL           string `json:"url"`
		IsPrivate     bool   `json:"isPrivate"`
	} `json:"repository"`
}

// SearchGitHubCode uses `gh search code --filename SKILL.md` to find repos
// containing Claude Code skills. Filters by "claude" keyword in file content
// to reduce false positives.
func SearchGitHubCode(ctx context.Context, query string, limit int) ([]Skill, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 30
	}

	// Search for SKILL.md files containing "claude".
	searchQuery := "claude"
	if query != "" {
		searchQuery = fmt.Sprintf("claude %s", query)
	}

	args := []string{
		"search", "code",
		searchQuery,
		"--filename", "SKILL.md",
		"--limit", fmt.Sprintf("%d", limit),
		"--json", "repository,path",
	}

	cmd := exec.CommandContext(ctx, "gh", args...)
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil
		}
		log.Printf("skill: gh search code failed: %v", err)
		return nil, nil
	}

	var results []ghCodeSearchResultV2
	if err := json.Unmarshal(out, &results); err != nil {
		log.Printf("skill: parsing gh search code: %v", err)
		return nil, nil
	}

	// Deduplicate by repo.
	seen := make(map[string]bool)
	var skills []Skill
	for _, r := range results {
		if r.Repository.IsPrivate {
			continue
		}
		repoKey := r.Repository.NameWithOwner
		if seen[repoKey] {
			continue
		}
		seen[repoKey] = true

		repoURL := r.Repository.URL
		if repoURL == "" {
			repoURL = fmt.Sprintf("https://github.com/%s", repoKey)
		}

		// Derive skill name from path.
		name := repoKey
		parts := strings.Split(r.Path, "/")
		if len(parts) >= 2 {
			// Use parent dir of SKILL.md as name.
			name = parts[len(parts)-2]
		}

		// Fetch star count from repo metadata.
		stars := fetchRepoStars(ctx, repoKey)

		skills = append(skills, Skill{
			Name:        name,
			Description: fmt.Sprintf("Found in %s", repoKey),
			Source:      SourceGitHub,
			URL:         repoURL,
			Stars:       stars,
			ReadmeURL:   fmt.Sprintf("%s/blob/main/%s", repoURL, r.Path),
		})
	}

	return skills, nil
}

// fetchRepoStars fetches stargazers count for a single repo via gh api.
// Returns 0 on any error (non-blocking).
func fetchRepoStars(ctx context.Context, repoFullName string) int {
	cmd := exec.CommandContext(ctx, "gh", "api", fmt.Sprintf("repos/%s", repoFullName), "--jq", ".stargazers_count")
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	s := strings.TrimSpace(string(out))
	var stars int
	fmt.Sscanf(s, "%d", &stars)
	return stars
}

// deduplicateCodeResults returns one entry per unique repository FullName.
func deduplicateCodeResults(results []ghCodeSearchResult) []ghCodeSearchResult {
	seen := make(map[string]bool, len(results))
	unique := make([]ghCodeSearchResult, 0, len(results))
	for _, r := range results {
		if seen[r.Repository.FullName] {
			continue
		}
		seen[r.Repository.FullName] = true
		unique = append(unique, r)
	}
	return unique
}

// deduplicateSkills returns one entry per unique URL.
func deduplicateSkills(skills []Skill) []Skill {
	seen := make(map[string]bool, len(skills))
	unique := make([]Skill, 0, len(skills))
	for _, s := range skills {
		if seen[s.URL] {
			continue
		}
		seen[s.URL] = true
		unique = append(unique, s)
	}
	return unique
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
