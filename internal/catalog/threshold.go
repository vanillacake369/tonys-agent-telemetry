package catalog

import (
	"os"
	"strconv"
)

// MinViableEntries is the minimum corpus size below which the Skills tab
// renders a "catalog stale / partial fetch" warning instead of the
// recommendation pane. Per PIVOT_PLAN Phase 1 gate.
const MinViableEntries = 100

// ResolveMinViable returns the minimum-viable entry count, optionally
// overridden by the TONYS_CATALOG_MIN environment variable.
//
// Rules:
//   - Empty string (or unset env) → returns MinViableEntries (100).
//   - A valid positive integer string → returns that value.
//   - Zero, negative, non-numeric, or overflowing values → returns MinViableEntries.
//
// envVal should be the raw value of os.Getenv("TONYS_CATALOG_MIN"). The caller
// is responsible for reading the env so this function stays pure and testable.
func ResolveMinViable(envVal string) int {
	if envVal == "" {
		return MinViableEntries
	}
	n, err := strconv.Atoi(envVal)
	if err != nil || n <= 0 {
		return MinViableEntries
	}
	return n
}

// resolveMinViableFromEnv is the production helper that reads the env variable
// and delegates to ResolveMinViable. Kept separate so tests can call
// ResolveMinViable directly without depending on the process environment.
func resolveMinViableFromEnv() int {
	return ResolveMinViable(os.Getenv("TONYS_CATALOG_MIN"))
}
