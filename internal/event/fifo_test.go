package event

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func makeTempFIFO(t *testing.T) (fifoPath string, pidPath string) {
	t.Helper()
	dir := t.TempDir()
	fifoPath = filepath.Join(dir, "test.fifo")
	pidPath = filepath.Join(dir, "test.pid")
	return fifoPath, pidPath
}

// removeFIFOAt cleans up a test FIFO and PID file.
func removeFIFOAt(fifoPath, pidPath string) {
	_ = os.Remove(fifoPath)
	_ = os.Remove(pidPath)
}

// writeToFIFOAt is a path-parameterised variant of WriteToFIFO for testing.
func writeToFIFOAt(fifoPath string, payload []byte, hookType string, timeout time.Duration) error {
	info, err := os.Stat(fifoPath)
	if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		return nil
	}

	done := make(chan error, 1)
	go func() {
		f, err := os.OpenFile(fifoPath, os.O_WRONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			done <- err
			return
		}
		defer f.Close()
		msg := hookType + "\t" + string(payload) + "\n"
		_, err = f.Write([]byte(msg))
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return nil // timeout is non-fatal in tests
	}
}

// ── CreateFIFO ────────────────────────────────────────────────────────────────

func TestCreateFIFO_CreatesNamedPipe(t *testing.T) {
	fifoPath, pidPath := makeTempFIFO(t)
	defer removeFIFOAt(fifoPath, pidPath)

	if err := createFIFOImpl(fifoPath, pidPath); err != nil {
		t.Fatalf("createFIFOImpl: %v", err)
	}

	info, err := os.Stat(fifoPath)
	if err != nil {
		t.Fatalf("stat fifo: %v", err)
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		t.Errorf("expected named pipe, got mode %v", info.Mode())
	}
}

func TestCreateFIFO_WritesPIDFile(t *testing.T) {
	fifoPath, pidPath := makeTempFIFO(t)
	defer removeFIFOAt(fifoPath, pidPath)

	if err := createFIFOImpl(fifoPath, pidPath); err != nil {
		t.Fatalf("createFIFOImpl: %v", err)
	}

	data, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("read pid file: %v", err)
	}
	if len(data) == 0 {
		t.Error("PID file should not be empty")
	}
}

func TestCreateFIFO_StalePIDFile_CleansUpAndCreates(t *testing.T) {
	fifoPath, pidPath := makeTempFIFO(t)
	defer removeFIFOAt(fifoPath, pidPath)

	// Write a PID that cannot exist (very large number).
	deadPID := "9999999"
	if err := os.WriteFile(pidPath, []byte(deadPID), 0600); err != nil {
		t.Fatalf("write pid file: %v", err)
	}
	// Create a stale non-FIFO file to simulate a leftover.
	if err := os.WriteFile(fifoPath, nil, 0600); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	if err := createFIFOImpl(fifoPath, pidPath); err != nil {
		t.Fatalf("createFIFOImpl with stale pid: %v", err)
	}

	info, err := os.Stat(fifoPath)
	if err != nil {
		t.Fatalf("stat fifo: %v", err)
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		t.Errorf("expected named pipe after stale cleanup, got mode %v", info.Mode())
	}
}

func TestCreateFIFO_LivePID_ReturnsError(t *testing.T) {
	fifoPath, pidPath := makeTempFIFO(t)
	defer removeFIFOAt(fifoPath, pidPath)

	// Write our own PID — guaranteed to be alive.
	livePID := fmt.Sprintf("%d", os.Getpid())
	if err := os.WriteFile(pidPath, []byte(livePID), 0600); err != nil {
		t.Fatalf("write pid file: %v", err)
	}

	err := createFIFOImpl(fifoPath, pidPath)
	if err == nil {
		t.Error("expected error for live PID, got nil")
	}
}

// ── RemoveFIFO ────────────────────────────────────────────────────────────────

func TestRemoveFIFO_CleansUpBothFiles(t *testing.T) {
	fifoPath, pidPath := makeTempFIFO(t)

	if err := createFIFOImpl(fifoPath, pidPath); err != nil {
		t.Fatalf("createFIFOImpl: %v", err)
	}

	removeFIFOAt(fifoPath, pidPath)

	if _, err := os.Stat(fifoPath); !os.IsNotExist(err) {
		t.Errorf("FIFO should be removed, got stat err: %v", err)
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Errorf("PID file should be removed, got stat err: %v", err)
	}
}

// ── WriteToFIFO + ReadFIFO roundtrip ─────────────────────────────────────────

func TestFIFO_Roundtrip(t *testing.T) {
	fifoPath, pidPath := makeTempFIFO(t)
	defer removeFIFOAt(fifoPath, pidPath)

	if err := createFIFOImpl(fifoPath, pidPath); err != nil {
		t.Fatalf("createFIFOImpl: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := ReadFIFOFromPath(ctx, fifoPath)

	// Write an event after a brief moment so the reader goroutine is ready.
	go func() {
		time.Sleep(50 * time.Millisecond)
		payload := []byte(`{"tool":"Bash"}`)
		_ = writeToFIFOAt(fifoPath, payload, "PostToolUse", 2*time.Second)
	}()

	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatal("channel closed before receiving event")
		}
		if ev.HookType != "PostToolUse" {
			t.Errorf("HookType = %q, want %q", ev.HookType, "PostToolUse")
		}
		if string(ev.Payload) != `{"tool":"Bash"}` {
			t.Errorf("Payload = %q, want %q", string(ev.Payload), `{"tool":"Bash"}`)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for event")
	}
}

func TestFIFO_MalformedData_Skipped(t *testing.T) {
	fifoPath, pidPath := makeTempFIFO(t)
	defer removeFIFOAt(fifoPath, pidPath)

	if err := createFIFOImpl(fifoPath, pidPath); err != nil {
		t.Fatalf("createFIFOImpl: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch := ReadFIFOFromPath(ctx, fifoPath)

	go func() {
		time.Sleep(50 * time.Millisecond)
		f, err := os.OpenFile(fifoPath, os.O_WRONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			return
		}
		defer f.Close()
		// Write a malformed line (no tab separator), then a valid line.
		_, _ = f.WriteString("no-tab-here\n")
		_, _ = f.WriteString("PostToolUse\t{\"ok\":true}\n")
	}()

	select {
	case ev, ok := <-ch:
		if !ok {
			t.Fatal("channel closed before receiving valid event")
		}
		if ev.HookType != "PostToolUse" {
			t.Errorf("HookType = %q, want %q", ev.HookType, "PostToolUse")
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for valid event after malformed line")
	}
}

func TestFIFO_ContextCancellation_ClosesChannel(t *testing.T) {
	fifoPath, pidPath := makeTempFIFO(t)
	defer removeFIFOAt(fifoPath, pidPath)

	if err := createFIFOImpl(fifoPath, pidPath); err != nil {
		t.Fatalf("createFIFOImpl: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	ch := ReadFIFOFromPath(ctx, fifoPath)

	// Cancel immediately to trigger goroutine shutdown.
	cancel()

	// Channel should close. Give a generous deadline.
	timeout := time.After(3 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				// Channel closed — test passes.
				return
			}
		case <-timeout:
			t.Fatal("channel did not close after context cancellation")
		}
	}
}
