// Package claudecode implements the first telemetry.ProviderIngestor: a
// Claude Code adapter that translates the existing JSONL session files and
// hook payloads into provider-agnostic telemetry.Span records.
//
// This package is purely additive. It does not move or modify anything in
// internal/data/ — that package continues to own the Claude-specific
// DetailTurn, FileChange, ParseFullConversation, ParseFileChanges, etc.
// used by the current TUI tabs. As the telemetry abstraction matures,
// consumers can choose either API depending on whether they want raw
// Claude data or the normalized Span shape.
package claudecode

import (
	"context"
	"os"
	"path/filepath"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// Ingestor implements telemetry.ProviderIngestor for Claude Code.
type Ingestor struct{}

// New returns a default Ingestor. The Claude data directory is auto-detected
// from $HOME/.claude on Detect.
func New() *Ingestor { return &Ingestor{} }

// ProviderID returns the stable identifier "claudecode".
func (i *Ingestor) ProviderID() string { return "claudecode" }

// claudeDir returns the path to the user's Claude Code data directory.
// Exposed as a var to keep tests independent of $HOME.
var claudeDir = func() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude")
}

// Detect reports whether ~/.claude exists — the cheapest reliable signal
// that Claude Code has ever run on this machine.
func (i *Ingestor) Detect(ctx context.Context) bool {
	dir := claudeDir()
	if dir == "" {
		return false
	}
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

// Ingest runs the backfill pass (historical Spans from ~/.claude/projects)
// and then blocks until ctx is cancelled. Live-tailing of the hook FIFO is
// not yet wired here; the existing TUI continues to consume FIFO events
// directly via internal/event during the transition.
func (i *Ingestor) Ingest(ctx context.Context, out chan<- telemetry.Span) error {
	_ = i.backfill(ctx, out)
	<-ctx.Done()
	return ctx.Err()
}
