package skill

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const sampleAwesomeReadme = `# Awesome Claude Code Skills

A curated list of awesome Claude Code skills.

## Deployment

- [deploy-skill](https://github.com/alice/deploy-skill) - Deploy services automatically
- [k8s-skill](https://github.com/bob/k8s-skill) - Manage Kubernetes workloads
* [helm-skill](https://github.com/carol/helm-skill): Helm chart management tool

## Code Review

- [pr-review](https://github.com/dave/pr-review) - Auto review pull requests

## Other

- [no-link-item]
- just text without a link
`

// TestParseAwesomeList_ExtractsLinks verifies that markdown links and descriptions
// are correctly parsed from an awesome-list README.
func TestParseAwesomeList_ExtractsLinks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(sampleAwesomeReadme))
	}))
	defer srv.Close()

	ctx := context.Background()
	skills, err := parseAwesomeList(ctx, srv.URL)
	if err != nil {
		t.Fatalf("parseAwesomeList: %v", err)
	}

	// Should find: deploy-skill, k8s-skill, helm-skill, pr-review — 4 entries.
	if len(skills) != 4 {
		t.Fatalf("got %d skills, want 4: %v", len(skills), skills)
	}

	// Verify first entry.
	if skills[0].Name != "deploy-skill" {
		t.Errorf("skills[0].Name = %q, want %q", skills[0].Name, "deploy-skill")
	}
	if skills[0].URL != "https://github.com/alice/deploy-skill" {
		t.Errorf("skills[0].URL = %q", skills[0].URL)
	}
	if skills[0].Source != SourceAwesome {
		t.Errorf("skills[0].Source = %q, want %q", skills[0].Source, SourceAwesome)
	}
	if skills[0].Description != "Deploy services automatically" {
		t.Errorf("skills[0].Description = %q", skills[0].Description)
	}
}

// TestParseAwesomeList_DescriptionFormats verifies both " - " and ": " description delimiters.
func TestParseAwesomeList_DescriptionFormats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(sampleAwesomeReadme))
	}))
	defer srv.Close()

	ctx := context.Background()
	skills, err := parseAwesomeList(ctx, srv.URL)
	if err != nil {
		t.Fatalf("parseAwesomeList: %v", err)
	}

	// helm-skill uses ": " delimiter.
	var helmSkill *Skill
	for i := range skills {
		if skills[i].Name == "helm-skill" {
			helmSkill = &skills[i]
			break
		}
	}
	if helmSkill == nil {
		t.Fatalf("helm-skill not found in results")
	}
	if helmSkill.Description != "Helm chart management tool" {
		t.Errorf("helm-skill.Description = %q, want %q", helmSkill.Description, "Helm chart management tool")
	}
}

// TestSearchAwesome_QueryFiltering verifies that only matching skills are returned
// when a query is provided.
func TestSearchAwesome_QueryFiltering(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(sampleAwesomeReadme))
	}))
	defer srv.Close()

	ctx := context.Background()
	skills, err := searchAwesomeWithURLs(ctx, "k8s", []string{srv.URL})
	if err != nil {
		t.Fatalf("searchAwesomeWithURLs: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1 (only k8s-skill matches)", len(skills))
	}
	if skills[0].Name != "k8s-skill" {
		t.Errorf("skills[0].Name = %q, want %q", skills[0].Name, "k8s-skill")
	}
}

// TestSearchAwesome_EmptyQueryReturnsAll verifies that an empty query returns all skills.
func TestSearchAwesome_EmptyQueryReturnsAll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(sampleAwesomeReadme))
	}))
	defer srv.Close()

	ctx := context.Background()
	skills, err := searchAwesomeWithURLs(ctx, "", []string{srv.URL})
	if err != nil {
		t.Fatalf("searchAwesomeWithURLs: %v", err)
	}
	if len(skills) != 4 {
		t.Fatalf("got %d skills, want 4", len(skills))
	}
}

// TestSearchAwesome_QueryMatchesDescription verifies that query matching checks description too.
func TestSearchAwesome_QueryMatchesDescription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(sampleAwesomeReadme))
	}))
	defer srv.Close()

	ctx := context.Background()
	// "pull requests" is in the description of pr-review only.
	skills, err := searchAwesomeWithURLs(ctx, "pull requests", []string{srv.URL})
	if err != nil {
		t.Fatalf("searchAwesomeWithURLs: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if skills[0].Name != "pr-review" {
		t.Errorf("skills[0].Name = %q, want %q", skills[0].Name, "pr-review")
	}
}

// TestSearchAwesome_CaseInsensitiveFiltering verifies case-insensitive query matching.
func TestSearchAwesome_CaseInsensitiveFiltering(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(sampleAwesomeReadme))
	}))
	defer srv.Close()

	ctx := context.Background()
	skills, err := searchAwesomeWithURLs(ctx, "DEPLOY", []string{srv.URL})
	if err != nil {
		t.Fatalf("searchAwesomeWithURLs: %v", err)
	}
	// Should match "deploy-skill" (name) and "Deploy services automatically" (description).
	if len(skills) == 0 {
		t.Fatalf("got 0 skills, want at least 1 for query DEPLOY")
	}
	found := false
	for _, s := range skills {
		if strings.Contains(strings.ToLower(s.Name), "deploy") ||
			strings.Contains(strings.ToLower(s.Description), "deploy") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no deploy-related skill found for query DEPLOY")
	}
}

// TestParseAwesomeList_SkipsNonLinks verifies that lines without valid markdown links
// are skipped gracefully.
func TestParseAwesomeList_SkipsNonLinks(t *testing.T) {
	readme := `## Section
- just text without a link
- [no-url]
- [valid](https://example.com/skill) - A valid skill
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(readme))
	}))
	defer srv.Close()

	ctx := context.Background()
	skills, err := parseAwesomeList(ctx, srv.URL)
	if err != nil {
		t.Fatalf("parseAwesomeList: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if skills[0].Name != "valid" {
		t.Errorf("skills[0].Name = %q, want %q", skills[0].Name, "valid")
	}
}

// TestParseAwesomeList_ContextCancellation verifies graceful handling of cancelled context.
func TestParseAwesomeList_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(sampleAwesomeReadme))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := parseAwesomeList(ctx, srv.URL)
	// Should either be nil or context.Canceled — must not panic.
	if err != nil && err != context.Canceled {
		t.Logf("parseAwesomeList cancelled context returned err (may be expected): %v", err)
	}
}
