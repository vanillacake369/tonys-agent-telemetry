package catalog

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestFetcher_SentinelSHA_ReturnsErrSentinelSHA verifies that Fetch returns
// ErrSentinelSHA immediately when SourceSHA has not been replaced.
// This test must ALWAYS pass — bypassing it is forbidden per PIVOT_PLAN.
func TestFetcher_SentinelSHA_ReturnsErrSentinelSHA(t *testing.T) {
	f := &HTTPFetcher{
		Client:  &http.Client{},
		BaseURL: "https://example.com",
		SHA:     sentinelSHA, // inject sentinel directly so test is stable regardless of source.go
	}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("Fetch with sentinel SHA should return error, got nil")
	}
	if !errors.Is(err, ErrSentinelSHA) {
		t.Errorf("Fetch with sentinel SHA returned %v, want ErrSentinelSHA", err)
	}
}

// TestFetcher_ValidServer_ReturnsParsedItems verifies that a 200 response
// with valid JSON is parsed and returned correctly.
func TestFetcher_ValidServer_ReturnsParsedItems(t *testing.T) {
	payload := `[
		{"id":"skill/tdd","title":"TDD","type":"skill","description":"Test first.","tags":["tdd"],"maturity_level":4,"source_url":"https://example.com"},
		{"id":"template/api","title":"API Template","type":"template","description":"REST scaffold.","tags":["api"],"maturity_level":3,"source_url":"https://example.com"}
	]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	f := &HTTPFetcher{
		Client:  srv.Client(),
		BaseURL: srv.URL,
		SHA:     "abc1234567890abcdef1234567890abcdef12345678", // fake but non-sentinel
	}

	items, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch returned unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("Fetch returned %d items, want 2", len(items))
	}
	if items[0].ID != "skill/tdd" {
		t.Errorf("items[0].ID = %q, want %q", items[0].ID, "skill/tdd")
	}
}

// TestFetcher_404Server_ReturnsError verifies that a non-200 response
// produces an explicit typed error with the status code.
func TestFetcher_404Server_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}))
	defer srv.Close()

	f := &HTTPFetcher{
		Client:  srv.Client(),
		BaseURL: srv.URL,
		SHA:     "abc1234567890abcdef1234567890abcdef12345678",
	}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("Fetch with 404 server should return error, got nil")
	}

	var httpErr *HTTPStatusError
	if !errors.As(err, &httpErr) {
		t.Errorf("error %v should be *HTTPStatusError", err)
	} else if httpErr.StatusCode != http.StatusNotFound {
		t.Errorf("HTTPStatusError.StatusCode = %d, want %d", httpErr.StatusCode, http.StatusNotFound)
	}
}
