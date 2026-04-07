# claude-monitor

Terminal UI for monitoring [Claude Code](https://claude.ai/code) token and cost usage in real time.

![claude-monitor screenshot](https://github.com/a2d2-dev/claude-usage-monitor/releases/download/v0.1.1/screenshot.png)

## Features

- **Overview** — active session progress bar, burn rate, time remaining
- **Sessions** — sortable history table, drill into any session for per-message cost breakdown
- **Daily** — 52-week contribution graph, cost summary, scrollable per-day table
- Chart highlights the selected message's position in time
- Auto-refreshes every 10 seconds; press `r` to force refresh

## Installation

### npx (no install required)

```bash
npx @a2d2/claude-monitor@latest
```

### npm global

```bash
npm install -g @a2d2/claude-monitor
claude-monitor
```

### go install

```bash
go install github.com/a2d2-dev/claude-usage-monitor@latest
```

### Download binary

Grab the binary for your platform from the [Releases page](https://github.com/a2d2-dev/claude-usage-monitor/releases/latest):

| Platform | File |
|----------|------|
| macOS Apple Silicon | `claude-monitor-darwin-arm64` |
| macOS Intel | `claude-monitor-darwin-x86_64` |
| Linux x64 | `claude-monitor-linux-x86_64` |
| Linux ARM64 | `claude-monitor-linux-arm64` |
| Windows x64 | `claude-monitor-windows-x86_64.exe` |

```bash
# macOS / Linux
chmod +x claude-monitor-*
./claude-monitor-darwin-arm64
```

## Usage

```
claude-monitor [--plan <plan>] [--data-path <path>]

Flags:
  --plan        Subscription plan: pro, max5, max20  (default: pro)
  --data-path   Path to Claude projects dir          (default: ~/.claude/projects)
```

## Keyboard shortcuts

| Key | Action |
|-----|--------|
| `1` / `2` / `3` | Switch tabs |
| `Tab` / `Shift+Tab` | Cycle tabs |
| `↑` / `↓` or `k` / `j` | Move cursor |
| `PgUp` / `PgDn` | Page up / down (Sessions) |
| `g` / `G` | Jump to top / bottom |
| `Enter` | Open session detail |
| `Esc` | Back to session list |
| `s` / `S` | Cycle sort column forward / backward |
| `/` | Toggle sort direction |
| `r` | Force refresh |
| `q` | Quit |

## Requirements

Claude Code stores usage data in `~/.claude/projects/`. The monitor reads those JSONL files directly — no network access, no accounts, fully local.

## License

MIT
