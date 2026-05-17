package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestHighlightMatch_CJKSafeSlicing(t *testing.T) {
	// Korean text with search query that appears mid-string.
	text := "현재 기능 작동 여부를 모두 검토하고 완료되면 claude 뿐만"
	query := "검토"

	baseStyle := lipgloss.NewStyle()
	result := HighlightMatch(text, query, baseStyle)

	// Must contain the query highlighted.
	if !strings.Contains(result, "검토") {
		t.Errorf("expected highlighted '검토' in result, got: %q", result)
	}

	// Must not produce invalid UTF-8 (lipgloss.Width panics on invalid).
	w := lipgloss.Width(result)
	if w == 0 {
		t.Error("result has zero width — likely invalid UTF-8")
	}
}

func TestHighlightMatch_MixedCJKASCII(t *testing.T) {
	text := "fix the 로그인 bug in auth"
	query := "로그인"

	baseStyle := lipgloss.NewStyle()
	result := HighlightMatch(text, query, baseStyle)
	if !strings.Contains(result, "로그인") {
		t.Errorf("expected '로그인' in result, got: %q", result)
	}
	_ = lipgloss.Width(result) // must not panic
}

func TestFindMatchContext_CJKRuneSafe(t *testing.T) {
	text := "이것은 테스트 문자열입니다 검토 완료 확인"
	query := "검토"

	ctx := findMatchContext(text, query, 10)
	if ctx == "" {
		t.Error("expected non-empty context")
	}
	// Must be valid UTF-8 — every rune must be valid.
	for i, r := range ctx {
		if r == 0xFFFD {
			t.Errorf("invalid UTF-8 replacement char at position %d in context: %q", i, ctx)
		}
	}
	if !strings.Contains(ctx, "검토") {
		t.Errorf("expected '검토' in context, got: %q", ctx)
	}
}

func TestHighlightMatch_NoMatchReturnsFullText(t *testing.T) {
	text := "hello world"
	query := "없는검색어"
	baseStyle := lipgloss.NewStyle()
	result := HighlightMatch(text, query, baseStyle)
	if !strings.Contains(result, "hello world") {
		t.Errorf("no-match case should return full text, got: %q", result)
	}
}
