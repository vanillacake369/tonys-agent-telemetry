package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// SpanCollectedMsg is sent by main.go for a single Span. Kept for the
// hot-path (live single events).
type SpanCollectedMsg struct {
	Span telemetry.Span
}

// SpanBatchMsg is sent by main.go for bursts of Spans (notably backfill
// from JSONL files containing thousands of records).
type SpanBatchMsg struct {
	Spans []telemetry.Span
}

// dagViewMode tracks which sub-view the DAG tab is showing. The user
// drills in (Enter / l) and out (Esc / h) through three levels:
//
//	traces  → graph    → span detail
//	(list)    (select)   (full attrs)
//
// Inspired by k9s's master-detail navigation pattern.
type dagViewMode int

const (
	dagViewTraces dagViewMode = iota
	dagViewGraph
	dagViewSpan
)

// DAGTab visualizes collected Spans as a navigable graph rather than the
// previous nested-stack indent tree.
type DAGTab struct {
	width, height int
	spans         []telemetry.Span

	mode dagViewMode

	// trace-list state
	traces      []traceEntry
	traceCursor int

	// graph state (only meaningful when mode >= dagViewGraph)
	activeTrace string
	flatNodes   []*telemetry.SpanNode
	nodeCursor  int

	// graph render cache — invalidated when any of the cache-key fields
	// change (trace selected, spans added, cursor moved, panel resized).
	// Without this, large traces re-render on every keystroke and freeze
	// the UI thread for hundreds of ms.
	graphCache    string
	graphCacheKey string

	// flash message — shown briefly at the bottom (e.g. "yanked!")
	flash      string
	flashUntil time.Time
}

type traceEntry struct {
	TraceID   string
	System    string
	SpanCount int
	MaxDepth  int
	Status    string
	LastSeen  time.Time
}

// NewDAGTab returns an initialised DAGTab.
func NewDAGTab() *DAGTab { return &DAGTab{} }

func (d *DAGTab) Init() tea.Cmd { return nil }

func (d *DAGTab) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	switch msg := msg.(type) {
	case SpanCollectedMsg:
		d.spans = append(d.spans, msg.Span)
		d.rebuildTraces()
		return d, nil
	case SpanBatchMsg:
		d.spans = append(d.spans, msg.Spans...)
		d.rebuildTraces()
		return d, nil
	case tea.KeyMsg:
		return d.handleKey(msg)
	}
	return d, nil
}

func (d *DAGTab) handleKey(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	switch d.mode {
	case dagViewTraces:
		return d.handleKeyTraces(msg)
	case dagViewGraph:
		return d.handleKeyGraph(msg)
	case dagViewSpan:
		return d.handleKeySpan(msg)
	}
	return d, nil
}

func (d *DAGTab) handleKeyTraces(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	switch s := msg.String(); s {
	case "j", "down":
		if len(d.traces) > 0 && d.traceCursor < len(d.traces)-1 {
			d.traceCursor++
		}
	case "k", "up":
		if d.traceCursor > 0 {
			d.traceCursor--
		}
	case "enter", "l", "right":
		if len(d.traces) > 0 {
			d.activeTrace = d.traces[d.traceCursor].TraceID
			d.flatNodes = d.flattenTrace(d.activeTrace)
			d.nodeCursor = 0
			d.mode = dagViewGraph
		}
	case "g":
		d.traceCursor = 0
	case "G":
		if len(d.traces) > 0 {
			d.traceCursor = len(d.traces) - 1
		}
	}
	return d, nil
}

func (d *DAGTab) handleKeyGraph(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	switch s := msg.String(); s {
	case "j", "down":
		if d.nodeCursor < len(d.flatNodes)-1 {
			d.nodeCursor++
		}
	case "k", "up":
		if d.nodeCursor > 0 {
			d.nodeCursor--
		}
	case "enter", "l", "right":
		if len(d.flatNodes) > 0 {
			d.mode = dagViewSpan
		}
	case "esc", "h", "left":
		d.mode = dagViewTraces
	case "y":
		if len(d.flatNodes) > 0 {
			return d, d.yankCmd(d.flatNodes[d.nodeCursor].Span)
		}
	case "e":
		if len(d.flatNodes) > 0 {
			return d, d.editCmd(d.flatNodes[d.nodeCursor].Span)
		}
	case "r":
		// Failsafe: clear the render cache so the next View() recomputes.
		// Useful if a terminal-multiplexer pane resize was missed.
		d.graphCache = ""
		d.graphCacheKey = ""
	}
	return d, nil
}

func (d *DAGTab) handleKeySpan(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	switch s := msg.String(); s {
	case "esc", "h", "left":
		d.mode = dagViewGraph
	case "y":
		if len(d.flatNodes) > 0 {
			return d, d.yankCmd(d.flatNodes[d.nodeCursor].Span)
		}
	case "e":
		if len(d.flatNodes) > 0 {
			return d, d.editCmd(d.flatNodes[d.nodeCursor].Span)
		}
	}
	return d, nil
}

// SetSize implements TabModel. Resets the render cache so the next View()
// reflows the graph for the new dimensions — critical for tmux/zellij
// pane resizes which arrive as WindowSizeMsg → propagateSize → SetSize.
func (d *DAGTab) SetSize(width, height int) TabModel {
	if width != d.width || height != d.height {
		d.graphCache = ""
		d.graphCacheKey = ""
	}
	d.width = width
	d.height = height
	return d
}

// rebuildTraces aggregates per-trace metadata from the current span slice.
// Cheap O(N) walk; called after each batch arrives.
func (d *DAGTab) rebuildTraces() {
	idx := make(map[string]*traceEntry)
	for _, s := range d.spans {
		t, ok := idx[s.TraceID]
		if !ok {
			t = &traceEntry{TraceID: s.TraceID, System: s.System, Status: "done"}
			idx[s.TraceID] = t
		}
		t.SpanCount++
		if s.System != "" && t.System == "" {
			t.System = s.System
		}
		if s.Status == "running" && t.Status != "error" {
			t.Status = "running"
		}
		if s.Status == "error" {
			t.Status = "error"
		}
		if s.StartTime.After(t.LastSeen) {
			t.LastSeen = s.StartTime
		}
	}
	// MaxDepth from trees.
	trees := telemetry.BuildTrees(d.spans)
	for traceID, root := range trees {
		if entry, ok := idx[traceID]; ok {
			_, d := treeStats(root)
			entry.MaxDepth = d
		}
	}

	traces := make([]traceEntry, 0, len(idx))
	for _, t := range idx {
		traces = append(traces, *t)
	}
	sort.SliceStable(traces, func(i, j int) bool {
		ti := traces[i].LastSeen
		tj := traces[j].LastSeen
		if ti.IsZero() != tj.IsZero() {
			return !ti.IsZero()
		}
		return ti.After(tj)
	})
	d.traces = traces
	if d.traceCursor >= len(d.traces) {
		d.traceCursor = 0
	}
}

// flattenTrace returns the spans of one trace in depth-first order so the
// graph view can address each by its row index for cursor navigation.
// Uses BuildForests so traces with multiple orphan roots are fully visible
// (BuildTrees would silently keep only one root per trace).
func (d *DAGTab) flattenTrace(traceID string) []*telemetry.SpanNode {
	traceSpans := make([]telemetry.Span, 0)
	for _, s := range d.spans {
		if s.TraceID == traceID {
			traceSpans = append(traceSpans, s)
		}
	}
	forests := telemetry.BuildForests(traceSpans)
	var out []*telemetry.SpanNode
	var walk func(n *telemetry.SpanNode)
	walk = func(n *telemetry.SpanNode) {
		if n == nil {
			return
		}
		out = append(out, n)
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, root := range forests[traceID] {
		walk(root)
	}
	return out
}

// yankCmd writes the span JSON to the clipboard via pbcopy / xclip /
// wl-copy. The flash message confirms the operation.
func (d *DAGTab) yankCmd(s telemetry.Span) tea.Cmd {
	return func() tea.Msg {
		b, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			return flashMsg{text: "yank failed: " + err.Error()}
		}
		if err := clipboardCopy(string(b)); err != nil {
			return flashMsg{text: "yank failed: " + err.Error()}
		}
		return flashMsg{text: fmt.Sprintf("yanked span %s to clipboard", shortID(s.SpanID))}
	}
}

// editCmd opens a temp file containing the span's JSON in $EDITOR. The
// file is for inspection/sharing only — edits are not persisted back
// into the in-memory store.
func (d *DAGTab) editCmd(s telemetry.Span) tea.Cmd {
	return tea.ExecProcess(buildEditCommand(s), func(err error) tea.Msg {
		if err != nil {
			return flashMsg{text: "edit failed: " + err.Error()}
		}
		return flashMsg{text: "span exported to /tmp"}
	})
}

func buildEditCommand(s telemetry.Span) *exec.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	f, err := os.CreateTemp("", "tat-span-*.json")
	if err != nil {
		return exec.Command(editor, "/dev/null")
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	_, _ = f.Write(b)
	path := f.Name()
	_ = f.Close()
	return exec.Command(editor, path)
}

// clipboardCopy is a thin wrapper around the platform clipboard.
// Defined as a var so tests can stub it.
var clipboardCopy = func(text string) error {
	candidates := [][]string{
		{"pbcopy"},
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
	}
	for _, args := range candidates {
		if _, err := exec.LookPath(args[0]); err == nil {
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Stdin = strings.NewReader(text)
			return cmd.Run()
		}
	}
	return fmt.Errorf("no clipboard tool found (install pbcopy/xclip/wl-copy)")
}

// flashMsg dispatches a brief status notification to the bottom bar.
type flashMsg struct{ text string }

func (d *DAGTab) View() string {
	if d.width == 0 {
		return ""
	}
	if len(d.spans) == 0 {
		return RenderEmptyState(
			"No telemetry spans collected yet. Launch a Claude Code session or wait for backfill.",
			d.width, d.height)
	}

	switch d.mode {
	case dagViewTraces:
		return d.renderTracesView()
	case dagViewGraph:
		return d.renderGraphView()
	case dagViewSpan:
		return d.renderSpanView()
	}
	return ""
}

// renderTracesView is the top-level list (k9s-style table).
func (d *DAGTab) renderTracesView() string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(colorDim)
	cursorStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(colorDim)

	var sb strings.Builder
	sb.WriteString(headerStyle.Render(fmt.Sprintf("%-4s %-8s %-30s %-15s %-7s %-7s %s",
		"#", "STATUS", "TRACE", "SYSTEM", "SPANS", "DEPTH", "LAST")))
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render(strings.Repeat("─", min(d.width, 90))))
	sb.WriteString("\n")

	const maxRows = 30
	end := len(d.traces)
	if end > maxRows {
		end = maxRows
	}
	for i := 0; i < end; i++ {
		t := d.traces[i]
		cursor := "  "
		row := fmt.Sprintf("%-4d %-8s %-30s %-15s %-7d %-7d %s",
			i+1,
			statusIcon(t.Status),
			shortID(t.TraceID),
			truncate(t.System, 15),
			t.SpanCount,
			t.MaxDepth,
			t.LastSeen.Local().Format("15:04:05"),
		)
		if i == d.traceCursor {
			cursor = cursorStyle.Render("▶ ")
			row = cursorStyle.Render(row)
		}
		sb.WriteString(cursor + row + "\n")
	}
	if len(d.traces) > maxRows {
		sb.WriteString(dimStyle.Render(fmt.Sprintf("  … %d more\n", len(d.traces)-maxRows)))
	}
	sb.WriteString("\n")
	sb.WriteString(d.renderHelp([]helpKey{
		{"j/k", "navigate"}, {"enter/l", "open graph"}, {"g/G", "top/bottom"},
	}))
	sb.WriteString(d.renderFlash())
	return RenderPanel(fmt.Sprintf("Agent DAG · %d traces · %d spans", len(d.traces), len(d.spans)),
		sb.String(), d.width, max(3, d.height), true)
}

// renderGraphView shows the selected trace as a left-to-right flow with
// the currently-selected node highlighted, plus a right-pane preview of
// the selected span's attributes.
//
// Width allocation must be exact: RenderPanel wraps content with Width(N),
// and lipgloss wraps (not truncates) on overflow. Wrapped box-drawing
// chars look broken to the user. The contract is:
//
//	leftPanel.width + spacer + rightPanel.width == d.width - 2 (panel border)
func (d *DAGTab) renderGraphView() string {
	if len(d.flatNodes) == 0 {
		return RenderEmptyState("(no spans in this trace)", d.width, d.height)
	}

	// Total inner content area inside RenderPanel = d.width - 2 (the
	// panel's left + right border characters).
	contentBudget := d.width - 2
	if contentBudget < 20 {
		contentBudget = 20
	}

	// Right pane is only useful if it has room for the span attribute
	// labels (≈28 cols). Otherwise hide it and give all space to the graph.
	const minRightPaneW = 28
	const spacerW = 2 // visual gap between panes
	leftW := contentBudget
	rightW := 0
	spacer := ""
	if contentBudget >= minRightPaneW*2+spacerW {
		// Both panes fit. Right takes ~40% (bounded by minRightPaneW).
		rightW = contentBudget * 2 / 5
		if rightW < minRightPaneW {
			rightW = minRightPaneW
		}
		leftW = contentBudget - spacerW - rightW
		spacer = "  "
	}

	leftPanel := d.renderGraph(leftW)
	body := leftPanel
	if rightW > 0 {
		rightPanel := d.renderSelectedSpanCompact(rightW)
		body = lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, spacer, rightPanel)
	}

	help := d.renderHelp([]helpKey{
		{"j/k", "select node"}, {"enter/l", "detail"}, {"y", "yank JSON"},
		{"e", "edit copy"}, {"r", "force redraw"}, {"esc/h", "back"},
	})

	return RenderPanel(
		fmt.Sprintf("Trace %s · %d nodes", shortID(d.activeTrace), len(d.flatNodes)),
		body+"\n\n"+help+d.renderFlash(),
		d.width, max(3, d.height), true)
}

// renderSpanView is the full-screen detail of a selected span.
func (d *DAGTab) renderSpanView() string {
	if d.nodeCursor >= len(d.flatNodes) {
		d.mode = dagViewGraph
		return d.renderGraphView()
	}
	s := d.flatNodes[d.nodeCursor].Span
	body := d.renderSpanFull(s)
	help := d.renderHelp([]helpKey{
		{"y", "yank JSON"}, {"e", "edit copy in \\$EDITOR"}, {"esc/h", "back to graph"},
	})
	return RenderPanel(
		fmt.Sprintf("Span %s · %s", shortID(s.SpanID), s.System),
		body+"\n\n"+help+d.renderFlash(),
		d.width, max(3, d.height), true)
}

// renderGraph draws the trace as a true 2D DAG with box nodes connected
// by L-shaped elbow connectors.
//
// Layout (n8n-style, not "col=depth"):
//
//	Single-child links stay in the SAME column → chain reads top-down,
//	  one node per row, like an n8n linear workflow.
//	Multi-child branches fan out to the next column → siblings stack
//	  vertically at col+1, parent's trunk fans into each.
//
// This avoids the diagonal staircase that pure col=depth produces for
// long Claude tool chains (which would overflow the terminal width
// after ~6 levels).
// maxVisibleNodes caps how many spans are rendered at once. The grid +
// per-line lipgloss styling is O(N), so unbounded N freezes the UI on
// 5000+ span traces. The user navigates the full trace by sliding the
// window via j/k; cursor stays roughly 1/3 down the viewport.
const maxVisibleNodes = 50

func (d *DAGTab) renderGraph(width int) string {
	if len(d.flatNodes) == 0 {
		return ""
	}

	// Cache hit: same trace, same cursor, same width → reuse last render.
	cacheKey := fmt.Sprintf("%s|n=%d|c=%d|w=%d",
		d.activeTrace, len(d.flatNodes), d.nodeCursor, width)
	if d.graphCache != "" && d.graphCacheKey == cacheKey {
		return d.graphCache
	}

	// Chain-aware layout. Walk the forest; a single-child link inherits
	// the parent's column (vertical chain), while a multi-child branch
	// pushes every child to col+1 (horizontal fanout, vertically stacked
	// by visit order).
	idSet := map[string]bool{}
	for _, n := range d.flatNodes {
		idSet[n.Span.SpanID] = true
	}
	var positions []gridPos
	posByID := make(map[string]int, len(d.flatNodes))
	maxCol := 0
	nextRow := 0
	var visit func(*telemetry.SpanNode, int)
	visit = func(n *telemetry.SpanNode, col int) {
		if col > maxCol {
			maxCol = col
		}
		positions = append(positions, gridPos{col: col, row: nextRow, node: n})
		posByID[n.Span.SpanID] = len(positions) - 1
		nextRow++
		switch len(n.Children) {
		case 0:
			// leaf
		case 1:
			visit(n.Children[0], col) // chain — same column
		default:
			for _, c := range n.Children {
				visit(c, col+1) // branch — all siblings to col+1
			}
		}
	}
	// Top-level roots: walk those whose parent isn't in this trace.
	for _, n := range d.flatNodes {
		if _, seen := posByID[n.Span.SpanID]; seen {
			continue
		}
		if n.Span.ParentSpanID == "" || !idSet[n.Span.ParentSpanID] {
			if len(positions) > 0 {
				nextRow++ // blank row between separate root subtrees
			}
			visit(n, 0)
		}
	}

	// Windowing: render only the chunk around the cursor. Cursor sits
	// at ~1/3 down the visible window so the user sees both context
	// above and what's coming below.
	totalNodes := len(positions)
	winStart := d.nodeCursor - maxVisibleNodes/3
	if winStart < 0 {
		winStart = 0
	}
	winEnd := winStart + maxVisibleNodes
	if winEnd > totalNodes {
		winEnd = totalNodes
		winStart = winEnd - maxVisibleNodes
		if winStart < 0 {
			winStart = 0
		}
	}
	windowPositions := positions[winStart:winEnd]
	if len(windowPositions) == 0 {
		return ""
	}
	// Re-anchor row indices so the window starts at row 0.
	baseRow := windowPositions[0].row
	winPosByID := make(map[string]int, len(windowPositions))
	for i := range windowPositions {
		windowPositions[i].row -= baseRow
		winPosByID[windowPositions[i].node.Span.SpanID] = i
	}
	positions = windowPositions
	posByID = winPosByID
	// Recompute maxCol within the window.
	maxCol = 0
	for _, p := range positions {
		if p.col > maxCol {
			maxCol = p.col
		}
	}

	// Box dimensions adapt to panel width so the graph reflows correctly
	// when the user resizes their tmux/zellij pane. nodeW shrinks toward
	// a 12-char minimum before any single line exceeds the panel.
	const (
		nodeH      = 3 // box height (lines)
		hgap       = 4 // chars between columns (elbow space)
		vgap       = 1 // blank lines between rows
		nodeWMin   = 12
		nodeWMax   = 28
		nodeWIdeal = 22
	)
	// Pre-walk window maxCol to pick a box width that fits the panel.
	// Available cols for boxes = width - hgap*maxCol (gaps).
	// nodeW such that (maxCol+1)*nodeW + maxCol*hgap ≤ width.
	nodeW := nodeWIdeal
	if width > 0 {
		// Compute maxCol of the FULL layout first; we'll narrow below.
		fitCols := maxCol + 1
		usable := width - hgap*maxCol
		candidate := usable / fitCols
		if candidate < nodeWIdeal {
			nodeW = clampInt(candidate, nodeWMin, nodeWIdeal)
		}
	}
	pitchX := nodeW + hgap
	pitchY := nodeH + vgap

	// gridH must derive from the LAST occupied row (positions are sparse
	// when orphan-subtree gaps insert blank rows), not len(positions).
	maxRow := 0
	for _, p := range positions {
		if p.row > maxRow {
			maxRow = p.row
		}
	}
	gridW := (maxCol+1)*pitchX - hgap
	gridH := (maxRow+1)*pitchY - vgap
	if gridH < 1 {
		gridH = 1
	}

	// Allocate a 2D rune grid. Cells default to space; box / connector
	// drawing overwrites specific cells.
	grid := make([][]rune, gridH)
	for i := range grid {
		grid[i] = make([]rune, gridW)
		for j := range grid[i] {
			grid[i][j] = ' '
		}
	}

	// Draw boxes.
	for _, p := range positions {
		x := p.col * pitchX
		y := p.row * pitchY
		drawBox(grid, x, y, nodeW, nodeH, nodeLabel(p.node.Span))
	}

	// Draw connectors. Chain (single child, same col) gets a vertical line
	// between parent and child; branch (multi child or differing col) gets
	// the trunk-and-elbow group routing.
	for _, p := range positions {
		if len(p.node.Children) == 0 {
			continue
		}
		var kids []gridPos
		for _, c := range p.node.Children {
			if idx, ok := posByID[c.Span.SpanID]; ok {
				kids = append(kids, positions[idx])
			}
		}
		if len(kids) == 0 {
			continue
		}
		if len(kids) == 1 && kids[0].col == p.col {
			drawChainConnector(grid, p, kids[0], nodeW, nodeH, pitchX, pitchY)
		} else {
			drawConnectorGroup(grid, p, kids, nodeW, nodeH, pitchX, pitchY)
		}
	}

	// Convert grid to string, applying colors per row. We can't easily
	// color individual cells with lipgloss styles applied at conversion
	// time, so per-line styling is the practical balance: the user gets
	// readable graph + a cursor arrow on the selected row.
	cursorStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(colorDim)
	errStyle := lipgloss.NewStyle().Foreground(colorError)
	runStyle := lipgloss.NewStyle().Foreground(colorWarning)
	doneStyle := lipgloss.NewStyle().Foreground(colorSuccess)

	// Which lines belong to which node's box? For each node, three lines.
	type rowColor struct{ start, end int; style lipgloss.Style }
	colorRanges := []rowColor{}
	for _, p := range positions {
		y := p.row * pitchY
		var style lipgloss.Style
		switch p.node.Span.Status {
		case "error":
			style = errStyle
		case "running":
			style = runStyle
		default:
			style = doneStyle
		}
		colorRanges = append(colorRanges, rowColor{start: y, end: y + nodeH, style: style})
	}

	// Show a window header so the user knows what's visible.
	var sb strings.Builder
	if totalNodes > maxVisibleNodes {
		hint := lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render(
			fmt.Sprintf("Nodes %d – %d of %d  (j/k slides window)", winStart+1, winEnd, totalNodes))
		sb.WriteString(hint + "\n")
	}
	for y, row := range grid {
		// Determine if this row is part of the selected node's box.
		// The cursor index is global (into d.flatNodes); translate to
		// the local windowed positions array.
		selectedRow := false
		localCursor := d.nodeCursor - winStart
		if localCursor >= 0 && localCursor < len(positions) {
			sp := positions[localCursor]
			boxYStart := sp.row * pitchY
			if y >= boxYStart && y < boxYStart+nodeH {
				selectedRow = true
			}
		}

		line := string(row)
		// Style: cursor arrow prefix on selected node's middle line.
		if selectedRow {
			line = cursorStyle.Render(line)
		} else {
			// Pick the style based on which node (if any) this row belongs to.
			var styled bool
			for _, cr := range colorRanges {
				if y >= cr.start && y < cr.end {
					line = cr.style.Render(line)
					styled = true
					break
				}
			}
			if !styled {
				// Pure connector row — dim styling.
				line = dimStyle.Render(line)
			}
		}

		// Cursor marker for the selected node's middle row.
		if selectedRow {
			localCursor := d.nodeCursor - winStart
			if localCursor >= 0 && localCursor < len(positions) {
				sp := positions[localCursor]
				midY := sp.row*pitchY + nodeH/2
				if y == midY {
					line = cursorStyle.Render("▶ ") + line
				} else {
					line = "  " + line
				}
			} else {
				line = "  " + line
			}
		} else {
			line = "  " + line
		}

		sb.WriteString(line)
		if y < len(grid)-1 {
			sb.WriteString("\n")
		}
	}
	rendered := sb.String()
	// Hard cap: even after dynamic nodeW, a deep chain can still exceed
	// the panel. Truncate every line so lipgloss never has to wrap our
	// box-drawing chars (which would garble the layout).
	if width > 0 {
		rendered = clipLinesToWidth(rendered, width)
	}
	d.graphCache = rendered
	d.graphCacheKey = cacheKey
	return rendered
}

// clipLinesToWidth truncates each line to the given char limit, appending
// a "›" marker when content was clipped. Visible whitespace is preserved
// so the box alignment within the visible region still reads correctly.
func clipLinesToWidth(s string, max int) string {
	if max <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		plain := stripAnsiSeq(line)
		if len([]rune(plain)) <= max {
			continue
		}
		// Truncate the plain form, append marker. For lines that contain
		// lipgloss ANSI sequences this loses styling on the truncated
		// portion — acceptable because clip is the fail-safe path.
		runes := []rune(plain)
		lines[i] = string(runes[:max-1]) + "›"
	}
	return strings.Join(lines, "\n")
}

// stripAnsiSeq removes ANSI escape sequences (kept local; tests use
// stripAnsi). Defined here so the file is self-contained when tests
// aren't compiled.
func stripAnsiSeq(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
				j++
			}
			if j < len(s) {
				j++
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// drawBox writes a 3-row box border + label into the grid starting at (x, y).
// Box layout:
//
//	┌──────────────┐
//	│ <label>      │
//	└──────────────┘
func drawBox(grid [][]rune, x, y, w, h int, label string) {
	if y+h > len(grid) || x+w > len(grid[0]) {
		return
	}
	// Borders.
	grid[y][x] = '┌'
	grid[y][x+w-1] = '┐'
	grid[y+h-1][x] = '└'
	grid[y+h-1][x+w-1] = '┘'
	for j := x + 1; j < x+w-1; j++ {
		grid[y][j] = '─'
		grid[y+h-1][j] = '─'
	}
	for i := y + 1; i < y+h-1; i++ {
		grid[i][x] = '│'
		grid[i][x+w-1] = '│'
	}
	// Label (one row at vertical centre).
	labelY := y + h/2
	labelRunes := []rune(" " + label)
	max := w - 2
	if len(labelRunes) > max {
		labelRunes = append(labelRunes[:max-1], '…')
	}
	for i, r := range labelRunes {
		grid[labelY][x+1+i] = r
	}
}

// gridPos is the layout position of a node in the 2D graph grid.
type gridPos struct {
	col, row int
	node     *telemetry.SpanNode
}

// drawChainConnector draws the simple vertical link used when a parent has
// exactly one child and the child sits in the same column (a "chain"
// link, top to bottom). Single ▼ arrow head replaces the inter-box gap.
func drawChainConnector(grid [][]rune, parent, child gridPos, nodeW, nodeH, pitchX, pitchY int) {
	mid := parent.col*pitchX + nodeW/2
	yStart := parent.row*pitchY + nodeH       // first row below parent box
	yEnd := child.row * pitchY                // first row of child box
	if yStart >= len(grid) || mid >= len(grid[0]) {
		return
	}
	for y := yStart; y < yEnd-1; y++ {
		grid[y][mid] = '│'
	}
	if yEnd-1 >= 0 && yEnd-1 < len(grid) {
		grid[yEnd-1][mid] = '▼'
	}
}

// childRow is the y-coordinate + col of a node, used internally by
// drawConnectorGroup to sort children top-to-bottom for clean trunk routing.
type childRow struct {
	y, col int
}

// drawConnectorGroup draws a trunk from `parent`'s right edge that branches
// to each child's left edge. The trunk uses ┬ at the top (parent row),
// │ down the column, ├ at each non-last child branch, and └ at the last.
// Each branch ends with → pointing into the child's left edge.
func drawConnectorGroup(grid [][]rune, parent gridPos, kids []gridPos, nodeW, nodeH, pitchX, pitchY int) {
	// Parent's right-edge midpoint.
	pMidY := parent.row*pitchY + nodeH/2
	pRightX := parent.col*pitchX + nodeW
	if pRightX+1 >= len(grid[0]) {
		return
	}
	// Trunk column = 2 chars right of parent.
	trunkX := pRightX + 1

	// Step right one ─ between parent and trunk.
	if pMidY < len(grid) && pRightX < len(grid[0]) {
		grid[pMidY][pRightX] = '─'
	}

	// Compute child Y positions and total trunk span.
	var rows []childRow
	for _, k := range kids {
		rows = append(rows, childRow{y: k.row*pitchY + nodeH/2, col: k.col})
	}
	// Sort by y so the trunk is drawn linearly.
	sortChildRows(rows)
	lastY := rows[len(rows)-1].y

	// Top of trunk on parent's row.
	if len(rows) == 1 && rows[0].y == pMidY {
		// Same row — flat arrow.
		grid[pMidY][trunkX] = '─'
	} else {
		grid[pMidY][trunkX] = '┬'
		// Vertical line down to last child.
		for y := pMidY + 1; y <= lastY; y++ {
			if y < len(grid) {
				grid[y][trunkX] = '│'
			}
		}
	}

	// For each child: branch off the trunk at the child's row.
	for i, cr := range rows {
		ch := cr.y
		if ch < len(grid) {
			if i == len(rows)-1 {
				grid[ch][trunkX] = '└'
			} else if ch != pMidY {
				grid[ch][trunkX] = '├'
			}
		}
		// Horizontal from trunkX+1 to child's left edge - 1.
		childLeftX := cr.col * pitchX
		for x := trunkX + 1; x < childLeftX-1; x++ {
			if x < len(grid[0]) && ch < len(grid) {
				grid[ch][x] = '─'
			}
		}
		if childLeftX-1 >= 0 && childLeftX-1 < len(grid[0]) && ch < len(grid) {
			grid[ch][childLeftX-1] = '→'
		}
	}
}

// sortChildRows performs a tiny in-place sort by y ascending. Kept local
// to avoid the sort import here (already imported elsewhere — but the
// trivial implementation is clearer than wiring sort.Slice).
func sortChildRows(rows []childRow) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rows[j-1].y > rows[j].y; j-- {
			rows[j-1], rows[j] = rows[j], rows[j-1]
		}
	}
}

// renderSelectedSpanCompact shows the currently-selected span's key
// attributes in the right pane of the graph view.
func (d *DAGTab) renderSelectedSpanCompact(width int) string {
	if width <= 0 || d.nodeCursor >= len(d.flatNodes) {
		return ""
	}
	s := d.flatNodes[d.nodeCursor].Span
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	labelStyle := lipgloss.NewStyle().Foreground(colorDim)
	valStyle := lipgloss.NewStyle().Foreground(colorText)

	rows := [][2]string{
		{"span", shortID(s.SpanID)},
		{"parent", shortID(s.ParentSpanID)},
		{"system", s.System},
		{"model", s.Model},
		{"status", s.Status},
		{"tokens", fmt.Sprintf("%d in / %d out", s.InputTokens, s.OutputTokens)},
	}
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Selected"))
	sb.WriteString("\n")
	for _, r := range rows {
		if r[1] == "" {
			continue
		}
		sb.WriteString(fmt.Sprintf("%s %s\n",
			labelStyle.Render(fmt.Sprintf("%-7s", r[0])),
			valStyle.Render(r[1])))
	}
	return sb.String()
}

// renderSpanFull renders all known fields and attributes of a span.
func (d *DAGTab) renderSpanFull(s telemetry.Span) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	labelStyle := lipgloss.NewStyle().Foreground(colorDim)
	valStyle := lipgloss.NewStyle().Foreground(colorText)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Identity"))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("  trace_id "), valStyle.Render(s.TraceID)))
	sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("  span_id  "), valStyle.Render(s.SpanID)))
	sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("  parent   "), valStyle.Render(s.ParentSpanID)))
	sb.WriteString("\n")

	sb.WriteString(titleStyle.Render("GenAI"))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("  system   "), valStyle.Render(s.System)))
	sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("  model    "), valStyle.Render(s.Model)))
	sb.WriteString(fmt.Sprintf("%s %d / %d\n", labelStyle.Render("  tokens   "), s.InputTokens, s.OutputTokens))
	sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("  status   "), valStyle.Render(s.Status)))
	sb.WriteString("\n")

	sb.WriteString(titleStyle.Render("Timing"))
	sb.WriteString("\n")
	if !s.StartTime.IsZero() {
		sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("  start    "), valStyle.Render(s.StartTime.Local().Format(time.RFC3339))))
	}
	if !s.EndTime.IsZero() {
		sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("  end      "), valStyle.Render(s.EndTime.Local().Format(time.RFC3339))))
		if d := s.Duration(); d > 0 {
			sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("  duration "), valStyle.Render(d.String())))
		}
	}
	sb.WriteString("\n")

	sb.WriteString(titleStyle.Render("Attributes"))
	sb.WriteString("\n")
	keys := make([]string, 0, len(s.Attrs))
	for k := range s.Attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		sb.WriteString(fmt.Sprintf("  %s %s\n",
			labelStyle.Render(fmt.Sprintf("%-28s", k)),
			valStyle.Render(s.Attrs[k])))
	}
	return sb.String()
}

type helpKey struct{ k, d string }

func (d *DAGTab) renderHelp(keys []helpKey) string {
	keyStyle := lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(colorDim)
	var parts []string
	for _, kp := range keys {
		parts = append(parts, keyStyle.Render("<"+kp.k+">")+" "+descStyle.Render(kp.d))
	}
	return strings.Join(parts, "  ")
}

func (d *DAGTab) renderFlash() string {
	if d.flash == "" || time.Now().After(d.flashUntil) {
		return ""
	}
	style := lipgloss.NewStyle().Foreground(colorSuccess).Italic(true)
	return "\n" + style.Render(d.flash)
}

func statusIcon(status string) string {
	switch status {
	case "running":
		return lipgloss.NewStyle().Foreground(colorWarning).Render("▶ running")
	case "error":
		return lipgloss.NewStyle().Foreground(colorError).Render("✗ error")
	default:
		return lipgloss.NewStyle().Foreground(colorSuccess).Render("✓ done")
	}
}

func shortID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:8] + "…"
}

// treeStats counts spans + max depth in a tree.
func treeStats(node *telemetry.SpanNode) (count, depth int) {
	if node == nil {
		return 0, 0
	}
	count = 1
	maxChild := 0
	for _, c := range node.Children {
		cc, cd := treeStats(c)
		count += cc
		if cd > maxChild {
			maxChild = cd
		}
	}
	depth = 1 + maxChild
	return
}

// nodeLabel returns a short, human-readable label for a span. Prefers
// semantic info (tool/operation + token totals) over the long opaque ID.
func nodeLabel(s telemetry.Span) string {
	tool := s.Attrs["gen_ai.tool.name"]
	op := s.Attrs["gen_ai.operation.name"]
	var head string
	switch {
	case tool != "":
		head = tool
	case op != "":
		head = op
	default:
		head = "span"
	}
	if s.InputTokens > 0 || s.OutputTokens > 0 {
		// Compact "1.5k/420" form so long counts fit in 20 chars.
		return fmt.Sprintf("%s %s", head, fmtTokens(s.InputTokens, s.OutputTokens))
	}
	// Append short ID as a disambiguator when no token info.
	return fmt.Sprintf("%s %s", head, shortID(s.SpanID))
}

// fmtTokens renders an input/output pair compactly (uses k suffix for ≥1000).
func fmtTokens(in, out int) string {
	return compactInt(in) + "/" + compactInt(out)
}

func compactInt(n int) string {
	if n >= 1000 {
		if n >= 100000 {
			return fmt.Sprintf("%dk", n/1000)
		}
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// truncate trims s to n runes (also used by tab_dag_test.go).
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
