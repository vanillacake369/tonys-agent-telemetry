package catalog

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// sentinelSHA is the placeholder value that signals SourceSHA has not yet been
// replaced with a real commit. Exported as a package-private constant so
// fetcher tests can inject it directly without importing source.go values.
const sentinelSHA = "REPLACE_WITH_REAL_SHA_BEFORE_PHASE_1_INGEST"

// ErrSentinelSHA is returned by HTTPFetcher.Fetch when the SHA field still
// holds the sentinel placeholder value. Callers must not bypass this check.
var ErrSentinelSHA = errors.New("catalog: SourceSHA is still the sentinel placeholder; replace with a real 40-char hex SHA before fetching")

// HTTPStatusError carries the HTTP status code from a non-200 response.
// Use errors.As to extract it from an error chain.
type HTTPStatusError struct {
	StatusCode int
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("catalog: HTTP fetch returned status %d", e.StatusCode)
}

// Fetcher is the interface for retrieving catalog items from the upstream source.
// Implementations: HTTPFetcher (production), and test doubles via httptest.
type Fetcher interface {
	Fetch(ctx context.Context) ([]Item, error)
}

// HTTPFetcher fetches the catalog JSON from the pinned upstream repository.
// SRP: this file only handles HTTP fetch + hand-off to Parse. Disk caching is in cache.go.
//
// URL pattern: {BaseURL}/{owner}/{repo}/raw/{SHA}/catalog.json
// In production, BaseURL = "https://raw.githubusercontent.com" and the owner/repo
// are derived from SourceRepoURL. In tests, BaseURL points at an httptest.Server.
type HTTPFetcher struct {
	// Client is the HTTP client to use. Injectable so tests can use httptest.Server.Client().
	// Production code should pass an *http.Client with DefaultHTTPTimeout.
	Client *http.Client

	// BaseURL is the scheme+host for the raw content URL.
	// Production: "https://raw.githubusercontent.com"
	// Tests: httptest.Server.URL
	BaseURL string

	// SHA is the pinned commit hash. Must not equal sentinelSHA.
	SHA string
}

// NewHTTPFetcher returns an HTTPFetcher configured for production use.
// It targets the pinned SourceSHA from source.go. The caller is responsible
// for detecting ErrSentinelSHA and handling the unresolved-SHA case gracefully.
func NewHTTPFetcher() *HTTPFetcher {
	return &HTTPFetcher{
		Client:  &http.Client{Timeout: DefaultHTTPTimeout},
		BaseURL: "https://raw.githubusercontent.com",
		SHA:     SourceSHA,
	}
}

// Fetch fetches catalog.json from the upstream repository at the pinned SHA.
// Returns ErrSentinelSHA immediately if SHA is still the placeholder.
// Returns *HTTPStatusError for non-200 responses.
func (f *HTTPFetcher) Fetch(ctx context.Context) ([]Item, error) {
	if f.SHA == sentinelSHA {
		return nil, ErrSentinelSHA
	}

	// URL: {BaseURL}/FlorianBruniaux/claude-code-ultimate-guide/raw/{SHA}/catalog.json
	// We hard-code the owner/repo from SourceRepoURL rather than parsing it at
	// runtime to keep the fetcher deterministic and testable.
	url := fmt.Sprintf("%s/FlorianBruniaux/claude-code-ultimate-guide/raw/%s/catalog.json", f.BaseURL, f.SHA)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("catalog: build request: %w", err)
	}

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("catalog: HTTP GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPStatusError{StatusCode: resp.StatusCode}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("catalog: read body: %w", err)
	}

	return Parse(body)
}
