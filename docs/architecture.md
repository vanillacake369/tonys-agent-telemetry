# Architecture

A walkthrough of the internal packages and how data flows from a
provider on the host all the way to the Advisor pane in the Skills tab
or a JSON line on stdout. Read top-to-bottom for a coherent mental
model; treat individual section headings as a search index after that.

## High-level data flow

```
┌─────────────────────────────────────────────────────────────────────┐
│  Hosts emit telemetry                                                │
│    • Claude Code   — writes JSONL to ~/.claude/projects/             │
│    • LangGraph/etc.— OTLP/JSON POST to :4318                         │
│    • vLLM          — Prometheus :8000/metrics                        │
│    • Ollama        — /api/ps poll                                    │
└────────────────────────────┬────────────────────────────────────────┘
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│  internal/provider/{claudecode,otlp,vllm,ollama}                     │
│    each implements the Ingestor contract → telemetry.Span             │
│    wrapped in provider.RecoverIngest panic guard                     │
└────────────────────────────┬────────────────────────────────────────┘
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│  internal/telemetry                                                  │
│    canonical Span schema · BuildForests reconstructs parent-child    │
│    trees per TraceID                                                 │
└──────┬───────────────────────────────────────────┬──────────────────┘
       ▼                                           ▼
┌──────────────────────────┐         ┌──────────────────────────────┐
│ internal/tui span buffer │         │ internal/snapshot            │
│  (DAGTab — cap 50k)      │         │  record/replay (--snapshot   │
│                          │         │  -record · --replay)         │
└──────┬───────────────────┘         └──────────────────────────────┘
       ▼
┌─────────────────────────────────────────────────────────────────────┐
│  internal/tui/forest_cache                                           │
│    memoised BuildForests + signal.Extract per span-buffer generation │
└──────┬───────────────────────────────────────────┬──────────────────┘
       ▼                                           ▼
┌──────────────────────────┐         ┌──────────────────────────────┐
│ internal/signal          │         │ internal/signalstore         │
│  4 detectors:            │         │  JSONL store, flock,         │
│  stalled_node            │         │  schema_version, auto-rotate │
│  duplicate_subagent_work │         │  at 64 MiB                   │
│  unused_installed_skill  │         └──────────┬───────────────────┘
│  failed_handoff          │                    ▼
└──────┬───────────────────┘         ┌──────────────────────────────┐
       │                             │ internal/trends              │
       │                             │  BuildLiveSnapshots merges   │
       │                             │  store + live spans →        │
       │                             │  Aggregate → []Bucket        │
       │                             └──────────┬───────────────────┘
       ▼                                        ▼
┌──────────────────────────┐         ┌──────────────────────────────┐
│ internal/recommender     │         │ TUI Trends tab               │
│  + internal/catalog      │         │  sparkline · Start/Last/Δ    │
│  Engine.Recommend →      │         │  fidelity tier legend        │
│  []Recommendation w/     │         └──────────────────────────────┘
│  dual citation           │
└──────┬───────────────────┘
       ▼
┌──────────────────────────┐
│ TUI Skills/Advisor pane  │
│  (with EnforceEvidence)  │
└──────────────────────────┘
```

## Package responsibilities

### Ingest layer

| Package | Responsibility |
|---|---|
| `internal/provider` | Shared `Ingestor` interface + `RecoverIngest` goroutine panic guard used by every provider |
| `internal/provider/claudecode` | JSONL backfill from `~/.claude/projects/` + live event ingest. Parent linkage from `parentUuid`. |
| `internal/provider/otlp` | HTTP receiver bound to `127.0.0.1:4318` (override via `TONYS_OTLP_BIND`). Promotes `gen_ai.*` attrs into canonical Span fields. TOCTOU-tested under concurrent start. |
| `internal/provider/vllm` | Prometheus `:8000/metrics` scrape. Aggregate tier — no per-call parent linkage (would require vLLM's OTel SDK opt-in). |
| `internal/provider/ollama` | `/api/ps` poll. Presence tier — no token counts (the endpoint reports only loaded models). |
| `internal/telemetry` | Canonical `Span` struct. `BuildForests` constructs parent-child trees per TraceID. |

### Domain / pure-function core

| Package | Responsibility |
|---|---|
| `internal/signal` | `Extract(forest, opts) []Signal` — pure function. Four detectors: `stalled_node`, `duplicate_subagent_work` (rolling hash O(K·M + K²·T)), `unused_installed_skill`, `failed_handoff`. Deterministic Signal.ID hash. |
| `internal/catalog` | Best-practice corpus ingest. Markdown parser via stdlib `regexp` + `bufio.Scanner` (no goldmark dep). Pinned to a specific upstream Git commit SHA, license verified (CC-BY-SA-4.0). |
| `internal/recommender` | `Engine.Recommend(signals, items) []Recommendation`. Mapping table (signal type → catalog tags) is the SSoT. Jaccard scoring + MaturityLevel boost. `policy.EnforceEvidence` rejects any output missing SignalID or CatalogItemID. |
| `internal/signalstore` | JSONL append-only persistence. Header with `schema_version`. flock-protected per-session files. Auto-rotation at 64 MiB. `LoadRange` skips schema-mismatched files non-fatally. |
| `internal/trends` | Per-bucket aggregation over `SessionSnapshot`. `BuildLiveSnapshots` derives snapshots from a live span buffer for instant cold-start (no waiting for the 5-minute flush). |
| `internal/control` | Policy TOML loading (fail-open), budget store, denial JSONL log. Read-only in v0.1.0 — no runtime enforcement against Claude Code in this release. |
| `internal/snapshot` | `--snapshot-record` / `--replay` for offline JSONL capture and TUI playback. |
| `internal/skill` | Local `~/.claude/skills/` scan + GitHub fetch with disk cache. |
| `internal/data` | Session/cost/agent data loading (legacy JSONL parser). Read-only from `internal/tui/`. |
| `internal/event` | Real-time event types + FIFO consumer skeleton. Read-side only since the hook-handler binary was removed. |
| `internal/platform` | OS detection, clipboard, terminal multiplexer detection. |

### TUI layer

| Package | Responsibility |
|---|---|
| `internal/tui` | Bubbletea root `App` model with 7 tabs (Sessions, Skills, Cost, Hooks, DAG, Trends, Control). `TabModel` interface enforces a per-view height contract; `clipContentToHeight` is the safety net. `ForestCache` shared by `AdvisorPipeline` and `TrendsPersistence` to avoid duplicate BuildForests calls. |

## Notable contracts

### TabModel height contract (`internal/tui/app.go`)

```go
type TabModel interface {
    Init() tea.Cmd
    Update(msg tea.Msg) (TabModel, tea.Cmd)
    View() string
    SetSize(width, height int) TabModel
}
```

`View()` MUST return a string whose visible row count does not exceed
the `height` most recently passed to `SetSize`. `App.View` hard-clips
the bottom rows if the tab overflows — the safety net keeps the tab bar
and outer border visible at all costs, but tabs SHOULD self-budget so
nothing important is lost. The user-reported "tab bar disappears when I
open Skills" bug was a contract violation; we now have a multi-size
visual smoke test as the regression guard.

### Recommendation evidence contract (`internal/recommender/types.go`)

```go
type Recommendation struct {
    SignalID      string   // citation: which signal triggered this
    TraceID       string   // for "press 5 to view DAG" affordance
    CatalogItemID string   // citation: which catalog item is recommended
    Title         string
    Reasoning     string
    Score         float64
    CreatedAt     time.Time
}
```

Both `SignalID` and `CatalogItemID` are mandatory. `policy.EnforceEvidence`
walks the slice and returns an error if either is empty.
`Engine.Recommend` calls EnforceEvidence on its own output before
returning — if it ever fires, it's a programmer error inside the engine,
not a runtime condition.

### Signal extraction determinism (`internal/signal/extractor.go`)

`Extract` is a pure function: same input → same output, byte-identical.
`EmittedAt` is the single time read per call so it can be captured
deterministically by tests. `Signal.ID` is a SHA-256 prefix of
`(Type, TraceID, sorted SpanIDs)` so re-extracting from the same forest
produces stable IDs for snapshot/dedup logic.

### Multi-provider fidelity tier

Per provider, what Phase 3 longitudinal features can produce:

| Provider | Tier | What works |
|---|---|---|
| claudecode | **Full** | Per-call timestamps, token counts, parent-child, longitudinal deltas |
| otlp (with caller's OTel SDK) | **Full** | Same as claudecode when caller emits proper `gen_ai.*` + `parent_span_id` |
| vllm | **Aggregate** | Per-model trend lines (req/s, tokens/s, p99 latency over time). NOT per-call DAG nodes. |
| ollama | **Presence** | Model-load presence timeline. NO token counts. |

The Trends tab renders this tier table as a legend so users understand
why their sparkline might be flat for a specific provider.

## Cross-cutting design rules

These show up repeatedly in the codebase and the test discipline:

- **SRP per file** — one detector per file in `internal/signal/`, one
  helper per file in `internal/tui/` (colors.go, provider_badge.go,
  dag_search.go, etc.).
- **DRY via SSoT** — colour values, threshold constants, mapping rules
  all live in one file each. `defaults.go` is the convention.
- **TDD discipline** — every new feature has the failing test written
  first. The pre-commit hook + CI lint-new gate enforce gofmt + vet +
  short tests on every commit.
- **Contract-driven** — signal.Signal, catalog.Item, recommender.Recommendation,
  trends.Bucket are all explicit types whose shape is the contract; the
  pipeline stages compose against the types, not against each other's
  packages.
- **Async-verify** — pre-commit + CI lint + race tests catch most
  regressions before they land; integration-style "press every tab at
  three terminal sizes" tests catch composition-layer bugs the unit
  suite misses.

## What this architecture does NOT include

- **No external doc site / no API reference site.** Docs are markdown
  in this repo. `internal/*` packages are internal — not a public API.
- **No plugin loading.** The OTLP receiver IS the integration interface.
  Anything that exports OTLP/JSON to `:4318` is ingested.
- **No external metrics push (Phase-4 telemetry sinks are opt-in).**
  Default behaviour is local-only.
- **No active enforcement against Claude Code in v0.1.0.** The Control
  tab is read-only. The standalone hook-handler binary that did
  enforcement in earlier prototypes was removed before v0.1.0.
