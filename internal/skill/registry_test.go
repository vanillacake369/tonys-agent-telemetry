package skill

import (
	"context"
	"os"
	"os/exec"
	"testing"
)

// skipUnlessLive skips the registry tests when running on CI or without
// TONYS_RUN_LIVE=1 — they call the real GitHub API via `gh`, which is
// rate-limited / unauthenticated under the default GH Actions token in
// ways that make these tests flaky as a gate. Run locally with
// TONYS_RUN_LIVE=1 to exercise.
func skipUnlessLive(t *testing.T) {
	t.Helper()
	if os.Getenv("TONYS_RUN_LIVE") != "1" || os.Getenv("CI") == "true" {
		t.Skip("live registry test — set TONYS_RUN_LIVE=1 and unset CI to run")
	}
}

func TestSearchRegistries_ReturnsSkills(t *testing.T) {
	skipUnlessLive(t)
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh not available")
	}

	ctx := context.Background()
	skills, err := SearchRegistries(ctx, "")
	if err != nil {
		t.Fatalf("SearchRegistries error: %v", err)
	}
	// mattpocock/skills has ~24 non-deprecated skills.
	if len(skills) < 10 {
		t.Errorf("expected at least 10 skills from registries, got %d", len(skills))
	}
	t.Logf("Found %d skills from registries", len(skills))
	for i, s := range skills {
		if i >= 5 {
			break
		}
		t.Logf("  %s: %s", s.Name, s.URL)
	}
}

func TestSearchRegistries_FiltersByQuery(t *testing.T) {
	skipUnlessLive(t)
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh not available")
	}

	ctx := context.Background()
	skills, err := SearchRegistries(ctx, "grill")
	if err != nil {
		t.Fatalf("SearchRegistries error: %v", err)
	}
	if len(skills) == 0 {
		t.Error("expected at least 1 skill matching 'grill'")
	}
	for _, s := range skills {
		t.Logf("  matched: %s", s.Name)
	}
}

func TestSearchRegistries_SkipsDeprecated(t *testing.T) {
	skipUnlessLive(t)
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh not available")
	}

	ctx := context.Background()
	skills, err := SearchRegistries(ctx, "")
	if err != nil {
		t.Fatalf("SearchRegistries error: %v", err)
	}
	for _, s := range skills {
		if s.Name == "qa" || s.Name == "design-an-interface" {
			t.Errorf("deprecated skill %q should not be included", s.Name)
		}
	}
}
