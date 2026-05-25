package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/provider/claudecode"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// TestDAGShape_RealJSONLFile loads ONE actual Claude session file (if
// any exist on the test host) and renders the largest trace. Useful for
// catching layout edge cases that only appear at real-data scale.
func TestDAGShape_RealJSONLFile(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	projects := filepath.Join(home, ".claude", "projects")
	if _, err := os.Stat(projects); err != nil {
		t.Skip("~/.claude/projects not present")
	}

	// Pick the first JSONL file we find.
	var picked string
	filepath.WalkDir(projects, func(p string, _ os.DirEntry, _ error) error {
		if picked != "" {
			return filepath.SkipAll
		}
		if strings.HasSuffix(p, ".jsonl") {
			picked = p
		}
		return nil
	})
	if picked == "" {
		t.Skip("no JSONL files under ~/.claude/projects")
	}

	// Parse spans via claudecode's converter.
	data, err := os.ReadFile(picked)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(string(data), "\n")
	var spans []telemetry.Span
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sp, err := claudecode.ConvertHookPayload("", []byte(line))
		if err != nil {
			continue
		}
		if sp.TraceID == "" || sp.SpanID == "" {
			continue
		}
		spans = append(spans, sp)
	}
	t.Logf("loaded %d spans from %s", len(spans), filepath.Base(picked))
	if len(spans) == 0 {
		t.Skip("no usable spans")
	}

	// Drive DAGTab.
	d := NewDAGTab().SetSize(160, 60).(*DAGTab)
	tab, _ := d.Update(SpanBatchMsg{Spans: spans})
	d = tab.(*DAGTab)

	if len(d.traces) == 0 {
		t.Fatal("no traces extracted")
	}
	t.Logf("DAG built %d traces from %d spans", len(d.traces), len(spans))

	// Drill into the trace with the most spans.
	bestIdx := 0
	bestCount := 0
	for i, tr := range d.traces {
		if tr.SpanCount > bestCount {
			bestCount = tr.SpanCount
			bestIdx = i
		}
	}
	d.traceCursor = bestIdx
	tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d = tab.(*DAGTab)

	rendered := d.renderGraph(156)
	lines = strings.Split(rendered, "\n")

	// Sanity checks — must contain box borders and at least one arrow type.
	plain := stripAnsi(rendered)
	if !strings.Contains(plain, "┌") {
		t.Error("real-data graph: missing box border")
	}
	hasChainArrow := strings.Contains(plain, "▼")
	hasBranchArrow := strings.Contains(plain, "→")
	if !hasChainArrow && !hasBranchArrow {
		t.Error("real-data graph: no chain (▼) or branch (→) arrow found")
	}

	t.Logf("Largest trace: %s, %d spans, depth %d", shortID(d.activeTrace), bestCount, 0)
	t.Logf("Rendered grid: %d lines × ~%d cols",
		len(lines), maxLineWidth(rendered))

	// Print first 60 lines for visual inspection.
	preview := rendered
	if len(lines) > 60 {
		preview = strings.Join(lines[:60], "\n") + "\n  ... " + fmt.Sprintf("(%d more lines)", len(lines)-60)
	}
	t.Logf("\n--- real-data graph preview (first 60 lines) ---\n%s", preview)
}

func maxLineWidth(s string) int {
	max := 0
	for _, line := range strings.Split(s, "\n") {
		w := len(stripAnsi(line))
		if w > max {
			max = w
		}
	}
	return max
}
