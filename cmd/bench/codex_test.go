//go:build ignore

package main

import (
	"fmt"
	"github.com/a2d2-dev/claude-usage-monitor/internal/data"
)

func main() {
	entries, err := data.LoadCodexEntries("")
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}
	fmt.Printf("Loaded %d Codex entries\n", len(entries))
	if len(entries) > 0 {
		e := entries[len(entries)-1]
		fmt.Printf("Latest: %s model=%s in=%d out=%d source=%s\n",
			e.Timestamp.Format("2006-01-02"), e.Model, e.InputTokens, e.OutputTokens, e.Source)
	}
	for i, e := range entries {
		if i >= 5 { break }
		fmt.Printf("  [%d] %s model=%q in=%d out=%d\n", i, e.Timestamp.Format("2006-01-02 15:04"), e.Model, e.InputTokens, e.OutputTokens)
	}
}
