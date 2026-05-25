package control

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Policy is the top-level configuration loaded from policy.toml.
type Policy struct {
	Budget BudgetPolicy `toml:"budget"`
	Tools  ToolPolicy   `toml:"tools"`
	Models ModelsPolicy `toml:"models"`
}

// BudgetPolicy defines USD spending caps.
type BudgetPolicy struct {
	SessionMaxUSD  float64 `toml:"session_max_usd"`
	DailyMaxUSD    float64 `toml:"daily_max_usd"`
	WarnAtFraction float64 `toml:"warn_at_fraction"`
}

// ToolPolicy defines allowlist and denylist glob patterns.
type ToolPolicy struct {
	Denylist  []string `toml:"denylist"`
	Allowlist []string `toml:"allowlist"`
}

// ModelsPolicy holds per-model pricing overrides.
type ModelsPolicy struct {
	Pricing map[string]ModelPrice `toml:"pricing"`
}

// ModelPrice is the USD cost per 1M tokens for a specific model.
type ModelPrice struct {
	Input  float64 `toml:"input"`
	Output float64 `toml:"output"`
}

// DefaultPolicy returns a fail-open policy (no caps, no allow/denylists).
func DefaultPolicy() Policy {
	return Policy{
		Budget: BudgetPolicy{
			SessionMaxUSD:  0,
			DailyMaxUSD:    0,
			WarnAtFraction: 0.8,
		},
		Tools: ToolPolicy{
			Denylist:  []string{},
			Allowlist: []string{},
		},
		Models: ModelsPolicy{
			Pricing: map[string]ModelPrice{},
		},
	}
}

// policyPath returns the canonical path to policy.toml, respecting XDG_CONFIG_HOME.
func policyPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "tonys-agent-telemetry", "policy.toml")
}

// LoadPolicy reads the policy file. On any error it logs to stderr and returns DefaultPolicy.
func LoadPolicy() (Policy, error) {
	p := policyPath()
	if p == "" {
		fmt.Fprintln(os.Stderr, "[tat-control] policy path unavailable — using defaults (fail-open)")
		return DefaultPolicy(), nil
	}

	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return DefaultPolicy(), nil
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "[tat-control] cannot read policy file %s: %v — using defaults (fail-open)\n", p, err)
		return DefaultPolicy(), err
	}

	var pol Policy
	if _, err := toml.Decode(string(data), &pol); err != nil {
		fmt.Fprintf(os.Stderr, "[tat-control] malformed policy file %s: %v — using defaults (fail-open)\n", p, err)
		return DefaultPolicy(), err
	}

	if pol.Models.Pricing == nil {
		pol.Models.Pricing = map[string]ModelPrice{}
	}
	return pol, nil
}

// Match reports whether target matches any of the glob patterns.
// * matches any sequence of characters including path separators (unlike path.Match).
// ? matches any single character.
func Match(patterns []string, target string) bool {
	for _, pat := range patterns {
		if matchGlob(pat, target) {
			return true
		}
	}
	return false
}

// matchGlob implements simple glob matching where * matches any sequence of
// characters (including / and spaces) and ? matches any single character.
func matchGlob(pattern, s string) bool {
	p := []rune(pattern)
	t := []rune(s)
	return matchGlobRunes(p, t)
}

func matchGlobRunes(p, t []rune) bool {
	for len(p) > 0 {
		switch p[0] {
		case '*':
			// Skip consecutive stars.
			for len(p) > 0 && p[0] == '*' {
				p = p[1:]
			}
			if len(p) == 0 {
				return true
			}
			// Try matching the rest of the pattern at every position in t.
			for i := 0; i <= len(t); i++ {
				if matchGlobRunes(p, t[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(t) == 0 {
				return false
			}
			p = p[1:]
			t = t[1:]
		default:
			if len(t) == 0 || p[0] != t[0] {
				return false
			}
			p = p[1:]
			t = t[1:]
		}
	}
	return len(t) == 0
}
