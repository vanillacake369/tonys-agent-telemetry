# Changelog

All notable changes to `tonys-agent-telemetry` are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Release artefacts (tar.gz, .deb, .rpm, .apk, Homebrew formula, SBOM) are
generated automatically by GoReleaser on every `v*` tag push. See
[`.goreleaser.yml`](./.goreleaser.yml) for the full matrix.

## [Unreleased]

Nothing scheduled.

## [0.1.0] — TBD

First public release.

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
