// Package upload provides data aggregation and HTTP upload functionality for
// submitting monthly usage statistics to the claude-top leaderboard backend.
package upload

import (
	"fmt"
	"time"

	"github.com/a2d2-dev/claude-usage-monitor/internal/data"
)

// MonthlyStats holds aggregated usage statistics for a single calendar month.
type MonthlyStats struct {
	// Period is the month in YYYY-MM format.
	Period string
	// TotalCostUSD is the sum of all session costs for the month.
	TotalCostUSD float64
	// InputTokens is the sum of all input tokens.
	InputTokens int
	// OutputTokens is the sum of all output tokens.
	OutputTokens int
	// CacheReadTokens is the sum of all cache read tokens.
	CacheReadTokens int
	// CacheWriteTokens is the sum of all cache creation tokens.
	CacheWriteTokens int
	// SessionCount is the number of completed session blocks in the month.
	SessionCount int
	// ModelBreakdown maps normalised model name to per-model stats.
	ModelBreakdown map[string]*ModelMonthlyStats
}

// TotalTokens returns the sum of all token types.
func (s *MonthlyStats) TotalTokens() int {
	return s.InputTokens + s.OutputTokens + s.CacheReadTokens + s.CacheWriteTokens
}

// ModelMonthlyStats holds per-model aggregated stats within a month.
type ModelMonthlyStats struct {
	// CostUSD is the total cost for this model.
	CostUSD float64
	// InputTokens is the number of input tokens for this model.
	InputTokens int
	// OutputTokens is the number of output tokens for this model.
	OutputTokens int
	// CacheReadTokens for this model.
	CacheReadTokens int
	// CacheWriteTokens for this model.
	CacheWriteTokens int
	// MessageCount is the number of assistant messages using this model.
	MessageCount int
}

// AggregateCurrentMonth computes MonthlyStats for the current calendar month
// from the provided session blocks. Gap and active blocks are excluded.
//
// Returns an error if blocks is nil; an empty MonthlyStats is valid (zero usage).
func AggregateCurrentMonth(blocks []data.SessionBlock) (*MonthlyStats, error) {
	if blocks == nil {
		return nil, fmt.Errorf("blocks must not be nil")
	}

	now := time.Now()
	period := now.Format("2006-01")
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthEnd := monthStart.AddDate(0, 1, 0)

	stats := &MonthlyStats{
		Period:         period,
		ModelBreakdown: make(map[string]*ModelMonthlyStats),
	}

	for i := range blocks {
		b := &blocks[i]
		// Skip gap and active blocks.
		if b.IsGap || b.IsActive {
			continue
		}
		// Include only blocks whose start time falls in the current month.
		if b.StartTime.Before(monthStart) || !b.StartTime.Before(monthEnd) {
			continue
		}

		stats.TotalCostUSD += b.CostUSD
		stats.InputTokens += b.TokenCounts.InputTokens
		stats.OutputTokens += b.TokenCounts.OutputTokens
		stats.CacheReadTokens += b.TokenCounts.CacheReadTokens
		stats.CacheWriteTokens += b.TokenCounts.CacheCreationTokens
		stats.SessionCount++

		// Accumulate per-model breakdown.
		for model, ms := range b.PerModelStats {
			if _, ok := stats.ModelBreakdown[model]; !ok {
				stats.ModelBreakdown[model] = &ModelMonthlyStats{}
			}
			m := stats.ModelBreakdown[model]
			m.CostUSD += ms.CostUSD
			m.InputTokens += ms.InputTokens
			m.OutputTokens += ms.OutputTokens
			m.CacheReadTokens += ms.CacheReadTokens
			m.CacheWriteTokens += ms.CacheCreationTokens
			m.MessageCount += ms.MessageCount
		}
	}

	return stats, nil
}
