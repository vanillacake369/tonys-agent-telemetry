package skill

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPDescriptor_IsMCP(t *testing.T) {
	cases := []struct {
		name string
		d    MCPDescriptor
		want bool
	}{
		{"empty", MCPDescriptor{}, false},
		{"name only", MCPDescriptor{Name: "x"}, false},
		{"name + tool", MCPDescriptor{Name: "x", Tools: []string{"t"}}, true},
		{"name + resource", MCPDescriptor{Name: "x", Resources: []string{"r"}}, true},
		{"tool only no name", MCPDescriptor{Tools: []string{"t"}}, false},
	}
	for _, c := range cases {
		if got := c.d.IsMCP(); got != c.want {
			t.Errorf("%s: IsMCP = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestFetchMCPDescriptor_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/.well-known/mcp.json") {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"name":"weather-mcp",
			"version":"1.2.0",
			"description":"Weather lookup tools",
			"tools":["get_forecast","get_current"]
		}`))
	}))
	defer srv.Close()

	d, ok := FetchMCPDescriptor(context.Background(), srv.URL)
	if !ok || d == nil {
		t.Fatal("expected descriptor")
	}
	if d.Name != "weather-mcp" || len(d.Tools) != 2 {
		t.Errorf("unexpected descriptor: %+v", d)
	}
}

func TestFetchMCPDescriptor_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()
	if _, ok := FetchMCPDescriptor(context.Background(), srv.URL); ok {
		t.Error("expected (nil, false) on 404")
	}
}

func TestFetchMCPDescriptor_InvalidShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"name":"x"}`)) // no tools/resources
	}))
	defer srv.Close()
	if _, ok := FetchMCPDescriptor(context.Background(), srv.URL); ok {
		t.Error("expected (nil, false) when no tools/resources")
	}
}

func TestFetchMCPDescriptor_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()
	if _, ok := FetchMCPDescriptor(context.Background(), srv.URL); ok {
		t.Error("expected (nil, false) on malformed JSON")
	}
}

func TestGitHubRawBase(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"https://github.com/foo/bar", "https://raw.githubusercontent.com/foo/bar/main"},
		{"https://github.com/foo/bar.git", "https://raw.githubusercontent.com/foo/bar/main"},
		{"https://github.com/foo/bar/tree/dev", "https://raw.githubusercontent.com/foo/bar/main"},
		{"https://gitlab.com/foo/bar", ""},
		{"https://github.com/foo", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := GitHubRawBase(c.in); got != c.want {
			t.Errorf("GitHubRawBase(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
