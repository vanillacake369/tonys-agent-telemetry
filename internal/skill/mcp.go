package skill

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// MCPDescriptor is the subset of `.well-known/mcp.json` we care about. The
// spec is still moving; we tolerate unknown fields and only consume what
// helps the marketplace UI label MCP-capable skills.
type MCPDescriptor struct {
	Name        string   `json:"name"`
	Version     string   `json:"version,omitempty"`
	Description string   `json:"description,omitempty"`
	Tools       []string `json:"tools,omitempty"`
	Resources   []string `json:"resources,omitempty"`
}

// IsMCP reports whether the descriptor looks structurally valid — at minimum
// it must declare a name and at least one tool or resource. Otherwise any
// random JSON would pass.
func (d MCPDescriptor) IsMCP() bool {
	return d.Name != "" && (len(d.Tools) > 0 || len(d.Resources) > 0)
}

// FetchMCPDescriptor attempts to download .well-known/mcp.json from a base
// URL (typically a GitHub repo's raw/main root). Returns (nil, false) on
// any error, missing key, or invalid JSON — callers treat absence as
// "not MCP" rather than a hard failure.
//
// Best-effort timeout: 1s. This runs alongside other GitHub HTTP requests
// and must not gate marketplace UI.
func FetchMCPDescriptor(ctx context.Context, baseURL string) (*MCPDescriptor, bool) {
	url := strings.TrimRight(baseURL, "/") + "/.well-known/mcp.json"
	rctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false
	}
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if err != nil {
		return nil, false
	}
	var d MCPDescriptor
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, false
	}
	if !d.IsMCP() {
		return nil, false
	}
	return &d, true
}

// GitHubRawBase converts an https://github.com/owner/repo URL into the
// raw.githubusercontent.com/owner/repo/main root used to fetch
// well-known files without API auth. Returns the empty string for inputs
// that don't match the expected shape.
func GitHubRawBase(repoURL string) string {
	const prefix = "https://github.com/"
	if !strings.HasPrefix(repoURL, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(repoURL, prefix)
	rest = strings.TrimSuffix(rest, ".git")
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	return "https://raw.githubusercontent.com/" + parts[0] + "/" + parts[1] + "/main"
}
