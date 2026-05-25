package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRenderProviderEmptyState_ContainsAllProviders(t *testing.T) {
	got := RenderProviderEmptyState()
	plain := stripAnsiSeq(got)

	required := []string{"claudecode", "otlp", "vllm", "ollama"}
	for _, p := range required {
		if !strings.Contains(plain, p) {
			t.Errorf("RenderProviderEmptyState: expected %q in output, not found.\nOutput: %s", p, plain)
		}
	}
}

// TestDAGTab_EmptyStateContainsAllProviders is the App-level integration test:
// instantiate App, switch to DAG tab with no spans, call app.View(), assert all 4
// provider strings appear.
func TestDAGTab_EmptyStateContainsAllProviders(t *testing.T) {
	a := NewApp()
	a.width, a.height = 120, 30
	a = a.propagateSize()

	// No spans delivered — DAG tab should show empty state.
	a.activeTab = TabDAG

	// Deliver no spans: just get the view.
	updatedModel, _ := a.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	a = updatedModel.(App)

	view := a.View()
	plain := stripAnsiSeq(view)

	required := []string{"claudecode", "otlp", "vllm", "ollama"}
	for _, p := range required {
		if !strings.Contains(plain, p) {
			t.Errorf("App.View() DAG empty state: expected %q, not found.\nView: %s", p, plain)
		}
	}
}
