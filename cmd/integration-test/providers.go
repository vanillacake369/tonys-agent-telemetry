package main

import (
	"fmt"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/data"
)

func testProviders() {
	// Verify resume commands format.
	fmt.Println("\n--- Resume command verification ---")
	testSessions := []data.Session{
		{ID: "abc-123", Provider: data.ProviderClaude, CWD: "/tmp/test"},
		{ID: "def-456", Provider: data.ProviderCodex, CWD: "/tmp/test"},
		{ID: "ghi-789", Provider: data.ProviderGemini, CWD: "/tmp/test"},
	}
	for _, s := range testSessions {
		p := data.GetProvider(s.Provider)
		if p != nil {
			fmt.Printf("  %s: %s\n", s.Provider, p.ResumeCommand(s))
		}
	}

	for _, p := range data.Providers() {
		fmt.Printf("Provider: %s, Available: %v, DataDir: %s\n", p.Name(), p.Available(), p.DataDir())
	}
	fmt.Println("\n--- Sessions from all providers ---")
	sessions, err := data.DiscoverAllSessions()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Total sessions: %d\n", len(sessions))
	counts := map[data.ProviderName]int{}
	for _, s := range sessions {
		counts[s.Provider]++
	}
	for p, c := range counts {
		fmt.Printf("  %s: %d sessions\n", p, c)
	}
	for i, s := range sessions {
		if i >= 5 {
			break
		}
		prompt := s.FirstPrompt
		if len(prompt) > 50 {
			prompt = prompt[:50]
		}
		fmt.Printf("  [%s] %s - %s\n", s.Provider, s.Timestamp.Format("01-02 15:04"), prompt)
	}

	fmt.Println("\n--- Costs from all providers ---")
	costs, err := data.DiscoverAllCostsMulti()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	summary := data.Summarize(costs)
	fmt.Printf("Total cost: $%.2f, Sessions: %d, Tokens: %d\n",
		summary.TotalCostUSD, summary.TotalSessions, summary.TotalTokens)
	for p, ms := range summary.ByProvider {
		fmt.Printf("  %s: $%.2f, %d tok, %d turns\n", p, ms.Cost, ms.Tokens, ms.Turns)
	}
}
