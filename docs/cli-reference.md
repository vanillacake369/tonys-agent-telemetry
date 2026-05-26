# CLI reference

Every command-line flag and environment variable, with the default,
example, and source-of-truth pointer. The TUI key bindings sit at the
bottom.

For a 30-second "press this, see that" walkthrough, see
[`README.md#quickstart`](../README.md#quickstart).

## Commands

```sh
tonys-agent-telemetry                Launch the TUI with auto-detected providers
tonys-agent-telemetry --version      Print the binary version
tonys-agent-telemetry --help         Print usage
```

## Flags

| Flag | Description | Default |
|---|---|---|
| `--otlp-export URL` | Forward every collected span to a remote OTLP/JSON receiver (Tempo, Honeycomb, Langfuse, …) in parallel with the local TUI. Also via `TAT_OTLP_EXPORT` env. | unset |
| `--snapshot-record FILE` | Append every span to FILE as JSONL for later inspection or replay. Also via `TAT_SNAPSHOT_RECORD` env. | unset |
| `--replay FILE` | Read spans from FILE instead of starting live providers. Useful for reproducing reported issues. | unset |
| `--emit-signals` | Extract behavioural signals (`stalled_node`, `duplicate_subagent_work`, `unused_installed_skill`, `failed_handoff`) from the ingested spans, print as JSON to stdout, exit. Combine with `--replay` for offline analysis. | off |

## Environment variables

### TONYS_* (current namespace)

| Var | Purpose | Default |
|---|---|---|
| `TONYS_OTLP_BIND` | OTLP receiver bind address. Set to `0.0.0.0:4318` if you need LAN-accessible ingest (no auth — only on a trusted network). | `127.0.0.1:4318` |
| `TONYS_MAX_SPANS` | Span buffer cap in the TUI. The buffer evicts oldest-first by `EndTime` when full. | `50000` |
| `TONYS_SIGNAL_STORE` | Override the per-project signal-store directory. | `$XDG_CACHE_HOME/tonys-agent-telemetry/signals/` |
| `TONYS_CATALOG_PATH` | Override the catalog cache JSON path. | `$XDG_CACHE_HOME/tonys-agent-telemetry/catalog/items.json` |
| `TONYS_CATALOG_MIN` | Minimum viable catalog entries before the Skills tab shows the corpus pane. Below this, a "partial" warning renders instead. | `100` |
| `TONYS_LIVE_UPSTREAM` | Opt into the live-upstream catalog smoke test. CI does not set this. | unset |
| `TONYS_RUN_LIVE` | Opt into tests that hit the live GitHub API (skill registry tests). CI does not set this. | unset |

### TAT_* (kept for backward compatibility)

| Var | Purpose | Default |
|---|---|---|
| `TAT_OTLP_EXPORT` | Same as `--otlp-export`. The flag wins when both are set. | unset |
| `TAT_SNAPSHOT_RECORD` | Same as `--snapshot-record`. The flag wins. | unset |
| `TAT_DEBUG` | When set to `1`, writes diagnostic logs to `/tmp/tat-debug.log`. Useful for triaging OTLP receiver issues. | unset |

> Why the prefix split? The `TAT_*` namespace predates the `TONYS_*` namespace. Once the next breaking release lands we'll deprecate the `TAT_*` variants; until then both work.

### Standard

| Var | Purpose | Default |
|---|---|---|
| `NO_COLOR` | Disables ANSI colour. Status icons (`✓`/`▶`/`✗`) remain distinguishable. | unset |
| `$EDITOR` | Used by the Control tab's `e` keystroke to open `policy.toml`. | OS default |

## TUI key bindings

### Tab navigation

| Key | Action |
|---|---|
| `1` | Sessions |
| `2` | Skills (+ Catalog + Advisor) |
| `3` | Cost |
| `4` | Hooks |
| `5` | DAG |
| `6` | Trends |
| `Ctrl+G` | Control (governance) — **not** in the `Tab`/`Shift+Tab` cycle by design |
| `Tab` | Next numbered tab (cycles 1 → 6 → 1) |
| `Shift+Tab` | Previous numbered tab |

### Universal

| Key | Action |
|---|---|
| `Enter` | Select / confirm / open detail |
| `Esc` | Back / cancel search / close overlay |
| `j` / `↓` | Down |
| `k` / `↑` | Up |
| `h` / `←` | Left |
| `l` / `→` | Right |
| `/` | Focus search |
| `?` | Which-key help overlay |
| `r` | Refresh current tab |
| `q` | Quit |
| `Ctrl+C` | Force quit (any state) |

### Sessions tab

| Key | Action |
|---|---|
| `v` | Open detail overlay |
| `Enter` | Resume |
| `f` | Fork session |
| `y` | Copy to clipboard |
| `s` | Sort |

### Skills tab

| Key | Action |
|---|---|
| `s` | Sort |
| `o` | Open in browser |
| `Enter` | Launch the Analyze wizard (`claude` or `gemini` CLI; see [troubleshooting](./troubleshooting.md)) |

### DAG tab (in graph view)

| Key | Action |
|---|---|
| `Enter` | Open the selected trace into the graph view |
| `g` | Toggle the compact overview mode |
| `/<query>` | Highlight matching nodes |
| `n` / `N` | Next / previous search match |
| `Esc` | Back to traces list |

### Control tab (`Ctrl+G` to enter)

| Key | Action |
|---|---|
| `r` | Reload budgets and denials from disk |
| `e` | Open `policy.toml` in `$EDITOR` |
| `c` | Clear the denial log |

## Source-of-truth pointers

If this page drifts from reality, the authoritative sources are:

- Flags / env vars → `main.go::printUsage`
- TUI key bindings → `internal/tui/keymap.go::DefaultKeyMap`
- Per-tab keys → the respective `tab_*.go::handleKey*` functions
- Signal extraction defaults → `internal/signal/defaults.go`
- Catalog defaults → `internal/catalog/defaults.go`
- Signal store defaults → `internal/signalstore/defaults.go`
- Trends defaults → `internal/trends/defaults.go`
