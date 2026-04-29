package core

import (
	"testing"
	"time"

	"github.com/a2d2-dev/claude-usage-monitor/internal/data"
)

// makeEntry creates a UsageEntry at the given time with specified token counts and cost.
func makeEntry(ts time.Time, input, output, cacheRead, cacheWrite int, cost float64) data.UsageEntry {
	return data.UsageEntry{
		Timestamp:           ts,
		InputTokens:         input,
		OutputTokens:        output,
		CacheReadTokens:     cacheRead,
		CacheCreationTokens: cacheWrite,
		CostUSD:             cost,
	}
}

// TestBuildMonthlyStats_Empty verifies that empty input returns empty output.
func TestBuildMonthlyStats_Empty(t *testing.T) {
	result := BuildMonthlyStats(nil)
	if len(result) != 0 {
		t.Fatalf("expected 0 months, got %d", len(result))
	}
}

// TestBuildMonthlyStats_SingleMonth verifies aggregation within one month.
func TestBuildMonthlyStats_SingleMonth(t *testing.T) {
	loc := time.Local
	blocks := []data.SessionBlock{
		{
			Entries: []data.UsageEntry{
				makeEntry(time.Date(2025, 3, 1, 10, 0, 0, 0, loc), 100, 50, 10, 5, 0.10),
				makeEntry(time.Date(2025, 3, 1, 14, 0, 0, 0, loc), 200, 100, 20, 10, 0.20),
				makeEntry(time.Date(2025, 3, 15, 9, 0, 0, 0, loc), 300, 150, 30, 15, 0.30),
			},
		},
	}

	result := BuildMonthlyStats(blocks)
	if len(result) != 1 {
		t.Fatalf("expected 1 month, got %d", len(result))
	}

	ms := result[0]
	if ms.Date.Month() != time.March || ms.Date.Year() != 2025 {
		t.Errorf("expected March 2025, got %v", ms.Date)
	}
	if ms.MessageCount != 3 {
		t.Errorf("expected 3 messages, got %d", ms.MessageCount)
	}
	if ms.DayCount != 2 {
		t.Errorf("expected 2 active days, got %d", ms.DayCount)
	}
	if ms.TokenCounts.InputTokens != 600 {
		t.Errorf("expected 600 input tokens, got %d", ms.TokenCounts.InputTokens)
	}
	expectedCost := 0.60
	if ms.CostUSD < expectedCost-0.001 || ms.CostUSD > expectedCost+0.001 {
		t.Errorf("expected cost ~%.2f, got %.4f", expectedCost, ms.CostUSD)
	}
}

// TestBuildMonthlyStats_MultipleMonths verifies sorting and multi-month aggregation.
func TestBuildMonthlyStats_MultipleMonths(t *testing.T) {
	loc := time.Local
	blocks := []data.SessionBlock{
		{
			Entries: []data.UsageEntry{
				makeEntry(time.Date(2025, 1, 5, 10, 0, 0, 0, loc), 100, 50, 0, 0, 0.05),
				makeEntry(time.Date(2025, 3, 10, 10, 0, 0, 0, loc), 200, 100, 0, 0, 0.10),
				makeEntry(time.Date(2025, 1, 20, 10, 0, 0, 0, loc), 150, 75, 0, 0, 0.08),
			},
		},
	}

	result := BuildMonthlyStats(blocks)
	if len(result) != 2 {
		t.Fatalf("expected 2 months, got %d", len(result))
	}

	// Should be sorted oldest first: January, then March.
	if result[0].Date.Month() != time.January {
		t.Errorf("expected first month January, got %v", result[0].Date.Month())
	}
	if result[1].Date.Month() != time.March {
		t.Errorf("expected second month March, got %v", result[1].Date.Month())
	}

	// January: 2 entries, 2 distinct days.
	if result[0].MessageCount != 2 {
		t.Errorf("Jan: expected 2 messages, got %d", result[0].MessageCount)
	}
	if result[0].DayCount != 2 {
		t.Errorf("Jan: expected 2 days, got %d", result[0].DayCount)
	}

	// March: 1 entry, 1 day.
	if result[1].MessageCount != 1 {
		t.Errorf("Mar: expected 1 message, got %d", result[1].MessageCount)
	}
	if result[1].DayCount != 1 {
		t.Errorf("Mar: expected 1 day, got %d", result[1].DayCount)
	}
}

// TestBuildMonthlyStats_SkipsGapBlocks verifies that gap blocks are excluded.
func TestBuildMonthlyStats_SkipsGapBlocks(t *testing.T) {
	loc := time.Local
	blocks := []data.SessionBlock{
		{
			IsGap: true,
			Entries: []data.UsageEntry{
				makeEntry(time.Date(2025, 2, 1, 10, 0, 0, 0, loc), 100, 50, 0, 0, 0.05),
			},
		},
		{
			Entries: []data.UsageEntry{
				makeEntry(time.Date(2025, 2, 15, 10, 0, 0, 0, loc), 200, 100, 0, 0, 0.10),
			},
		},
	}

	result := BuildMonthlyStats(blocks)
	if len(result) != 1 {
		t.Fatalf("expected 1 month, got %d", len(result))
	}
	if result[0].MessageCount != 1 {
		t.Errorf("expected 1 message (gap excluded), got %d", result[0].MessageCount)
	}
}

// TestBuildMonthlyStats_DayCountSameDay verifies that multiple entries on the same day
// count as one active day.
func TestBuildMonthlyStats_DayCountSameDay(t *testing.T) {
	loc := time.Local
	blocks := []data.SessionBlock{
		{
			Entries: []data.UsageEntry{
				makeEntry(time.Date(2025, 4, 10, 8, 0, 0, 0, loc), 100, 50, 0, 0, 0.05),
				makeEntry(time.Date(2025, 4, 10, 12, 0, 0, 0, loc), 100, 50, 0, 0, 0.05),
				makeEntry(time.Date(2025, 4, 10, 18, 0, 0, 0, loc), 100, 50, 0, 0, 0.05),
			},
		},
	}

	result := BuildMonthlyStats(blocks)
	if len(result) != 1 {
		t.Fatalf("expected 1 month, got %d", len(result))
	}
	if result[0].DayCount != 1 {
		t.Errorf("expected 1 active day, got %d", result[0].DayCount)
	}
	if result[0].MessageCount != 3 {
		t.Errorf("expected 3 messages, got %d", result[0].MessageCount)
	}
}
