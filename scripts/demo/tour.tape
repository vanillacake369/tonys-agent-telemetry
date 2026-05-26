# Single comprehensive tour — used by both vhs (→ tour.gif) and as the
# reference script for asciinema recordings. Mirrors the "lazygit demo"
# convention: one cohesive narrative, narration via bash echoes, then
# walk through every tab in order.
#
# Run: vhs scripts/demo/tour.tape
# Output: docs/demo/tour.gif

Output docs/demo/tour.gif

Set Shell        "bash"
Set FontSize     14
Set Width        1200
Set Height       720
Set Padding      20
Set TypingSpeed  55ms

# ── Seed demo data + show the narration intro ──────────────────────────
Hide
Type "export TONYS_DEMO_HOME=/tmp/tonys-demo HOME=/tmp/tonys-demo XDG_CACHE_HOME=/tmp/tonys-demo/cache TONYS_SIGNAL_STORE=/tmp/tonys-demo/cache/tonys-agent-telemetry/signals"
Enter
Type "bash scripts/demo/seed.sh > /dev/null 2>&1 && clear"
Enter
Show
Sleep 400ms

Type "bash scripts/demo/intro.sh"
Enter
Sleep 6000ms

# Press anything to dismiss the intro.
Type " "
Sleep 600ms

# ── Launch the TUI ────────────────────────────────────────────────────
Type "clear && ./bin/tonys-agent-telemetry"
Enter
Sleep 3000ms

# Tab 1: Sessions — already focused. Pause so viewer reads the list.
Sleep 1800ms

# Tab 2: Skills + Catalog + Advisor.
Type "2"
Sleep 4500ms

# Tab 3: Cost.
Type "3"
Sleep 2200ms

# Tab 4: Hooks workflow.
Type "4"
Sleep 3000ms

# Tab 5: DAG headline.
Type "5"
Sleep 2200ms
# Navigate to second trace, enter it.
Type "j"
Sleep 700ms
Enter
Sleep 2200ms
# Toggle into overview mode.
Type "g"
Sleep 2500ms
Type "g"
Sleep 1200ms
# Search.
Type "/bash"
Sleep 800ms
Enter
Sleep 1800ms
Type "n"
Sleep 1200ms
Escape
Sleep 800ms
# Back to traces list.
Escape
Sleep 800ms

# Tab 6: Trends sparkline.
Type "6"
Sleep 4500ms

# ^G: Control.
Ctrl+g
Sleep 3000ms

# Quit cleanly.
Type "q"
Sleep 500ms

# ── Closing narration: headless CLI path ──────────────────────────────
Type "clear"
Enter
Sleep 200ms

Type "# Headless path: extract behavioural signals as JSON for offline analysis."
Sleep 1200ms
Enter
Sleep 400ms
Type "./bin/tonys-agent-telemetry --emit-signals --replay /tmp/tonys-demo/sample.jsonl | jq '.[] | {type, trace_id, evidence}'"
Enter
Sleep 5000ms

Type "# Releases are cosign-signed + SLSA L3 provenanced. Verify with cosign verify-blob + slsa-verifier."
Sleep 1500ms
Enter
Sleep 2500ms
