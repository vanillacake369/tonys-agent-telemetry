package snapshot

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

func TestRecorderPlayer_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snap.jsonl")

	rec, err := NewRecorder(path)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	in := make(chan telemetry.Span, 4)
	recCtx, recCancel := context.WithCancel(context.Background())
	recDone := make(chan struct{})
	go func() { _ = rec.Run(recCtx, in); close(recDone) }()

	originals := []telemetry.Span{
		{TraceID: "t1", SpanID: "a", System: "anthropic", Model: "claude", InputTokens: 100},
		{TraceID: "t1", SpanID: "b", ParentSpanID: "a", System: "anthropic", OutputTokens: 50},
		{TraceID: "t2", SpanID: "c", System: "openai"},
	}
	for _, sp := range originals {
		in <- sp
	}
	close(in)
	<-recDone
	recCancel()

	// Now replay.
	player := NewPlayer(path)
	out := make(chan telemetry.Span, 8)
	playCtx, playCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer playCancel()
	playDone := make(chan struct{})
	go func() { _ = player.Run(playCtx, out); close(out); close(playDone) }()

	var got []telemetry.Span
	for sp := range out {
		got = append(got, sp)
	}
	<-playDone

	if len(got) != len(originals) {
		t.Fatalf("got %d, want %d", len(got), len(originals))
	}
	for i, sp := range got {
		if sp.SpanID != originals[i].SpanID || sp.System != originals[i].System {
			t.Errorf("[%d] mismatch: got %+v want %+v", i, sp, originals[i])
		}
		if sp.InputTokens != originals[i].InputTokens {
			t.Errorf("[%d] InputTokens: %d vs %d", i, sp.InputTokens, originals[i].InputTokens)
		}
	}
}

func TestPlayer_MalformedLineSkipped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.jsonl")
	if err := writeFile(path, []byte(
		"not json\n"+
			`{"traceID":"t","spanID":"u"}`+"\n"+
			"also not json\n",
	)); err != nil {
		t.Fatal(err)
	}

	player := NewPlayer(path)
	out := make(chan telemetry.Span, 4)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() { _ = player.Run(ctx, out); close(out); close(done) }()

	var got []telemetry.Span
	for sp := range out {
		got = append(got, sp)
	}
	<-done

	if len(got) != 1 {
		t.Errorf("got %d spans, want 1 (1 valid + 2 garbage skipped)", len(got))
	}
}

func TestRecorder_PathReturnsConfigured(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.jsonl")
	rec, err := NewRecorder(path)
	if err != nil {
		t.Fatal(err)
	}
	defer rec.Close()
	if rec.Path() != path {
		t.Errorf("Path() = %q, want %q", rec.Path(), path)
	}
}

func TestPlayer_CancelStopsEmission(t *testing.T) {
	path := filepath.Join(t.TempDir(), "many.jsonl")
	// Write many lines so cancel can land mid-stream.
	var content []byte
	for i := 0; i < 1000; i++ {
		content = append(content, []byte(`{"traceID":"t","spanID":"x"}`+"\n")...)
	}
	if err := writeFile(path, content); err != nil {
		t.Fatal(err)
	}

	player := NewPlayer(path)
	out := make(chan telemetry.Span) // unbuffered: forces send to block
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- player.Run(ctx, out) }()

	// Read a couple then cancel.
	<-out
	<-out
	cancel()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Player did not stop within 500ms of cancel")
	}
}

func writeFile(path string, data []byte) error {
	f, err := openCreate(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}
