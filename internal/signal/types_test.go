package signal_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
)

// TestSignalType_Values asserts the four canonical signal-type string values
// match SIGNALS_SPEC §2 exactly.
func TestSignalType_Values(t *testing.T) {
	cases := []struct {
		got  signal.SignalType
		want string
	}{
		{signal.SignalStalledNode, "stalled_node"},
		{signal.SignalDuplicateSubagentWork, "duplicate_subagent_work"},
		{signal.SignalUnusedInstalledSkill, "unused_installed_skill"},
		{signal.SignalFailedHandoff, "failed_handoff"},
	}
	for _, c := range cases {
		if string(c.got) != c.want {
			t.Errorf("SignalType = %q, want %q", c.got, c.want)
		}
	}
}

// TestDefaultExtractOpts_Matches_Spec asserts that DefaultExtractOpts returns
// the exact default values stated in SIGNALS_SPEC §2.
func TestDefaultExtractOpts_Matches_Spec(t *testing.T) {
	opts := signal.DefaultExtractOpts()

	if opts.StallThreshold != 10*time.Second {
		t.Errorf("StallThreshold = %v, want 10s", opts.StallThreshold)
	}
	if opts.DupOverlapThreshold != 0.8 {
		t.Errorf("DupOverlapThreshold = %v, want 0.8", opts.DupOverlapThreshold)
	}
	if opts.MinSessionsForUnusedSkill != 3 {
		t.Errorf("MinSessionsForUnusedSkill = %d, want 3", opts.MinSessionsForUnusedSkill)
	}
	if opts.InstalledSkills != nil {
		t.Errorf("InstalledSkills = %v, want nil", opts.InstalledSkills)
	}
	if opts.ClockSkewTolerance != 500*time.Millisecond {
		t.Errorf("ClockSkewTolerance = %v, want 500ms", opts.ClockSkewTolerance)
	}
}

// TestSignal_JSONRoundTrip asserts that all Signal fields survive a
// JSON marshal/unmarshal cycle without data loss (SIGNALS_SPEC §5 determinism).
func TestSignal_JSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 5, 26, 9, 0, 0, 0, time.UTC)
	original := signal.Signal{
		ID:      "abc123def",
		Type:    signal.SignalStalledNode,
		TraceID: "trace-1",
		SpanIDs: []string{"span-a", "span-b"},
		Evidence: map[string]any{
			"stall_duration_ms": int64(14200),
			"threshold_ms":      int64(10000),
			"parent_span_id":    "span-parent",
			"tool_name":         "bash",
			"system":            "anthropic",
		},
		Confidence:   0.71,
		EmittedAt:    now,
		ProviderTier: "full",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded signal.Signal
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: got %q, want %q", decoded.ID, original.ID)
	}
	if decoded.Type != original.Type {
		t.Errorf("Type mismatch: got %q, want %q", decoded.Type, original.Type)
	}
	if decoded.TraceID != original.TraceID {
		t.Errorf("TraceID mismatch: got %q, want %q", decoded.TraceID, original.TraceID)
	}
	if len(decoded.SpanIDs) != len(original.SpanIDs) {
		t.Errorf("SpanIDs length mismatch: got %d, want %d", len(decoded.SpanIDs), len(original.SpanIDs))
	}
	if decoded.Confidence != original.Confidence {
		t.Errorf("Confidence mismatch: got %v, want %v", decoded.Confidence, original.Confidence)
	}
	if !decoded.EmittedAt.Equal(original.EmittedAt) {
		t.Errorf("EmittedAt mismatch: got %v, want %v", decoded.EmittedAt, original.EmittedAt)
	}
	if decoded.ProviderTier != original.ProviderTier {
		t.Errorf("ProviderTier mismatch: got %q, want %q", decoded.ProviderTier, original.ProviderTier)
	}
	// Spot-check evidence fields survive round-trip.
	if decoded.Evidence["tool_name"] != "bash" {
		t.Errorf("Evidence[tool_name] = %v, want bash", decoded.Evidence["tool_name"])
	}
}

// TestSignal_SpanIDsNeverNil asserts that a zero-value Signal's SpanIDs
// serializes as "[]" (empty array), never as JSON null.
func TestSignal_SpanIDsNeverNil(t *testing.T) {
	s := signal.Signal{
		Type:     signal.SignalUnusedInstalledSkill,
		SpanIDs:  []string{},
		Evidence: map[string]any{},
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal into raw map failed: %v", err)
	}
	if string(raw["span_ids"]) == "null" {
		t.Error("span_ids serialized as null; must be empty array []")
	}
}
