#!/usr/bin/env bash
# Live tour launcher — for asciinema recording (the "type real keys"
# path). Wraps seed + intro + binary launch so the recorder only needs
# to start asciinema once and walk through the tabs themselves.
#
# Workflow:
#   asciinema rec --idle-time-limit 1 --title 'tonys-agent-telemetry tour' tour.cast
#   bash scripts/demo/tour.sh
#   <press keys to walk through tabs — see suggested sequence below>
#   q
#   <exit>
#   asciinema upload tour.cast    # → asciinema.org URL for README link
#
# Suggested key sequence (each pause ~3s for viewers to read):
#   2 (Skills + Advisor)
#   3 (Cost)
#   4 (Hooks)
#   5 (DAG) → j → Enter → g (overview) → g → /bash<Enter> → n → Esc → Esc
#   6 (Trends)
#   Ctrl+G (Control)
#   q
#
# When done: typing the "headless" command at the end of the cast
# rounds out the narrative — see TOUR_SCRIPT.md for the exact words.

set -euo pipefail

export TONYS_DEMO_HOME="${TONYS_DEMO_HOME:-/tmp/tonys-demo}"
export HOME="$TONYS_DEMO_HOME"
export XDG_CACHE_HOME="$TONYS_DEMO_HOME/cache"
export TONYS_SIGNAL_STORE="$XDG_CACHE_HOME/tonys-agent-telemetry/signals"

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$REPO_ROOT"

if [ ! -x bin/tonys-agent-telemetry ]; then
  echo "→ binary missing, building it now"
  make build > /dev/null
fi

bash scripts/demo/seed.sh > /dev/null
bash scripts/demo/intro.sh

clear
./bin/tonys-agent-telemetry

clear
echo "# Headless path — extract behavioural signals as JSON:"
echo
echo "./bin/tonys-agent-telemetry --emit-signals --replay /tmp/tonys-demo/sample.jsonl | jq '.[] | {type, trace_id, evidence}'"
echo
echo "# Run the above when ready, then exit the recorder."
