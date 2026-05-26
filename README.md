# tonys-agent-telemetry

TUI dashboard for Claude Code sessions, agents, DAG visualization, and skill marketplace.

![Go Version](https://img.shields.io/badge/go-1.26-blue)
![License](https://img.shields.io/badge/license-MIT-green)
![Platform](https://img.shields.io/badge/platform-linux%20%7C%20macOS-lightgrey)

<!-- TODO: add terminal screenshot -->

## Features

- **Sessions** — Fuzzy-find and resume past Claude Code sessions; fork or continue any session
- **Skills** — Local + GitHub skill search **plus** a best-practice catalog (181 entries from `FlorianBruniaux/claude-code-ultimate-guide`, CC-BY-SA-4.0) **plus** an Advisor pane that recommends skills based on signals extracted from your real session activity, with `(SignalID, CatalogItemID)` dual citation on every recommendation
- **Cost** — Aggregated cost/usage dashboard by provider, model, project
- **Hooks** — Visualize Claude Code hook configuration (`~/.claude/settings.json`) with workflow diagram
- **DAG** — Live agent orchestration graph across all detected providers (Claude Code, vLLM, Ollama, anything OTel-emitting), with provider badges, status colors, in-graph `/`-search, and tmux/zellij-safe dynamic resize
- **Trends** — Longitudinal signal sparklines (`▁▂▃▄▅▆▇█`) with per-signal-type Start/Last/Δ vs avg, fed by automatic 5-minute flushes to a local signal store (JSONL with flock + auto-rotation)
- **Control** — Runtime governance: per-session/per-day USD budget caps, tool allow/denylists, live denial log

### Auto-detected providers (Phase 3)

Launch the TUI; each provider runs only if detected:

| Provider | Detection | Telemetry |
|----------|-----------|-----------|
| **Claude Code** | `~/.claude/projects/` | Backfill from JSONL + live hook events |
| **OTLP/HTTP** | `:4318` bindable | Receives spans from LangGraph, CrewAI, AutoGen, OpenAI Agents SDK, LiteLLM, TGI, OpenRouter, Letta, smolagents, etc. |
| **vLLM** | `:8000/metrics` returns `vllm:` prefix | Prometheus scrape, per-model token deltas |
| **Ollama** | `:11434/api/tags` returns JSON with `models` | Poll `/api/ps` for loaded models |

## Installation

### Nix (recommended)

```sh
nix run github:vanillacake369/tonys-agent-telemetry
```

Or add to your flake:

```nix
inputs.tonys-agent-telemetry.url = "github:vanillacake369/tonys-agent-telemetry";
```

### Go

```sh
go install github.com/vanillacake369/tonys-agent-telemetry@latest
```

### Homebrew (future)

```sh
brew install vanillacake369/tap/tonys-agent-telemetry
```

### Binary

Download the latest release from [GitHub Releases](https://github.com/vanillacake369/tonys-agent-telemetry/releases) and extract:

```sh
tar -xzf tonys-agent-telemetry_linux_amd64.tar.gz
mv tonys-agent-telemetry /usr/local/bin/
```

## Usage

```sh
tonys-agent-telemetry          # Launch TUI
tonys-agent-telemetry --help   # Print usage
tonys-agent-telemetry --version
```

### Key Bindings

| Key          | Action                                  |
|--------------|-----------------------------------------|
| `1`          | Switch to Sessions tab                  |
| `2`          | Switch to Skills tab                    |
| `3`          | Switch to Cost tab                      |
| `4`          | Switch to Hooks tab                     |
| `5`          | Switch to DAG tab                       |
| `6`          | Switch to Trends tab                    |
| `Ctrl+G`     | Switch to Control tab (Governance)      |
| `Tab`        | Next tab                                |
| `Shift+Tab`  | Previous tab                            |
| `Enter`      | Select / confirm                        |
| `Esc`        | Back / cancel search                    |
| `r`          | Refresh current tab                     |
| `f`          | Fork session (Sessions tab)             |
| `y`          | Copy to clipboard                       |
| `s`          | Sort (Skills tab)                       |
| `o`          | Open in browser (Skills tab)            |
| `/`          | Focus search                            |
| `?`          | Which-key help overlay                  |
| `q`          | Quit                                    |

#### Control tab keys

| Key | Action                                         |
|-----|------------------------------------------------|
| `r` | Reload budgets and denials from disk           |
| `e` | Open `policy.toml` in `$EDITOR`               |
| `c` | Clear denial log                               |

## Control Plane (Phase 2)

`tonys-agent-telemetry` can enforce runtime policies on your Claude Code sessions:
- Per-session and per-day USD budget caps
- Tool allowlists/denylists (e.g., block `rm -rf` globally)
- Live observability of budget burn-down

Configure via `~/.config/tonys-agent-telemetry/policy.toml`. See [example policy](./examples/policy.toml).

When a policy violation triggers, the hook returns exit code 2 to Claude Code,
which surfaces the denial message to the model as a tool error. The agent
typically reacts by trying a different approach or asking for guidance.

Press `Ctrl+G` to view the Control tab with live budget bars and denial log.

## Telemetry sinks & replay (Phase 4)

Every span collected by the auto-detected providers can be forwarded,
recorded, or replayed via CLI flags. The producing pipeline is never
blocked: each branch has its own buffer and slow consumers drop rather
than backpressure the source.

```sh
# Forward spans to a remote OTLP/JSON receiver (Tempo, Honeycomb, Langfuse, ...).
tonys-agent-telemetry --otlp-export http://tempo:4318/v1/traces

# Record every span to a local file for later inspection.
tonys-agent-telemetry --snapshot-record /tmp/agents-2026-05-25.jsonl

# Replay a recorded session into the TUI (live providers are disabled).
tonys-agent-telemetry --replay /tmp/agents-2026-05-25.jsonl

# Extract behavioral signals from spans (stalled_node, duplicate_subagent_work,
# unused_installed_skill, failed_handoff) as JSON. Combine with --replay to
# analyse a recorded session offline.
tonys-agent-telemetry --emit-signals --replay /tmp/agents-2026-05-25.jsonl
```

Env vars `TAT_OTLP_EXPORT` and `TAT_SNAPSHOT_RECORD` provide the same
behavior without CLI flags.

### Environment variables

| Var | Purpose | Default |
|---|---|---|
| `TONYS_OTLP_BIND` | OTLP receiver bind address (opt in to LAN with `0.0.0.0:4318`) | `127.0.0.1:4318` |
| `TONYS_MAX_SPANS` | Span buffer cap in the TUI | `50000` |
| `TONYS_SIGNAL_STORE` | Override signal store directory | `$XDG_CACHE_HOME/tonys-agent-telemetry/signals/` |
| `TONYS_CATALOG_PATH` | Override catalog cache JSON path | `$XDG_CACHE_HOME/tonys-agent-telemetry/catalog/items.json` |
| `TONYS_CATALOG_MIN` | Minimum viable catalog entries before Skills tab renders the corpus pane | `100` |
| `TONYS_LIVE_UPSTREAM` | Opt in to the live-upstream catalog smoke test | unset |
| `NO_COLOR` | Standard — disables ANSI color (status still distinguishable via `✓`/`▶`/`✗`) | unset |

### Plugin SDK — the OTLP receiver IS the plugin interface

Any process that emits OTLP/JSON spans to `http://localhost:4318/v1/traces`
is automatically ingested into the same pipeline as the native providers.
No Go-plugin loading, no custom protocol — just standard OTel export.

Examples:

```sh
# Python agent using OpenLLMetry → exports to our receiver
OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://localhost:4318 \
  python my_langgraph_agent.py
```

```sh
# LiteLLM proxy → emits per-request spans for any model it routes
litellm --otel-export-url http://localhost:4318
```

This is why no LiteLLM-style proxy is built into this binary: LiteLLM
already emits OTLP and pointing its exporter here gives you the same
visibility plus LiteLLM's routing/cost-tracking on top.

## Verifying release artefacts

Every tagged release ships with:

- **Cosign keyless signatures** — each archive, Linux package, checksums file, and SBOM has a sibling `.sig` (signature) and `.pem` (Fulcio certificate).
- **SLSA L3 provenance** — `tonys-agent-telemetry.intoto.jsonl` in the release lists every artefact, the source commit, and the workflow that built it.

Verify a downloaded binary:

```sh
# 1. Cosign signature (no key needed — verifies via GH Actions OIDC identity).
cosign verify-blob \
  --certificate tonys-agent-telemetry_linux_amd64.tar.gz.pem \
  --signature   tonys-agent-telemetry_linux_amd64.tar.gz.sig \
  --certificate-identity-regexp '^https://github\.com/vanillacake369/tonys-agent-telemetry/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  tonys-agent-telemetry_linux_amd64.tar.gz

# 2. SLSA provenance.
slsa-verifier verify-artifact \
  --provenance-path tonys-agent-telemetry.intoto.jsonl \
  --source-uri github.com/vanillacake369/tonys-agent-telemetry \
  tonys-agent-telemetry_linux_amd64.tar.gz
```

## For contributors

```sh
# One-time hook install per clone — runs gofmt + vet + short tests on
# every commit and enforces Conventional Commits subject lines.
make hooks-install

# Local pre-PR checks (matches what CI gates on).
make ci
```

## Claude Code integration

`tonys-agent-telemetry` reads Claude Code activity directly from the JSONL files under `~/.claude/projects/` — no hook installation required. Provider auto-detection (see Features) picks up sessions as they accumulate; the DAG / Sessions / Skills tabs reflect them on the next refresh.

Live OTLP-style ingest from other agent runtimes (LangGraph, CrewAI, LiteLLM, vLLM, etc.) is supported via the OTLP receiver on `127.0.0.1:4318` (configurable with `TONYS_OTLP_BIND`). See [Telemetry sinks & replay](#telemetry-sinks--replay-phase-4).

> Note: the standalone `tonys-agent-telemetry-hook` binary used in earlier versions to bridge Claude Code hooks into a FIFO has been removed. The Control tab now reads policy state and budgets from disk for visualisation; runtime tool-call enforcement against Claude Code is no longer wired through this binary.

## Architecture

```
.
├── main.go                    # TUI entry point + CLI flags (--emit-signals, --replay, ...)
└── internal/
    ├── catalog/               # Best-practice corpus ingest (markdown parser + cache + fetcher)
    ├── control/               # Policy loading, budget store, denial log, decision engine
    ├── data/                  # Session/agent data loading (JSONL parser, models)
    ├── event/                 # Real-time event types + FIFO consumer (read-side)
    ├── platform/              # OS detection, clipboard, terminal utilities
    ├── provider/              # Multi-provider ingest: claudecode / otlp / vllm / ollama
    ├── recommender/           # Evidence-backed Recommendation engine (mapping + scoring)
    ├── signal/                # Signal Extractor v0 (4 detectors against telemetry forest)
    ├── signalstore/           # JSONL signal persistence with flock + rotation
    ├── skill/                 # Local skill scan + GitHub fetch + cache
    ├── snapshot/              # Span snapshot record/replay
    ├── telemetry/             # Canonical Span + Forest builders
    ├── trends/                # Time-series Bucket aggregation
    └── tui/                   # Bubbletea TUI: app, 7 tabs, DAG renderer, advisor/trends wiring
```

### Key packages

- `internal/control` — policy TOML loading (fail-open), budget accumulation with flock, denial JSONL log, decision engine
- `internal/data` — reads `~/.claude/projects/**/*.jsonl` session files and agent metadata
- `internal/event` — non-blocking FIFO write with timeout; silent no-op when TUI is not running
- `internal/skill` — local skill scanner + GitHub API fetcher with disk cache
- `internal/tui` — five-tab Bubbletea application; DAG renderer for agent orchestration graphs

## Requirements

- Go 1.26+ (for building from source)
- `gh` (optional) — used by the Skills tab for authenticated GitHub API calls
- `fzf` (optional) — enhanced fuzzy search in Sessions tab

## License

MIT — see [LICENSE](LICENSE).
