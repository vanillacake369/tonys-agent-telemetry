# Demo recording guide

How to produce the GIFs / videos that ship in `README.md` and the release
page. Each recording is a deterministic `.tape` script — anyone with `vhs`
installed can regenerate the same output from the same source commit.

## Tool: `vhs` (recommended)

[vhs](https://github.com/charmbracelet/vhs) is built by Charm (same shop
as bubbletea/lipgloss — our TUI stack). It runs a headless terminal under
a `.tape` script and outputs `.gif` / `.mp4` / `.webm` deterministically.

```sh
# macOS
brew install vhs ttyd ffmpeg
# Linux (apt)
sudo apt-get install ttyd ffmpeg
go install github.com/charmbracelet/vhs@latest
```

### Why not asciinema?

asciinema records a live TTY session — non-deterministic by nature (your
typing speed leaks into the cast). GitHub README also can't render the
asciinema player inline (`<script>` is stripped); you have to settle for
a clickable static SVG link to asciinema.org. `vhs` solves both:

- Deterministic — `.tape` is source-controlled, same script ⇒ same GIF
- Native GIF/MP4 — GitHub renders both inline without scripts

If you still prefer asciinema, see the "Alternative" section at the
bottom. For canonical README demos we use `vhs`.

---

## Scenarios — what to record

Five short clips (each 20–60s) — together they tell the product story.
Save GIFs to `docs/demo/<slug>.gif` so the README placeholders resolve.

| # | File | What it shows | Length |
|---|---|---|---|
| 1 | `quickstart.gif` | Launch + cycle 7 tabs + quit | ~25s |
| 2 | `dag-flow.gif` | DAG: traces → enter → graph → `g` overview → `/` search → `n`/`N` | ~45s |
| 3 | `advisor-flow.gif` | Skills: catalog load + Advisor pane populates with citations | ~35s |
| 4 | `trends-flow.gif` | Trends: sparklines + Start/Last/Δ + fidelity tier legend | ~20s |
| 5 | `cli-emit-signals.gif` | `--help` + `--emit-signals --replay sample.jsonl \| jq` | ~20s |

Each scenario is captured by a `.tape` file under `scripts/demo/`. They
share a setup block that seeds realistic synthetic data so recordings
look "lived in" regardless of who runs them.

### Scenario 1 — Quickstart (`quickstart.tape`)

The first impression. Open the binary, press each tab number, show the
seven tabs side by side, quit.

**Goal**: viewer sees the navigation chrome + understands "this is a 7-tab
TUI for agent telemetry."

### Scenario 2 — DAG flow (`dag-flow.tape`)

The headline differentiation. Show multi-trace traces list with provider
badges + color, open a trace, exercise overview mode (`g`), exercise
search (`/bash` → `n`,`N`).

**Goal**: viewer sees Swarm DAG reconstruction working end-to-end —
something cass / Phoenix / Langfuse don't ship in a terminal.

### Scenario 3 — Advisor flow (`advisor-flow.tape`)

The Phase 2 deliverable. Land on Skills tab, watch Catalog pane populate,
watch Advisor pane fill in with recommendations carrying SignalID +
TraceID citations.

**Goal**: viewer sees "evidence-backed" claim live — every recommendation
has a citation trail and a "press 5 to view DAG" affordance.

### Scenario 4 — Trends (`trends-flow.tape`)

Phase 3 longitudinal MVP. Press 6, show sparkline + Start/Last/Δ vs avg
table + fidelity tier legend.

**Goal**: viewer sees the "month-over-month" framing that prompt-based
audit tools can't deliver.

### Scenario 5 — CLI signals (`cli-emit-signals.tape`)

The headless path. Run `--help`, then `--emit-signals --replay <sample>`
piped through `jq` to show structured JSON output.

**Goal**: viewer sees the tool is scriptable, not TUI-only.

---

## Recording — quick start

Each .tape lives at `scripts/demo/<name>.tape`. To regenerate every GIF:

```sh
make demo
```

This calls `vhs` on every `.tape` file and writes GIFs to `docs/demo/`.
Commit the generated GIFs along with the .tape if they change.

To regenerate one clip:

```sh
vhs scripts/demo/dag-flow.tape
# output: docs/demo/dag-flow.gif
```

The .tape format is straightforward:

```tape
Output docs/demo/quickstart.gif
Set FontSize 14
Set Width 1200
Set Height 720
Set TypingSpeed 60ms

Type "tonys-agent-telemetry"
Enter
Sleep 2s

Type "2"  # Skills
Sleep 1500ms
Type "3"  # Cost
Sleep 1500ms
# ...
Type "q"
```

Full syntax: https://github.com/charmbracelet/vhs

### Data seeding for reproducibility

Reviewers cloning the repo may not have `~/.claude/projects/` populated.
Each .tape begins with `Source scripts/demo/seed.sh` to populate a fixed
`$TONYS_SIGNAL_STORE` and a small set of synthetic claudecode sessions
under `$TONYS_DEMO_HOME/.claude/projects/`. The seed script is idempotent
and runs in well under a second.

---

## Where to embed in README

The README should put the demo near the top so a 5-second skimmer "gets"
the project. Suggested placement:

```markdown
# tonys-agent-telemetry

<one-line tagline>

<!-- ▼ demo block goes here ▼ -->
<p align="center">
  <img src="docs/demo/quickstart.gif" alt="Launch + 7-tab tour" width="720" />
</p>

<details>
<summary>📺 More demos (DAG · Advisor · Trends · CLI)</summary>

| Feature | Recording |
|---|---|
| DAG: traces → graph → overview → search | <img src="docs/demo/dag-flow.gif" alt="DAG flow" width="720" /> |
| Skills + Advisor (evidence-backed) | <img src="docs/demo/advisor-flow.gif" alt="Advisor flow" width="720" /> |
| Trends sparklines | <img src="docs/demo/trends-flow.gif" alt="Trends" width="720" /> |
| CLI `--emit-signals` | <img src="docs/demo/cli-emit-signals.gif" alt="CLI signals" width="720" /> |
</details>
<!-- ▲ demo block ▲ -->

## Features
...
```

Reasoning for this placement:

- The **quickstart GIF goes above the Features list**, between the
  tagline and the first `##`. This is the "first impression" slot.
- The four other clips are tucked inside a `<details>` collapsible so
  the README doesn't become a 50MB-on-load page.
- GIFs are `width="720"` so they fit GitHub's content column on desktop
  without horizontal scroll.
- All GIFs live under `docs/demo/` so they're easy to find and skip
  during git diffs.

## PR checklist for demo additions

When opening a PR that adds or updates a recording:

- [ ] `.tape` file under `scripts/demo/` is committed (source of truth)
- [ ] Generated `.gif` is under `docs/demo/`, under 5 MB ideally
- [ ] `make demo` produces the same output from the .tape file
- [ ] README placeholder paths resolve (no broken `<img src>`)
- [ ] Recording dimensions match the existing ones (1200×720) so the
      collapsible table renders evenly
- [ ] Demo data is from `scripts/demo/seed.sh` — no real-user PII in
      a recording
- [ ] PR description explains what the recording demonstrates and
      links the relevant Issue / Phase

## Alternative: asciinema

Some users prefer asciinema's hosting + playback. To produce an asciinema
cast (not committed; uploaded to asciinema.org):

```sh
brew install asciinema
asciinema rec --idle-time-limit 1 docs/demo/quickstart.cast
asciinema upload docs/demo/quickstart.cast  # → asciinema.org URL
```

Embed in README:

```markdown
[![asciicast](https://asciinema.org/a/<id>.svg)](https://asciinema.org/a/<id>)
```

Clickable static SVG; the play button opens the cast on asciinema.org.
This is the fallback path — `vhs` GIFs remain the canonical README demos.
