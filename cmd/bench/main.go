package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/a2d2-dev/claude-usage-monitor/internal/core"
	"github.com/a2d2-dev/claude-usage-monitor/internal/data"
	"github.com/a2d2-dev/claude-usage-monitor/internal/ui"
)

func main() {
	home, _ := os.UserHomeDir()
	dataPath := filepath.Join(home, ".claude", "projects")

	t0 := time.Now()
	cached, _ := data.LoadCached()
	t1 := time.Now()
	fmt.Printf("LoadCached:         %8v  (%d entries)\n", t1.Sub(t0).Round(time.Millisecond), len(cached))

	t2 := time.Now()
	entries, err := data.LoadEntries(dataPath)
	t3 := time.Now()
	fmt.Printf("LoadEntries:        %8v  (%d entries, err=%v)\n", t3.Sub(t2).Round(time.Millisecond), len(entries), err)

	t4 := time.Now()
	blocks := core.BuildSessionBlocks(entries)
	t5 := time.Now()
	fmt.Printf("BuildSessionBlocks: %8v  (%d blocks)\n", t5.Sub(t4).Round(time.Millisecond), len(blocks))

	// Find largest completed session.
	maxIdx, maxCount := 0, 0
	for i, b := range blocks {
		if len(b.Entries) > maxCount && !b.IsGap && !b.IsActive {
			maxCount = len(b.Entries)
			maxIdx = i
		}
	}
	if maxCount == 0 {
		fmt.Println("no completed sessions found")
		return
	}
	b := blocks[maxIdx]
	fmt.Printf("\nLargest session: %d entries, cost=$%.4f\n  dir=%s\n", maxCount, b.CostUSD, b.Directory)

	// Render detail panel at standard size (220 cols × 50 rows).
	const W, H = 220, 50
	frame := ui.RenderDetailPanelForTest(b, W, H)

	// Count actual lines rendered (strip ANSI for counting).
	lineCount := strings.Count(frame, "\n") + 1
	fmt.Printf("\nRendered detail panel: %d lines (terminal height=%d, content area=%d)\n", lineCount, H, H-3)
	fmt.Println(strings.Repeat("─", 80))
	fmt.Println(frame)
	fmt.Println(strings.Repeat("─", 80))
}
