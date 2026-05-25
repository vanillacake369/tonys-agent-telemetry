package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/event"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/provider/claudecode"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
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

	// Bubbletea requires a tty; piped/redirected stdout causes ioctl errors.
	// Detect early so the user gets a clear message instead of a stack trace.
	if !term.IsTerminal(os.Stdout.Fd()) {
		fmt.Fprintln(os.Stderr, "tonys-agent-telemetry: stdout is not a terminal (TUI requires a tty)")
		os.Exit(1)
	}

	// Create the FIFO for real-time hook events.
	// Ignore errors — TUI works fine without real-time updates.
	if err := event.CreateFIFO(); err == nil {
		defer event.RemoveFIFO()
	}

	// Start the telemetry registry. Currently registers the Claude Code
	// adapter only; vLLM / Ollama / OTLP receivers will join later. The
	// span channel is drained for now (no UI consumer yet); the DAG tab
	// will take over when implemented.
	telCtx, telCancel := context.WithCancel(context.Background())
	defer telCancel()
	reg := telemetry.NewRegistry()
	reg.Register(claudecode.New())
	spans := make(chan telemetry.Span, 256)
	go reg.StartAll(telCtx, spans)
	go func() {
		for range spans {
			// discard until a consumer takes over
		}
	}()

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
