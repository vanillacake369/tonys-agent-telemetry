package signal_test

import (
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// makeTraceForest builds a forest of N traces, each with a single root span
// that invokes the given tool names via child leaf spans.
func makeTraceForest(n int, invokedTools []string) signal.Forest {
	now := time.Date(2026, 5, 26, 9, 0, 0, 0, time.UTC)
	forest := make(signal.Forest)
	for i := 0; i < n; i++ {
		traceID := traceIDFor(i)
		root := &telemetry.SpanNode{
			Span: telemetry.Span{
				TraceID:   traceID,
				SpanID:    spanIDFor("root", i),
				StartTime: now.Add(-5 * time.Second),
				EndTime:   now,
				Status:    "done",
			},
		}
		for j, tool := range invokedTools {
			leaf := &telemetry.SpanNode{
				Span: telemetry.Span{
					TraceID:      traceID,
					SpanID:       spanIDFor("leaf", i*100+j),
					ParentSpanID: spanIDFor("root", i),
					StartTime:    now.Add(-time.Duration(j+1) * time.Second),
					EndTime:      now.Add(-time.Duration(j) * time.Second),
					Status:       "done",
					Attrs: map[string]string{
						"gen_ai.tool.name": tool,
					},
				},
			}
			root.Children = append(root.Children, leaf)
		}
		forest[traceID] = []*telemetry.SpanNode{root}
	}
	return forest
}

func traceIDFor(i int) string {
	return "trace-" + intToStr(i)
}

func spanIDFor(prefix string, i int) string {
	return prefix + "-" + intToStr(i)
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// TestUnusedSkill_Positive is SIGNALS_SPEC T4: 5 traces, both skills installed,
// only skill-a invoked → signal for skill-b.
// confidence = min(1.0, 5/9) ≈ 0.5556
func TestUnusedSkill_Positive(t *testing.T) {
	forest := makeTraceForest(5, []string{"skill-a"})
	opts := signal.DefaultExtractOpts()
	opts.InstalledSkills = []string{"skill-a", "skill-b"}

	signals := signal.Extract(forest, opts)

	var unused []signal.Signal
	for _, s := range signals {
		if s.Type == signal.SignalUnusedInstalledSkill {
			unused = append(unused, s)
		}
	}
	if len(unused) != 1 {
		t.Fatalf("expected 1 unused_installed_skill signal, got %d", len(unused))
	}
	u := unused[0]
	if u.Evidence["skill_name"] != "skill-b" {
		t.Errorf("skill_name = %v, want skill-b", u.Evidence["skill_name"])
	}
	// confidence = min(1.0, 5/(3*3)) = 5/9 ≈ 0.5556
	wantConf := 5.0 / 9.0
	if diff := u.Confidence - wantConf; diff < -0.001 || diff > 0.001 {
		t.Errorf("Confidence = %.4f, want ~%.4f", u.Confidence, wantConf)
	}
	if u.TraceID != "" {
		t.Errorf("TraceID = %q; cross-trace signal must have empty TraceID", u.TraceID)
	}
	if len(u.SpanIDs) != 0 {
		t.Errorf("SpanIDs = %v; cross-trace signal must have empty SpanIDs", u.SpanIDs)
	}
}

// TestUnusedSkill_Negative_BothUsed: T4 negative — both skills invoked.
func TestUnusedSkill_Negative_BothUsed(t *testing.T) {
	forest := makeTraceForest(5, []string{"skill-a", "skill-b"})
	opts := signal.DefaultExtractOpts()
	opts.InstalledSkills = []string{"skill-a", "skill-b"}

	signals := signal.Extract(forest, opts)
	for _, s := range signals {
		if s.Type == signal.SignalUnusedInstalledSkill {
			t.Errorf("both skills invoked; expected no signal, got %+v", s)
		}
	}
}

// TestUnusedSkill_BelowMinSessions is SIGNALS_SPEC T5: 2 traces < MinSessions=3.
func TestUnusedSkill_BelowMinSessions(t *testing.T) {
	forest := makeTraceForest(2, []string{}) // neither skill invoked
	opts := signal.DefaultExtractOpts()      // MinSessionsForUnusedSkill=3
	opts.InstalledSkills = []string{"skill-a"}

	signals := signal.Extract(forest, opts)
	for _, s := range signals {
		if s.Type == signal.SignalUnusedInstalledSkill {
			t.Errorf("below MinSessions gate; expected no signal, got %+v", s)
		}
	}
}

// TestUnusedSkill_E7_EmptyInstalledSkills: E7 — empty slice → no signals.
func TestUnusedSkill_E7_EmptyInstalledSkills(t *testing.T) {
	forest := makeTraceForest(5, []string{})
	opts := signal.DefaultExtractOpts()
	opts.InstalledSkills = []string{} // empty

	signals := signal.Extract(forest, opts)
	for _, s := range signals {
		if s.Type == signal.SignalUnusedInstalledSkill {
			t.Errorf("empty InstalledSkills must not emit signal, got %+v", s)
		}
	}
}

// TestUnusedSkill_E5_DuplicateInstalledSkills: E5 — duplicates in InstalledSkills
// must produce at most one signal per unique name.
func TestUnusedSkill_E5_DuplicateInstalledSkills(t *testing.T) {
	forest := makeTraceForest(5, []string{"skill-a"})
	opts := signal.DefaultExtractOpts()
	opts.InstalledSkills = []string{"skill-b", "skill-b", "skill-b"} // triplicate

	signals := signal.Extract(forest, opts)
	count := 0
	for _, s := range signals {
		if s.Type == signal.SignalUnusedInstalledSkill {
			count++
		}
	}
	if count != 1 {
		t.Errorf("duplicate InstalledSkills: expected exactly 1 signal for skill-b, got %d", count)
	}
}

// TestUnusedSkill_E2_InProgressSpansCounted: E2 — in-progress spans still
// contribute their tool invocations to the invoked set.
func TestUnusedSkill_E2_InProgressSpansCounted(t *testing.T) {
	now := time.Now()
	forest := make(signal.Forest)
	for i := 0; i < 5; i++ {
		traceID := traceIDFor(i)
		root := &telemetry.SpanNode{
			Span: telemetry.Span{
				TraceID:   traceID,
				SpanID:    spanIDFor("root", i),
				StartTime: now.Add(-5 * time.Second),
				EndTime:   now,
				Status:    "done",
			},
		}
		// One completed child, one in-progress child that invokes skill-b.
		leaf1 := &telemetry.SpanNode{
			Span: telemetry.Span{
				TraceID:      traceID,
				SpanID:       spanIDFor("l1", i),
				ParentSpanID: spanIDFor("root", i),
				StartTime:    now.Add(-3 * time.Second),
				EndTime:      now,
				Status:       "done",
				Attrs:        map[string]string{"gen_ai.tool.name": "skill-a"},
			},
		}
		leaf2 := &telemetry.SpanNode{
			Span: telemetry.Span{
				TraceID:      traceID,
				SpanID:       spanIDFor("l2", i),
				ParentSpanID: spanIDFor("root", i),
				StartTime:    now.Add(-2 * time.Second),
				// EndTime zero = in-progress
				Status: "running",
				Attrs:  map[string]string{"gen_ai.tool.name": "skill-b"},
			},
		}
		root.Children = []*telemetry.SpanNode{leaf1, leaf2}
		forest[traceID] = []*telemetry.SpanNode{root}
	}

	opts := signal.DefaultExtractOpts()
	opts.InstalledSkills = []string{"skill-a", "skill-b"}

	signals := signal.Extract(forest, opts)
	for _, s := range signals {
		if s.Type == signal.SignalUnusedInstalledSkill {
			t.Errorf("in-progress spans should count as invoked; unexpected signal %+v", s)
		}
	}
}

// TestUnusedSkill_Evidence_Fields verifies required evidence fields.
func TestUnusedSkill_Evidence_Fields(t *testing.T) {
	forest := makeTraceForest(5, []string{"skill-a"})
	opts := signal.DefaultExtractOpts()
	opts.InstalledSkills = []string{"skill-a", "skill-b"}

	signals := signal.Extract(forest, opts)
	var u *signal.Signal
	for i := range signals {
		if signals[i].Type == signal.SignalUnusedInstalledSkill {
			u = &signals[i]
			break
		}
	}
	if u == nil {
		t.Fatal("no unused_installed_skill signal found")
	}
	for _, key := range []string{"skill_name", "sessions_checked", "min_sessions", "invoked_tool_names"} {
		if _, ok := u.Evidence[key]; !ok {
			t.Errorf("evidence missing key %q", key)
		}
	}
}
