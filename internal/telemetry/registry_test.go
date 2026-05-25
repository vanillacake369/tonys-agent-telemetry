package telemetry

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type mockIngestor struct {
	id          string
	detect      bool
	ingestCalls atomic.Int32
	ingestDone  chan struct{}
}

func (m *mockIngestor) ProviderID() string                  { return m.id }
func (m *mockIngestor) Detect(ctx context.Context) bool     { return m.detect }
func (m *mockIngestor) Ingest(ctx context.Context, out chan<- Span) error {
	m.ingestCalls.Add(1)
	if m.ingestDone != nil {
		defer close(m.ingestDone)
	}
	<-ctx.Done()
	return ctx.Err()
}

func TestRegistry_NoIngestorsRegistered(t *testing.T) {
	r := NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan Span, 1)
	r.StartAll(ctx, out)
	if d := r.Detected(); len(d) != 0 {
		t.Errorf("Detected = %d, want 0", len(d))
	}
}

func TestRegistry_DetectedIngestorIsStarted(t *testing.T) {
	r := NewRegistry()
	done := make(chan struct{})
	m := &mockIngestor{id: "mock", detect: true, ingestDone: done}
	r.Register(m)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan Span, 1)
	r.StartAll(ctx, out)

	// Give the goroutine a chance to start.
	deadline := time.After(500 * time.Millisecond)
	for m.ingestCalls.Load() == 0 {
		select {
		case <-deadline:
			t.Fatalf("Ingest was never called")
		case <-time.After(5 * time.Millisecond):
		}
	}
	if got := r.Detected(); len(got) != 1 || got[0].ProviderID() != "mock" {
		t.Errorf("Detected = %v, want [mock]", got)
	}
}

func TestRegistry_UndetectedIngestorNotStarted(t *testing.T) {
	r := NewRegistry()
	m := &mockIngestor{id: "mock", detect: false}
	r.Register(m)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan Span, 1)
	r.StartAll(ctx, out)

	time.Sleep(50 * time.Millisecond) // would have started by now if it were going to
	if got := m.ingestCalls.Load(); got != 0 {
		t.Errorf("ingestCalls = %d, want 0 (undetected)", got)
	}
	if d := r.Detected(); len(d) != 0 {
		t.Errorf("Detected = %d, want 0", len(d))
	}
}

func TestRegistry_CancelStopsIngest(t *testing.T) {
	r := NewRegistry()
	done := make(chan struct{})
	m := &mockIngestor{id: "mock", detect: true, ingestDone: done}
	r.Register(m)

	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan Span, 1)
	r.StartAll(ctx, out)

	// Wait for Ingest to start.
	deadline := time.After(500 * time.Millisecond)
	for m.ingestCalls.Load() == 0 {
		select {
		case <-deadline:
			t.Fatalf("Ingest never started")
		case <-time.After(5 * time.Millisecond):
		}
	}

	cancel()
	select {
	case <-done:
		// Ingest returned within reasonable time after cancel.
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("Ingest did not return within 200ms of cancel")
	}
}

func TestRegistry_PriorityOrderPreserved(t *testing.T) {
	r := NewRegistry()
	m1 := &mockIngestor{id: "first", detect: true}
	m2 := &mockIngestor{id: "second", detect: true}
	r.Register(m1)
	r.Register(m2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan Span, 1)
	r.StartAll(ctx, out)

	got := r.Detected()
	if len(got) != 2 {
		t.Fatalf("Detected len = %d, want 2", len(got))
	}
	if got[0].ProviderID() != "first" || got[1].ProviderID() != "second" {
		t.Errorf("Detected order = [%s, %s], want [first, second]",
			got[0].ProviderID(), got[1].ProviderID())
	}
}
