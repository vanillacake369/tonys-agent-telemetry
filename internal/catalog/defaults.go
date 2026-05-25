package catalog

import (
	"os"
	"path/filepath"
	"time"
)

// DefaultTTL is the time after which a cached catalog is considered stale.
// SSoT: callers (cache, Skills tab) all reference this constant.
const DefaultTTL = 24 * time.Hour

// DefaultHTTPTimeout is the timeout for a single catalog fetch HTTP request.
// SSoT: HTTPFetcher uses this as its default client timeout.
const DefaultHTTPTimeout = 5 * time.Second

// catalogEnvPath is the environment variable name for overriding the cache path.
const catalogEnvPath = "TONYS_CATALOG_PATH"

// ResolveCachePath returns the filesystem path for the catalog cache JSON file.
// Priority: TONYS_CATALOG_PATH env var → OS user cache dir / tonys-agent-telemetry/catalog/items.json.
// Returns a non-empty path in all cases (falls back to a relative path on error).
func ResolveCachePath() string {
	if override := os.Getenv(catalogEnvPath); override != "" {
		return override
	}
	base, err := os.UserCacheDir()
	if err != nil {
		// Unlikely but safe fallback: write next to the binary.
		return filepath.Join("cache", "tonys-agent-telemetry", "catalog", "items.json")
	}
	return filepath.Join(base, "tonys-agent-telemetry", "catalog", "items.json")
}
