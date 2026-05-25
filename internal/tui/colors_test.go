package tui

import (
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

func TestStatusColor_ErrorSpan(t *testing.T) {
	s := telemetry.Span{Status: "error"}
	got := StatusColor(s)
	if got != colorStatusError {
		t.Errorf("error span: got %v, want %v", got, colorStatusError)
	}
}

func TestStatusColor_RunningSpan(t *testing.T) {
	s := telemetry.Span{Status: "running"}
	got := StatusColor(s)
	if got != colorStatusWarn {
		t.Errorf("running span: got %v, want warn/yellow %v", got, colorStatusWarn)
	}
}

func TestStatusColor_DoneSpan(t *testing.T) {
	s := telemetry.Span{Status: "done"}
	got := StatusColor(s)
	if got != colorStatusOK {
		t.Errorf("done span: got %v, want %v", got, colorStatusOK)
	}
}

func TestStatusColor_NoStatus(t *testing.T) {
	s := telemetry.Span{}
	got := StatusColor(s)
	if got != colorStatusOK {
		t.Errorf("no-status span: got %v, want %v", got, colorStatusOK)
	}
}

func TestDurationColor_SlowRedAboveSlowMs(t *testing.T) {
	got := DurationColor(6000)
	if got != colorStatusError {
		t.Errorf("6000ms: got %v, want error-red %v", got, colorStatusError)
	}
}

func TestDurationColor_SlowYellowBetweenThresholds(t *testing.T) {
	got := DurationColor(1500)
	if got != colorStatusWarn {
		t.Errorf("1500ms: got %v, want warn-yellow %v", got, colorStatusWarn)
	}
}

func TestDurationColor_FastDefaultBelow1000ms(t *testing.T) {
	got := DurationColor(100)
	if got != colorStatusOK {
		t.Errorf("100ms: got %v, want ok-green %v", got, colorStatusOK)
	}
}

func TestDurationColor_ExactlySlowMsIsRed(t *testing.T) {
	got := DurationColor(SlowMs)
	if got != colorStatusError {
		t.Errorf("%dms exactly: got %v, want error-red %v", SlowMs, got, colorStatusError)
	}
}

func TestDurationColor_ExactlyWarnMsIsYellow(t *testing.T) {
	got := DurationColor(WarnMs)
	if got != colorStatusWarn {
		t.Errorf("%dms exactly: got %v, want warn-yellow %v", WarnMs, got, colorStatusWarn)
	}
}

func TestStatusColor_SlowSpanUsesErrorColor(t *testing.T) {
	// A "done" span that took > SlowMs should use StatusColor from span status,
	// but DurationColor should catch it.  This test verifies DurationColor is
	// used for duration-based coloring, independent of StatusColor.
	start := time.Now().Add(-6 * time.Second)
	end := time.Now()
	s := telemetry.Span{
		Status:    "done",
		StartTime: start,
		EndTime:   end,
	}
	dur := s.Duration().Milliseconds()
	got := DurationColor(dur)
	if got != colorStatusError {
		t.Errorf("done span 6s: DurationColor(%d) = %v, want error-red", dur, got)
	}
}

// Compile-time check that StatusColor returns lipgloss.Color-compatible type.
var _ lipgloss.TerminalColor = StatusColor(telemetry.Span{})
var _ lipgloss.TerminalColor = DurationColor(0)
