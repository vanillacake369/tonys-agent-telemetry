# Scenario 4: Trends sparklines (Phase 3 longitudinal).
# Press 6, show sparkline + Start/Last/Δ + fidelity tier legend.

Output docs/demo/trends-flow.gif

Set Shell        "bash"
Set FontSize     14
Set Width        1200
Set Height       720
Set Padding      20
Set TypingSpeed  50ms

Hide
Type "export TONYS_DEMO_HOME=/tmp/tonys-demo HOME=/tmp/tonys-demo XDG_CACHE_HOME=/tmp/tonys-demo/cache TONYS_SIGNAL_STORE=/tmp/tonys-demo/cache/tonys-agent-telemetry/signals && bash scripts/demo/seed.sh"
Enter
Sleep 600ms
Type "clear"
Enter
Show

Type "./bin/tonys-agent-telemetry"
Enter
Sleep 2200ms

# Press 6 → Trends.
Type "6"
Sleep 4000ms  # let loadTrendsCmd populate from the seeded signal store

# Quit.
Type "q"
Sleep 500ms
