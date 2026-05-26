#!/usr/bin/env bash
# Pretty-printed project intro shown before the TUI launches. Used as the
# opening of the asciinema/vhs demo so the viewer knows what they're
# about to watch in the next ~90 seconds.
#
# Run directly to preview: bash scripts/demo/intro.sh

set -u

# Color codes (POSIX, no tput dependency).
B=$'\033[1m'      # bold
D=$'\033[2m'      # dim
P=$'\033[35m'     # magenta (matches TUI accent)
C=$'\033[36m'     # cyan (numbered tabs)
G=$'\033[32m'     # green
Y=$'\033[33m'     # yellow
R=$'\033[0m'      # reset

clear

cat <<EOF

  ${B}${P}tonys-agent-telemetry${R}${B}${R} — local-first LLM-agent telemetry & control TUI

  ${D}A 90-second tour of every tab + the headless CLI.${R}

  ${B}What you'll see${R}

    ${C}1${R}  Sessions  ${D}— browse + resume past Claude Code sessions${R}
    ${C}2${R}  Skills    ${D}— local + GitHub search + best-practice catalog + Advisor${R}
    ${C}3${R}  Cost      ${D}— per-provider cost & token breakdown${R}
    ${C}4${R}  Hooks     ${D}— visualise ~/.claude/settings.json hook config${R}
    ${C}5${R}  DAG       ${D}— live multi-provider agent orchestration graph${R}
    ${C}6${R}  Trends    ${D}— longitudinal sparkline of behavioural signals${R}
    ${C}^G${R} Control   ${D}— policy + budget + denial log (read-only)${R}

  ${B}Differentiation${R}

    • ${G}Multi-provider${R}: Claude Code, OTLP/JSON, vLLM, Ollama
    • ${G}Evidence-backed${R}: every recommendation cites (SignalID, CatalogItemID)
    • ${G}Verifiable${R}: cosign keyless signatures + SLSA L3 provenance

  ${Y}Press any key to launch…${R}

EOF

# Wait for any keypress (silent, no echo). Skip wait when STDIN isn't a TTY
# (CI / vhs runs without TTY would otherwise block forever).
if [ -t 0 ]; then
  read -n 1 -s
fi
