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

// SetSize implements TabModel.
func (d *DAGTab) SetSize(width, height int) TabModel {
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
func (d *DAGTab) flattenTrace(traceID string) []*telemetry.SpanNode {
	traceSpans := make([]telemetry.Span, 0)
	for _, s := range d.spans {
		if s.TraceID == traceID {
			traceSpans = append(traceSpans, s)
		}
	}
	trees := telemetry.BuildTrees(traceSpans)
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
	for _, root := range trees {
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
func (d *DAGTab) renderGraphView() string {
	if len(d.flatNodes) == 0 {
		return RenderEmptyState("(no spans in this trace)", d.width, d.height)
	}

	leftW := d.width * 3 / 5
	if leftW < 30 {
		leftW = max(20, d.width-2)
	}
	rightW := max(0, d.width-leftW-3)

	leftPanel := d.renderGraph(leftW)
	rightPanel := d.renderSelectedSpanCompact(rightW)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, "  ", rightPanel)

	help := d.renderHelp([]helpKey{
		{"j/k", "select node"}, {"enter/l", "detail"}, {"y", "yank JSON"},
		{"e", "edit copy"}, {"esc/h", "back to traces"},
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
// by L-shaped elbow connectors. Cols = depth, rows = pre-order traversal
// position. The selected node is marked with a cursor arrow next to its
// box.
//
// This replaces the previous nested-indent ASCII tree which users said
// looked like a "nested bullet point list" rather than an n8n/airflow DAG.
func (d *DAGTab) renderGraph(width int) string {
	if len(d.flatNodes) == 0 {
		return ""
	}

	// Layout: assign (col, row) to each node. col = depth, row = visit order.
	// flatNodes is already in pre-order from flattenTrace, so we can use the
	// index directly as the row.

	// Compute depth per node (= longest path from any root).
	idSet := map[string]bool{}
	for _, n := range d.flatNodes {
		idSet[n.Span.SpanID] = true
	}
	depthByID := map[string]int{}
	var setDepth func(*telemetry.SpanNode, int)
	setDepth = func(n *telemetry.SpanNode, depth int) {
		if n == nil {
			return
		}
		if existing, ok := depthByID[n.Span.SpanID]; !ok || depth > existing {
			depthByID[n.Span.SpanID] = depth
		}
		for _, c := range n.Children {
			setDepth(c, depth+1)
		}
	}
	for _, n := range d.flatNodes {
		if n.Span.ParentSpanID == "" || !idSet[n.Span.ParentSpanID] {
			setDepth(n, 0)
		}
	}

	positions := make([]gridPos, len(d.flatNodes))
	posByID := make(map[string]int, len(d.flatNodes))
	maxCol := 0
	for i, n := range d.flatNodes {
		positions[i] = gridPos{
			col:  depthByID[n.Span.SpanID],
			row:  i,
			node: n,
		}
		posByID[n.Span.SpanID] = i
		if positions[i].col > maxCol {
			maxCol = positions[i].col
		}
	}

	const (
		nodeW = 16 // chars per box including borders
		nodeH = 3  // lines per box
		hgap  = 6  // chars between columns (room for elbow)
		vgap  = 1  // blank lines between rows
	)
	pitchX := nodeW + hgap
	pitchY := nodeH + vgap

	gridW := (maxCol+1)*pitchX - hgap
	gridH := len(positions)*pitchY - vgap
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

	// Draw connectors for each parent → child relation. We do one
	// connector group per parent so a parent with multiple children gets
	// a proper trunk with ┬ at the top and ├/└ at each branch off.
	for _, p := range positions {
		if len(p.node.Children) == 0 {
			continue
		}
		// Filter to children we actually have positions for (handles
		// cross-trace pointers if any slip through).
		var kids []gridPos
		for _, c := range p.node.Children {
			if idx, ok := posByID[c.Span.SpanID]; ok {
				kids = append(kids, positions[idx])
			}
		}
		if len(kids) == 0 {
			continue
		}
		drawConnectorGroup(grid, p, kids, nodeW, nodeH, pitchX, pitchY)
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

	var sb strings.Builder
	for y, row := range grid {
		// Determine if this row is part of the selected node's box.
		selectedRow := false
		if d.nodeCursor < len(positions) {
			sp := positions[d.nodeCursor]
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
		if selectedRow && d.nodeCursor < len(positions) {
			sp := positions[d.nodeCursor]
			midY := sp.row*pitchY + nodeH/2
			if y == midY {
				line = cursorStyle.Render("▶ ") + line
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
	return sb.String()
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

// nodeLabel returns a short label for a span.
func nodeLabel(s telemetry.Span) string {
	tool := s.Attrs["gen_ai.tool.name"]
	op := s.Attrs["gen_ai.operation.name"]
	tokens := ""
	if s.InputTokens > 0 || s.OutputTokens > 0 {
		tokens = fmt.Sprintf("  [%d/%d]", s.InputTokens, s.OutputTokens)
	}
	id := shortID(s.SpanID)
	if tool != "" {
		return fmt.Sprintf("%s  %s%s", id, tool, tokens)
	}
	if op != "" {
		return fmt.Sprintf("%s  %s%s", id, op, tokens)
	}
	return id + tokens
}

// truncate trims s to n runes (also used by tab_dag_test.go).
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
