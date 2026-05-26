package catalog

import (
	"testing"
)

// TestResolveMinViable_DefaultIs100 verifies that an empty env string returns
// the canonical MinViableEntries constant (100), per PIVOT_PLAN Phase 1 gate.
func TestResolveMinViable_DefaultIs100(t *testing.T) {
	got := ResolveMinViable("")
	if got != MinViableEntries {
		t.Errorf("ResolveMinViable(%q) = %d, want %d (MinViableEntries)", "", got, MinViableEntries)
	}
	if MinViableEntries != 100 {
		t.Errorf("MinViableEntries = %d, want 100 per PIVOT_PLAN Phase 1 gate", MinViableEntries)
	}
}

// TestResolveMinViable_EnvOverride verifies that a valid positive integer
// string overrides the default.
func TestResolveMinViable_EnvOverride(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"50", 50},
		{"1", 1},
		{"200", 200},
		{"100", 100},
	}
	for _, tc := range cases {
		got := ResolveMinViable(tc.input)
		if got != tc.want {
			t.Errorf("ResolveMinViable(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

// TestResolveMinViable_InvalidEnvFallsBackToDefault verifies that zero,
// negative numbers, and non-numeric strings all fall back to MinViableEntries.
func TestResolveMinViable_InvalidEnvFallsBackToDefault(t *testing.T) {
	invalidCases := []string{
		"0",                      // zero is invalid (threshold must be positive)
		"-1",                     // negative
		"-100",                   // negative large
		"abc",                    // non-numeric
		"3.14",                   // float (not a whole int)
		"",                       // empty (covered by default test but also valid here)
		" ",                      // whitespace only
		"9999999999999999999999", // overflow
	}
	for _, input := range invalidCases {
		got := ResolveMinViable(input)
		if got != MinViableEntries {
			t.Errorf("ResolveMinViable(%q) = %d, want default %d for invalid input", input, got, MinViableEntries)
		}
	}
}

// TestMinViableEntries_IsPositive is a compile-time-reachable sanity guard:
// the constant must be strictly positive.
func TestMinViableEntries_IsPositive(t *testing.T) {
	if MinViableEntries <= 0 {
		t.Errorf("MinViableEntries = %d; must be > 0", MinViableEntries)
	}
}
