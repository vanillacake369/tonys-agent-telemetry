# Roadmap

**Last updated:** 2026-05-25
**Strategy:** Control Plane First (γ) → Universal Ingest (α) → Ecosystem (ζ). See `AGENT_LOG.md` Part 3 for the full strategy comparison.

Status legend: ✅ done · 🚧 in progress · 📋 planned · 💡 candidate

---

## Phase 0 — Foundations ✅

- Bubbletea TUI scaffolding (Sessions, Agents, DAG, Skills, Hooks, Cost tabs)
- Claude Code session reader (JSONL parsing from `~/.claude/projects/`)
- Named-FIFO hook integration (`/tmp/tonys-agent-telemetry.fifo`)
- Skill marketplace prototype (local + GitHub search + recommender)
- Multi-provider scaffolding (Claude, Codex, Gemini readers)

---

## Phase 1 — Telemetry Foundations ✅

Goal: rewire the codebase so that **any** LLM provider can be plugged in as an adapter.

- **W0:** OS guardrails — `signal.Notify` for SIGTERM/SIGHUP, non-TTY detection, FIFO lifecycle cancellation, `tea.ExecProcess` for external commands
- **S1:** `internal/telemetry/` package — canonical `Span` (OTel GenAI semconv shape), `ProviderIngestor` interface, `Registry` with auto-detection, `BuildTrees` for DAG reconstruction
- **S2:** `internal/provider/claudecode/` package — Claude Code rewired as the first adapter; FIFO wire format v2 (span-shaped JSON, v1 backward-compat preserved); `cmd/hook-handler` updated

Outcome: `Ctrl+S` / `Ctrl+D` / `Ctrl+K` UX is unchanged for end users while the engine underneath is provider-agnostic.

---

## Phase 2 — Control Plane MVP 🚧 (current focus, ~3 weeks)

Goal: ship the **unique differentiator** — runtime governance of agent execution. Claude Code is the only provider this phase; universal expansion comes in Phase 3.

- Per-agent budget tracking (USD limit per session and per sub-agent)
- Kill switch — when a budget cap is hit, the next `PreToolUse` hook returns exit-code 2 (deny) with a diagnostic message
- Tool allowlist/denylist policy (e.g., never allow `Bash(rm -rf *)` regardless of agent)
- Approval queue — risky tools require interactive `y/n` in the TUI before execution
- `tab_control.go` — live budget bar, pending approvals, deny log
- `~/.config/tonys-agent-telemetry/policy.toml` — user-editable policy

Outcome: a demo where a runaway swarm hits its $5 cap and is killed mid-step, visible in the TUI.

---

## Phase 3 — Universal Ingest 📋 (~5 weeks)

Goal: become provider-agnostic in practice. Anything emitting OTel GenAI spans is auto-absorbed.

- `OTLPReceiverIngestor` on `localhost:4317` — absorbs LangGraph, CrewAI, AutoGen, OpenAI Agents SDK, Letta, smolagents, LiteLLM, OpenRouter, TGI when they emit OTel
- `VLLMIngestor` — Prometheus `/metrics` scrape on `:8000`
- `OllamaIngestor` — poll `/api/ps` and `/api/chat` on `:11434`
- Provider auto-detection (cascading: port probe → process inspect → config-path scan)
- DAG renderer (`tab_dag.go`) generalized to OTel `trace_id` / `parent_span_id` grouping
- Status bar shows which providers are currently detected

Outcome: launching the TUI with vLLM + Claude Code + a LangGraph script running produces one unified DAG across all three.

---

## Phase 4 — Ecosystem 💡 (~8 weeks)

Goal: leverage community for the long-tail adapter problem.

- **Plugin SDK** — `ProviderIngestor` implementations loadable from `~/.config/tonys-agent-telemetry/plugins/` (Go plugin, or out-of-process via gRPC)
- **MCP registry integration** — make the Skills tab MCP-aware; parse `.well-known/mcp.json`, surface MCP servers as a discoverable catalog
- **Optional proxy mode** — LiteLLM-style intercept on configurable port, for agents that have no hooks and no native OTel (legacy scripts, closed-source tools)
- **Snapshot / replay** — capture an agent run and re-execute with synthetic responses (debugging multi-agent failures)
- **Multi-host aggregation** — opt-in OTLP forward to a chosen sink (Langfuse, Honeycomb, Datadog, self-hosted Tempo)

---

## Phase 5 — Distribution 💡 (ongoing)

- Homebrew tap (`brew install vanillacake369/tap/tonys-agent-telemetry`)
- Linux package repos (`.deb`, `.rpm`, AUR)
- Static binaries via GoReleaser for every release
- Docker image for headless OTLP-export mode (`docker run -p 4317:4317 ...`)
- Nix flake remains canonical for power users

---

## Out of scope (will not do)

- Web GUI — Claudia / Opcode fill that niche
- Hosted SaaS dashboard — runs counter to the local-first principle
- Model training / evaluation — Arize Phoenix and Weights & Biases own that space
- Generic non-LLM tracing — not a Jaeger replacement

---

## Decision log

| Date | Decision | Rationale |
|---|---|---|
| 2026-05-25 | Adopt Control Plane First (γ) over Stay-the-Course (α) | Unique-feature moat established before universal scope; control plane is the only function no competitor offers locally (see `AGENT_LOG.md` Part 3) |
| 2026-05-25 | Embedded lightweight `opentelemetry-proto` over full collector embedding | 50 MB binary cost not justified yet; revisit in Phase 4 if custom processors needed (see `AGENT_LOG.md` Part 6) |
| 2026-05-25 | FIFO wire format v1/v2 dual support | Avoid breaking users who upgrade the TUI but not the hook-handler |

---

## How to contribute (planned for Phase 4)

Until Phase 4 ships the plugin SDK, contributions are most useful at the spec layer:
- File an issue with the auto-detection signal for a provider you use
- Propose attribute mappings for unusual GenAI semconv fields
- Report telemetry gaps in your workflow (we want to hear what you wish existed)
