package provider

import (
	"strings"
	"sync"
	"testing"
)

// TestRecoverIngest_CapturesPanic verifies that a goroutine panic is
// recovered and the logFn is invoked.
func TestRecoverIngest_CapturesPanic(t *testing.T) {
	var mu sync.Mutex
	var logged []string
	logFn := func(format string, args ...any) {
		mu.Lock()
		logged = append(logged, format)
		mu.Unlock()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer RecoverIngest("testprovider", logFn)()
		panic("intentional test panic")
	}()
	<-done

	mu.Lock()
	count := len(logged)
	mu.Unlock()
	if count == 0 {
		t.Error("expected logFn to be called on panic, got zero calls")
	}
}

// TestRecoverIngest_NoOpOnSuccess verifies that logFn is NOT called when no
// panic occurs.
func TestRecoverIngest_NoOpOnSuccess(t *testing.T) {
	var called bool
	logFn := func(format string, args ...any) {
		called = true
	}
	func() {
		defer RecoverIngest("testprovider", logFn)()
		// deliberately no panic
	}()
	if called {
		t.Error("logFn should not be called when no panic occurs")
	}
}

// TestRecoverIngest_LogsProviderID verifies that the log message contains
// the provider ID string and a non-empty stack trace fragment.
func TestRecoverIngest_LogsProviderID(t *testing.T) {
	type call struct {
		format string
		args   []any
	}
	var mu sync.Mutex
	var calls []call
	logFn := func(format string, args ...any) {
		mu.Lock()
		calls = append(calls, call{format: format, args: append([]any{}, args...)})
		mu.Unlock()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer RecoverIngest("myprovider", logFn)()
		panic("deliberate panic for test")
	}()
	<-done

	mu.Lock()
	snapshot := append([]call{}, calls...)
	mu.Unlock()

	if len(snapshot) == 0 {
		t.Fatal("logFn was never called")
	}

	// Flatten format + args into one searchable string.
	var sb strings.Builder
	for _, c := range snapshot {
		sb.WriteString(c.format)
		for _, a := range c.args {
			if s, ok := a.(string); ok {
				sb.WriteString(s)
			}
		}
	}
	combined := sb.String()

	if !strings.Contains(combined, "myprovider") {
		t.Errorf("log output %q does not contain provider ID %q", combined, "myprovider")
	}
	// A stack trace always contains "goroutine" or a file path with ".go".
	if !strings.Contains(combined, ".go") && !strings.Contains(combined, "goroutine") {
		t.Errorf("log output %q does not appear to contain a stack trace", combined)
	}
}
