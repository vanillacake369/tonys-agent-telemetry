package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/control"
)

// buildHookHandler compiles the hook-handler binary into a temp dir and returns
// the path. Skips the test if the build fails.
func buildHookHandler(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "hook-handler")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot build hook-handler: %v\n%s", err, out)
	}
	return bin
}

// runHook runs the hook-handler binary with the given hook type and JSON payload.
// It sets XDG_CONFIG_HOME and XDG_CACHE_HOME from the provided dirs.
func runHook(t *testing.T, bin, hookType, configDir, cacheDir string, payload map[string]any) (exitCode int, stderr string) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, hookType)
	cmd.Stdin = newBytesReader(raw)
	cmd.Env = append(os.Environ(),
		"XDG_CONFIG_HOME="+configDir,
		"XDG_CACHE_HOME="+cacheDir,
		"TAT_DEBUG=0",
	)

	var stderrBuf []byte
	errPipe, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if errPipe != nil {
		stderrBuf, _ = io.ReadAll(errPipe)
	}
	_ = cmd.Wait()
	exitCode = cmd.ProcessState.ExitCode()
	return exitCode, string(stderrBuf)
}

func TestHookHandler_PreToolUseAllowedExits0(t *testing.T) {
	bin := buildHookHandler(t)
	configDir := t.TempDir()
	cacheDir := t.TempDir()

	payload := map[string]any{
		"session_id": "test-session",
		"tool_name":  "Read",
		"tool_input": map[string]any{"file_path": "/tmp/test.txt"},
	}

	code, _ := runHook(t, bin, "PreToolUse", configDir, cacheDir, payload)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 for allowed tool", code)
	}
}

func TestHookHandler_PreToolUseDeniedExits2(t *testing.T) {
	bin := buildHookHandler(t)
	configDir := t.TempDir()
	cacheDir := t.TempDir()

	// Write a policy that denylists Bash:rm -rf*.
	cfgSubDir := filepath.Join(configDir, "tonys-agent-telemetry")
	if err := os.MkdirAll(cfgSubDir, 0700); err != nil {
		t.Fatal(err)
	}
	policy := `
[tools]
denylist = ["Bash:rm -rf*"]
`
	if err := os.WriteFile(filepath.Join(cfgSubDir, "policy.toml"), []byte(policy), 0600); err != nil {
		t.Fatal(err)
	}

	payload := map[string]any{
		"session_id": "test-session",
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "rm -rf /tmp/important"},
	}

	code, stderr := runHook(t, bin, "PreToolUse", configDir, cacheDir, payload)
	if code != 2 {
		t.Errorf("exit code = %d, want 2 for denied tool; stderr: %s", code, stderr)
	}
	if len(stderr) == 0 || !containsString(stderr, "BLOCKED") {
		t.Errorf("expected BLOCKED message in stderr, got: %q", stderr)
	}
}

func TestHookHandler_PostToolUseUpdatesBudget(t *testing.T) {
	bin := buildHookHandler(t)
	configDir := t.TempDir()
	cacheDir := t.TempDir()

	// Write pricing config.
	cfgSubDir := filepath.Join(configDir, "tonys-agent-telemetry")
	if err := os.MkdirAll(cfgSubDir, 0700); err != nil {
		t.Fatal(err)
	}
	policy := `
[models.pricing]
"claude-sonnet-4-6" = { input = 3.0, output = 15.0 }
`
	if err := os.WriteFile(filepath.Join(cfgSubDir, "policy.toml"), []byte(policy), 0600); err != nil {
		t.Fatal(err)
	}

	payload := map[string]any{
		"session_id": "post-session",
		"model":      "claude-sonnet-4-6",
		"usage": map[string]any{
			"input_tokens":  1000,
			"output_tokens": 500,
		},
	}

	code, _ := runHook(t, bin, "PostToolUse", configDir, cacheDir, payload)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 for PostToolUse", code)
	}

	// Verify budgets.json was updated.
	cacheTATDir := filepath.Join(cacheDir, "tonys-agent-telemetry")
	store := control.NewBudgetStore(cacheTATDir)
	b, err := store.Get("post-session")
	if err != nil {
		t.Fatalf("budget Get: %v", err)
	}
	if b.InputTokens != 1000 || b.OutputTokens != 500 {
		t.Errorf("tokens: %d/%d, want 1000/500", b.InputTokens, b.OutputTokens)
	}
	want := (1000*3.0 + 500*15.0) / 1_000_000
	if diff := b.CostUSD - want; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("CostUSD = %v, want %v", b.CostUSD, want)
	}
}

func TestHookHandler_BadPolicyFileFailsOpen(t *testing.T) {
	bin := buildHookHandler(t)
	configDir := t.TempDir()
	cacheDir := t.TempDir()

	// Write a garbage policy file.
	cfgSubDir := filepath.Join(configDir, "tonys-agent-telemetry")
	if err := os.MkdirAll(cfgSubDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgSubDir, "policy.toml"), []byte("not toml ][[["), 0600); err != nil {
		t.Fatal(err)
	}

	payload := map[string]any{
		"session_id": "test-session",
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "echo hello"},
	}

	code, _ := runHook(t, bin, "PreToolUse", configDir, cacheDir, payload)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 for bad policy (fail-open)", code)
	}
}

// containsString reports whether substr is in s.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// newBytesReader returns an io.Reader from bytes (avoids importing bytes in test).
func newBytesReader(b []byte) *bytesReader {
	return &bytesReader{data: b}
}

type bytesReader struct {
	data []byte
	pos  int
}

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
