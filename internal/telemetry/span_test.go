package telemetry

import (
	"testing"
	"time"
)

func TestSpan_ZeroValueIsValid(t *testing.T) {
	var s Span
	// Accessing all fields on a zero-value Span must not panic.
	_ = s.TraceID
	_ = s.SpanID
	_ = s.ParentSpanID
	_ = s.System
	_ = s.Model
	_ = s.InputTokens
	_ = s.OutputTokens
	_ = s.StartTime
	_ = s.EndTime
	_ = s.Status
	_ = s.Attrs
}

func TestSpan_DurationFromTimes(t *testing.T) {
	start := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	end := start.Add(2500 * time.Millisecond)

	s := Span{
		StartTime: start,
		EndTime:   end,
	}

	d := s.EndTime.Sub(s.StartTime)
	if d < 0 {
		t.Errorf("duration = %v, want >= 0", d)
	}
	if d != 2500*time.Millisecond {
		t.Errorf("duration = %v, want 2.5s", d)
	}
}
