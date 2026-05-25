package signalstore_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signalstore"
)

func TestResolveStorePath_DefaultUnderUserCacheDir(t *testing.T) {
	// Clear the env override so we get the default path.
	t.Setenv("TONYS_SIGNAL_STORE", "")

	got, err := signalstore.ResolveStorePath()
	if err != nil {
		t.Fatalf("ResolveStorePath: %v", err)
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		t.Skipf("os.UserCacheDir unavailable: %v", err)
	}

	want := filepath.Join(cacheDir, "tonys-agent-telemetry", "signals")
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestResolveStorePath_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TONYS_SIGNAL_STORE", dir)

	got, err := signalstore.ResolveStorePath()
	if err != nil {
		t.Fatalf("ResolveStorePath: %v", err)
	}
	if got != dir {
		t.Errorf("got %q want %q", got, dir)
	}
}

func TestSessionFilename_SanitizesUnsafeChars(t *testing.T) {
	cases := []struct {
		name  string
		input string
		check func(string) bool
	}{
		{
			name:  "slashes removed",
			input: "some/path/session",
			check: func(got string) bool { return !strings.Contains(got, "/") },
		},
		{
			name:  "leading dot replaced",
			input: ".hidden-session",
			check: func(got string) bool { return !strings.HasPrefix(got, ".") },
		},
		{
			name:  "length capped",
			input: strings.Repeat("a", 300),
			check: func(got string) bool {
				// Strip .jsonl suffix for length check.
				base := strings.TrimSuffix(got, ".jsonl")
				return len(base) <= 200
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := signalstore.SessionFilename(tc.input)
			if err != nil {
				t.Fatalf("SessionFilename(%q): %v", tc.input, err)
			}
			if !tc.check(got) {
				t.Errorf("SessionFilename(%q) = %q failed safety check", tc.input, got)
			}
		})
	}
}

func TestSessionFilename_RejectsEmpty(t *testing.T) {
	_, err := signalstore.SessionFilename("")
	if err == nil {
		t.Fatal("expected error for empty sessionID, got nil")
	}
}
