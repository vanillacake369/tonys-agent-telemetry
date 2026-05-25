package recommender

import (
	"testing"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
)

// allSignalTypes enumerates every SignalType defined in SIGNALS_SPEC §2.
// This list is the SSoT for coverage assertions below.
var allSignalTypes = []signal.SignalType{
	signal.SignalStalledNode,
	signal.SignalDuplicateSubagentWork,
	signal.SignalUnusedInstalledSkill,
	signal.SignalFailedHandoff,
}

// TestSignalMappings_AllTypesHaveEntry asserts that every signal type defined
// in SIGNALS_SPEC has a corresponding entry in the SignalMappings table.
// A missing entry would cause the engine to silently produce zero recommendations
// for that signal type, which is a silent failure, not a panicking error.
func TestSignalMappings_AllTypesHaveEntry(t *testing.T) {
	for _, st := range allSignalTypes {
		if _, ok := SignalMappings[st]; !ok {
			t.Errorf("SignalMappings is missing entry for signal type %q", st)
		}
	}
}

// TestSignalMappings_BaseTagsNonEmpty asserts that every MappingRule has at
// least one BaseTag. A rule with no base tags would match nothing, making the
// entry dead code.
func TestSignalMappings_BaseTagsNonEmpty(t *testing.T) {
	for st, rule := range SignalMappings {
		if len(rule.BaseTags) == 0 {
			t.Errorf("SignalMappings[%q].BaseTags is empty — every rule needs at least one base tag", st)
		}
	}
}

// TestSignalMappings_StalledNode_BaseTags asserts the exact base tags for
// stalled_node per the task spec.
func TestSignalMappings_StalledNode_BaseTags(t *testing.T) {
	rule, ok := SignalMappings[signal.SignalStalledNode]
	if !ok {
		t.Fatal("SignalMappings missing stalled_node")
	}
	want := []string{"performance", "responsiveness"}
	if !equalStringSlices(rule.BaseTags, want) {
		t.Errorf("stalled_node BaseTags = %v, want %v", rule.BaseTags, want)
	}
}

// TestSignalMappings_StalledNode_Refinement_Bash tests that evidence key
// "tool_name"="bash" adds the "shell" extra tag.
func TestSignalMappings_StalledNode_Refinement_Bash(t *testing.T) {
	rule := SignalMappings[signal.SignalStalledNode]
	tags := applyRefinements(rule, map[string]any{"tool_name": "bash"})
	if !containsTag(tags, "shell") {
		t.Errorf("stalled_node with tool_name=bash: expected tag %q in %v", "shell", tags)
	}
}

// TestSignalMappings_StalledNode_Refinement_Read tests that evidence key
// "tool_name"="read" adds the "file-io" extra tag.
func TestSignalMappings_StalledNode_Refinement_Read(t *testing.T) {
	rule := SignalMappings[signal.SignalStalledNode]
	tags := applyRefinements(rule, map[string]any{"tool_name": "read"})
	if !containsTag(tags, "file-io") {
		t.Errorf("stalled_node with tool_name=read: expected tag %q in %v", "file-io", tags)
	}
}

// TestSignalMappings_StalledNode_Refinement_Write tests that evidence key
// "tool_name"="write" adds the "file-io" extra tag.
func TestSignalMappings_StalledNode_Refinement_Write(t *testing.T) {
	rule := SignalMappings[signal.SignalStalledNode]
	tags := applyRefinements(rule, map[string]any{"tool_name": "write"})
	if !containsTag(tags, "file-io") {
		t.Errorf("stalled_node with tool_name=write: expected tag %q in %v", "file-io", tags)
	}
}

// TestSignalMappings_StalledNode_Refinement_Edit tests that evidence key
// "tool_name"="edit" adds the "file-io" extra tag.
func TestSignalMappings_StalledNode_Refinement_Edit(t *testing.T) {
	rule := SignalMappings[signal.SignalStalledNode]
	tags := applyRefinements(rule, map[string]any{"tool_name": "edit"})
	if !containsTag(tags, "file-io") {
		t.Errorf("stalled_node with tool_name=edit: expected tag %q in %v", "file-io", tags)
	}
}

// TestSignalMappings_StalledNode_NoRefinement_Unknown tests that an unknown
// tool_name value falls back to base tags only.
func TestSignalMappings_StalledNode_NoRefinement_Unknown(t *testing.T) {
	rule := SignalMappings[signal.SignalStalledNode]
	tags := applyRefinements(rule, map[string]any{"tool_name": "some_unknown_tool"})
	// Should have base tags but not the refinement extras from known tools.
	if containsTag(tags, "shell") {
		t.Errorf("stalled_node with unknown tool_name: should not have tag %q, got %v", "shell", tags)
	}
	if containsTag(tags, "file-io") {
		t.Errorf("stalled_node with unknown tool_name: should not have tag %q, got %v", "file-io", tags)
	}
	if !containsTag(tags, "performance") {
		t.Errorf("stalled_node with unknown tool_name: expected base tag %q in %v", "performance", tags)
	}
}

// TestSignalMappings_DuplicateSubagentWork_BaseTags asserts the exact base tags
// for duplicate_subagent_work.
func TestSignalMappings_DuplicateSubagentWork_BaseTags(t *testing.T) {
	rule, ok := SignalMappings[signal.SignalDuplicateSubagentWork]
	if !ok {
		t.Fatal("SignalMappings missing duplicate_subagent_work")
	}
	want := []string{"orchestration", "fan-out"}
	if !equalStringSlices(rule.BaseTags, want) {
		t.Errorf("duplicate_subagent_work BaseTags = %v, want %v", rule.BaseTags, want)
	}
}

// TestSignalMappings_UnusedInstalledSkill_BaseTags asserts the exact base tags
// for unused_installed_skill.
func TestSignalMappings_UnusedInstalledSkill_BaseTags(t *testing.T) {
	rule, ok := SignalMappings[signal.SignalUnusedInstalledSkill]
	if !ok {
		t.Fatal("SignalMappings missing unused_installed_skill")
	}
	want := []string{"skill-utilization"}
	if !equalStringSlices(rule.BaseTags, want) {
		t.Errorf("unused_installed_skill BaseTags = %v, want %v", rule.BaseTags, want)
	}
}

// TestSignalMappings_UnusedInstalledSkill_Refinement_SkillName tests that the
// skill_name evidence value is appended as an extra tag, enabling the engine to
// prefer the specific skill item in the catalog if it exists.
func TestSignalMappings_UnusedInstalledSkill_Refinement_SkillName(t *testing.T) {
	rule := SignalMappings[signal.SignalUnusedInstalledSkill]
	skillName := "my-awesome-skill"
	tags := applyRefinements(rule, map[string]any{"skill_name": skillName})
	if !containsTag(tags, skillName) {
		t.Errorf("unused_installed_skill with skill_name=%q: expected the skill name as a tag in %v", skillName, tags)
	}
}

// TestSignalMappings_FailedHandoff_BaseTags asserts the exact base tags for
// failed_handoff.
func TestSignalMappings_FailedHandoff_BaseTags(t *testing.T) {
	rule, ok := SignalMappings[signal.SignalFailedHandoff]
	if !ok {
		t.Fatal("SignalMappings missing failed_handoff")
	}
	want := []string{"error-recovery", "retry-patterns"}
	if !equalStringSlices(rule.BaseTags, want) {
		t.Errorf("failed_handoff BaseTags = %v, want %v", rule.BaseTags, want)
	}
}

// TestSignalMappings_FirstRefinementWins asserts that for stalled_node, once a
// refinement matches (bash → shell), a second potential match (if the same
// evidence had another matching key) does not double-apply. We verify that
// first-match semantics hold by checking the output length stays bounded.
func TestSignalMappings_FirstRefinementWins(t *testing.T) {
	rule := SignalMappings[signal.SignalStalledNode]
	// Provide tool_name=bash; only the bash refinement should fire.
	tags := applyRefinements(rule, map[string]any{"tool_name": "bash"})
	// Count how many refinement-extra tags landed.
	extraCount := 0
	for _, tag := range tags {
		if tag == "shell" || tag == "file-io" {
			extraCount++
		}
	}
	if extraCount > 1 {
		t.Errorf("first-refinement-wins violated: got %d extra tags from a single evidence match in %v", extraCount, tags)
	}
}

// ---------------------------------------------------------------------------
// helpers used in this test file only
// ---------------------------------------------------------------------------

// equalStringSlices returns true when a and b contain the same elements in the
// same order.
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// containsTag returns true when tag appears in tags.
func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}
