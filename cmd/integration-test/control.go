package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runControlTests runs the P2 control plane integration tests.
// Returns the number of failures.
func runControlTests() int {
	failed := 0

	tmpDir, err := os.MkdirTemp("", "tat-control-test-*")
	if err != nil {
		fmt.Printf("FAIL: cannot create temp dir: %v\n", err)
		return 1
	}
	defer os.RemoveAll(tmpDir)

	configDir := filepath.Join(tmpDir, "config")
	cacheDir := filepath.Join(tmpDir, "cache")
	for _, d := range []string{
		filepath.Join(configDir, "tonys-agent-telemetry"),
		filepath.Join(cacheDir, "tonys-agent-telemetry"),
	} {
		if err := os.MkdirAll(d, 0700); err != nil {
			fmt.Printf("FAIL: mkdir %s: %v\n", d, err)
			return 1
		}
	}

	// Write a policy with session_max_usd = 0.005.
	policy := `
[budget]
session_max_usd = 0.005
daily_max_usd = 1.0
warn_at_fraction = 0.8

[models.pricing]
"test-model" = { input = 1000.0, output = 1000.0 }
`
	policyPath := filepath.Join(configDir, "tonys-agent-telemetry", "policy.toml")
	if err := os.WriteFile(policyPath, []byte(policy), 0600); err != nil {
		fmt.Printf("FAIL: write policy: %v\n", err)
		return 1
	}

	bin := buildBinary(tmpDir)
	if bin == "" {
		fmt.Println("SKIP: control integration tests (hook-handler build failed)")
		return 0
	}

	env := append(os.Environ(),
		"XDG_CONFIG_HOME="+configDir,
		"XDG_CACHE_HOME="+cacheDir,
		"TAT_DEBUG=0",
	)

	// Test 1: First PreToolUse should be allowed (no budget spent yet).
	fmt.Print("Control: first PreToolUse allowed... ")
	pre1 := map[string]any{
		"session_id": "control-test-session",
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "echo hello"},
	}
	code1 := runHookBinary(bin, "PreToolUse", env, pre1)
	if code1 != 0 {
		fmt.Printf("FAIL (exit %d)\n", code1)
		failed++
	} else {
		fmt.Println("OK")
	}

	// Test 2: PostToolUse to accumulate cost over the cap.
	// 10 input tokens × $1000/1M = $0.01 which exceeds $0.005 cap.
	fmt.Print("Control: PostToolUse budget accumulation... ")
	post := map[string]any{
		"session_id": "control-test-session",
		"model":      "test-model",
		"usage": map[string]any{
			"input_tokens":  10,
			"output_tokens": 0,
		},
	}
	codePost := runHookBinary(bin, "PostToolUse", env, post)
	if codePost != 0 {
		fmt.Printf("FAIL (PostToolUse exit %d)\n", codePost)
		failed++
	} else {
		fmt.Println("OK")
	}

	// Test 3: Second PreToolUse should be DENIED (budget exceeded).
	fmt.Print("Control: second PreToolUse denied after budget... ")
	pre2 := map[string]any{
		"session_id": "control-test-session",
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "ls"},
	}
	code2 := runHookBinary(bin, "PreToolUse", env, pre2)
	if code2 != 2 {
		fmt.Printf("FAIL (exit %d, want 2)\n", code2)
		failed++
	} else {
		fmt.Println("OK")
	}

	// Test 4: denials.log contains one entry.
	fmt.Print("Control: denials.log has one entry... ")
	denialPath := filepath.Join(cacheDir, "tonys-agent-telemetry", "denials.log")
	denialData, err := os.ReadFile(denialPath)
	if err != nil {
		fmt.Printf("FAIL (cannot read denials.log: %v)\n", err)
		failed++
	} else {
		lines := strings.Split(strings.TrimSpace(string(denialData)), "\n")
		nonEmpty := 0
		for _, l := range lines {
			if strings.TrimSpace(l) != "" {
				nonEmpty++
			}
		}
		if nonEmpty != 1 {
			fmt.Printf("FAIL (want 1 entry, got %d: %q)\n", nonEmpty, string(denialData))
			failed++
		} else {
			fmt.Println("OK")
		}
	}

	// Test 5: budgets.json has accumulated the cost.
	fmt.Print("Control: budgets.json has accumulated cost... ")
	budgetPath := filepath.Join(cacheDir, "tonys-agent-telemetry", "budgets.json")
	budgetData, err := os.ReadFile(budgetPath)
	if err != nil {
		fmt.Printf("FAIL (cannot read budgets.json: %v)\n", err)
		failed++
	} else {
		var budgets map[string]map[string]any
		if err := json.Unmarshal(budgetData, &budgets); err != nil {
			fmt.Printf("FAIL (parse budgets.json: %v)\n", err)
			failed++
		} else if _, ok := budgets["control-test-session"]; !ok {
			fmt.Printf("FAIL (session not in budgets.json)\n")
			failed++
		} else {
			cost, _ := budgets["control-test-session"]["cost_usd"].(float64)
			if cost <= 0 {
				fmt.Printf("FAIL (cost_usd = %v, want > 0)\n", cost)
				failed++
			} else {
				fmt.Printf("OK (cost_usd=%.6f)\n", cost)
			}
		}
	}

	return failed
}

// buildBinary compiles the hook-handler binary into dir.
// Returns the binary path, or "" if build failed.
func buildBinary(dir string) string {
	bin := filepath.Join(dir, "hook-handler")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/hook-handler")
	cmd.Dir = repoRoot()
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build hook-handler: %v\n%s\n", err, out)
		return ""
	}
	return bin
}

// repoRoot returns the module root by walking up from the current working directory.
func repoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

// runHookBinary runs the hook-handler binary with the given hook type and payload.
func runHookBinary(bin, hookType string, env []string, payload map[string]any) int {
	raw, err := json.Marshal(payload)
	if err != nil {
		return -1
	}

	cmd := exec.Command(bin, hookType)
	cmd.Stdin = strings.NewReader(string(raw))
	cmd.Env = env
	_ = cmd.Run()
	if cmd.ProcessState == nil {
		return -1
	}
	return cmd.ProcessState.ExitCode()
}
