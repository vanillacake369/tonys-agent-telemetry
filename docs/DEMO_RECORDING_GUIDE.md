# Demo recording guide

How to produce `docs/demo/tour.gif` — the lazygit / k9s / gh convention:
**one cohesive ~55-second tour**, narrated by a bash intro, walking
through every tab with a realistic interaction (not just a key-mash),
closing with the headless CLI path.

Single tool, single .tape, single output.

## Tool: `vhs`

[vhs](https://github.com/charmbracelet/vhs) is built by Charm (same shop
as bubbletea / lipgloss — our TUI stack). It runs a headless terminal
under a `.tape` script and outputs a deterministic GIF. Anyone with vhs
installed can regenerate the same GIF from the same commit.

```sh
brew install vhs ttyd ffmpeg
# Linux:
sudo apt-get install ttyd ffmpeg
go install github.com/charmbracelet/vhs@latest
```

## Generate

```sh
make demo
```

This builds the binary, runs `vhs scripts/demo/tour.tape`, and writes
`docs/demo/tour.gif`. Typical regen time: 60–90 s.

To iterate just the .tape file (skip the build):

```sh
vhs scripts/demo/tour.tape
```

## Story arc — what tour.tape actually shows

| Step | Tab | Realistic interaction (not just "press key") | Why it matters |
|---|---|---|---|
| 0 | intro | bash echoes project tagline + tab list + differentiation | viewer knows what they're about to see |
| 1 | Sessions | j j (browse list) | finds a past session |
| 2 | Skills | `/` + `tdd` (search) → Esc → pause on Catalog + Advisor | shows search + evidence-backed Advisor populating |
| 3 | Cost | (passive view) | per-provider breakdown |
| 4 | Hooks | (workflow diagram visible) | claude-code hook config visualised |
| 5 | DAG | j → Enter (open trace) → `g` (overview) → `g` → `/bash` Enter → `n` → Esc | full headline interaction |
| 6 | Trends | (10-day seeded history shows real sparkline) | longitudinal value |
| 7 | Control | (^G — read-only policy + budgets + denials) | governance surface |
| 8 | CLI | `--emit-signals --replay … \| jq` | scriptable, not TUI-only |

Total ~55 s. Typing speed is 35 ms (faster than the previous 55 ms) so
the demo doesn't feel sluggish.

## Source files

```
scripts/demo/
├── intro.sh    # pretty-printed project intro (POSIX colours, TTY-safe)
├── seed.sh     # deterministic fixtures under /tmp/tonys-demo:
│               #   - 3 synthetic claudecode sessions (.claude/projects/)
│               #   - 10 days of bucketed signal history (signalstore)
│               #   - 6 catalog items spanning every signal-mapping tag
│               #   - policy.toml + denials.jsonl + budgets.json (Control tab)
│               #   - 4 local skills (one unused — fires the unused-skill signal)
│               #   - sample.jsonl for closing --emit-signals demo
└── tour.tape   # vhs script — single source of truth for the demo
```

The seed runs idempotently into `/tmp/tonys-demo`, so it never touches a
real `~/.claude`. tour.tape exports every relevant env var
(`TONYS_DEMO_HOME`, `HOME`, `XDG_*`, `TONYS_SIGNAL_STORE`,
`TONYS_CATALOG_PATH`, `TONYS_CATALOG_MIN=3`) before launching the binary
so every read path resolves under the seeded tree.

## README placement

The Demo section sits between badges and Features — the first-impression
slot. Single hero GIF auto-loads inline:

```markdown
# tonys-agent-telemetry
<tagline>
<badges>

## Demo

<one-sentence framing>

<p align="center">
  <img src="docs/demo/tour.gif" alt="tour" width="900" />
</p>

## Features
...
```

No `<details>` collapsible, no second hosted asciinema link — a single
recording is the canonical demo and keeps the README page weight low.

## PR checklist for demo updates

- [ ] `scripts/demo/tour.tape` reflects the new story (single source)
- [ ] `make demo` produces a green-path GIF from the .tape
- [ ] `docs/demo/tour.gif` ≤ 5 MB (compress with `gifsicle -O3` if needed)
- [ ] README embed resolves (no broken `<img src>`)
- [ ] Demo data comes from `scripts/demo/seed.sh` only — no real-user PII
- [ ] PR description explains what changed and links the relevant Issue
