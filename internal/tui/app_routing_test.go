package tui

import (
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signalstore"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// These tests exercise App.Update's MESSAGE ROUTING — the layer that
// previous unit tests bypassed by calling tab.Update() directly. The
// "doesn't load until refresh" + "DAG never shows spans" bugs lived here
// because tabs whose load-messages weren't in the routing switch had
// those messages silently dispatched to whatever was the active tab.

// TestApp_RoutesHooksLoadedMsg verifies that HooksLoadedMsg lands at
// TabHooks regardless of active tab. Regression for the "Hooks tab empty
// until refresh" bug.
func TestApp_RoutesHooksLoadedMsg(t *testing.T) {
	a := NewApp()
	a = a.propagateSize() // ensure tabs have non-zero dims
	a.width, a.height = 80, 24
	a = a.propagateSize()

	// Active tab is Sessions at start.
	if a.activeTab != TabSessions {
		t.Fatalf("initial active = %d, want %d", a.activeTab, TabSessions)
	}

	// Inject a HooksLoadedMsg with a synthetic config.
	cfg := HooksConfig{
		Hooks: map[string][]HookGroup{
			"PreToolUse": {{Matcher: "", Hooks: []HookEntry{{Type: "command", Command: "/tmp/h.sh"}}}},
		},
	}
	updated, _ := a.Update(HooksLoadedMsg{Config: cfg, Err: nil})
	got := updated.(App)

	// The Hooks tab should now be loaded (not still in 'loading' state).
	h := got.tabs[TabHooks].(HooksTab)
	if h.loading {
		t.Error("HooksTab.loading still true after HooksLoadedMsg — routing dropped the message")
	}
	if len(h.config.Hooks) == 0 {
		t.Error("HooksTab.config not populated")
	}
}

// TestApp_RoutesControlRefreshMsg verifies ControlRefreshMsg lands at
// TabControl regardless of active tab. We assert state change (loading
// flag flipping false) which is observable behavior even with the
// value-receiver TabModel pattern.
func TestApp_RoutesControlRefreshMsg(t *testing.T) {
	a := NewApp()
	a.width, a.height = 80, 24
	a = a.propagateSize()

	// Before: Control tab is in loading state.
	before := a.tabs[TabControl].(ControlTab)
	if !before.loading {
		t.Fatal("ControlTab should start in loading state")
	}

	updated, _ := a.Update(ControlRefreshMsg{Err: nil})
	got := updated.(App)

	after := got.tabs[TabControl].(ControlTab)
	if after.loading {
		t.Error("ControlTab.loading still true after ControlRefreshMsg — routing dropped the message")
	}
}

// TestApp_RoutesSpanBatchMsg verifies SpanBatchMsg lands at TabDAG, NOT
// the active (Sessions) tab. This is the bug that hid Claude session data
// even when the backfill produced real spans.
func TestApp_RoutesSpanBatchMsg(t *testing.T) {
	a := NewApp()
	a.width, a.height = 80, 24
	a = a.propagateSize()

	// Active is Sessions; route a batch.
	batch := []telemetry.Span{
		{TraceID: "real-session", SpanID: "u1", System: "anthropic"},
		{TraceID: "real-session", SpanID: "u2", ParentSpanID: "u1", System: "anthropic"},
	}
	updated, _ := a.Update(SpanBatchMsg{Spans: batch})
	got := updated.(App)

	d := got.tabs[TabDAG].(*DAGTab)
	if len(d.spans) != 2 {
		t.Errorf("DAG tab received %d spans, want 2 — routing dropped them", len(d.spans))
	}
}

// TestApp_RoutesSpanCollectedMsg verifies single-span variant also routes.
func TestApp_RoutesSpanCollectedMsg(t *testing.T) {
	a := NewApp()
	a.width, a.height = 80, 24
	a = a.propagateSize()

	updated, _ := a.Update(SpanCollectedMsg{Span: telemetry.Span{
		TraceID: "t", SpanID: "s", System: "anthropic",
	}})
	got := updated.(App)

	d := got.tabs[TabDAG].(*DAGTab)
	if len(d.spans) != 1 {
		t.Errorf("DAG tab received %d spans, want 1", len(d.spans))
	}
}

// TestApp_HooksLoadingStartsTrueResolvesToFalse — even without routing, a
// freshly-constructed HooksTab is in loading state. This documents the
// invariant that triggered the user-visible bug: the loading flag flips
// to false ONLY when HooksLoadedMsg arrives. So if routing drops the
// message, the tab is forever "Loading…".
func TestApp_HooksLoadingStartsTrueResolvesToFalse(t *testing.T) {
	h := NewHooksTab()
	if !h.loading {
		t.Fatal("HooksTab should start in loading state")
	}
	tab, _ := h.Update(HooksLoadedMsg{Config: HooksConfig{}, Err: nil})
	hh := tab.(HooksTab)
	if hh.loading {
		t.Error("HooksTab.loading should be false after HooksLoadedMsg")
	}
}

// TestApp_RoutesTrendsFlushTickMsg_WithStore constructs an App with a real
// (temp) signal store and injects TrendsFlushTickMsg. Asserts (a) no panic
// and (b) a non-nil cmd is returned (flush cmd + next tick).
func TestApp_RoutesTrendsFlushTickMsg_WithStore(t *testing.T) {
	dir := t.TempDir()
	store := signalstore.NewStoreAt(dir)

	a := NewApp()
	a.width, a.height = 80, 24
	a = a.propagateSize()
	// Override the trendsPersistence with a temp-dir-backed store.
	a.trendsPersistence = NewTrendsPersistence(store)

	// Inject some spans into the DAG tab so the flush has something to process.
	batchMsg := SpanBatchMsg{Spans: []telemetry.Span{
		{TraceID: "flush-t", SpanID: "flush-s1", System: "anthropic",
			StartTime: time.Now().Add(-30 * time.Second), EndTime: time.Now(),
			Status: "done", Attrs: map[string]string{}},
	}}
	updated, _ := a.Update(batchMsg)
	a = updated.(App)

	// Inject TrendsFlushTickMsg.
	updated2, cmd := a.Update(TrendsFlushTickMsg{At: time.Now()})
	_ = updated2
	if cmd == nil {
		t.Error("TrendsFlushTickMsg with non-nil store must return a non-nil cmd (flush+retick)")
	}
}

// TestApp_RoutesTrendsFlushTickMsg_NilStore constructs an App with no
// trendsPersistence (nil store scenario) and injects TrendsFlushTickMsg.
// Asserts no panic and the returned cmd is nil (no-op path).
func TestApp_RoutesTrendsFlushTickMsg_NilStore(t *testing.T) {
	a := NewApp()
	a.width, a.height = 80, 24
	a = a.propagateSize()
	// Force nil persistence to simulate store-init failure.
	a.trendsPersistence = nil

	updated, cmd := a.Update(TrendsFlushTickMsg{At: time.Now()})
	_ = updated
	if cmd != nil {
		t.Errorf("TrendsFlushTickMsg with nil store should return nil cmd, got %T", cmd)
	}
}
