# Single comprehensive ~55-second tour with realistic interactions per
# feature. Each tab gets a concrete usage example, not just a passive
# screen-grab. Targets the lazygit/k9s/gh demo convention.
#
# Run: make demo  (or: vhs scripts/demo/tour.tape)
# Output: docs/demo/tour.gif

Output docs/demo/tour.gif

Set Shell        "bash"
Set FontSize     14
Set Width        1200
Set Height       720
Set Padding      18
Set TypingSpeed  35ms
Set PlaybackSpeed 1.0

# ── Seed demo data (hidden from cast) ──────────────────────────────────
Hide
Type "export TONYS_DEMO_HOME=/tmp/tonys-demo HOME=/tmp/tonys-demo XDG_CACHE_HOME=/tmp/tonys-demo/cache XDG_CONFIG_HOME=/tmp/tonys-demo/config TONYS_SIGNAL_STORE=/tmp/tonys-demo/cache/tonys-agent-telemetry/signals TONYS_CATALOG_PATH=/tmp/tonys-demo/cache/tonys-agent-telemetry/catalog/items.json TONYS_CATALOG_MIN=3"
Enter
Type "rm -rf /tmp/tonys-demo && bash scripts/demo/seed.sh > /dev/null 2>&1 && clear"
Enter
Show
Sleep 300ms

# ── Intro: what is this and why should I watch ────────────────────────
Type "bash scripts/demo/intro.sh"
Enter
Sleep 3500ms
Type " "
Sleep 300ms

# ── Launch ────────────────────────────────────────────────────────────
Type "clear && ./bin/tonys-agent-telemetry"
Enter
Sleep 2200ms

# ── Tab 1: Sessions — fuzzy-find a past Claude Code session ───────────
# Use case: "find the session where I refactored auth"
Type "j"
Sleep 400ms
Type "j"
Sleep 1100ms

# ── Tab 2: Skills + Catalog + Advisor ─────────────────────────────────
# Use case: "search for TDD skills + see what Advisor recommends"
Type "2"
Sleep 1800ms
Type "/"
Sleep 300ms
Type "tdd"
Sleep 1200ms
Escape
Sleep 800ms
# Pause so viewer reads Catalog + Advisor footer.
Sleep 2200ms

# ── Tab 3: Cost ───────────────────────────────────────────────────────
Type "3"
Sleep 1500ms

# ── Tab 4: Hooks workflow ─────────────────────────────────────────────
# Use case: "what hooks fire when I run a tool?"
Type "4"
Sleep 1800ms

# ── Tab 5: DAG — the headline feature ─────────────────────────────────
# Use case: "open a trace, see the orchestration, jump to a tool node"
Type "5"
Sleep 1500ms
Type "j"
Sleep 400ms
Enter
Sleep 1600ms
# Compact overview of the whole trace.
Type "g"
Sleep 2000ms
Type "g"
Sleep 500ms
# Search for the bash tool call.
Type "/"
Sleep 200ms
Type "bash"
Sleep 600ms
Enter
Sleep 1500ms
Type "n"
Sleep 900ms
Escape
Sleep 400ms
Escape
Sleep 600ms

# ── Tab 6: Trends — longitudinal sparkline (10 days of history) ───────
Type "6"
Sleep 3000ms

# ── ^G: Control — policy + budgets + denials (read-only) ──────────────
Ctrl+g
Sleep 2500ms

# ── Quit ──────────────────────────────────────────────────────────────
Type "q"
Sleep 400ms

# ── Closing: headless CLI extract ─────────────────────────────────────
# Use case: "I want signal data offline for analysis or CI"
Type "clear"
Enter
Sleep 200ms
Type "# Headless: extract signals as JSON for offline analysis."
Sleep 900ms
Enter
Type "./bin/tonys-agent-telemetry --emit-signals --replay /tmp/tonys-demo/sample.jsonl | jq -c '.[] | {type, trace_id, evidence}'"
Enter
Sleep 3500ms
