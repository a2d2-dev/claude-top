# claude-top

Terminal UI for monitoring [Claude Code](https://claude.ai/code) token and cost usage in real time.

![claude-top screenshot](https://github.com/a2d2-dev/claude-top/releases/download/v0.1.1/screenshot.png)

## Screenshots

![Overview](backend/src/assert/screenshot-1.png)
![Sessions](backend/src/assert/screenshot-2.png)
![Daily](backend/src/assert/screenshot-3.png)

## Features

- **Overview** — active session progress bar, burn rate, time remaining
- **Sessions** — sortable history table, drill into any session for per-message cost breakdown
- **Daily** — 52-week contribution graph, cost summary, scrollable per-day table
- Chart highlights the selected message's position in time
- Auto-refreshes every 10 seconds; press `r` to force refresh

## Installation

### npx (no install required)

```bash
npx @a2d2/claude-top@latest
```

### npm global

```bash
npm install -g @a2d2/claude-top
claude-top
```

### go install

```bash
go install github.com/a2d2-dev/claude-top@latest
```

### Download binary

Grab the binary for your platform from the [Releases page](https://github.com/a2d2-dev/claude-top/releases/latest):

| Platform | File |
|----------|------|
| macOS Apple Silicon | `claude-top-darwin-arm64` |
| macOS Intel | `claude-top-darwin-x86_64` |
| Linux x64 | `claude-top-linux-x86_64` |
| Linux ARM64 | `claude-top-linux-arm64` |
| Windows x64 | `claude-top-windows-x86_64.exe` |

```bash
# macOS / Linux
chmod +x claude-top-*
./claude-top-darwin-arm64
```

## Usage

```
claude-top [--plan <plan>] [--data-path <path>]

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
