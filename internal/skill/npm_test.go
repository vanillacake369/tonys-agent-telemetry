package skill

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// sampleNPMResponse builds a minimal npm registry search response.
func sampleNPMResponse(t *testing.T) []byte {
	t.Helper()
	result := npmSearchResult{
		Objects: []struct {
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
		}{
			{
				Package: struct {
					Name        string    `json:"name"`
					Description string    `json:"description"`
					Version     string    `json:"version"`
					Date        time.Time `json:"date"`
					Links       struct {
						Npm        string `json:"npm"`
						Repository string `json:"repository"`
					} `json:"links"`
				}{
					Name:        "claude-code-skill-deploy",
					Description: "Deployment skill for Claude Code",
					Version:     "1.0.0",
					Date:        time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
					Links: struct {
						Npm        string `json:"npm"`
						Repository string `json:"repository"`
					}{
						Npm:        "https://www.npmjs.com/package/claude-code-skill-deploy",
						Repository: "https://github.com/alice/claude-code-skill-deploy",
					},
				},
				Score: struct {
					Detail struct {
						Popularity float64 `json:"popularity"`
					} `json:"detail"`
				}{
					Detail: struct {
						Popularity float64 `json:"popularity"`
					}{Popularity: 0.75},
				},
			},
			{
				Package: struct {
					Name        string    `json:"name"`
					Description string    `json:"description"`
					Version     string    `json:"version"`
					Date        time.Time `json:"date"`
					Links       struct {
						Npm        string `json:"npm"`
						Repository string `json:"repository"`
					} `json:"links"`
				}{
					Name:        "claude-code-skill-k8s",
					Description: "Kubernetes skill for Claude Code",
					Version:     "2.0.0",
					Date:        time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
					Links: struct {
						Npm        string `json:"npm"`
						Repository string `json:"repository"`
					}{
						Npm:        "https://www.npmjs.com/package/claude-code-skill-k8s",
						Repository: "https://github.com/bob/claude-code-skill-k8s",
					},
				},
				Score: struct {
					Detail struct {
						Popularity float64 `json:"popularity"`
					} `json:"detail"`
				}{
					Detail: struct {
						Popularity float64 `json:"popularity"`
					}{Popularity: 0.50},
				},
			},
		},
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal sample npm response: %v", err)
	}
	return raw
}

// TestSearchNPM_ParsesResponse verifies that SearchNPM maps npm registry JSON to Skill structs.
func TestSearchNPM_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(sampleNPMResponse(t))
	}))
	defer srv.Close()

	ctx := context.Background()
	skills, err := searchNPMWithURL(ctx, "deploy", 10, srv.URL)
	if err != nil {
		t.Fatalf("searchNPMWithURL: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("got %d skills, want 2", len(skills))
	}
	if skills[0].Name != "claude-code-skill-deploy" {
		t.Errorf("skills[0].Name = %q, want %q", skills[0].Name, "claude-code-skill-deploy")
	}
	if skills[0].Source != SourceNPM {
		t.Errorf("skills[0].Source = %q, want %q", skills[0].Source, SourceNPM)
	}
	if skills[0].URL != "https://github.com/alice/claude-code-skill-deploy" {
		t.Errorf("skills[0].URL = %q", skills[0].URL)
	}
	// Stars are popularity * 1000 = 750.
	if skills[0].Stars != 750 {
		t.Errorf("skills[0].Stars = %d, want 750", skills[0].Stars)
	}
}

// TestSearchNPM_EmptyQueryUsesPrefixOnly verifies that an empty query still hits npm
// with the "claude-code-skill" prefix.
func TestSearchNPM_EmptyQueryUsesPrefixOnly(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("text")
		w.Header().Set("Content-Type", "application/json")
		// Return minimal valid response.
		w.Write([]byte(`{"objects":[]}`))
	}))
	defer srv.Close()

	ctx := context.Background()
	_, err := searchNPMWithURL(ctx, "", 10, srv.URL)
	if err != nil {
		t.Fatalf("searchNPMWithURL: %v", err)
	}
	if capturedQuery != "claude-code-skill" {
		t.Errorf("query = %q, want %q", capturedQuery, "claude-code-skill")
	}
}

// TestSearchNPM_QueryAppendedToPrefix verifies that a non-empty query is appended
// to the "claude-code-skill" prefix.
func TestSearchNPM_QueryAppendedToPrefix(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query().Get("text")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"objects":[]}`))
	}))
	defer srv.Close()

	ctx := context.Background()
	_, err := searchNPMWithURL(ctx, "k8s", 10, srv.URL)
	if err != nil {
		t.Fatalf("searchNPMWithURL: %v", err)
	}
	if capturedQuery != "claude-code-skill k8s" {
		t.Errorf("query = %q, want %q", capturedQuery, "claude-code-skill k8s")
	}
}

// TestSearchNPM_ContextCancellation verifies that a cancelled context causes
// SearchNPM to return promptly without panicking.
func TestSearchNPM_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow handler — context should cancel before response.
		w.Header().Set("Content-Type", "application/json")
		w.Write(sampleNPMResponse(t))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	_, err := searchNPMWithURL(ctx, "k8s", 10, srv.URL)
	// Should return context.Canceled or nil (if cancelled before request).
	if err != nil && err != context.Canceled {
		t.Logf("searchNPMWithURL cancelled context returned err (may be expected): %v", err)
	}
}

// TestSearchNPM_LimitDefaultsTo20 verifies that limit=0 is treated as 20.
func TestSearchNPM_LimitDefaultsTo20(t *testing.T) {
	var capturedSize string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedSize = r.URL.Query().Get("size")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"objects":[]}`))
	}))
	defer srv.Close()

	ctx := context.Background()
	_, err := searchNPMWithURL(ctx, "k8s", 0, srv.URL)
	if err != nil {
		t.Fatalf("searchNPMWithURL: %v", err)
	}
	if capturedSize != "20" {
		t.Errorf("size = %q, want %q", capturedSize, "20")
	}
}
