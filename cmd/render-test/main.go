package main

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/data"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/tui"
)

func main() {
	app := tui.NewApp()

	// Simulate window size
	m, _ := app.Update(tea.WindowSizeMsg{Width: 110, Height: 30})

	// Simulate sessions loaded by sending the message through Update
	m, _ = m.Update(tui.SessionsLoadedMsg{
		Sessions: []data.Session{
			{ID: "184c340a-0ba4-4824", CWD: "/Users/test/tonys-nix", GitBranch: "main", FirstPrompt: "claude status line 에 대해 구현 전략을 모색하고 트레이드오프를 제공해봐", Timestamp: time.Now(), Model: "claude-opus-4-6", FilePath: "/tmp/test.jsonl"},
			{ID: "7297253b-e807-4e88", CWD: "/Users/test/tonys-nix", GitBranch: "main", FirstPrompt: "proxy.nix의 보안을 검토해줘", Timestamp: time.Now().Add(-1 * time.Hour), Model: "claude-opus-4-6", FilePath: "/tmp/test2.jsonl"},
			{ID: "e7403a69-3e87-46c8", CWD: "/Users/test/tonys-homelab", GitBranch: "feature", FirstPrompt: "agent 완료되면 notification 을 받고 싶어", Timestamp: time.Now().Add(-2 * time.Hour), Model: "claude-sonnet-4-6", FilePath: "/tmp/test3.jsonl"},
			{ID: "aa6182c0-d59a-49e7", CWD: "/Users/test/tonys-nix", GitBranch: "main", FirstPrompt: "zellij config 설정", Timestamp: time.Now().Add(-3 * time.Hour), Model: "claude-opus-4-6", FilePath: "/tmp/test4.jsonl"},
		},
	})

	fmt.Println("=== SESSIONS TAB (with data) ===")
	fmt.Println(m.(tui.App).View())
}
