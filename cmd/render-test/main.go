package main

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/data"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/tui"
)

func main() {
	sessions := []data.Session{
		{ID: "184c340a-0ba4-4824", CWD: "/Users/test/tonys-nix", GitBranch: "main", FirstPrompt: "claude status line 에 대해 구현 전략을 모색하고 트레이드오프를 제공해봐", Timestamp: time.Now(), Model: "claude-opus-4-6", FilePath: "/tmp/test.jsonl"},
		{ID: "7297253b-e807-4e88", CWD: "/Users/test/tonys-nix", GitBranch: "main", FirstPrompt: "proxy.nix의 보안을 검토해줘", Timestamp: time.Now().Add(-1 * time.Hour), Model: "claude-opus-4-6", FilePath: "/tmp/test2.jsonl"},
		{ID: "e7403a69-3e87-46c8", CWD: "/Users/test/tonys-homelab", GitBranch: "feature", FirstPrompt: "agent 완료되면 notification 을 받고 싶어", Timestamp: time.Now().Add(-2 * time.Hour), Model: "claude-sonnet-4-6", FilePath: "/tmp/test3.jsonl"},
	}

	for _, width := range []int{80, 120, 160} {
		app := tui.NewApp()
		m, _ := app.Update(tea.WindowSizeMsg{Width: width, Height: 24})
		m, _ = m.Update(tui.SessionsLoadedMsg{Sessions: sessions})

		fmt.Printf("\n=== WIDTH %d ===\n", width)
		fmt.Println(m.(tui.App).View())
	}
}
