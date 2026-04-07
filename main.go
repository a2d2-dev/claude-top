// claude-usage-monitor is a terminal UI for monitoring Claude Code token and cost usage.
// It reads JSONL session data from ~/.claude/projects and displays live stats.
//
// Usage:
//
//	claude-usage-monitor [--plan pro|max5|max20] [--data-path /path/to/projects]
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/a2d2-dev/claude-usage-monitor/internal/ui"
)

func main() {
	planName := flag.String("plan", "pro", "Subscription plan: pro, max5, max20")
	dataPath := flag.String("data-path", "", "Path to Claude projects dir (default: ~/.claude/projects)")
	flag.Parse()

	// Resolve default data path.
	if *dataPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot determine home directory: %v\n", err)
			os.Exit(1)
		}
		*dataPath = filepath.Join(home, ".claude", "projects")
	}

	// Verify data path exists.
	if _, err := os.Stat(*dataPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "data path does not exist: %s\n", *dataPath)
		os.Exit(1)
	}

	model := ui.NewModel(*planName, *dataPath)
	prog := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error running monitor: %v\n", err)
		os.Exit(1)
	}
}
