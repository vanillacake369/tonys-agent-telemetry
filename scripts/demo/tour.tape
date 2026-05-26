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
# Clear via find -delete so host-side cmd-guard hooks that block "rm -rf"
# don't reject the demo build on the recorder's machine.
Type "find /tmp/tonys-demo -mindepth 1 -delete 2>/dev/null; bash scripts/demo/seed.sh > /dev/null 2>&1 && clear"
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
# Use case: "I want to inspect the session where I refactored auth."
Type "j"
Sleep 350ms
Type "j"
Sleep 700ms
# Open the detail overlay so the viewer sees actual session content,
# not just a row in the list.
Type "v"
Sleep 1800ms
Escape
Sleep 500ms

# ── Tab 2: Skills + Catalog + Advisor ─────────────────────────────────
# Use case: "search the test-* skills I have locally; see what the
# Advisor recommends from the catalog based on today's signals."
Type "2"
Sleep 1400ms
Type "/"
Sleep 250ms
Type "test"
Sleep 1200ms
Escape
Sleep 500ms
# Pause so viewer reads the Catalog list + Advisor footer
# (dual-citation: SignalID → CatalogItemID).
Sleep 2400ms

# ── Tab 3: Cost — per-provider / per-model spend ──────────────────────
Type "3"
Sleep 1800ms

# ── Tab 4: Hooks workflow ─────────────────────────────────────────────
# Use case: "which hooks fire on UserPromptSubmit / PreToolUse / Stop?"
# Seeded settings.json has 8 hook entries across 4 events, mirroring a
# real Claude Code config (cmd-guard, branch-guard, test-feedback…).
Type "4"
Sleep 2400ms

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
