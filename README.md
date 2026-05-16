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

| Key      | Action                  |
|----------|-------------------------|
| `Ctrl+S` | Switch to Sessions tab  |
| `Ctrl+A` | Switch to Agents tab    |
| `Ctrl+D` | Switch to DAG tab       |
| `Ctrl+K` | Switch to Skills tab    |
| `Enter`  | Select / confirm        |
| `Esc`    | Back / cancel           |
| `Ctrl+F` | Fork session            |
| `Ctrl+N` | New session             |
| `Ctrl+Y` | Copy to clipboard       |
| `Ctrl+R` | Refresh                 |
| `q`      | Quit                    |

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
├── cmd/hook-handler/main.go   # Hook handler binary (stdin → FIFO)
└── internal/
    ├── data/                  # Session/agent data loading (JSONL parser, models)
    ├── event/                 # FIFO write logic for real-time hook events
    ├── platform/              # OS detection, clipboard, terminal utilities
    ├── skill/                 # Skill marketplace: local scan + GitHub fetch + cache
    └── tui/                   # Bubbletea TUI: app, tabs, DAG renderer, styles, keymap
```

### Key packages

- `internal/data` — reads `~/.claude/projects/**/*.jsonl` session files and agent metadata
- `internal/event` — non-blocking FIFO write with timeout; silent no-op when TUI is not running
- `internal/skill` — local skill scanner + GitHub API fetcher with disk cache
- `internal/tui` — four-tab Bubbletea application; DAG renderer for agent orchestration graphs

## Requirements

- Go 1.26+ (for building from source)
- `gh` (optional) — used by the Skills tab for authenticated GitHub API calls
- `fzf` (optional) — enhanced fuzzy search in Sessions tab

## License

MIT — see [LICENSE](LICENSE).
