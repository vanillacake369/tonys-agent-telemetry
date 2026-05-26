# FAQ

Quick answers. For step-by-step recipes see
[`troubleshooting.md`](./troubleshooting.md); for the CLI surface see
[`cli-reference.md`](./cli-reference.md).

## What is this?

A terminal UI that watches local AI-agent activity (Claude Code, OTLP
emitters, vLLM, Ollama), reconstructs the orchestration as a DAG,
extracts behavioural signals from it, and recommends best-practice
skills from a curated corpus. Plus longitudinal sparklines and a
read-only policy/budget viewer.

It's a single static Go binary. No daemon, no database, no SaaS, no
API key.

## Who is it for?

Developers running AI agents on their own machine who want one place
to:
- See what their agents actually did (the DAG).
- Catch wasted effort (signals like duplicate subagent work or
  stalled tool calls).
- Get evidence-backed pointers to skills/patterns that would have
  helped.
- Watch cost burn-down across providers.

It is **not** a multi-tenant observability platform. If you want
SaaS-grade trace storage with retention policies and access control,
look at Langfuse / Phoenix / Helicone — and feel free to use this in
parallel by setting `--otlp-export <their-endpoint>`.

## How is this different from cass / claude-monitor / Phoenix / Langfuse?

| | cass | claude-monitor | Phoenix / Langfuse | tonys-agent-telemetry |
|---|---|---|---|---|
| Local-first | ✓ | ✓ | varies | ✓ |
| Multi-provider | partial | Claude only | varies | **claudecode + OTLP + vllm + ollama** |
| Swarm DAG view in terminal | flat text | — | browser-only | **terminal-native** |
| Evidence-backed recommendations | — | — | — | **dual citation (SignalID + CatalogItemID)** |
| Longitudinal signals | — | — | varies | **sparkline + Δ vs avg** |
| Read-only policy viewer | — | — | — | **Control tab** |
| Signed releases (cosign + SLSA) | — | — | varies | **L3 provenanced** |

This is not "better than the others" — it's a different tool for the
person who wants the whole local stack in one terminal binary they
can `cosign verify` themselves.

## Can I use this with non-Claude agents?

Yes — that's what the OTLP receiver is for. Any OTLP/JSON exporter
pointed at `127.0.0.1:4318` (or wherever you set `TONYS_OTLP_BIND`)
flows into the same DAG. Tested with LangGraph, CrewAI, AutoGen,
OpenAI Agents SDK, LiteLLM, TGI, OpenRouter, Letta, smolagents — and
should work with anything that speaks OTLP.

## How does the Advisor know what to recommend?

It runs behavioural signal extraction over the live DAG (four detectors:
`stalled_node`, `duplicate_subagent_work`, `unused_installed_skill`,
`failed_handoff`), then maps each signal to candidate catalog tags via
the [mapping table][mapping]. Catalog items whose tags match best get
recommended, with both `SignalID` (which signal triggered it) and
`CatalogItemID` (which catalog entry was picked) carried on every
recommendation. The "no recommendation without evidence" rule is
enforced in code — see
[`internal/recommender/policy.go`](../internal/recommender/policy.go).

[mapping]: ../internal/recommender/mapping.go

## Where does the catalog come from?

[`FlorianBruniaux/claude-code-ultimate-guide`](https://github.com/FlorianBruniaux/claude-code-ultimate-guide),
pinned to commit `37e9335` (see [`internal/catalog/source.go`](../internal/catalog/source.go)).
We don't auto-track upstream; bumping the SHA is a deliberate PR.
License is CC-BY-SA-4.0 — attribution rendered in the Skills tab.

## Is there a hosted version / cloud SaaS?

No, and there are no plans for one. Distributing a local binary is the
point.

## Why "Phase 2 / Phase 3 / Phase 4" — are some features incomplete?

Earlier docs leaked internal sprint labels into section headings.
Those have been removed for v0.1.0. Everything documented in the
README + `docs/` is shipping in the v0.1.0 release.

## Why is the OTLP receiver localhost-only by default?

It accepts arbitrary span data with no authentication. On a shared
network, anyone who can reach `:4318` could inject spans. Localhost-only
is the conservative default; opt in via `TONYS_OTLP_BIND=0.0.0.0:4318`
when you control the network.

## What's the relationship to the `cmd/hook-handler` binary I see in old docs?

It's gone as of v0.1.0. The binary used to bridge Claude Code's PostToolUse
hook into a FIFO for runtime tool-call enforcement. The Control tab is
now read-only; runtime enforcement is out of scope until we can ship it
without the pre-release stability problems that bridge had.

## Do you collect telemetry from this tool?

No. The tool *processes* your local telemetry — it doesn't *emit* any
of its own. There is no opt-in/opt-out switch because there's nothing
to opt out of.

## How do I uninstall?

```sh
# Homebrew
brew uninstall vanillacake369/tap/tonys-agent-telemetry
brew untap vanillacake369/tap

# Caches + data (Linux/macOS)
rm -rf "${XDG_CACHE_HOME:-$HOME/.cache}/tonys-agent-telemetry"
rm -rf "${XDG_CONFIG_HOME:-$HOME/.config}/tonys-agent-telemetry"
```

Source-built binaries: just `rm bin/tonys-agent-telemetry`. The tool
never installs anything outside its own cache + config directories.

## How do I contribute?

See [`CONTRIBUTING.md`](../CONTRIBUTING.md). TL;DR: `make hooks-install`,
`make ci`, conventional commits, disclose AI assistance in the PR
description.

## Is the binary safe to run?

Every release ships with cosign keyless signatures and SLSA L3
provenance. The [`SECURITY.md`](../SECURITY.md) doc has the exact
verification commands. The binary is pure-Go (CGO disabled), so there
are no transitive C dependencies to worry about.
