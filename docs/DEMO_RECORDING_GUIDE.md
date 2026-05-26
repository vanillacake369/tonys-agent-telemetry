# Demo recording guide

How to produce the demo that ships in `README.md` — the lazygit/k9s/htop
convention: **one cohesive ~90-second tour**, narrated by bash echoes,
walking through every tab in order, ending with the headless CLI path.

Not five separate clips. Not a static screenshot. One file, one story.

## Two paths

| Path | When | Tool | Output |
|---|---|---|---|
| **A — asciinema** | Authentic feel, hosted on asciinema.org, README embeds a clickable SVG link | `asciinema rec` + manual keystrokes | `.cast` (uploaded) |
| **B — vhs** | Deterministic, reproducible from source, README embeds the GIF inline | `vhs scripts/demo/tour.tape` | `docs/demo/tour.gif` |

Both consume the same artefacts:

- `scripts/demo/seed.sh` — idempotent demo-data seeder (synthetic
  claudecode sessions + 10-day signal store + sample replay file under
  `/tmp/tonys-demo`)
- `scripts/demo/intro.sh` — pretty-printed project narration (the "what
  you'll see" intro before the binary launches)
- `scripts/demo/tour.sh` — composite launcher: seed → intro → binary →
  closing CLI demo. Drives the asciinema recording path.
- `scripts/demo/tour.tape` — fully scripted vhs version of the same
  story arc, with deterministic timing.

The asciinema path produces an organic recording (someone actually
typing the keys). The vhs path produces a CI-reproducible GIF. **Use
both** — asciinema link as the canonical "interactive playable cast"
and the vhs GIF as the always-loads-fast README hero.

---

## Path A — asciinema (recommended canonical demo)

### Prerequisites

```sh
brew install asciinema   # or: pipx install asciinema
```

### Record

```sh
# 1. Start the recorder. --idle-time-limit collapses long pauses so the
#    cast plays back snappier than real-time.
asciinema rec --idle-time-limit 1 --title 'tonys-agent-telemetry tour' tour.cast

# 2. Inside the recorder shell, run the composite launcher.
bash scripts/demo/tour.sh

# 3. When the intro prompts "Press any key to launch", press space.
#    The TUI opens. Walk through the suggested key sequence below.

# 4. Press q to exit the TUI. The launcher prints the headless
#    --emit-signals command — paste it and run it for the closing.

# 5. Exit the recorder (Ctrl+D or `exit`). asciinema saves tour.cast.

# 6. Preview locally:
asciinema play tour.cast

# 7. Upload to asciinema.org so README can link to it.
asciinema upload tour.cast
# → prints https://asciinema.org/a/<id>
```

### Suggested key sequence

Hold each tab for ~3 seconds so viewers can read it. Total walk: ~90s.

| Step | Key | What viewer sees |
|---|---|---|
| 1 | (already on `1`) | Sessions list loaded |
| 2 | `2` | Skills tab — Catalog populates, Advisor pane fills with citations |
| 3 | `3` | Cost breakdown |
| 4 | `4` | Hooks workflow diagram |
| 5 | `5` | DAG traces list with provider badges + status colour |
| 5a | `j` then `Enter` | Open second trace into graph view |
| 5b | `g` | Toggle into compact overview mode |
| 5c | `g` | Back to graph |
| 5d | `/bash` then `Enter` | Search highlights matching nodes |
| 5e | `n`, `n`, `N` | Cycle search matches |
| 5f | `Esc`, `Esc` | Back to traces list |
| 6 | `6` | Trends sparkline + Start/Last/Δ + fidelity tier legend |
| 7 | `Ctrl+G` | Control tab — policy + budgets + denial log |
| 8 | `q` | Exit TUI |
| 9 | (paste printed `--emit-signals` cmd) | JSON output to stdout |

After upload, README embeds the asciinema.org link as a clickable SVG:

```markdown
[![asciicast](https://asciinema.org/a/<id>.svg)](https://asciinema.org/a/<id>)
```

---

## Path B — vhs (deterministic, CI-friendly)

### Prerequisites

```sh
brew install vhs ttyd ffmpeg
# or:
go install github.com/charmbracelet/vhs@latest
```

### Generate

```sh
make demo
# → docs/demo/tour.gif (size: ~3-5 MB typical)
```

The `.tape` file under `scripts/demo/tour.tape` is the source of truth.
Anyone with vhs can regenerate the same GIF from the same commit. The
output is what README embeds inline (loads on every visit).

The .tape encodes the same narrative as the asciinema path: seed →
intro → binary launch → walk through every tab in order → closing CLI
command. Timings are tuned for legibility (each tab visible 2-4
seconds, search and overview interactions get longer pauses).

### Iterate on a single beat

To tweak just one part (e.g., the DAG search section), edit the .tape
and rerun. vhs is fast — typically 30-60 seconds for a 90-second GIF.

```sh
vhs scripts/demo/tour.tape
```

---

## README placement

The Demo section is placed between badges and Features — the
"first impression" slot. The single GIF auto-loads inline; the
asciinema link sits beneath as the "play interactively" affordance.

```markdown
# tonys-agent-telemetry

<one-line tagline>
<badges>

## Demo

<p align="center">
  <img src="docs/demo/tour.gif" alt="tonys-agent-telemetry tour" width="900" />
</p>

<p align="center">
  <a href="https://asciinema.org/a/<id>">▶ Watch on asciinema (interactive)</a>
</p>

## Features
...
```

When asciinema is uploaded, swap `<id>` for the real cast ID. If only
one of the two paths is available, drop the corresponding link/img.

---

## PR checklist for demo updates

When opening a PR that changes the recording:

- [ ] `scripts/demo/tour.tape` reflects the new story (single source for vhs)
- [ ] If asciinema path: re-record `tour.cast` and re-upload; update the
      cast ID in README
- [ ] `make demo` produces an identical GIF from the .tape
- [ ] `docs/demo/tour.gif` ≤ 5 MB (compress with `gifsicle -O3` if needed)
- [ ] README embeds resolve (no broken `<img src>` / dead asciinema link)
- [ ] Demo data comes from `scripts/demo/seed.sh` only — no real-user PII
- [ ] PR description explains what changed and links the relevant Issue

## Why one comprehensive recording, not five clips

Earlier versions of this guide proposed five short clips (quickstart,
dag-flow, advisor-flow, trends-flow, cli-emit-signals). The lazygit /
k9s / htop / gh / ripgrep convention is the opposite: **a single
cohesive narrative beats fragmented context-switches**. Viewers form a
mental model from one continuous walkthrough; jumping between five
auto-playing GIFs is disorienting and bloats the README page weight.

The single tour also keeps each .tape change to one file. If the TUI
layout shifts, only `tour.tape` needs an update — not five.
