package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/event"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/provider/claudecode"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/provider/ollama"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/provider/otlp"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/provider/vllm"
	sinkotlp "github.com/vanillacake369/tonys-agent-telemetry/internal/sink/otlp"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/snapshot"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/tui"
)

var version = "dev"

func main() {
	// Phase 4 CLI flags. Defined via flag package; parse only when no
	// version/help short-circuit applies, so legacy callers keep working.
	exportURL := flag.String("otlp-export", os.Getenv("TAT_OTLP_EXPORT"),
		"Forward collected spans to a remote OTLP/JSON URL (e.g. http://tempo:4318/v1/traces).")
	snapshotFile := flag.String("snapshot-record", os.Getenv("TAT_SNAPSHOT_RECORD"),
		"Append every collected span to this JSONL file for later replay/debug.")
	replayFile := flag.String("replay", "",
		"Read spans from this JSONL file instead of starting live providers.")

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
	flag.Parse()

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

	telCtx, telCancel := context.WithCancel(context.Background())
	defer telCancel()
	spans := make(chan telemetry.Span, 256)

	if *replayFile != "" {
		// Replay mode: bypass live providers, stream from snapshot file.
		go func() {
			defer close(spans)
			_ = snapshot.NewPlayer(*replayFile).Run(telCtx, spans)
		}()
	} else {
		// Live mode: start every detected provider.
		reg := telemetry.NewRegistry()
		reg.Register(claudecode.New()) // ~/.claude — no port collision
		reg.Register(otlp.New())       // listens on :4318 if free
		reg.Register(vllm.New())       // probes :8000 /metrics with vllm: prefix
		reg.Register(ollama.New())     // probes :11434 /api/tags
		go reg.StartAll(telCtx, spans)
	}

	p := tea.NewProgram(tui.NewApp(), tea.WithAltScreen())

	// Fan-out: every collected span goes to (1) the TUI as a Bubbletea
	// batch message, (2) an optional remote OTLP forwarder, (3) an optional
	// snapshot recorder. Branches use blocking sends with large buffers so
	// no span is silently dropped — backfills of tens of thousands of JSONL
	// records are common.
	tuiCh, exportCh, recordCh := splitChannel(telCtx, spans)
	go runTUIBatcher(telCtx, tuiCh, p.Send)
	if *exportURL != "" {
		exp := sinkotlp.New(*exportURL)
		go func() { _ = exp.Run(telCtx, exportCh) }()
	} else {
		go func() { // drain to keep splitChannel happy
			for range exportCh {
			}
		}()
	}
	if *snapshotFile != "" {
		if rec, err := snapshot.NewRecorder(*snapshotFile); err == nil {
			go func() { _ = rec.Run(telCtx, recordCh) }()
		} else {
			go func() {
				for range recordCh {
				}
			}()
		}
	} else {
		go func() {
			for range recordCh {
			}
		}()
	}

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
	fmt.Print(`tonys-agent-telemetry - local-first control plane for LLM agents

Usage:
  tonys-agent-telemetry              Launch TUI with auto-detected providers
  tonys-agent-telemetry --version    Print version
  tonys-agent-telemetry --help       Print this help

Flags:
  --otlp-export URL        Forward spans to a remote OTLP/JSON endpoint
                           (also via TAT_OTLP_EXPORT env)
  --snapshot-record FILE   Append every span to FILE for later replay
                           (also via TAT_SNAPSHOT_RECORD env)
  --replay FILE            Read spans from FILE instead of live providers

Tabs (1-5 + Ctrl+G):
  Sessions  Browse & resume Claude Code sessions
  Skills    Search the skill marketplace
  Cost      Cost/usage by provider, model, project
  Hooks     Visualize ~/.claude/settings.json hook config
  DAG       Live agent orchestration graph (all providers)
  Control   Runtime governance (Ctrl+G): budget caps + tool deny/allowlist
`)
}

// splitChannel fans out a single Span source into three downstream channels
// with generous buffers. All sends are BLOCKING — data integrity matters
// more than liveness here, because backfills can produce tens of thousands
// of spans in a burst and silent drops break the DAG tab.
//
// Trade-off: a genuinely stuck downstream (e.g. unreachable remote OTLP
// endpoint that is somehow not honoring its HTTP timeout) will stall the
// pipeline. The sink/otlp Exporter caps its HTTP calls at 5s so this is
// bounded in practice. Local consumers (TUI batcher, snapshot recorder)
// drain at memory speed.
func splitChannel(ctx context.Context, in <-chan telemetry.Span) (tuiCh, exportCh, recordCh chan telemetry.Span) {
	const bufSize = 4096
	tuiCh = make(chan telemetry.Span, bufSize)
	exportCh = make(chan telemetry.Span, bufSize)
	recordCh = make(chan telemetry.Span, bufSize)
	go func() {
		defer close(tuiCh)
		defer close(exportCh)
		defer close(recordCh)
		for {
			select {
			case <-ctx.Done():
				return
			case sp, ok := <-in:
				if !ok {
					return
				}
				// Blocking sends: pipeline waits if a branch is full.
				// Each select includes ctx.Done() to exit on shutdown.
				select {
				case tuiCh <- sp:
				case <-ctx.Done():
					return
				}
				select {
				case exportCh <- sp:
				case <-ctx.Done():
					return
				}
				select {
				case recordCh <- sp:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return
}

// runTUIBatcher accumulates spans from tuiCh and dispatches them to the
// Bubbletea program in batches. A single SpanBatchMsg per ~100ms keeps the
// runtime's message queue happy under heavy backfill, while still feeling
// live to a user watching new spans arrive.
//
// The send argument is a function (typically p.Send) so the batcher can be
// tested without spinning up a full tea.Program.
func runTUIBatcher(ctx context.Context, tuiCh <-chan telemetry.Span, send func(tea.Msg)) {
	const (
		flushInterval = 100 * time.Millisecond
		maxBatch      = 512
	)
	var batch []telemetry.Span
	flush := func() {
		if len(batch) == 0 {
			return
		}
		snapshot := make([]telemetry.Span, len(batch))
		copy(snapshot, batch)
		batch = batch[:0]
		send(tui.SpanBatchMsg{Spans: snapshot})
	}
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case sp, ok := <-tuiCh:
			if !ok {
				flush()
				return
			}
			batch = append(batch, sp)
			if len(batch) >= maxBatch {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
