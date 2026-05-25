package main

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/tui"
)

// TestSplitChannel_DeliversAllSpansToAllBranches is the regression test for
// the silent-drop bug: the previous non-blocking try-send implementation
// dropped any span past the 256-buffer mark per branch. Backfills of real
// ~/.claude/projects directories contain 50k+ records.
func TestSplitChannel_DeliversAllSpansToAllBranches(t *testing.T) {
	const N = 10_000
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	in := make(chan telemetry.Span, 256)
	tuiCh, exportCh, recordCh := splitChannel(ctx, in)

	// Producer
	go func() {
		defer close(in)
		for i := 0; i < N; i++ {
			in <- telemetry.Span{TraceID: "t", SpanID: itoa(i)}
		}
	}()

	var tuiCount, exportCount, recordCount atomic.Int64
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); for range tuiCh { tuiCount.Add(1) } }()
	go func() { defer wg.Done(); for range exportCh { exportCount.Add(1) } }()
	go func() { defer wg.Done(); for range recordCh { recordCount.Add(1) } }()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("consumers did not finish; got tui=%d export=%d record=%d",
			tuiCount.Load(), exportCount.Load(), recordCount.Load())
	}

	if got := tuiCount.Load(); got != N {
		t.Errorf("tui branch received %d, want %d", got, N)
	}
	if got := exportCount.Load(); got != N {
		t.Errorf("export branch received %d, want %d", got, N)
	}
	if got := recordCount.Load(); got != N {
		t.Errorf("record branch received %d, want %d", got, N)
	}
}

func TestSplitChannel_CtxCancelStopsCleanly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	in := make(chan telemetry.Span)
	tuiCh, exportCh, recordCh := splitChannel(ctx, in)

	// No producer; cancel immediately.
	cancel()

	// All three downstreams should close within a short timeout.
	deadline := time.After(500 * time.Millisecond)
	channels := []<-chan telemetry.Span{tuiCh, exportCh, recordCh}
	for i, ch := range channels {
		select {
		case _, ok := <-ch:
			if ok {
				t.Errorf("branch %d delivered an unexpected span", i)
			}
		case <-deadline:
			t.Fatalf("branch %d did not close within 500ms of cancel", i)
		}
	}
}

func TestRunTUIBatcher_DeliversAllSpansViaBatches(t *testing.T) {
	const N = 5_000
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tuiCh := make(chan telemetry.Span, 1024)
	var received atomic.Int64
	send := func(msg tea.Msg) {
		batch, ok := msg.(tui.SpanBatchMsg)
		if !ok {
			t.Errorf("unexpected msg type %T", msg)
			return
		}
		received.Add(int64(len(batch.Spans)))
	}

	done := make(chan struct{})
	go func() { runTUIBatcher(ctx, tuiCh, send); close(done) }()

	go func() {
		defer close(tuiCh)
		for i := 0; i < N; i++ {
			tuiCh <- telemetry.Span{SpanID: itoa(i)}
		}
	}()

	<-done

	if got := received.Load(); got != N {
		t.Errorf("batcher delivered %d spans, want %d", got, N)
	}
}

func TestRunTUIBatcher_BatchesByCount(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tuiCh := make(chan telemetry.Span, 2048)
	var batchSizes []int
	var mu sync.Mutex
	send := func(msg tea.Msg) {
		if b, ok := msg.(tui.SpanBatchMsg); ok {
			mu.Lock()
			batchSizes = append(batchSizes, len(b.Spans))
			mu.Unlock()
		}
	}

	done := make(chan struct{})
	go func() { runTUIBatcher(ctx, tuiCh, send); close(done) }()

	// Burst-fill 1500 spans; expect at least one count-triggered batch (512).
	for i := 0; i < 1500; i++ {
		tuiCh <- telemetry.Span{SpanID: itoa(i)}
	}
	close(tuiCh)
	<-done

	mu.Lock()
	defer mu.Unlock()
	if len(batchSizes) == 0 {
		t.Fatal("no batches dispatched")
	}
	// At least one batch should have hit the 512 maxBatch ceiling.
	hitCeiling := false
	for _, n := range batchSizes {
		if n == 512 {
			hitCeiling = true
		}
	}
	if !hitCeiling {
		t.Errorf("expected at least one batch of size 512; got %v", batchSizes)
	}
}

// itoa is duplicated here so the test doesn't pull in strconv noise.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf []byte
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}
