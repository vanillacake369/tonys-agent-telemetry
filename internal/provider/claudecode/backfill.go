package claudecode

import (
	"bufio"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// backfill walks ~/.claude/projects/**/*.jsonl and emits one Span per
// recognisable message. Errors on individual files are skipped (best-effort);
// context cancellation aborts mid-walk and returns ctx.Err().
func (i *Ingestor) backfill(ctx context.Context, out chan<- telemetry.Span) error {
	projectsDir := filepath.Join(claudeDir(), "projects")
	if projectsDir == filepath.Join("", "projects") {
		return nil // home dir unresolvable; nothing to back-fill
	}

	walkErr := filepath.WalkDir(projectsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip entries we can't read
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		return i.scanFile(ctx, path, out)
	})
	if walkErr != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

// scanFile reads one JSONL file line-by-line and emits Spans for parseable
// records. Returns ctx.Err() if cancelled, nil otherwise (best-effort).
func (i *Ingestor) scanFile(ctx context.Context, path string, out chan<- telemetry.Span) error {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		span, err := ConvertHookPayload("", line)
		if err != nil {
			continue // malformed line, skip
		}
		if span.TraceID == "" || span.SpanID == "" {
			continue // not a recognizable message record
		}
		select {
		case out <- span:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
