package upload

import (
	"testing"
	"time"

	"github.com/a2d2-dev/claude-usage-monitor/internal/data"
)

// TestAggregateCurrentMonth verifies that only current-month non-gap non-active
// blocks are included in the monthly aggregate.
func TestAggregateCurrentMonth(t *testing.T) {
	now := time.Now()
	lastMonth := now.AddDate(0, -1, 0)

	blocks := []data.SessionBlock{
		// Current month, completed.
		{
			StartTime:    now.Add(-2 * time.Hour),
			CostUSD:      1.50,
			MessageCount: 3,
			TokenCounts: data.TokenCounts{
				InputTokens:         1000,
				OutputTokens:        500,
				CacheReadTokens:     200,
				CacheCreationTokens: 100,
			},
			PerModelStats: map[string]*data.ModelStats{
				"claude-sonnet-4-6": {CostUSD: 1.50, InputTokens: 1000, OutputTokens: 500, MessageCount: 3},
			},
		},
		// Last month — must be excluded.
		{
			StartTime: lastMonth,
			CostUSD:   5.00,
			TokenCounts: data.TokenCounts{
				InputTokens: 9999,
			},
		},
		// Gap block — must be excluded.
		{
			StartTime: now.Add(-1 * time.Hour),
			IsGap:     true,
			CostUSD:   0.50,
		},
		// Active block — must be excluded.
		{
			StartTime: now.Add(-30 * time.Minute),
			IsActive:  true,
			CostUSD:   0.20,
		},
	}

	stats, err := AggregateCurrentMonth(blocks, "all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.Period != now.Format("2006-01") {
		t.Errorf("Period = %q, want %q", stats.Period, now.Format("2006-01"))
	}
	if stats.TotalCostUSD != 1.50 {
		t.Errorf("TotalCostUSD = %.2f, want 1.50", stats.TotalCostUSD)
	}
	if stats.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", stats.InputTokens)
	}
	if stats.SessionCount != 1 {
		t.Errorf("SessionCount = %d, want 1", stats.SessionCount)
	}
	if _, ok := stats.ModelBreakdown["claude-sonnet-4-6"]; !ok {
		t.Error("expected claude-sonnet-4-6 in ModelBreakdown")
	}
}

func TestAggregateCurrentMonthNilError(t *testing.T) {
	_, err := AggregateCurrentMonth(nil, "all")
	if err == nil {
		t.Error("expected error for nil blocks")
	}
}

func TestAggregateCurrentMonthEmpty(t *testing.T) {
	stats, err := AggregateCurrentMonth([]data.SessionBlock{}, "all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.TotalCostUSD != 0 || stats.SessionCount != 0 {
		t.Error("expected zero stats for empty blocks")
	}
}

// TestAggregateCurrentMonthSourceFilter verifies that source filtering works.
func TestAggregateCurrentMonthSourceFilter(t *testing.T) {
	now := time.Now()
	blocks := []data.SessionBlock{
		{
			StartTime: now.Add(-2 * time.Hour),
			Source:    "claude",
			CostUSD:   1.00,
			TokenCounts: data.TokenCounts{InputTokens: 100},
			PerModelStats: map[string]*data.ModelStats{},
		},
		{
			StartTime: now.Add(-1 * time.Hour),
			Source:    "codex",
			CostUSD:   2.00,
			TokenCounts: data.TokenCounts{InputTokens: 200},
			PerModelStats: map[string]*data.ModelStats{},
		},
	}

	// Filter to claude only.
	claudeStats, err := AggregateCurrentMonth(blocks, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if claudeStats.TotalCostUSD != 1.00 {
		t.Errorf("claude filter: TotalCostUSD = %.2f, want 1.00", claudeStats.TotalCostUSD)
	}
	if claudeStats.SessionCount != 1 {
		t.Errorf("claude filter: SessionCount = %d, want 1", claudeStats.SessionCount)
	}

	// Filter to codex only.
	codexStats, err := AggregateCurrentMonth(blocks, "codex")
	if err != nil {
		t.Fatal(err)
	}
	if codexStats.TotalCostUSD != 2.00 {
		t.Errorf("codex filter: TotalCostUSD = %.2f, want 2.00", codexStats.TotalCostUSD)
	}

	// "all" includes both.
	allStats, err := AggregateCurrentMonth(blocks, "all")
	if err != nil {
		t.Fatal(err)
	}
	if allStats.TotalCostUSD != 3.00 {
		t.Errorf("all filter: TotalCostUSD = %.2f, want 3.00", allStats.TotalCostUSD)
	}
}
