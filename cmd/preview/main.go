// preview renders a single dashboard frame to stdout for visual inspection.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/a2d2-dev/claude-usage-monitor/internal/core"
	"github.com/a2d2-dev/claude-usage-monitor/internal/data"
	"github.com/a2d2-dev/claude-usage-monitor/internal/ui"
)

func main() {
	home, _ := os.UserHomeDir()
	dataPath := filepath.Join(home, ".claude", "projects")

	entries, err := data.LoadEntries(dataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load error: %v\n", err)
		os.Exit(1)
	}

	blocks := core.BuildSessionBlocks(entries)
	m := ui.NewModel("pro", dataPath, "claude", "")

	// Inject loaded data directly for preview (bypasses bubbletea).
	_ = blocks
	fmt.Print(m.View())
}
