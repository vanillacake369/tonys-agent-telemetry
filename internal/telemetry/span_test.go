package telemetry

import (
	"testing"
	"time"
)

func TestSpan_ZeroValueIsValid(t *testing.T) {
	var s Span
	// Reading any field must not panic.
	_ = s.TraceID
	_ = s.SpanID
	_ = s.InputTokens
	_ = s.Attrs
	if d := s.Duration(); d != 0 {
		t.Errorf("zero-value Duration() = %v, want 0", d)
	}
}

func TestSpan_Duration(t *testing.T) {
	start := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	end := start.Add(2500 * time.Millisecond)
	s := Span{StartTime: start, EndTime: end}
	if got := s.Duration(); got != 2500*time.Millisecond {
		t.Errorf("Duration = %v, want 2.5s", got)
	}
}

func TestSpan_DurationZeroWhenNotEnded(t *testing.T) {
	s := Span{StartTime: time.Now()}
	if got := s.Duration(); got != 0 {
		t.Errorf("Duration with zero EndTime = %v, want 0", got)
	}
}
