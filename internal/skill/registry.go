package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
)

// registryRepo is a known monorepo containing multiple skills in a tree structure.
// Each has a pattern like "skills/{category}/{name}/SKILL.md".
type registryRepo struct {
	Owner       string // e.g. "mattpocock"
	Repo        string // e.g. "skills"
	Branch      string // e.g. "main"
	Description string // repo-level description
}

// knownRegistries lists curated skill monorepos to crawl.
var knownRegistries = []registryRepo{
	{Owner: "mattpocock", Repo: "skills", Branch: "main", Description: "Matt Pocock's Claude Code skills collection"},
}

// ghTreeEntry is one entry from the GitHub Git Trees API.
type ghTreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"` // "blob" or "tree"
}

// ghTreeResponse is the response from `gh api repos/{owner}/{repo}/git/trees/{branch}?recursive=1`.
type ghTreeResponse struct {
	Tree []ghTreeEntry `json:"tree"`
}

// SearchRegistries crawls known skill registry repos and returns skills found.
// Each SKILL.md file in the tree is treated as a separate skill.
// When query is non-empty, filters by name/path match.
func SearchRegistries(ctx context.Context, query string) ([]Skill, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, nil
	}

	var all []Skill
	for _, reg := range knownRegistries {
		skills, err := crawlRegistry(ctx, reg)
		if err != nil {
			log.Printf("skill: crawling registry %s/%s: %v", reg.Owner, reg.Repo, err)
			continue
		}
		all = append(all, skills...)
	}

	if query == "" {
		return all, nil
	}

	// Filter by query.
	lower := strings.ToLower(query)
	filtered := make([]Skill, 0)
	for _, s := range all {
		if strings.Contains(strings.ToLower(s.Name), lower) ||
			strings.Contains(strings.ToLower(s.Description), lower) {
			filtered = append(filtered, s)
		}
	}
	return filtered, nil
}

// crawlRegistry fetches the git tree of a registry repo and extracts SKILL.md paths.
func crawlRegistry(ctx context.Context, reg registryRepo) ([]Skill, error) {
	endpoint := fmt.Sprintf("repos/%s/%s/git/trees/%s?recursive=1", reg.Owner, reg.Repo, reg.Branch)
	cmd := exec.CommandContext(ctx, "gh", "api", endpoint)
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("gh api %s: %w", endpoint, err)
	}

	var tree ghTreeResponse
	if err := json.Unmarshal(out, &tree); err != nil {
		return nil, fmt.Errorf("parsing tree: %w", err)
	}

	repoURL := fmt.Sprintf("https://github.com/%s/%s", reg.Owner, reg.Repo)
	var skills []Skill
	for _, entry := range tree.Tree {
		if entry.Type != "blob" {
			continue
		}
		if filepath.Base(entry.Path) != "SKILL.md" {
			continue
		}

		// Derive skill name from parent directory.
		dir := filepath.Dir(entry.Path)
		name := filepath.Base(dir)
		if name == "." || name == "" {
			continue
		}

		// Skip deprecated skills.
		if strings.Contains(dir, "deprecated") {
			continue
		}

		// Category from grandparent dir (e.g. "engineering", "productivity").
		category := ""
		grandparent := filepath.Dir(dir)
		if grandparent != "." {
			category = filepath.Base(grandparent)
		}

		desc := reg.Description
		if category != "" {
			desc = fmt.Sprintf("[%s] %s", category, reg.Description)
		}

		skillURL := fmt.Sprintf("%s/blob/%s/%s", repoURL, reg.Branch, entry.Path)

		skills = append(skills, Skill{
			Name:        name,
			Description: desc,
			Source:      SourceGitHub,
			URL:         skillURL,
			ReadmeURL:   skillURL,
		})
	}

	return skills, nil
}

// AddRegistry adds a custom registry repo to the known list at runtime.
// Useful for user-configured registries.
func AddRegistry(owner, repo, branch string) {
	knownRegistries = append(knownRegistries, registryRepo{
		Owner:  owner,
		Repo:   repo,
		Branch: branch,
	})
}
