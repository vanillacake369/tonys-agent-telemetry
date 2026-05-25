# tonys-agent-telemetry

TUI dashboard for Claude Code sessions, agents, DAG visualization, and skill marketplace.

![Go Version](https://img.shields.io/badge/go-1.26-blue)
![License](https://img.shields.io/badge/license-MIT-green)
![Platform](https://img.shields.io/badge/platform-linux%20%7C%20macOS-lightgrey)

<!-- TODO: add terminal screenshot -->

## Features

- **Sessions** — Fuzzy-find and resume past Claude Code sessions; fork or continue any session
- **Agents** — Browse and launch configured agents from a searchable list
- **DAG** — Live agent orchestration graph showing real-time tool calls and sub-agent delegation
- **Skills** — Search the skill marketplace (local and GitHub-sourced skills with fuzzy filtering)
- **Control** — Runtime governance: per-session/per-day USD budget caps, tool allow/denylists, live denial log

## Installation

### Nix (recommended)

```sh
nix run github:vanillacake369/tonys-agent-telemetry
```

Or add to your flake:

```nix
inputs.tonys-agent-telemetry.url = "github:vanillacake369/tonys-agent-telemetry";
```

### Go

```sh
go install github.com/vanillacake369/tonys-agent-telemetry@latest
```

### Homebrew (future)

```sh
brew install vanillacake369/tap/tonys-agent-telemetry
```

### Binary

Download the latest release from [GitHub Releases](https://github.com/vanillacake369/tonys-agent-telemetry/releases) and extract:

```sh
tar -xzf tonys-agent-telemetry_linux_amd64.tar.gz
mv tonys-agent-telemetry /usr/local/bin/
mv tonys-agent-telemetry-hook /usr/local/bin/
```

## Usage

```sh
tonys-agent-telemetry          # Launch TUI
tonys-agent-telemetry --help   # Print usage
tonys-agent-telemetry --version
```

### Key Bindings

| Key          | Action                                  |
|--------------|-----------------------------------------|
| `1`          | Switch to Sessions tab                  |
| `2`          | Switch to Agents tab                    |
| `3`          | Switch to DAG tab                       |
| `4`          | Switch to Skills tab                    |
| `Ctrl+G`     | Switch to Control tab (Governance)      |
| `Tab`        | Next tab                                |
| `Shift+Tab`  | Previous tab                            |
| `Enter`      | Select / confirm                        |
| `Esc`        | Back / cancel search                    |
| `r`          | Refresh current tab                     |
| `f`          | Fork session (Sessions tab)             |
| `y`          | Copy to clipboard                       |
| `s`          | Sort (Skills tab)                       |
| `o`          | Open in browser (Skills tab)            |
| `/`          | Focus search                            |
| `?`          | Which-key help overlay                  |
| `q`          | Quit                                    |

#### Control tab keys

| Key | Action                                         |
|-----|------------------------------------------------|
| `r` | Reload budgets and denials from disk           |
| `e` | Open `policy.toml` in `$EDITOR`               |
| `c` | Clear denial log                               |

## Control Plane (Phase 2)

`tonys-agent-telemetry` can enforce runtime policies on your Claude Code sessions:
- Per-session and per-day USD budget caps
- Tool allowlists/denylists (e.g., block `rm -rf` globally)
- Live observability of budget burn-down

Configure via `~/.config/tonys-agent-telemetry/policy.toml`. See [example policy](./examples/policy.toml).

When a policy violation triggers, the hook returns exit code 2 to Claude Code,
which surfaces the denial message to the model as a tool error. The agent
typically reacts by trying a different approach or asking for guidance.

Press `Ctrl+G` to view the Control tab with live budget bars and denial log.

## Hook Setup

`tonys-agent-telemetry` receives live events from Claude Code via a named FIFO at `/tmp/tonys-agent-telemetry.fifo`.

Install the hook handler binary, then register it in your Claude Code hooks configuration (`~/.claude/settings.json`):

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "tonys-agent-telemetry-hook PostToolUse"
          }
        ]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "tonys-agent-telemetry-hook PreToolUse"
          }
        ]
      }
    ]
  }
}
```

The hook handler reads the JSON payload from stdin and writes it to the FIFO. It always exits 0 to never block Claude Code. If the TUI is not running the FIFO does not exist and the hook is a no-op.

## Architecture

```
.
├── main.go                    # TUI entry point; version injection target
├── cmd/hook-handler/main.go   # Hook handler binary (stdin → FIFO + policy enforcement)
├── examples/policy.toml       # Sample policy configuration
└── internal/
    ├── control/               # Policy loading, budget store, denial log, decision engine
    ├── data/                  # Session/agent data loading (JSONL parser, models)
    ├── event/                 # FIFO write logic for real-time hook events
    ├── platform/              # OS detection, clipboard, terminal utilities
    ├── skill/                 # Skill marketplace: local scan + GitHub fetch + cache
    └── tui/                   # Bubbletea TUI: app, tabs, DAG renderer, styles, keymap
```

### Key packages

- `internal/control` — policy TOML loading (fail-open), budget accumulation with flock, denial JSONL log, decision engine
- `internal/data` — reads `~/.claude/projects/**/*.jsonl` session files and agent metadata
- `internal/event` — non-blocking FIFO write with timeout; silent no-op when TUI is not running
- `internal/skill` — local skill scanner + GitHub API fetcher with disk cache
- `internal/tui` — five-tab Bubbletea application; DAG renderer for agent orchestration graphs

## Requirements

- Go 1.26+ (for building from source)
- `gh` (optional) — used by the Skills tab for authenticated GitHub API calls
- `fzf` (optional) — enhanced fuzzy search in Sessions tab

## License

MIT — see [LICENSE](LICENSE).
