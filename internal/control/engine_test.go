package control

import (
	"testing"
)

// makeEngine creates an Engine with a fresh BudgetStore and DenialLog in a temp dir.
func makeEngine(t *testing.T, pol Policy) *Engine {
	t.Helper()
	dir := t.TempDir()
	budgets := NewBudgetStore(dir)
	denials := NewDenialLog(dir)
	return NewEngine(pol, budgets, denials)
}

func TestPreToolUse_AllowByDefault(t *testing.T) {
	e := makeEngine(t, DefaultPolicy())
	d := e.PreToolUse("sess-1", "Bash", "echo hello")
	if d.Action != "allow" {
		t.Errorf("Action = %q, want %q", d.Action, "allow")
	}
}

func TestPreToolUse_DenylistBlocks(t *testing.T) {
	pol := DefaultPolicy()
	pol.Tools.Denylist = []string{"Bash:rm -rf*"}

	dir := t.TempDir()
	budgets := NewBudgetStore(dir)
	denials := NewDenialLog(dir)
	e := NewEngine(pol, budgets, denials)

	d := e.PreToolUse("sess-1", "Bash", "rm -rf /tmp/data")
	if d.Action != "deny" {
		t.Fatalf("Action = %q, want deny", d.Action)
	}
	if d.Reason != "tool_denylisted" {
		t.Errorf("Reason = %q, want tool_denylisted", d.Reason)
	}

	// Verify denial was logged.
	recent, err := denials.Recent(1)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("expected 1 denial logged, got %d", len(recent))
	}
	if recent[0].Reason != "tool_denylisted" {
		t.Errorf("logged reason = %q, want tool_denylisted", recent[0].Reason)
	}
}

func TestPreToolUse_AllowlistBlocksUnmatched(t *testing.T) {
	pol := DefaultPolicy()
	pol.Tools.Allowlist = []string{"Read:*"}
	e := makeEngine(t, pol)

	d := e.PreToolUse("sess-1", "Write", "/etc/passwd")
	if d.Action != "deny" {
		t.Fatalf("Action = %q, want deny", d.Action)
	}
	if d.Reason != "tool_not_allowlisted" {
		t.Errorf("Reason = %q, want tool_not_allowlisted", d.Reason)
	}

	// Verify an allowlisted tool is still allowed.
	d2 := e.PreToolUse("sess-1", "Read", "/etc/passwd")
	if d2.Action != "allow" {
		t.Errorf("Read:* should be allowed, got %q", d2.Action)
	}
}

func TestPreToolUse_BudgetCapDenies(t *testing.T) {
	pol := DefaultPolicy()
	pol.Budget.SessionMaxUSD = 5.0
	pricing := map[string]ModelPrice{"m": {Input: 1000.0, Output: 1000.0}}
	pol.Models.Pricing = pricing

	dir := t.TempDir()
	budgets := NewBudgetStore(dir)
	denials := NewDenialLog(dir)
	e := NewEngine(pol, budgets, denials)

	// Directly inject a cost that exceeds the cap.
	// 1M input tokens at $1000/1M = $1000 per call; use 1 input token of a
	// fictional model that costs $5.01 to ensure cap is hit.
	// Simpler: add enough tokens to exceed $5.00 with our pricing.
	// 5010 input tokens × $1000/1M = $5.01
	_, err := budgets.Add("sess-1", "m", 5010, 0, pricing)
	if err != nil {
		t.Fatal(err)
	}

	d := e.PreToolUse("sess-1", "Bash", "ls")
	if d.Action != "deny" {
		t.Fatalf("Action = %q, want deny; detail: %s", d.Action, d.Detail)
	}
	if d.Reason != "budget_exceeded" {
		t.Errorf("Reason = %q, want budget_exceeded", d.Reason)
	}
}

func TestPreToolUse_BudgetWarnFraction(t *testing.T) {
	pol := DefaultPolicy()
	pol.Budget.SessionMaxUSD = 5.0
	pol.Budget.WarnAtFraction = 0.8
	pricing := map[string]ModelPrice{"m": {Input: 1000.0, Output: 1000.0}}
	pol.Models.Pricing = pricing

	dir := t.TempDir()
	budgets := NewBudgetStore(dir)
	denials := NewDenialLog(dir)
	e := NewEngine(pol, budgets, denials)

	// 4000 input tokens × $1000/1M = $4.00 (80% of $5.00 cap).
	_, err := budgets.Add("sess-1", "m", 4000, 0, pricing)
	if err != nil {
		t.Fatal(err)
	}

	d := e.PreToolUse("sess-1", "Bash", "ls")
	if d.Action != "warn" {
		t.Fatalf("Action = %q, want warn; detail: %s", d.Action, d.Detail)
	}
	if d.Reason != "approaching_budget" {
		t.Errorf("Reason = %q, want approaching_budget", d.Reason)
	}
}

func TestPreToolUse_DailyCapDenies(t *testing.T) {
	pol := DefaultPolicy()
	pol.Budget.DailyMaxUSD = 50.0
	pricing := map[string]ModelPrice{"m": {Input: 1000.0, Output: 1000.0}}
	pol.Models.Pricing = pricing

	dir := t.TempDir()
	budgets := NewBudgetStore(dir)
	denials := NewDenialLog(dir)
	e := NewEngine(pol, budgets, denials)

	// 50010 tokens × $1000/1M = $50.01.
	_, err := budgets.Add("sess-daily", "m", 50010, 0, pricing)
	if err != nil {
		t.Fatal(err)
	}

	d := e.PreToolUse("sess-daily", "Bash", "ls")
	if d.Action != "deny" {
		t.Fatalf("Action = %q, want deny; detail: %s", d.Action, d.Detail)
	}
	if d.Reason != "budget_exceeded" {
		t.Errorf("Reason = %q, want budget_exceeded", d.Reason)
	}
}

func TestPostToolUse_UpdatesBudget(t *testing.T) {
	pol := DefaultPolicy()
	pricing := map[string]ModelPrice{"claude-sonnet-4-6": {Input: 3.0, Output: 15.0}}
	pol.Models.Pricing = pricing

	dir := t.TempDir()
	budgets := NewBudgetStore(dir)
	denials := NewDenialLog(dir)
	e := NewEngine(pol, budgets, denials)

	if err := e.PostToolUse("sess-post", "claude-sonnet-4-6", 1000, 500); err != nil {
		t.Fatalf("PostToolUse: %v", err)
	}

	b, err := budgets.Get("sess-post")
	if err != nil {
		t.Fatal(err)
	}
	if b.InputTokens != 1000 || b.OutputTokens != 500 {
		t.Errorf("tokens: %d/%d, want 1000/500", b.InputTokens, b.OutputTokens)
	}
	want := (1000*3.0 + 500*15.0) / 1_000_000
	if diff := b.CostUSD - want; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("CostUSD = %v, want %v", b.CostUSD, want)
	}
}
