package skill

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
)

// awesomeLists contains known awesome-list URLs for Claude Code skills.
// These are the most popular curated lists (60K+, 12K+, 9K+ stars).
var awesomeLists = []string{
	"https://raw.githubusercontent.com/travisvn/awesome-claude-skills/main/README.md",
	"https://raw.githubusercontent.com/ComposioHQ/awesome-claude-skills/main/README.md",
	"https://raw.githubusercontent.com/BehiSecc/awesome-claude-skills/main/README.md",
}

// linkRegex matches a single Markdown hyperlink: [text](url)
var linkRegex = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

// SearchAwesome fetches and parses awesome-list READMEs for skill links.
// When query is non-empty, only skills whose name or description contains the
// query string (case-insensitive) are returned.
func SearchAwesome(ctx context.Context, query string) ([]Skill, error) {
	return searchAwesomeWithURLs(ctx, query, awesomeLists)
}

// searchAwesomeWithURLs is the testable inner implementation that accepts a list
// of URLs so tests can substitute a local httptest.Server.
func searchAwesomeWithURLs(ctx context.Context, query string, lists []string) ([]Skill, error) {
	var all []Skill

	for _, listURL := range lists {
		skills, err := parseAwesomeList(ctx, listURL)
		if err != nil {
			log.Printf("skill: parsing awesome list %s: %v", listURL, err)
			continue
		}
		all = append(all, skills...)
	}

	if query == "" {
		return all, nil
	}

	// Filter by query — case-insensitive substring match on name or description.
	lower := strings.ToLower(query)
	filtered := make([]Skill, 0, len(all))
	for _, s := range all {
		if strings.Contains(strings.ToLower(s.Name), lower) ||
			strings.Contains(strings.ToLower(s.Description), lower) {
			filtered = append(filtered, s)
		}
	}
	return filtered, nil
}

// parseAwesomeList fetches a single awesome-list README and extracts Skill
// entries from Markdown list items that contain hyperlinks.
func parseAwesomeList(ctx context.Context, listURL string) ([]Skill, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return nil, fmt.Errorf("skill: building awesome list request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("skill: fetching awesome list: %w", err)
	}
	defer resp.Body.Close()

	var skills []Skill
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip headings and empty lines.
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Only process list items (- or *).
		if !strings.HasPrefix(trimmed, "- ") && !strings.HasPrefix(trimmed, "* ") {
			continue
		}

		matches := linkRegex.FindStringSubmatch(line)
		if len(matches) < 3 {
			continue
		}

		name := matches[1]
		url := matches[2]
		desc := extractDescription(line)

		skills = append(skills, Skill{
			Name:        name,
			Description: desc,
			Source:      SourceAwesome,
			URL:         url,
		})
	}

	if err := scanner.Err(); err != nil {
		return skills, fmt.Errorf("skill: scanning awesome list: %w", err)
	}

	return skills, nil
}

// extractDescription pulls the descriptive text after a Markdown link in a list item.
// Supports both " - " and ": " delimiters after the closing parenthesis.
//
//	"- [name](url) - some description"  → "some description"
//	"- [name](url): some description"   → "some description"
func extractDescription(line string) string {
	// Find the end of the last closing parenthesis of the link.
	idx := strings.Index(line, ") - ")
	if idx >= 0 {
		return strings.TrimSpace(line[idx+4:])
	}
	idx = strings.Index(line, "): ")
	if idx >= 0 {
		return strings.TrimSpace(line[idx+3:])
	}
	return ""
}
