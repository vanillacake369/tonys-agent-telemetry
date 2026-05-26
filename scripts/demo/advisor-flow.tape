# Scenario 3: Advisor evidence-backed recommendation flow.
# Skills tab: catalog loads + Advisor pane populates with citations.

Output docs/demo/advisor-flow.gif

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

# Skills tab.
Type "2"
Sleep 4000ms  # catalog fetch + advisor pipeline runs

# Scroll through skills list.
Type "j"
Sleep 800ms
Type "j"
Sleep 800ms
Type "j"
Sleep 800ms

# Pause to show catalog + Advisor + attribution footer.
Sleep 3500ms

# Quit.
Type "q"
Sleep 500ms
