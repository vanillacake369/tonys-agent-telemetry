# Scenario 5: CLI headless analysis via --emit-signals.

Output docs/demo/cli-emit-signals.gif

Set Shell        "bash"
Set FontSize     14
Set Width        1200
Set Height       720
Set Padding      20
Set TypingSpeed  55ms

Hide
Type "bash scripts/demo/seed.sh"
Enter
Sleep 400ms
Type "clear"
Enter
Show

# --help first to show the flag exists.
Type "./bin/tonys-agent-telemetry --help | head -25"
Enter
Sleep 4000ms

Type "clear"
Enter
Sleep 200ms

# Run --emit-signals on the sample replay; pipe to jq for legibility.
Type "./bin/tonys-agent-telemetry --emit-signals --replay /tmp/tonys-demo/sample.jsonl | jq '.[] | {type, trace_id, evidence}'"
Enter
Sleep 5000ms
