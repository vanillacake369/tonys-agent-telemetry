package catalog

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestFetcher_LiveUpstream_Smoke performs a live network round-trip against
// the real upstream repository and asserts that ParseMarkdown produces at
// least MinViableEntries valid items.
//
// IMPORTANT: This test is default-skipped because it requires network access
// and would make CI dependent on GitHub availability.
//
// To run manually:
//
//	TONYS_LIVE_UPSTREAM=1 go test ./internal/catalog/... -run TestFetcher_LiveUpstream_Smoke -v
//
// The test fetches examples/CATALOG.md from the raw.githubusercontent.com CDN
// at the pinned SourceSHA (not the live main branch), so the result is
// reproducible for as long as the SHA remains reachable.
func TestFetcher_LiveUpstream_Smoke(t *testing.T) {
	if os.Getenv("TONYS_LIVE_UPSTREAM") != "1" {
		t.Skip("skipping live upstream smoke test; set TONYS_LIVE_UPSTREAM=1 to run")
	}
	if SourceSHA == sentinelSHA {
		t.Fatal("SourceSHA is still the sentinel placeholder; cannot run live smoke test")
	}

	const rawBaseURL = "https://raw.githubusercontent.com"
	const urlFmt = rawBaseURL + "/FlorianBruniaux/claude-code-ultimate-guide/" + SourceSHA + "/examples/CATALOG.md"

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlFmt, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HTTP status %d, want 200", resp.StatusCode)
	}

	body := make([]byte, 0, 1<<16)
	buf := make([]byte, 4096)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			body = append(body, buf[:n]...)
		}
		if readErr != nil {
			break
		}
	}

	items, err := ParseMarkdown(body)
	if err != nil {
		t.Fatalf("ParseMarkdown error: %v", err)
	}

	minViable := resolveMinViableFromEnv()
	if len(items) < minViable {
		t.Errorf("live upstream returned %d valid items, want at least %d (MinViableEntries)",
			len(items), minViable)
	}
	t.Logf("live upstream smoke OK: %d valid items (MinViableEntries=%d, SHA=%s)",
		len(items), minViable, SourceSHA)
}
