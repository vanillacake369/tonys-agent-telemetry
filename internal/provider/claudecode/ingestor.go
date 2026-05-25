package claudecode

import (
	"context"
	"os"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/event"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// Ingestor implements telemetry.ProviderIngestor for Claude Code.
type Ingestor struct {
	fifoPath string
}

// NewIngestor creates a ClaudeCodeIngestor that listens on fifoPath.
func NewIngestor(fifoPath string) *Ingestor {
	return &Ingestor{fifoPath: fifoPath}
}

func (i *Ingestor) ProviderID() string { return "claudecode" }

// Detect returns true when ~/.claude exists on the filesystem.
// This check is fast (<200ms) and side-effect-free.
func (i *Ingestor) Detect(ctx context.Context) bool {
	_, err := os.Stat(ClaudeDir())
	return err == nil
}

// Ingest back-fills historical sessions as Spans and then streams live FIFO events.
// It runs until ctx is cancelled.
func (i *Ingestor) Ingest(ctx context.Context, out chan<- telemetry.Span) error {
	// Back-fill: convert historical JSONL sessions to Spans.
	if sessions, err := DiscoverSessionMetas(); err == nil {
		for _, s := range sessions {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			span := sessionMetaToSpan(s)
			select {
			case out <- span:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	// Live: read FIFO events and emit Spans.
	return i.runFIFOLoop(ctx, out)
}

// runFIFOLoop reads v1/v2 events from the FIFO and emits Spans until ctx is cancelled.
func (i *Ingestor) runFIFOLoop(ctx context.Context, out chan<- telemetry.Span) error {
	ch := event.ReadFIFOFromPath(ctx, i.fifoPath)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-ch:
			if !ok {
				return nil
			}
			span := eventToSpan(ev)
			select {
			case out <- span:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}
