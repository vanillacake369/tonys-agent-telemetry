package main

import (
	"fmt"
	"io"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/event"
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

	// Suppress log output during TUI — stderr corrupts the alt screen.
	// Use --debug flag or TAT_DEBUG=1 to enable logging to a file.
	if os.Getenv("TAT_DEBUG") == "1" {
		f, err := os.OpenFile("/tmp/tat-debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			log.SetOutput(f)
			defer f.Close()
		}
	} else {
		log.SetOutput(io.Discard)
	}

	// Create the FIFO for real-time hook events.
	// Ignore errors — TUI works fine without real-time updates.
	if err := event.CreateFIFO(); err == nil {
		defer event.RemoveFIFO()
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

Tabs (switch with 1/2/3/4):
  Sessions    Fuzzy-find and resume Claude sessions
  Agents      Browse and launch agents
  DAG         Live agent orchestration graph
  Skills      Search skill marketplace
`)
}
