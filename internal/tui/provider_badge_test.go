package tui

import (
	"strings"
	"testing"
)

func TestProviderBadge_Anthropic(t *testing.T) {
	got := ProviderBadge("anthropic")
	if !strings.Contains(got, "CC") {
		t.Errorf("anthropic badge: got %q, want to contain CC", got)
	}
}

func TestProviderBadge_OTLP(t *testing.T) {
	got := ProviderBadge("otlp")
	if !strings.Contains(got, "OTL") {
		t.Errorf("otlp badge: got %q, want to contain OTL", got)
	}
}

func TestProviderBadge_VLLM(t *testing.T) {
	got := ProviderBadge("vllm")
	if !strings.Contains(got, "VLM") {
		t.Errorf("vllm badge: got %q, want to contain VLM", got)
	}
}

func TestProviderBadge_Ollama(t *testing.T) {
	got := ProviderBadge("ollama")
	if !strings.Contains(got, "OLM") {
		t.Errorf("ollama badge: got %q, want to contain OLM", got)
	}
}

func TestProviderBadge_Unknown(t *testing.T) {
	got := ProviderBadge("langgraph")
	if !strings.Contains(got, "???") {
		t.Errorf("unknown badge: got %q, want to contain ???", got)
	}
}

func TestProviderBadge_EmptyString(t *testing.T) {
	got := ProviderBadge("")
	if !strings.Contains(got, "???") {
		t.Errorf("empty badge: got %q, want to contain ???", got)
	}
}

// TestProviderBadge_PlainTagIs3Chars verifies the uncolored 3-char invariant
// by stripping ANSI sequences from the output (including trailing spaces).
func TestProviderBadge_PlainTagIs3Chars(t *testing.T) {
	cases := []string{"anthropic", "otlp", "vllm", "ollama", "unknown"}
	for _, sys := range cases {
		badge := ProviderBadge(sys)
		plain := stripAnsiSeq(badge)
		runes := []rune(plain)
		if len(runes) != 3 {
			t.Errorf("ProviderBadge(%q) plain = %q (len %d), want exactly 3 chars (may include trailing space)", sys, plain, len(runes))
		}
	}
}
