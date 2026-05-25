package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

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

	// SIGTERM/SIGHUP: gracefully ask Bubbletea to quit so the terminal is
	// restored cleanly (alt-screen exit + raw mode reset). Without this, a
	// kill or SSH disconnect leaves the terminal in a corrupted state.
	// SIGINT is handled internally by Bubbletea.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sigCh
		p.Quit()
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`tonys-agent-telemetry - TUI dashboard for AI coding agents

Supported providers: Claude, Codex (OpenAI), Gemini (Google)

Usage:
  tonys-agent-telemetry              Launch TUI (default: sessions tab)
  tonys-agent-telemetry --version    Print version
  tonys-agent-telemetry --help       Print this help

Tabs (switch with 1/2/3):
  Sessions    Browse & resume sessions across all providers (C/X/G badge)
  Skills      Search skill marketplace
  Cost        Aggregated cost/usage dashboard by provider, model, project
`)
}
