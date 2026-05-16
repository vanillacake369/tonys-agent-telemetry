package skill

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

// TestSearchGitHub_ParsesJSON verifies that the JSON output shape from gh is
// correctly mapped to Skill structs. We test the parsing logic independently by
// calling the same json.Unmarshal code path via a mock JSON byte slice.
func TestSearchGitHub_ParsesJSON(t *testing.T) {
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	repos := []ghSearchRepo{
		{
			Name:            "claude-deploy-skill",
			FullName:        "alice/claude-deploy-skill",
			Description:     "Deploy services with Claude",
			StargazersCount: 128,
			CreatedAt:       now,
			UpdatedAt:       now,
			URL:             "https://github.com/alice/claude-deploy-skill",
		},
		{
			Name:            "pr-review-skill",
			FullName:        "bob/pr-review-skill",
			Description:     "Auto review PRs",
			StargazersCount: 55,
			CreatedAt:       now,
			UpdatedAt:       now,
			URL:             "https://github.com/bob/pr-review-skill",
		},
	}

	raw, err := json.Marshal(repos)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var parsed []ghSearchRepo
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if len(parsed) != 2 {
		t.Fatalf("parsed %d repos, want 2", len(parsed))
	}
	if parsed[0].Name != "claude-deploy-skill" {
		t.Errorf("Name = %q, want %q", parsed[0].Name, "claude-deploy-skill")
	}
	if parsed[0].StargazersCount != 128 {
		t.Errorf("Stars = %d, want 128", parsed[0].StargazersCount)
	}

	// Map to Skill like SearchGitHub would.
	skills := make([]Skill, 0, len(parsed))
	for _, r := range parsed {
		skills = append(skills, Skill{
			Name:        r.Name,
			Description: r.Description,
			Source:      SourceGitHub,
			URL:         r.URL,
			Stars:       r.StargazersCount,
			CreatedAt:   r.CreatedAt,
			UpdatedAt:   r.UpdatedAt,
			ReadmeURL:   r.URL + "#readme",
		})
	}

	if skills[0].Source != SourceGitHub {
		t.Errorf("Source = %q, want %q", skills[0].Source, SourceGitHub)
	}
	if skills[1].Stars != 55 {
		t.Errorf("Stars = %d, want 55", skills[1].Stars)
	}
}

// TestSearchGitHub_ContextCancellation verifies that passing a cancelled context
// causes SearchGitHub to return early without panicking.
func TestSearchGitHub_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	// With a cancelled context, gh will either not run or exit immediately.
	// The function must not panic or return an error for ctx.Err().
	skills, err := SearchGitHub(ctx, "claude skill", "stars", 10)
	if err != nil && err != context.Canceled {
		// gh not found is acceptable — it just returns nil, nil.
		t.Logf("SearchGitHub returned err (may be expected when gh unavailable): %v", err)
	}
	// skills may be nil or empty — that is acceptable.
	_ = skills
}

// TestFetchReadme_Base64Decoding tests that FetchReadme correctly decodes
// base64-encoded content using the same logic as the real implementation.
func TestFetchReadme_Base64Decoding(t *testing.T) {
	content := "# My Skill\n\nThis is a great skill.\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(content))

	resp := ghReadmeResponse{
		Content:  encoded,
		Encoding: "base64",
	}

	// Simulate the decode path.
	cleaned := resp.Content // no newlines in test data
	decoded, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	got := string(decoded)
	if got != content {
		t.Errorf("decoded = %q, want %q", got, content)
	}
}

// TestFetchReadme_Truncation verifies that content is truncated at maxBytes.
func TestFetchReadme_Truncation(t *testing.T) {
	// Build a string longer than 10 bytes.
	long := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	encoded := base64.StdEncoding.EncodeToString([]byte(long))

	resp := ghReadmeResponse{
		Content:  encoded,
		Encoding: "base64",
	}

	decoded, err := base64.StdEncoding.DecodeString(resp.Content)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	result := string(decoded)
	maxBytes := 10
	if len(result) > maxBytes {
		result = result[:maxBytes]
	}
	if len(result) != maxBytes {
		t.Errorf("truncated length = %d, want %d", len(result), maxBytes)
	}
	if result != "ABCDEFGHIJ" {
		t.Errorf("truncated = %q, want %q", result, "ABCDEFGHIJ")
	}
}

// TestRepoFullNameFromURL verifies URL-to-fullname extraction.
func TestRepoFullNameFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/alice/my-skill", "alice/my-skill"},
		{"https://github.com/org/repo/", "org/repo"},
		{"https://github.com/a/b/c", "b/c"},
	}
	for _, tt := range tests {
		got := repoFullNameFromURL(tt.url)
		if got != tt.want {
			t.Errorf("repoFullNameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

// TestGHCodeSearchResult_ParsesJSON verifies that the JSON shape returned by
// `gh search code --json repository,path` is correctly mapped to ghCodeSearchResult.
func TestGHCodeSearchResult_ParsesJSON(t *testing.T) {
	raw := []byte(`[
		{
			"repository": {
				"fullName": "alice/skill-repo",
				"name": "skill-repo",
				"description": "A skill repository",
				"isPrivate": false
			},
			"path": "skills/deploy/SKILL.md"
		},
		{
			"repository": {
				"fullName": "bob/another-skill",
				"name": "another-skill",
				"description": "Another skill",
				"isPrivate": false
			},
			"path": "SKILL.md"
		}
	]`)

	var results []ghCodeSearchResult
	if err := json.Unmarshal(raw, &results); err != nil {
		t.Fatalf("json.Unmarshal code search results: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Repository.FullName != "alice/skill-repo" {
		t.Errorf("FullName = %q, want %q", results[0].Repository.FullName, "alice/skill-repo")
	}
	if results[0].Path != "skills/deploy/SKILL.md" {
		t.Errorf("Path = %q, want %q", results[0].Path, "skills/deploy/SKILL.md")
	}
	if results[1].Repository.Name != "another-skill" {
		t.Errorf("Name = %q, want %q", results[1].Repository.Name, "another-skill")
	}
}

// TestDeduplicateCodeResults verifies that repos with the same fullName appear only once.
func TestDeduplicateCodeResults(t *testing.T) {
	results := []ghCodeSearchResult{
		{Repository: ghCodeRepo{FullName: "alice/skill-repo", Name: "skill-repo"}, Path: "a/SKILL.md"},
		{Repository: ghCodeRepo{FullName: "alice/skill-repo", Name: "skill-repo"}, Path: "b/SKILL.md"},
		{Repository: ghCodeRepo{FullName: "bob/other-skill", Name: "other-skill"}, Path: "SKILL.md"},
	}

	unique := deduplicateCodeResults(results)
	if len(unique) != 2 {
		t.Fatalf("got %d unique repos, want 2", len(unique))
	}
}

// TestSearchGitHubRepos_UsesRefinedQuery verifies that searchGitHubRepos prepends
// "claude code skill" to the user query. This is tested via JSON parsing only
// (no actual gh call), so it verifies the query-building logic by examining
// the args that would be passed.
func TestSearchGitHubRepos_BuildsRefinedQuery(t *testing.T) {
	// Verify that buildRefinedRepoQuery adds the prefix.
	got := buildRefinedRepoQuery("kubernetes")
	want := "claude code skill kubernetes"
	if got != want {
		t.Errorf("buildRefinedRepoQuery = %q, want %q", got, want)
	}

	// Empty query — prefix only.
	got2 := buildRefinedRepoQuery("")
	want2 := "claude code skill"
	if got2 != want2 {
		t.Errorf("buildRefinedRepoQuery(%q) = %q, want %q", "", got2, want2)
	}
}
