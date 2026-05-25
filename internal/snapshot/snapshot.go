// Package snapshot persists telemetry.Span streams to a JSONL file and
// replays them on demand. Useful for:
//   - debugging an agent run after the fact
//   - building reproducible TUI screenshots
//   - sharing traces with collaborators without exposing live data
//
// File format: one JSON-encoded telemetry.Span per line. Append-only on
// record; read-once and emit-all on replay.
package snapshot

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// Recorder appends Spans to a JSONL file as they flow through a channel.
type Recorder struct {
	path string

	mu sync.Mutex
	f  *os.File
}

// NewRecorder opens (or creates) the file at path for append.
func NewRecorder(path string) (*Recorder, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open snapshot file: %w", err)
	}
	return &Recorder{path: path, f: f}, nil
}

// Path returns the underlying file path.
func (r *Recorder) Path() string { return r.path }

// Run consumes spans from in and writes each as a JSON line until ctx is
// cancelled or the channel closes. Write errors are silent (best-effort).
func (r *Recorder) Run(ctx context.Context, in <-chan telemetry.Span) error {
	defer r.Close()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sp, ok := <-in:
			if !ok {
				return nil
			}
			r.write(sp)
		}
	}
}

func (r *Recorder) write(sp telemetry.Span) {
	b, err := json.Marshal(sp)
	if err != nil {
		return
	}
	b = append(b, '\n')
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = r.f.Write(b)
}

// Close flushes and closes the underlying file.
func (r *Recorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.f == nil {
		return nil
	}
	err := r.f.Close()
	r.f = nil
	return err
}

// Player reads a JSONL snapshot file and emits each Span to a channel.
type Player struct {
	path string
}

// NewPlayer constructs a Player for the file at path.
func NewPlayer(path string) *Player { return &Player{path: path} }

// Run opens path, reads each line as a Span, and emits to out. Returns when
// the file is exhausted or ctx is cancelled. Lines that fail to parse are
// silently skipped.
func (p *Player) Run(ctx context.Context, out chan<- telemetry.Span) error {
	f, err := os.Open(p.path)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var sp telemetry.Span
		if err := json.Unmarshal(sc.Bytes(), &sp); err != nil {
			continue
		}
		select {
		case out <- sp:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return sc.Err()
}
