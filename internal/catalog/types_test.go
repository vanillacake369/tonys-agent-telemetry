package catalog

import "testing"

func TestItem_IsValid(t *testing.T) {
	cases := []struct {
		name string
		item Item
		want bool
	}{
		{
			name: "valid skill",
			item: Item{ID: "skill/test-driven-flow", Title: "Test Driven Flow", Type: ItemTypeSkill},
			want: true,
		},
		{
			name: "valid template",
			item: Item{ID: "template/go-microservice", Title: "Go Microservice", Type: ItemTypeTemplate},
			want: true,
		},
		{
			name: "valid agent",
			item: Item{ID: "agent/reviewer", Title: "Reviewer Agent", Type: ItemTypeAgent},
			want: true,
		},
		{
			name: "valid hook",
			item: Item{ID: "hook/post-tool-use", Title: "PostToolUse Hook", Type: ItemTypeHook},
			want: true,
		},
		{
			name: "missing ID",
			item: Item{Title: "Anonymous", Type: ItemTypeSkill},
			want: false,
		},
		{
			name: "missing Title",
			item: Item{ID: "skill/x", Type: ItemTypeSkill},
			want: false,
		},
		{
			name: "unknown Type",
			item: Item{ID: "x/y", Title: "Mystery", Type: "mystery"},
			want: false,
		},
		{
			name: "empty Type",
			item: Item{ID: "x/y", Title: "Untyped"},
			want: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.item.IsValid(); got != c.want {
				t.Errorf("IsValid()=%v, want %v", got, c.want)
			}
		})
	}
}

func TestItemType_KnownConstants(t *testing.T) {
	// Lock the constant values so external persistence (JSONL, etc.) doesn't
	// drift if someone renames the Go identifiers.
	if ItemTypeSkill != "skill" {
		t.Errorf("ItemTypeSkill = %q, want %q", ItemTypeSkill, "skill")
	}
	if ItemTypeTemplate != "template" {
		t.Errorf("ItemTypeTemplate = %q, want %q", ItemTypeTemplate, "template")
	}
	if ItemTypeAgent != "agent" {
		t.Errorf("ItemTypeAgent = %q, want %q", ItemTypeAgent, "agent")
	}
	if ItemTypeHook != "hook" {
		t.Errorf("ItemTypeHook = %q, want %q", ItemTypeHook, "hook")
	}
}
