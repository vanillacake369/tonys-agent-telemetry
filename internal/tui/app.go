package tui

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/event"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/signalstore"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/trends"
)

// autoRefreshInterval is the period between automatic data refreshes.
const autoRefreshInterval = 30 * time.Second

// AutoRefreshMsg triggers a background refresh of session/cost data.
type AutoRefreshMsg struct{}

// Tab represents the active tab in the TUI.
type Tab int

const (
	TabSessions Tab = iota
	TabSkills
	TabCost
	TabHooks
	TabDAG
	TabControl
	TabTrends
)

// tabNames maps each Tab constant to its display label.
var tabNames = map[Tab]string{
	TabSessions: "Sessions",
	TabSkills:   "Skills",
	TabCost:     "Cost",
	TabHooks:    "Hooks",
	TabDAG:      "DAG",
	TabControl:  "Control",
	TabTrends:   "Trends",
}

// tabOrder defines the left-to-right display order of tabs.
// SSoT: cycleTab and renderTabBar both derive from this slice so adding a new
// tab requires editing one place, not three.
var tabOrder = []Tab{TabSessions, TabSkills, TabCost, TabHooks, TabDAG, TabControl, TabTrends}

// cycleTab returns the tab `delta` steps from `current` in tabOrder, wrapping.
// delta = +1 advances, delta = -1 goes back. Used by NextTab/PrevTab handlers.
func cycleTab(current Tab, delta int) Tab {
	idx := 0
	for i, t := range tabOrder {
		if t == current {
			idx = i
			break
		}
	}
	n := len(tabOrder)
	next := (idx + delta + n) % n
	return tabOrder[next]
}

// TabModel is the interface that every tab sub-model must implement.
// SetSize returns the updated model (value-receiver implementations must
// return their updated copy so the caller can store it).
type TabModel interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (TabModel, tea.Cmd)
	View() string
	SetSize(width, height int) TabModel
}

// SearchFocusMsg is sent to a tab to tell it to focus its search input.
type SearchFocusMsg struct{}

// SearchBlurMsg is sent to a tab to tell it to blur its search input.
type SearchBlurMsg struct{}

// App is the root Bubble Tea model managing tab switching.
type App struct {
	activeTab     Tab
	tabs          map[Tab]TabModel
	keys          KeyMap
	width         int
	height        int
	searchFocused bool // when true, key events pass directly to the active tab
	whichKey      WhichKeyOverlay
	detailView    *DetailView // non-nil when detail overlay is open
	fifoEvents    <-chan event.Event // nil when FIFO is not active
	fifoCtx       context.Context    // owns the lifecycle of fifo goroutines
	fifoCancel    context.CancelFunc
	// advisor is the debounced extract→recommend pipeline. It is updated
	// on every SpanBatchMsg / SpanCollectedMsg after routing to TabDAG.
	advisor *AdvisorPipeline

	// trendsPersistence drives the periodic signal extraction and persistence
	// to the signal store. Ticked by TrendsFlushTickMsg every FlushInterval.
	trendsPersistence *TrendsPersistence

	// sessionID is the project-scoped identifier used when persisting signals.
	// Derived from the working directory at App construction via ResolveSessionID().
	sessionID string

	// forestCache is shared by the advisor and trends pipelines so BuildForests
	// + Extract are called at most once per span-buffer generation.
	forestCache *ForestCache
}

const (
	tabBarHeight    = 1
	statusBarHeight = 1
	// outerBorderHeight accounts for top+bottom border lines of the outer frame.
	outerBorderHeight = 2
	// outerBorderWidth accounts for left+right border chars of the outer frame.
	outerBorderWidth = 2
)

// NewApp creates and returns an initialised App with placeholder tab models.
func NewApp() App {
	keys := DefaultKeyMap()
	tabs := map[Tab]TabModel{
		TabSessions: NewSessionsTab(),
		TabSkills:   NewSkillsTab(),
		TabCost:     NewCostTab(),
		TabHooks:    NewHooksTab(),
		TabDAG:      NewDAGTab(),
		TabControl:  NewControlTab(),
		TabTrends:   NewTrendsTab(),
	}
	store, err := signalstore.NewStore()
	if err != nil {
		// Persistence failures are non-fatal: the TUI still works, trends just
		// won't accumulate. Log to stderr and use a no-op nil store guard.
		// trendsPersistence is nil-safe via the flush guard in FlushCmd.
		store = nil
	}
	var persistence *TrendsPersistence
	if store != nil {
		persistence = NewTrendsPersistence(store)
	}
	return App{
		activeTab:         TabSessions,
		tabs:              tabs,
		keys:              keys,
		searchFocused:     false,
		advisor:           NewAdvisorPipeline(),
		trendsPersistence: persistence,
		sessionID:         ResolveSessionID(),
		forestCache:       NewForestCache(),
	}
}

func (a App) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, m := range a.tabs {
		if cmd := m.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Start listening for real-time events if the TUI FIFO exists.
	if info, err := os.Stat(event.DefaultFIFOPath); err == nil && info.Mode()&os.ModeNamedPipe != 0 {
		ctx, cancel := context.WithCancel(context.Background())
		a.fifoCtx = ctx
		a.fifoCancel = cancel
		a.fifoEvents = event.ReadFIFO(ctx)
		cmds = append(cmds, event.ListenForEvents(ctx, a.fifoEvents))
	}

	// Start auto-refresh polling.
	cmds = append(cmds, tea.Tick(autoRefreshInterval, func(time.Time) tea.Msg {
		return AutoRefreshMsg{}
	}))

	// Schedule the first periodic trends flush tick.
	if a.trendsPersistence != nil {
		cmds = append(cmds, a.trendsPersistence.NextTick())
	}

	return tea.Batch(cmds...)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// ── Detail overlay intercepts everything when open ──
	if a.detailView != nil {
		switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			a.width = msg.Width
			a.height = msg.Height
			a = a.propagateSize()
			a.detailView.SetSize(a.width, a.height)
			return a, nil
		case tea.KeyMsg:
			if msg.Type == tea.KeyCtrlC {
				return a, tea.Quit
			}
			if key.Matches(msg, a.keys.Escape) || key.Matches(msg, a.keys.Quit) {
				a.detailView = nil
				return a, nil
			}
			dv, cmd := a.detailView.Update(msg)
			a.detailView = &dv
			return a, cmd
		case DetailLoadedMsg:
			dv, cmd := a.detailView.Update(msg)
			a.detailView = &dv
			return a, cmd
		default:
			dv, cmd := a.detailView.Update(msg)
			a.detailView = &dv
			return a, cmd
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a = a.propagateSize()
		return a, nil

	case event.EventMsg:
		// Forward real-time events to the active tab and keep listening.
		updated, cmd := a.tabs[a.activeTab].Update(msg)
		a.tabs[a.activeTab] = updated
		var listenCmd tea.Cmd
		if a.fifoEvents != nil && a.fifoCtx != nil {
			listenCmd = event.ListenForEvents(a.fifoCtx, a.fifoEvents)
		}
		return a, tea.Batch(cmd, listenCmd)

	case tea.QuitMsg:
		// Cancel the FIFO context so its goroutines unblock and exit, instead of
		// leaking past process shutdown.
		if a.fifoCancel != nil {
			a.fifoCancel()
		}
		return a, nil

	case event.FIFOClosedMsg:
		// FIFO channel closed — stop subscribing.
		a.fifoEvents = nil
		return a, nil

	case AutoRefreshMsg:
		// Only refresh the active tab to avoid unnecessary work on hidden tabs.
		var cmds []tea.Cmd
		if cmd := a.tabs[a.activeTab].Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, tea.Tick(autoRefreshInterval, func(time.Time) tea.Msg {
			return AutoRefreshMsg{}
		}))
		return a, tea.Batch(cmds...)

	case OpenDetailMsg:
		dv := NewDetailView(msg.Session, a.width, a.height, msg.Query)
		a.detailView = &dv
		return a, dv.Init()

	case tea.KeyMsg:
		// Ctrl+C always quits, even when overlay or search is active.
		if msg.Type == tea.KeyCtrlC {
			return a, tea.Quit
		}

		// When the which-key overlay is visible, any keypress closes it.
		if a.whichKey.visible {
			a.whichKey.visible = false
			return a, nil
		}

		// Esc always unfocuses search, regardless of current mode.
		if key.Matches(msg, a.keys.Escape) {
			if a.searchFocused {
				a.searchFocused = false
				updated, cmd := a.tabs[a.activeTab].Update(SearchBlurMsg{})
				a.tabs[a.activeTab] = updated
				return a, cmd
			}
			return a, nil
		}

		// Tab / Shift+Tab cycle tabs regardless of search focus.
		// Driven by tabOrder so new tabs (e.g. TabTrends) are automatically reachable.
		if key.Matches(msg, a.keys.NextTab) {
			a.activeTab = cycleTab(a.activeTab, +1)
			return a, nil
		}
		if key.Matches(msg, a.keys.PrevTab) {
			a.activeTab = cycleTab(a.activeTab, -1)
			return a, nil
		}

		// When search is focused, pass all remaining keys to the active tab.
		if a.searchFocused {
			updated, cmd := a.tabs[a.activeTab].Update(msg)
			a.tabs[a.activeTab] = updated
			return a, cmd
		}

		// Navigation mode: "?" opens the which-key overlay.
		if key.Matches(msg, a.keys.Help) {
			a.whichKey.visible = true
			return a, nil
		}

		// Navigation mode: "/" focuses search.
		if key.Matches(msg, a.keys.Search) {
			a.searchFocused = true
			updated, cmd := a.tabs[a.activeTab].Update(SearchFocusMsg{})
			a.tabs[a.activeTab] = updated
			return a, cmd
		}

		// Navigation mode: number keys and Ctrl+G switch tabs.
		switch {
		case key.Matches(msg, a.keys.Tab1):
			a.activeTab = TabSessions
			return a, nil
		case key.Matches(msg, a.keys.Tab2):
			a.activeTab = TabSkills
			return a, nil
		case key.Matches(msg, a.keys.Tab3):
			a.activeTab = TabCost
			return a, nil
		case key.Matches(msg, a.keys.Tab4):
			a.activeTab = TabHooks
			return a, nil
		case key.Matches(msg, a.keys.Tab5):
			a.activeTab = TabDAG
			return a, nil
		case key.Matches(msg, a.keys.Tab6):
			a.activeTab = TabTrends
			// Trigger a trends aggregation load whenever the user navigates to
			// the Trends tab. The sessionID is a fixed constant for the TUI process;
			// a more elaborate scheme (per-project ID) is a Phase 4 follow-up.
			return a, a.loadTrendsCmd()
		case key.Matches(msg, a.keys.TabControl):
			a.activeTab = TabControl
			return a, nil
		case key.Matches(msg, a.keys.Quit):
			return a, tea.Quit
		}

		// Delegate all other navigation-mode keys to the active tab.
		updated, cmd := a.tabs[a.activeTab].Update(msg)
		a.tabs[a.activeTab] = updated
		return a, cmd
	}

	// Route non-key messages to the specific tab that handles them.
	// CRITICAL: any tab whose Init() returns a load Cmd must have its
	// load-response message type listed here, otherwise the message lands
	// on whatever tab happens to be active (Sessions at startup) and gets
	// dropped. The 'doesn't load until refresh' bug for Hooks/Control/DAG
	// was exactly this — initial loads fired but the messages were silently
	// discarded by the unrelated active tab.
	switch msg.(type) {
	case SessionsLoadedMsg, PreviewLoadedMsg, FileChangesLoadedMsg:
		updated, cmd := a.tabs[TabSessions].Update(msg)
		a.tabs[TabSessions] = updated
		return a, cmd
	case CostLoadedMsg:
		updated, cmd := a.tabs[TabCost].Update(msg)
		a.tabs[TabCost] = updated
		return a, cmd
	case LocalSkillsLoadedMsg, GitHubSkillsLoadedMsg, SkillsSearchResultMsg, SkillReadmeMsg,
		skillsDebounceMsg, skillsGitHubDebounceMsg, AnalyzeExecuteMsg, CatalogLoadedMsg:
		updated, cmd := a.tabs[TabSkills].Update(msg)
		a.tabs[TabSkills] = updated
		return a, cmd
	case HooksLoadedMsg:
		updated, cmd := a.tabs[TabHooks].Update(msg)
		a.tabs[TabHooks] = updated
		return a, cmd
	case ControlRefreshMsg:
		updated, cmd := a.tabs[TabControl].Update(msg)
		a.tabs[TabControl] = updated
		return a, cmd
	case SpanCollectedMsg, SpanBatchMsg:
		updated, cmd := a.tabs[TabDAG].Update(msg)
		a.tabs[TabDAG] = updated
		// After updating the DAG tab, attempt to trigger the advisor pipeline.
		// MaybeRun returns nil if debounce or span-delta conditions are not met.
		// Contract: SpanProvider interface (see span_provider.go); compile-time
		// assertion in that file guarantees *DAGTab satisfies it.
		dagTab := a.tabs[TabDAG].(SpanProvider)
		skillsTab := a.tabs[TabSkills].(SkillsTab)
		installedNames := skillsTab.LocalSkillNames()
		if advisorCmd := a.advisor.MaybeRun(dagTab.Spans(), skillsTab.CatalogItems(), installedNames, a.forestCache); advisorCmd != nil {
			return a, tea.Batch(cmd, advisorCmd)
		}
		return a, cmd
	case RecommendationsReadyMsg:
		updated, cmd := a.tabs[TabSkills].Update(msg)
		a.tabs[TabSkills] = updated
		return a, cmd
	case TrendsLoadedMsg:
		updated, cmd := a.tabs[TabTrends].Update(msg)
		a.tabs[TabTrends] = updated
		return a, cmd
	case TrendsFlushTickMsg:
		// Flush current spans to signal store, then re-schedule the next tick.
		// Contract: SpanProvider interface (see span_provider.go).
		var cmds []tea.Cmd
		if a.trendsPersistence != nil {
			dagTab := a.tabs[TabDAG].(SpanProvider)
			if flushCmd := a.trendsPersistence.FlushCmd(a.sessionID, dagTab.Spans(), a.forestCache); flushCmd != nil {
				cmds = append(cmds, flushCmd)
			}
			cmds = append(cmds, a.trendsPersistence.NextTick())
		}
		return a, tea.Batch(cmds...)
	default:
		// Unknown messages go to active tab only.
		updated, cmd := a.tabs[a.activeTab].Update(msg)
		a.tabs[a.activeTab] = updated
		return a, cmd
	}
}

func (a App) View() string {
	if a.width == 0 {
		// Not yet sized — render a minimal placeholder to avoid blank screen.
		return renderTabBar(a.activeTab, 80) + "\n" + a.tabs[a.activeTab].View()
	}

	// Inner width/height: subtract outer border chars.
	innerW := max(0, a.width-outerBorderWidth)

	tabBar := renderTabBar(a.activeTab, innerW)
	content := ContentStyle.
		Width(innerW).
		Height(a.contentHeight()).
		Render(a.tabs[a.activeTab].View())
	statusBar := a.renderStatusBar(innerW)

	inner := strings.Join([]string{tabBar, content, statusBar}, "\n")

	// Switch outer border color based on mode: primary (bright) in SEARCH mode.
	outerStyle := OuterBorderStyle
	if a.searchFocused {
		outerStyle = outerStyle.BorderForeground(colorPrimary)
	}

	full := outerStyle.
		Width(innerW).
		Render(inner)

	// Render the detail overlay on top of everything.
	if a.detailView != nil {
		return a.detailView.View()
	}

	// Render the which-key overlay centered on top of the full view.
	if a.whichKey.visible {
		a.whichKey.width = a.width
		a.whichKey.height = a.height
		overlay := a.whichKey.View()
		return lipgloss.Place(
			a.width, a.height,
			lipgloss.Center, lipgloss.Center,
			overlay,
			lipgloss.WithWhitespaceForeground(colorDim),
		)
	}

	return full
}

// contentHeight returns the number of rows available for tab content.
// It subtracts the tab bar, status bar, and the two outer border rows.
func (a App) contentHeight() int {
	h := a.height - tabBarHeight - statusBarHeight - outerBorderHeight
	if h < 0 {
		return 0
	}
	return h
}

// propagateSize distributes the current terminal dimensions to every tab model.
func (a App) propagateSize() App {
	cw := max(0, a.width-outerBorderWidth)
	ch := a.contentHeight()
	for tab, m := range a.tabs {
		a.tabs[tab] = m.SetSize(cw, ch)
	}
	return a
}

// loadTrendsCmd returns a tea.Cmd that reads the signal store, aggregates
// the last DefaultLookbackDays of buckets, and sends TrendsLoadedMsg.
// If the store was not initialised (persistence unavailable), it returns an
// empty TrendsLoadedMsg so the tab renders its "not enough data" empty state.
func (a App) loadTrendsCmd() tea.Cmd {
	store := a.trendsPersistenceStore()
	return func() tea.Msg {
		if store == nil {
			return TrendsLoadedMsg{Buckets: nil}
		}
		to := trends.RoundedNow()
		from := to.Add(-time.Duration(trends.DefaultLookbackDays) * 24 * time.Hour)
		sessions, err := store.LoadRange(from, to)
		if err != nil {
			return TrendsLoadedMsg{Buckets: nil}
		}
		buckets := trends.Aggregate(sessions, from, to, trends.DefaultBucketDuration)
		return TrendsLoadedMsg{Buckets: buckets}
	}
}

// trendsPersistenceStore returns the underlying *signalstore.Store if persistence
// is configured, or nil otherwise. This is the single access point so tests can
// override the App struct and get nil-safe behaviour.
func (a App) trendsPersistenceStore() *signalstore.Store {
	if a.trendsPersistence == nil {
		return nil
	}
	return a.trendsPersistence.store
}

// tabHints returns the context-sensitive hint string for the active tab.
func (a App) tabHints() string {
	switch a.activeTab {
	case TabSessions:
		return "v:view  ↵:resume  f:fork  y:copy  r:refresh"
	case TabSkills:
		return "↵:analyze  o:open  s:sort  y:copy  r:refresh"
	case TabCost:
		return "r:refresh"
	case TabHooks:
		return "r:refresh"
	case TabDAG:
		return "↑↓:scroll  pgup/pgdn:page"
	case TabControl:
		return "r:refresh  e:edit policy  c:clear denials"
	case TabTrends:
		return "sparkline:signal counts  lookback:30d"
	}
	return ""
}

// renderTabBar returns the tab bar string for the given active tab and total width.
// Uses k9s/btop-style numbered tabs: "1:Sessions │ 2:Skills │ 3:Cost │ 4:Hooks │ 5:DAG │ ^G:Control"
func renderTabBar(active Tab, width int) string {
	tabDefs := []struct {
		num string
		tab Tab
	}{
		{"1", TabSessions},
		{"2", TabSkills},
		{"3", TabCost},
		{"4", TabHooks},
		{"5", TabDAG},
		{"6", TabTrends},
		{"^G", TabControl},
	}

	var parts []string
	for i, td := range tabDefs {
		label := td.num + ":" + tabNames[td.tab]
		var rendered string
		if td.tab == active {
			// Prepend a dot indicator to the active tab.
			rendered = ActiveTabStyle.Render("● " + label)
		} else {
			// Pad with spaces to align with the dot indicator.
			rendered = InactiveTabStyle.Render("  " + label)
		}
		if i < len(tabDefs)-1 {
			rendered += TabSeparatorStyle.Render(tabSeparator)
		}
		parts = append(parts, rendered)
	}
	bar := strings.Join(parts, "")
	return TabBarStyle.Width(width).Render(bar)
}

// renderStatusBar returns a single-line status bar showing mode indicator and key hints.
// Format (normal): NORMAL │ 1:sessions 2:skills 3:cost │ <tab hints> │ /:search ?:help q:quit
// Format (search): SEARCH │ type to filter │ esc:back
func (a App) renderStatusBar(width int) string {
	innerWidth := max(0, width-StatusBarStyle.GetHorizontalPadding())

	var help string

	if a.searchFocused {
		// Search mode: bold label with background + hints.
		modeStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1A1A2E")).
			Background(colorPrimary).
			Padding(0, 1)
		mode := modeStyle.Render("SEARCH")
		help = mode + " type to filter │ esc:back"
	} else {
		// Normal mode: responsive status bar — omit sections as width shrinks.
		modeStyle := lipgloss.NewStyle().
			Foreground(colorDim).
			Padding(0, 1)
		mode := modeStyle.Render("NORMAL")
		tabSpecific := a.tabHints()

		// Build from right to left, dropping sections if they don't fit.
		// Priority: mode > tab hints > global hints > tab numbers
		globalHint := "/ search  ? help  q quit"
		tabNums := "1-3:tabs"

		// Try full version first
		var parts []string
		parts = append(parts, mode)
		if innerWidth > 100 {
			parts = append(parts, tabNums)
		}
		if tabSpecific != "" {
			parts = append(parts, tabSpecific)
		}
		if innerWidth > 60 {
			parts = append(parts, globalHint)
		}

		help = strings.Join(parts, " │ ")
	}

	return StatusBarStyle.Width(width).Render(
		lipgloss.PlaceHorizontal(innerWidth, lipgloss.Left, help),
	)
}
