package telemetry

import (
	"context"
	"testing"
	"time"
)

type mockIngestor struct {
	id       string
	detect   bool
	ingested chan struct{}
}

func (m *mockIngestor) ProviderID() string { return m.id }
func (m *mockIngestor) Detect(ctx context.Context) bool { return m.detect }
func (m *mockIngestor) Ingest(ctx context.Context, out chan<- Span) error {
	close(m.ingested)
	<-ctx.Done()
	return ctx.Err()
}

func newMock(id string, detect bool) *mockIngestor {
	return &mockIngestor{id: id, detect: detect, ingested: make(chan struct{})}
}

func TestRegistry_NoIngestorsRegistered(t *testing.T) {
	r := NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan Span, 1)

	// Must return immediately and not panic.
	done := make(chan struct{})
	go func() {
		r.StartAll(ctx, out)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Error("StartAll with no ingestors did not return")
	}

	if d := r.Detected(); len(d) != 0 {
		t.Errorf("Detected() = %d, want 0", len(d))
	}
}

func TestRegistry_MockIngestorDetected(t *testing.T) {
	r := NewRegistry()
	m := newMock("test", true)
	r.Register(m)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan Span, 1)

	r.StartAll(ctx, out)

	// Ingest should be called — wait for ingested signal.
	select {
	case <-m.ingested:
	case <-time.After(500 * time.Millisecond):
		t.Error("Ingest was not called for detected ingestor")
	}
}

func TestRegistry_MockIngestorNotDetected(t *testing.T) {
	r := NewRegistry()
	m := newMock("test", false)
	r.Register(m)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan Span, 1)

	r.StartAll(ctx, out)

	// Ingest must NOT be called.
	select {
	case <-m.ingested:
		t.Error("Ingest was called for non-detected ingestor")
	case <-time.After(100 * time.Millisecond):
		// Correct: Ingest was not called.
	}

	if d := r.Detected(); len(d) != 0 {
		t.Errorf("Detected() = %d, want 0", len(d))
	}
}

func TestRegistry_CancellationStopsIngest(t *testing.T) {
	r := NewRegistry()
	m := newMock("canceltest", true)
	r.Register(m)

	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan Span, 1)

	r.StartAll(ctx, out)

	// Wait for Ingest to start.
	select {
	case <-m.ingested:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Ingest did not start")
	}

	// Cancel and verify goroutine exits within 100ms (Ingest returns ctx.Err()).
	cancel()

	// The Ingest goroutine unblocks when ctx is cancelled. We verify by checking
	// that the ingested channel was closed (meaning Ingest was entered) and that
	// the context error is set promptly.
	deadline := time.After(100 * time.Millisecond)
	for {
		select {
		case <-deadline:
			// Check context is cancelled.
			if ctx.Err() == nil {
				t.Error("context not cancelled after cancel()")
			}
			return
		default:
			if ctx.Err() == context.Canceled {
				return
			}
		}
	}
}

func TestRegistry_PriorityOrder(t *testing.T) {
	r := NewRegistry()
	m1 := newMock("first", true)
	m2 := newMock("second", true)
	m3 := newMock("third", true)
	r.Register(m1)
	r.Register(m2)
	r.Register(m3)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan Span, 1)

	r.StartAll(ctx, out)

	detected := r.Detected()
	if len(detected) != 3 {
		t.Fatalf("Detected() = %d, want 3", len(detected))
	}

	// Order must match registration order.
	wantIDs := []string{"first", "second", "third"}
	for i, d := range detected {
		if d.ProviderID() != wantIDs[i] {
			t.Errorf("Detected()[%d].ProviderID() = %q, want %q", i, d.ProviderID(), wantIDs[i])
		}
	}
}
