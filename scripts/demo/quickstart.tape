# Scenario 1: Launch + cycle every tab + quit.
# Targets the 5-second skim — viewer sees navigation chrome.

Output docs/demo/quickstart.gif

Set Shell        "bash"
Set FontSize     14
Set Width        1200
Set Height       720
Set Padding      20
Set TypingSpeed  60ms
Set PlaybackSpeed 1.0

# Seed the demo home so we have realistic data.
Hide
Type "export TONYS_DEMO_HOME=/tmp/tonys-demo HOME=/tmp/tonys-demo XDG_CACHE_HOME=/tmp/tonys-demo/cache TONYS_SIGNAL_STORE=/tmp/tonys-demo/cache/tonys-agent-telemetry/signals && bash scripts/demo/seed.sh"
Enter
Sleep 800ms
Type "clear"
Enter
Show

# Launch.
Type "./bin/tonys-agent-telemetry"
Sleep 200ms
Enter
Sleep 2500ms

# Cycle: Skills, Cost, Hooks, DAG, Trends, Control.
Type "2"
Sleep 1800ms
Type "3"
Sleep 1500ms
Type "4"
Sleep 1500ms
Type "5"
Sleep 1800ms
Type "6"
Sleep 1800ms
Ctrl+g
Sleep 1800ms

# Back to Sessions and quit.
Type "1"
Sleep 1200ms
Type "q"
Sleep 500ms
