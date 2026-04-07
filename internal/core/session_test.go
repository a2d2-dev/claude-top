package core

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/a2d2-dev/claude-usage-monitor/internal/data"
)

func TestBuildSessionBlocks_RealData(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot get home dir: %v", err)
	}
	dataPath := filepath.Join(home, ".claude", "projects")
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		t.Skip("no Claude projects data found")
	}

	entries, err := data.LoadEntries(dataPath)
	if err != nil {
		t.Fatalf("LoadEntries: %v", err)
	}

	blocks := BuildSessionBlocks(entries)
	t.Logf("Built %d session blocks from %d entries", len(blocks), len(entries))

	var activeCnt, gapCnt, normalCnt int
	var totalCost float64
	for _, b := range blocks {
		switch {
		case b.IsActive:
			activeCnt++
		case b.IsGap:
			gapCnt++
		default:
			normalCnt++
		}
		totalCost += b.CostUSD
	}

	t.Logf("Active: %d, Gap: %d, Normal: %d", activeCnt, gapCnt, normalCnt)
	t.Logf("Total cost across all sessions: $%.4f", totalCost)

	// Verify block invariants.
	for i, b := range blocks {
		if !b.IsGap {
			if b.StartTime.IsZero() {
				t.Errorf("block %d has zero start time", i)
			}
			if b.EndTime.Before(b.StartTime) {
				t.Errorf("block %d end before start", i)
			}
		}
		if b.CostUSD < 0 {
			t.Errorf("block %d has negative cost: %.6f", i, b.CostUSD)
		}
	}

	// Print summary of last 3 blocks.
	start := len(blocks) - 3
	if start < 0 {
		start = 0
	}
	for _, b := range blocks[start:] {
		if b.IsGap {
			fmt.Printf("  [GAP]  %s → %s\n", b.StartTime.Local().Format("01-02 15:04"), b.EndTime.Local().Format("15:04"))
		} else {
			fmt.Printf("  [SESS] %s active=%-5v msgs=%3d tokens=%7d cost=$%.4f\n",
				b.StartTime.Local().Format("01-02 15:04"),
				b.IsActive, b.MessageCount,
				b.TokenCounts.TotalTokens(), b.CostUSD,
			)
		}
	}
}

func TestNormalizeModel(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"claude-opus-4-6", "Opus 4.6"},
		{"claude-sonnet-4-6", "Sonnet 4.6"},
		{"claude-haiku-4-5-20251001", "Haiku 4.5"},
		{"claude-opus-4-5-20251101", "Opus 4.5"},
		{"claude-3-5-sonnet-20241022", "Sonnet 3.5"},
		{"claude-3-haiku-20240307", "Haiku 3"},
		{"unknown-model", "unknown-model"},
		{"", "unknown"},
	}
	for _, tc := range cases {
		got := normalizeModel(tc.input)
		if got != tc.want {
			t.Errorf("normalizeModel(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
