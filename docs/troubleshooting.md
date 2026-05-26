# Troubleshooting

Common failure modes, what they look like, and how to fix them. If
something here is wrong or missing, please [file an issue][bug].

[bug]: https://github.com/vanillacake369/tonys-agent-telemetry/issues/new?template=bug_report.yml

## First launch: everything looks empty

**Symptom**: every tab shows "Loading…" or an empty-state hint.

**Cause**: `tonys-agent-telemetry` reads Claude Code activity from
`~/.claude/projects/`. If that directory doesn't exist (you've never
run Claude Code on this machine) or is empty (fresh install), there's
nothing to display.

**Fix**: run one Claude Code session, then re-launch the TUI. Or use
`--replay <file>` with a snapshot JSONL — see [CLI reference](./cli-reference.md#flags).

## OTLP receiver: port already in use

**Symptom**: warning on startup that the OTLP receiver failed to bind
`:4318`, no spans appear from external agents.

**Cause**: another process is listening on `127.0.0.1:4318` (commonly
an OTLP collector, Tempo, Jaeger, or another instance of this binary).

**Fix**: change the bind address — `TONYS_OTLP_BIND=127.0.0.1:24318
tonys-agent-telemetry`. Point your external agents at the new port.
The default is intentionally localhost-only; if you need LAN-accessible
ingest, set `TONYS_OTLP_BIND=0.0.0.0:4318` **only on a trusted
network** (no auth in v0.1.0).

## Docker container can't reach the OTLP receiver

**Symptom**: A LangGraph / CrewAI / LiteLLM container exports OTLP to
`http://localhost:4318` but nothing arrives.

**Cause**: the default bind is `127.0.0.1:4318`. Inside a Docker
container, `localhost` is the container, not the host.

**Fix**: two changes.
1. On the host: `TONYS_OTLP_BIND=0.0.0.0:4318 tonys-agent-telemetry`
2. In the container: point the exporter at the host. On Docker Desktop
   that's `host.docker.internal:4318`. On Linux it's the docker bridge
   gateway, usually `172.17.0.1:4318` (find with `ip route show
   default`).

## Skills tab: "Loading catalog…" never finishes

**Symptom**: Skills tab catalog pane stays at "Loading catalog…" for
longer than ~10 seconds, no entries appear.

**Causes**:
- Offline / corporate firewall blocks `raw.githubusercontent.com`.
- Network path to GitHub is fine but the upstream commit SHA we pin to
  was force-removed (very rare).

**Fix**: pre-populate the cache from a machine that has access:

```sh
# On a machine that can reach GitHub:
curl -L \
  https://github.com/FlorianBruniaux/claude-code-ultimate-guide/raw/37e9335457b829b8b307c12e0b8cbdf42be7cd8b/examples/CATALOG.md \
  | tonys-agent-telemetry --parse-catalog \
  > ~/catalog-items.json   # this subcommand is illustrative only

# Then on the offline machine:
TONYS_CATALOG_PATH=~/catalog-items.json tonys-agent-telemetry
```

Alternatively, lower `TONYS_CATALOG_MIN` so the Skills tab renders the
partial catalog warning instead of the loading state.

## Trends tab: "not enough data yet (N/2 required)"

**Symptom**: Trends tab shows `not enough data yet (0/2 required)` or
`(1/2 required)` even though Claude Code sessions exist.

**Cause**: the Trends sparkline needs **at least two daily buckets**
(`DefaultBucketDuration=24h`, `MinBucketsForDisplay=2`) with non-zero
counts. A user who ran only one Claude Code session today, or several
sessions all within the same 24-hour window, sees 1/2.

**Fix**: there's nothing wrong with the data. Trends populate
naturally over a few days. To verify the pipeline works immediately,
run with `--replay` against a fixture spanning multiple days, or
inspect the live buckets with:

```sh
tonys-agent-telemetry --emit-signals --replay <file> | jq -c .
```

## vllm or ollama: sparklines flat at zero

**Symptom**: Trends tab shows the four signal types but the sparkline
is flat at zero across all buckets for vllm or ollama spans.

**Cause**: per the fidelity tier model
(see [architecture](./architecture.md#multi-provider-fidelity-tier)),
vllm produces aggregate-tier data only and ollama produces
presence-tier data only — neither emits per-call parent linkage that
the behavioural signal detectors need.

**Fix**: this is expected. For full-tier signals from vllm,
enable vLLM's [native OTel SDK integration][vllm-otel] and route its
OTLP export to our receiver. ollama has no equivalent option today.

[vllm-otel]: https://docs.vllm.ai/en/latest/serving/usage_stats.html

## CJK / wide character rendering: line widths look wrong

**Symptom**: traces with Korean / Chinese / Japanese characters in the
tool name or model field render with misaligned columns or wrap
unexpectedly.

**Cause**: we measure visible widths by rune count, then test against
`lipgloss.Width` for line-level rendering. East Asian double-width
characters are handled in most places (the CJK regression test covers
the DAG renderer), but specific tab combinations may still slip
through.

**Fix**: file a bug with the exact span content and a screenshot —
this is a real bug, not a config issue.

## Catalog tap: `brew install` says "no formula"

**Symptom**:
```
$ brew install vanillacake369/tap/tonys-agent-telemetry
Warning: No available formula or cask with the name
"vanillacake369/tap/tonys-agent-telemetry". Did you mean
vanillacake369/tap/agent-collab?
```

**Cause**: the tap was already cloned before the formula landed. Brew
reads the cached copy.

**Fix**:
```sh
brew untap vanillacake369/tap
brew tap vanillacake369/tap
brew install vanillacake369/tap/tonys-agent-telemetry
```

If the formula is still missing after re-tapping, see the
[homebrew-tap repo][tap] for the current formula state.

[tap]: https://github.com/vanillacake369/homebrew-tap/tree/main/Formula

## Analyze wizard: Codex / Gemini selection fails

**Symptom**: Selecting `gemini` in the Skills tab Analyze wizard
returns an error; `codex` no longer appears as an option.

**Cause**: as of v0.1.0:
- The Codex CLI no longer exists upstream and was removed from the
  wizard (`ErrCodexRemoved` sentinel).
- `gemini` requires the actual `gemini` CLI on `PATH`. The wizard
  hides models whose binary isn't found.

**Fix**: install the `gemini` CLI, or stick with `claude`. The wizard
auto-selects the only available model when one binary is on PATH.

## "Tab bar disappeared from the top"

**Symptom**: at narrow terminal widths, the top of the TUI shows tab
content but no tab numbers / no outer border.

**Cause**: this was a real bug pre-v0.1.0. The current release
hard-clips tab content so the tab bar and outer border always remain
visible. If you see this on v0.1.0+, it's a regression — please file
a bug with the terminal width/height (run `stty size`).

## Signal store: "schema version mismatch"

**Symptom**: Trends tab shows `not enough data yet` after a binary
upgrade even though the store has data.

**Cause**: the signal store has a `schema_version` header. Newer
binaries refuse to read files with a different version.

**Fix**: in v0.1.0 this isn't reachable — schema is `"1"` and there's
nothing to migrate from. Future releases will add a migration path
when the schema bumps; for now the workaround is to delete the store
(you lose history).

```sh
rm -rf "${TONYS_SIGNAL_STORE:-$XDG_CACHE_HOME/tonys-agent-telemetry/signals}"
```

## Cosign verification fails

**Symptom**: `cosign verify-blob` returns "no matching signatures" or
"identity does not match".

**Causes**:
- Wrong artefact / wrong .pem / wrong .sig pairing (e.g., comparing the
  darwin_arm64 archive with linux_amd64 cert).
- The `--certificate-identity-regexp` is too narrow or doesn't match
  the actual signer.

**Fix**: see the verification recipe in [`SECURITY.md`](../SECURITY.md#verifying-release-artefacts).
The identity regex must be:

```
^https://github\.com/vanillacake369/tonys-agent-telemetry/\.github/workflows/release\.yml@refs/tags/
```

If you copied an older recipe with a broader regex, tighten it.

## Still stuck?

- Search [open issues][issues].
- Ask in [Discussions][discussions].
- Reproduce with `TAT_DEBUG=1 tonys-agent-telemetry 2>/tmp/tat.log`,
  then attach `/tmp/tat-debug.log` and `/tmp/tat.log` to a bug report.

[issues]: https://github.com/vanillacake369/tonys-agent-telemetry/issues
[discussions]: https://github.com/vanillacake369/tonys-agent-telemetry/discussions
