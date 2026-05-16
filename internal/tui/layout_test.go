package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
)

// ── SplitLayout ──────────────────────────────────────────────────────────────

func TestSplitLayout_NormalWidth(t *testing.T) {
	left, right, show := SplitLayout(120, 40)
	if !show {
		t.Error("showPreview should be true for width=120")
	}
	if left <= 0 {
		t.Errorf("left = %d, want > 0", left)
	}
	if right <= 0 {
		t.Errorf("right = %d, want > 0", right)
	}
	if left+right+1 != 120 {
		t.Errorf("left(%d) + right(%d) + 1 != 120", left, right)
	}
}

func TestSplitLayout_Width80_NoPreview(t *testing.T) {
	left, _, show := SplitLayout(80, 40)
	if show {
		t.Error("showPreview should be false for width=80 (below MinSplitWidth=90)")
	}
	if left != 80 {
		t.Errorf("left = %d, want 80 (full width)", left)
	}
}

func TestSplitLayout_NarrowWidth_NoPreview(t *testing.T) {
	left, right, show := SplitLayout(50, 40)
	if show {
		t.Error("showPreview should be false for width=50 (< MinSplitWidth=60)")
	}
	if right != 0 {
		t.Errorf("right = %d, want 0 when no preview", right)
	}
	if left != 50 {
		t.Errorf("left = %d, want 50 (full width)", left)
	}
}

func TestSplitLayout_VeryNarrowWidth_NoPreview(t *testing.T) {
	left, right, show := SplitLayout(30, 40)
	if show {
		t.Error("showPreview should be false for width=30 (< MinPreviewWidth=40)")
	}
	if right != 0 {
		t.Errorf("right = %d, want 0 when no preview", right)
	}
	_ = left
}

func TestSplitLayout_ZeroWidth(t *testing.T) {
	// Should not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("SplitLayout(0, 40) panicked: %v", r)
		}
	}()
	left, right, show := SplitLayout(0, 40)
	if show {
		t.Error("showPreview should be false for width=0")
	}
	_ = left
	_ = right
}

func TestSplitLayout_AtMinSplitBoundary(t *testing.T) {
	// Exactly at MinSplitWidth — preview should be shown.
	_, _, show := SplitLayout(MinSplitWidth, 50)
	if !show {
		t.Errorf("showPreview should be true at exactly MinSplitWidth=%d", MinSplitWidth)
	}

	// One below MinSplitWidth — preview should be hidden.
	_, _, show = SplitLayout(MinSplitWidth-1, 50)
	if show {
		t.Errorf("showPreview should be false at MinSplitWidth-1=%d", MinSplitWidth-1)
	}
}

func TestSplitLayout_PercentageRespected(t *testing.T) {
	left, _, show := SplitLayout(100, 40)
	if !show {
		t.Fatal("expected showPreview=true for width=100")
	}
	// left should be ~40% of 100 = 40
	if left < 38 || left > 42 {
		t.Errorf("left = %d, want approximately 40 (40%% of 100)", left)
	}
}

// ── ClampInt ─────────────────────────────────────────────────────────────────

func TestClampInt_WithinRange(t *testing.T) {
	if got := ClampInt(5, 0, 10); got != 5 {
		t.Errorf("ClampInt(5, 0, 10) = %d, want 5", got)
	}
}

func TestClampInt_BelowMin(t *testing.T) {
	if got := ClampInt(-3, 0, 10); got != 0 {
		t.Errorf("ClampInt(-3, 0, 10) = %d, want 0", got)
	}
}

func TestClampInt_AboveMax(t *testing.T) {
	if got := ClampInt(15, 0, 10); got != 10 {
		t.Errorf("ClampInt(15, 0, 10) = %d, want 10", got)
	}
}

func TestClampInt_AtMin(t *testing.T) {
	if got := ClampInt(0, 0, 10); got != 0 {
		t.Errorf("ClampInt(0, 0, 10) = %d, want 0", got)
	}
}

func TestClampInt_AtMax(t *testing.T) {
	if got := ClampInt(10, 0, 10); got != 10 {
		t.Errorf("ClampInt(10, 0, 10) = %d, want 10", got)
	}
}

func TestClampInt_EqualMinMax(t *testing.T) {
	if got := ClampInt(5, 3, 3); got != 3 {
		t.Errorf("ClampInt(5, 3, 3) = %d, want 3", got)
	}
}

// ── RenderSearchBar ───────────────────────────────────────────────────────────

func TestRenderSearchBar_NoRightLabel(t *testing.T) {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	result := RenderSearchBar(ti, 80, "", false)
	if result == "" {
		t.Error("RenderSearchBar should not return empty string")
	}
}

func TestRenderSearchBar_WithRightLabel(t *testing.T) {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	result := RenderSearchBar(ti, 80, "Sort: ⭐ Stars", false)
	if result == "" {
		t.Error("RenderSearchBar with rightLabel should not return empty string")
	}
	if !strings.Contains(result, "Stars") {
		t.Error("RenderSearchBar should contain right label text 'Stars'")
	}
}

func TestRenderSearchBar_NarrowWidth(t *testing.T) {
	ti := textinput.New()
	// Width=2 is below the minimum (4), should return empty string.
	result := RenderSearchBar(ti, 2, "", false)
	if result != "" {
		t.Errorf("RenderSearchBar with width=2 should return empty, got %q", result)
	}
}

func TestRenderSearchBar_ZeroWidth(t *testing.T) {
	ti := textinput.New()
	result := RenderSearchBar(ti, 0, "", false)
	if result != "" {
		t.Errorf("RenderSearchBar with width=0 should return empty, got %q", result)
	}
}

func TestRenderSearchBar_FocusedAddsBottomBorder(t *testing.T) {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	unfocused := RenderSearchBar(ti, 80, "", false)
	focused := RenderSearchBar(ti, 80, "", true)
	// Focused bar includes a bottom border line so its rendered height is larger.
	unfocusedLines := strings.Count(unfocused, "\n")
	focusedLines := strings.Count(focused, "\n")
	if focusedLines <= unfocusedLines {
		t.Errorf("focused search bar should have more lines than unfocused (focused=%d, unfocused=%d)", focusedLines, unfocusedLines)
	}
}

// ── RenderListItem ────────────────────────────────────────────────────────────

func TestRenderListItem_Selected(t *testing.T) {
	result := RenderListItem("my item", true, 40)
	if !strings.Contains(result, "▸") {
		t.Error("selected item should contain '▸' arrow indicator")
	}
	if !strings.Contains(result, "my item") {
		t.Error("item should contain the text")
	}
}

func TestRenderListItem_Normal(t *testing.T) {
	result := RenderListItem("my item", false, 40)
	if strings.Contains(result, "▸") {
		t.Error("normal item should not contain '▸' arrow indicator")
	}
	if !strings.Contains(result, "my item") {
		t.Error("item should contain the text")
	}
}

func TestRenderListItem_ZeroWidth(t *testing.T) {
	// Should not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RenderListItem with width=0 panicked: %v", r)
		}
	}()
	_ = RenderListItem("text", true, 0)
}

// ── RenderEmptyState ──────────────────────────────────────────────────────────

func TestRenderEmptyState_ContainsMessage(t *testing.T) {
	result := RenderEmptyState("No items found", 80, 20)
	if !strings.Contains(result, "No items found") {
		t.Error("empty state should contain the message")
	}
}

func TestRenderEmptyState_ZeroDimensions(t *testing.T) {
	// Should not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RenderEmptyState(0, 0) panicked: %v", r)
		}
	}()
	_ = RenderEmptyState("empty", 0, 0)
}

// ── RenderLoadingState ────────────────────────────────────────────────────────

func TestRenderLoadingState_NotEmpty(t *testing.T) {
	result := RenderLoadingState(80, 20)
	if result == "" {
		t.Error("loading state should not be empty")
	}
}

// ── RenderSplitView ───────────────────────────────────────────────────────────

func TestRenderSplitView_WithPreview(t *testing.T) {
	result := RenderSplitView("left content", "right content", 40, 39, 20, true)
	if result == "" {
		t.Error("split view should not be empty")
	}
}

func TestRenderSplitView_WithoutPreview(t *testing.T) {
	result := RenderSplitView("left content", "", 80, 0, 20, false)
	if !strings.Contains(result, "left content") {
		t.Error("should contain left content when no preview")
	}
}

func TestRenderSplitView_ZeroDimensions(t *testing.T) {
	// Should not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RenderSplitView with zero dimensions panicked: %v", r)
		}
	}()
	_ = RenderSplitView("left", "right", 0, 0, 0, true)
}
