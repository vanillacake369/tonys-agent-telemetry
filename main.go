package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/tui"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Printf("tonys-agent-telemetry %s\n", version)
			return
		case "--help", "-h":
			printUsage()
			return
		}
	}

	p := tea.NewProgram(tui.NewApp(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`tonys-agent-telemetry - TUI dashboard for Claude Code

Usage:
  tonys-agent-telemetry              Launch TUI (default: sessions tab)
  tonys-agent-telemetry --version    Print version
  tonys-agent-telemetry --help       Print this help

Tabs (switch with Ctrl+S/A/D/K):
  Sessions    Fuzzy-find and resume Claude sessions
  Agents      Browse and launch agents
  DAG         Live agent orchestration graph
  Skills      Search skill marketplace
`)
}
