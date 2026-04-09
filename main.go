// claude-usage-monitor is a terminal UI for monitoring Claude Code token and cost usage.
// It reads JSONL session data from ~/.claude/projects and optionally ~/.codex/sessions.
//
// Usage:
//
//	claude-usage-monitor [--plan pro|max5|max20] [--data-path /path/to/projects]
//	                     [--source all|claude|codex] [--codex-path /path/to/codex/sessions]
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/a2d2-dev/claude-usage-monitor/internal/config"
	"github.com/a2d2-dev/claude-usage-monitor/internal/ui"
)

func main() {
	planName  := flag.String("plan", "pro", "Subscription plan: pro, max5, max20")
	dataPath  := flag.String("data-path", "", "Path to Claude projects dir (default: ~/.claude/projects)")
	source    := flag.String("source", "all", "Data source: all, claude, or codex")
	codexPath := flag.String("codex-path", "", "Path to Codex sessions dir (default: ~/.codex/sessions)")
	flag.Parse()

	// Load persisted config as fallback for flags not explicitly set.
	cfg := config.Load()

	// CLI flags override persisted config; flags default to "" meaning "use config".
	// --source defaults to "all" in the flag definition, but we check if the user
	// actually passed it by comparing against the default value.
	// Use config value when the flag was left at its default "all" and config differs.
	// (A user who explicitly passes --source all still gets "all" — no conflict.)
	if *source == "all" && cfg.Source != "" && cfg.Source != "all" {
		*source = cfg.Source
	}
	if *codexPath == "" && cfg.CodexPath != "" {
		*codexPath = cfg.CodexPath
	}

	// Validate source flag.
	switch *source {
	case "all", "claude", "codex":
		// valid
	default:
		fmt.Fprintf(os.Stderr, "invalid --source value %q: must be all, claude, or codex\n", *source)
		os.Exit(1)
	}

	// Resolve default Claude data path (only needed when source includes claude).
	if *dataPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot determine home directory: %v\n", err)
			os.Exit(1)
		}
		*dataPath = filepath.Join(home, ".claude", "projects")
	}

	// Verify Claude data path when it's needed (not codex-only mode).
	if *source != "codex" {
		if _, err := os.Stat(*dataPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "data path does not exist: %s\n", *dataPath)
			os.Exit(1)
		}
	}

	model := ui.NewModel(*planName, *dataPath, *source, *codexPath)
	prog := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error running monitor: %v\n", err)
		os.Exit(1)
	}
}
