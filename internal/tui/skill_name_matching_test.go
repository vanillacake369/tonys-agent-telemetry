package tui_test

import (
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/skill"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// buildForestWithTool constructs a minimal Forest of n traces where each
// trace has a single root span containing one child leaf with the given
// gen_ai.tool.name attribute. This mimics how the unused_installed_skill
// detector receives invoked tool names.
func buildForestWithTool(n int, toolName string) signal.Forest {
	now := time.Date(2026, 5, 26, 9, 0, 0, 0, time.UTC)
	forest := make(signal.Forest)
	for i := 0; i < n; i++ {
		traceID := "trace-skill-" + string(rune('0'+i))
		root := &telemetry.SpanNode{
			Span: telemetry.Span{
				TraceID:   traceID,
				SpanID:    "root-" + traceID,
				StartTime: now.Add(-5 * time.Second),
				EndTime:   now,
				Status:    "done",
			},
		}
		if toolName != "" {
			leaf := &telemetry.SpanNode{
				Span: telemetry.Span{
					TraceID:      traceID,
					SpanID:       "leaf-" + traceID,
					ParentSpanID: "root-" + traceID,
					StartTime:    now.Add(-2 * time.Second),
					EndTime:      now,
					Status:       "done",
					Attrs: map[string]string{
						"gen_ai.tool.name": toolName,
					},
				},
			}
			root.Children = []*telemetry.SpanNode{leaf}
		}
		forest[traceID] = []*telemetry.SpanNode{root}
	}
	return forest
}

// TestSkillNameCanonicalForm_MatchesToolAttrName documents the HAPPY-PATH
// contract: when a span's gen_ai.tool.name exactly matches an installed
// skill's Name field, the unused_installed_skill detector does NOT emit a
// signal (the skill was invoked). (ν-6)
func TestSkillNameCanonicalForm_MatchesToolAttrName(t *testing.T) {
	// Construct a representative installed skill directly (no filesystem I/O).
	installedSkill := skill.Skill{
		Name:   "my-analysis-skill",
		Source: skill.SourceLocal,
	}

	// Build a forest where the tool name exactly matches the skill name.
	forest := buildForestWithTool(5, installedSkill.Name)

	opts := signal.DefaultExtractOpts()
	opts.InstalledSkills = []string{installedSkill.Name}

	sigs := signal.Extract(forest, opts)

	for _, s := range sigs {
		if s.Type == signal.SignalUnusedInstalledSkill {
			t.Errorf("exact-match case: detector emitted unused_installed_skill for %q, "+
				"but the skill WAS invoked (tool name = %q). "+
				"Evidence: %v", installedSkill.Name, installedSkill.Name, s.Evidence)
		}
	}
}

// TestSkillNameCanonicalForm_CaseMismatchSilentlySuppressesSignal documents
// the KNOWN LIMITATION: the detector performs exact string comparison, so a
// case mismatch between the installed skill name and gen_ai.tool.name causes
// the detector to emit unused_installed_skill even though the skill was
// effectively invoked.
//
// This test intentionally asserts the CURRENT (imperfect) behavior to
// document the limitation — it does NOT fix the detector. Fixing would
// require changes to internal/signal/ which are out of scope for Phase ν.
// (ν-6)
func TestSkillNameCanonicalForm_CaseMismatchSilentlySuppressesSignal(t *testing.T) {
	skillName := "My-Analysis-Skill"
	spanToolName := "my-analysis-skill" // different case

	forest := buildForestWithTool(5, spanToolName)

	opts := signal.DefaultExtractOpts()
	opts.InstalledSkills = []string{skillName}

	sigs := signal.Extract(forest, opts)

	foundUnused := false
	for _, s := range sigs {
		if s.Type == signal.SignalUnusedInstalledSkill {
			if name, _ := s.Evidence["skill_name"].(string); name == skillName {
				foundUnused = true
			}
		}
	}

	if !foundUnused {
		t.Errorf("case-mismatch case: expected unused_installed_skill for %q "+
			"(span used %q — different case), but detector did not emit. "+
			"This may mean the detector was fixed; update the test accordingly.",
			skillName, spanToolName)
	}

	// Document the limitation explicitly.
	t.Logf("KNOWN LIMITATION (ν-6): skill name %q != span tool name %q due to case. "+
		"The detector performs exact-string matching. "+
		"Callers of skill.ScanLocal() must canonicalize names (e.g. strings.ToLower) "+
		"before passing them to signal.Extract opts.InstalledSkills.",
		skillName, spanToolName)
}
