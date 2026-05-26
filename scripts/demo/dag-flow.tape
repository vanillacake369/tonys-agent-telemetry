# Scenario 2: DAG headline feature.
# traces list → enter trace → graph view → `g` overview → `/` search → n/N.

Output docs/demo/dag-flow.gif

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
Sleep 2500ms

# Press 5 → DAG tab.
Type "5"
Sleep 2000ms

# Navigate down one trace, then Enter to open.
Type "j"
Sleep 700ms
Type "j"
Sleep 700ms
Enter
Sleep 2500ms

# Overview mode (g).
Type "g"
Sleep 2500ms

# Back to graph.
Type "g"
Sleep 1500ms

# Search mode.
Type "/"
Sleep 600ms
Type "bash"
Sleep 800ms
Enter
Sleep 2000ms

# Cycle through matches.
Type "n"
Sleep 1500ms
Type "n"
Sleep 1500ms
Type "N"
Sleep 1500ms

# Esc clears search.
Escape
Sleep 1200ms

# Quit.
Type "q"
Sleep 500ms
