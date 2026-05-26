# Changelog

All notable changes to `tonys-agent-telemetry` are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Release artefacts (tar.gz, .deb, .rpm, .apk, Homebrew formula, SBOM) are
generated automatically by GoReleaser on every `v*` tag push. See
[`.goreleaser.yml`](./.goreleaser.yml) for the full matrix.

## [Unreleased]

Nothing scheduled.

## [0.1.0] — 2026-05-26

First public release.

### Manual-smoke fixes (gathered in the final user-test pass)

- **Tab bar overflow (F1)**: 7-tab bar wrapped to 2 lines at 80–100 cols.
  Compacted separator + dropped per-label padding so the bar fits one line at 80×24.
- **Trends tab missing inner Panel (F2)** and **Loading state missing border (F3)**:
  every tab now renders inside a `RenderPanel` regardless of populated/loading state.
- **Trends "9 sessions on disk but empty" (F4)**: `loadTrendsCmd` now derives
  `SessionSnapshot` entries from the live span buffer, grouped by trace, bucketed
  by trace end-time. Historical sessions appear immediately on tab open.
- **DAG nested / no whole-trace view / n,N invisible (F5)**: new `g` overview
  mode shows one-line-per-span depth-first walk of the entire trace with
  status icon + duration + error suffix. Search bar surfaces `match X of Y`,
  `no matches`, and the typing-in-progress states.
- **Codex/Gemini "unknown provider" + Claude black screen (F6)**: dropped the
  fragile `localhost:4001` cli-proxy-api routing; `claude` now invokes via
  `tea.ExecProcess` (full interactive PTY), `gemini` routes via the native CLI,
  `codex` is removed (no current Codex CLI). Wizard hides models whose binary
  isn't on PATH.
- **DAG functionality unclear (F7)**: visible help banners in both traces and
  graph modes spell out the available keys.

### Added

- **Multi-provider telemetry ingest**: claudecode (JSONL), OTLP/JSON receiver
  (default `127.0.0.1:4318`), vLLM Prometheus scrape, Ollama `/api/ps` poll.
  Per-provider auto-detection; canonical `telemetry.Span` normalisation across
  all four producers.
- **Swarm DAG visualisation**: live agent orchestration tree with chain-aware
  layout, provider badges, status colours, in-graph `/`-search with `n`/`N`
  cycle, `*` highlight on matches. Dynamic resize-safe across tmux/zellij
  panes; 5000 spans render in <3ms.
- **Skill catalog** ingest from
  [`FlorianBruniaux/claude-code-ultimate-guide`](https://github.com/FlorianBruniaux/claude-code-ultimate-guide)
  pinned to a specific commit SHA; markdown parser yields 181 items (CC-BY-SA-4.0
  attribution rendered in the Skills tab).
- **Advisor pane**: evidence-backed `Recommendation` engine — every output
  carries dual `(SignalID, CatalogItemID)` citation enforced at the struct
  level via `EnforceEvidence`. Renders TraceID + "press 5 to view DAG" hint
  for navigable evidence.
- **Signal Extractor v0**: four behavioural detectors (`stalled_node`,
  `duplicate_subagent_work`, `unused_installed_skill`, `failed_handoff`)
  with deterministic, pure-function `Extract(forest, opts)` contract. Also
  exposed via `--emit-signals` CLI flag for offline analysis.
- **Trends tab**: longitudinal sparklines with `Start`/`Last`/`Δ vs avg`
  per signal type. Backed by a JSONL signal store with flock concurrency
  + automatic rotation at 64 MiB. Per-provider fidelity tier legend
  (claudecode/otlp = full, vllm = aggregate, ollama = presence).
- **Control tab**: read-only visualisation of `policy.toml`, budget caps,
  and the denial log.
- **CLI flags**: `--otlp-export`, `--snapshot-record`, `--replay`,
  `--emit-signals`, `--version`, `--help`.
- **Environment**: `TONYS_OTLP_BIND`, `TONYS_MAX_SPANS`, `TONYS_SIGNAL_STORE`,
  `TONYS_CATALOG_PATH`, `TONYS_CATALOG_MIN`, `NO_COLOR`.

### Distribution

- macOS (amd64, arm64) and Linux (amd64, arm64) binaries via GitHub Releases.
- `.deb`, `.rpm`, `.apk` Linux packages.
- Homebrew tap (`vanillacake369/homebrew-tap`).
- Nix flake (`nix run github:vanillacake369/tonys-agent-telemetry`).
- SBOM (syft format) attached to each release.

### Security

- OTLP receiver defaults to `127.0.0.1:4318` (opt-in to LAN via `TONYS_OTLP_BIND`).
- All four provider ingest goroutines wrapped with `provider.RecoverIngest` —
  a malformed input cannot crash the process.
- Pure-Go builds (`CGO_ENABLED=0`) across every distribution channel.

### Known limitations

- Hook-handler bridge binary (previously `tonys-agent-telemetry-hook`) is
  removed. The Control tab is visualisation-only; runtime tool-call
  enforcement against Claude Code is no longer wired through this binary.
- vLLM and Ollama produce aggregate/presence-tier data only — per-call
  parent linkage and token counts require opt-in OTel SDK integration on
  the upstream side.
- `skill.Skill.Name` matching against `gen_ai.tool.name` is case-sensitive;
  case mismatches silently suppress the `unused_installed_skill` signal
  (documented in `internal/tui/skill_name_matching_test.go`).
