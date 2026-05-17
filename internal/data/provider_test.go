package data

import (
	"testing"
	"time"
)

func TestSortSessionsByTime(t *testing.T) {
	sessions := []Session{
		{ID: "a", Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "c", Timestamp: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)},
		{ID: "b", Timestamp: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)},
	}
	sortSessionsByTime(sessions)
	if sessions[0].ID != "c" || sessions[1].ID != "b" || sessions[2].ID != "a" {
		t.Errorf("expected c,b,a got %s,%s,%s", sessions[0].ID, sessions[1].ID, sessions[2].ID)
	}
}

func TestSortCostsByTime(t *testing.T) {
	costs := []SessionCost{
		{SessionID: "a", Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{SessionID: "c", Timestamp: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)},
		{SessionID: "b", Timestamp: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)},
	}
	sortCostsByTime(costs)
	if costs[0].SessionID != "c" || costs[1].SessionID != "b" || costs[2].SessionID != "a" {
		t.Errorf("expected c,b,a got %s,%s,%s", costs[0].SessionID, costs[1].SessionID, costs[2].SessionID)
	}
}

func TestRegisteredProviders(t *testing.T) {
	providers := Providers()
	if len(providers) < 3 {
		t.Fatalf("expected at least 3 providers, got %d", len(providers))
	}

	names := map[ProviderName]bool{}
	for _, p := range providers {
		names[p.Name()] = true
	}
	for _, expected := range []ProviderName{ProviderClaude, ProviderCodex, ProviderGemini} {
		if !names[expected] {
			t.Errorf("expected provider %s to be registered", expected)
		}
	}
}

func TestGetProvider(t *testing.T) {
	p := GetProvider(ProviderClaude)
	if p == nil {
		t.Fatal("expected ClaudeProvider, got nil")
	}
	if p.Name() != ProviderClaude {
		t.Errorf("expected claude, got %s", p.Name())
	}

	p = GetProvider("nonexistent")
	if p != nil {
		t.Errorf("expected nil for nonexistent provider, got %v", p)
	}
}

func TestSummarizeByProvider(t *testing.T) {
	costs := []SessionCost{
		{Provider: ProviderClaude, EstCostUSD: 10.0, TotalTokens: 1000, TurnCount: 5,
			Timestamp: time.Now(), ToolCalls: map[string]int{}},
		{Provider: ProviderCodex, EstCostUSD: 5.0, TotalTokens: 500, TurnCount: 3,
			Timestamp: time.Now(), ToolCalls: map[string]int{}},
		{Provider: ProviderClaude, EstCostUSD: 2.0, TotalTokens: 200, TurnCount: 2,
			Timestamp: time.Now(), ToolCalls: map[string]int{}},
	}
	summary := Summarize(costs)
	if len(summary.ByProvider) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(summary.ByProvider))
	}
	claude := summary.ByProvider[ProviderClaude]
	if claude.Cost != 12.0 {
		t.Errorf("expected claude cost 12.0, got %.2f", claude.Cost)
	}
	if claude.Turns != 7 {
		t.Errorf("expected claude turns 7, got %d", claude.Turns)
	}
	codex := summary.ByProvider[ProviderCodex]
	if codex.Cost != 5.0 {
		t.Errorf("expected codex cost 5.0, got %.2f", codex.Cost)
	}
}
