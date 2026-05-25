package control

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPolicy_DefaultWhenMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	pol, err := LoadPolicy()
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}

	def := DefaultPolicy()
	if pol.Budget.SessionMaxUSD != def.Budget.SessionMaxUSD {
		t.Errorf("SessionMaxUSD: got %v, want %v", pol.Budget.SessionMaxUSD, def.Budget.SessionMaxUSD)
	}
	if pol.Budget.DailyMaxUSD != def.Budget.DailyMaxUSD {
		t.Errorf("DailyMaxUSD: got %v, want %v", pol.Budget.DailyMaxUSD, def.Budget.DailyMaxUSD)
	}
	if len(pol.Tools.Denylist) != 0 {
		t.Errorf("Denylist: got %v, want empty", pol.Tools.Denylist)
	}
}

func TestLoadPolicy_MalformedReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfgDir := filepath.Join(dir, "tonys-agent-telemetry")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "policy.toml"), []byte("this is not valid toml ][[["), 0600); err != nil {
		t.Fatal(err)
	}

	pol, err := LoadPolicy()
	if err == nil {
		t.Error("expected non-nil error for malformed file")
	}

	def := DefaultPolicy()
	if pol.Budget.SessionMaxUSD != def.Budget.SessionMaxUSD {
		t.Errorf("SessionMaxUSD: got %v, want %v", pol.Budget.SessionMaxUSD, def.Budget.SessionMaxUSD)
	}
}

func TestLoadPolicy_FullFixture(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfgDir := filepath.Join(dir, "tonys-agent-telemetry")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}

	fixture := `
[budget]
session_max_usd = 5.0
daily_max_usd = 50.0
warn_at_fraction = 0.8

[tools]
denylist = ["Bash:rm -rf*", "WebFetch:*evil.com*"]
allowlist = []

[models.pricing]
"claude-sonnet-4-6" = { input = 3.0, output = 15.0 }
`
	if err := os.WriteFile(filepath.Join(cfgDir, "policy.toml"), []byte(fixture), 0600); err != nil {
		t.Fatal(err)
	}

	pol, err := LoadPolicy()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pol.Budget.SessionMaxUSD != 5.0 {
		t.Errorf("SessionMaxUSD: got %v, want 5.0", pol.Budget.SessionMaxUSD)
	}
	if pol.Budget.DailyMaxUSD != 50.0 {
		t.Errorf("DailyMaxUSD: got %v, want 50.0", pol.Budget.DailyMaxUSD)
	}
	if pol.Budget.WarnAtFraction != 0.8 {
		t.Errorf("WarnAtFraction: got %v, want 0.8", pol.Budget.WarnAtFraction)
	}
	if len(pol.Tools.Denylist) != 2 {
		t.Errorf("Denylist: got %d items, want 2", len(pol.Tools.Denylist))
	}
	if pol.Tools.Denylist[0] != "Bash:rm -rf*" {
		t.Errorf("Denylist[0]: got %q, want %q", pol.Tools.Denylist[0], "Bash:rm -rf*")
	}
	p, ok := pol.Models.Pricing["claude-sonnet-4-6"]
	if !ok {
		t.Fatal("missing pricing for claude-sonnet-4-6")
	}
	if p.Input != 3.0 || p.Output != 15.0 {
		t.Errorf("pricing: got {%v,%v}, want {3.0,15.0}", p.Input, p.Output)
	}
}

func TestMatch_GlobPatterns(t *testing.T) {
	patterns := []string{"Bash:rm -rf*", "WebFetch:*evil.com*"}

	cases := []struct {
		target string
		want   bool
	}{
		{"Bash:rm -rf /tmp/x", true},
		{"Bash:rm -rf*", true},
		{"Bash:rm /tmp/x", false},
		{"WebFetch:https://evil.com/path", true},
		{"WebFetch:https://good.com/path", false},
		{"Read:/etc/passwd", false},
	}

	for _, tc := range cases {
		got := Match(patterns, tc.target)
		if got != tc.want {
			t.Errorf("Match(%q) = %v, want %v", tc.target, got, tc.want)
		}
	}
}
