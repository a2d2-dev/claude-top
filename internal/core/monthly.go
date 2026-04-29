package core

import (
	"sort"
	"time"

	"github.com/a2d2-dev/claude-usage-monitor/internal/data"
)

// monthKey uniquely identifies a calendar month in local time.
type monthKey struct {
	year  int
	month time.Month
}

// BuildMonthlyStats aggregates all session block entries into per-month statistics,
// sorted chronologically (oldest first). Each month tracks the number of distinct
// active days via DayCount.
func BuildMonthlyStats(blocks []data.SessionBlock) []data.MonthlyStats {
	byMonth := make(map[monthKey]*data.MonthlyStats)
	// activeDays tracks unique days per month to compute DayCount.
	activeDays := make(map[monthKey]map[int]struct{})

	for i := range blocks {
		if blocks[i].IsGap {
			continue
		}
		for _, e := range blocks[i].Entries {
			t := e.Timestamp.Local()
			mk := monthKey{t.Year(), t.Month()}
			ms, ok := byMonth[mk]
			if !ok {
				ms = &data.MonthlyStats{
					Date: time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location()),
				}
				byMonth[mk] = ms
				activeDays[mk] = make(map[int]struct{})
			}
			ms.TokenCounts.InputTokens += e.InputTokens
			ms.TokenCounts.OutputTokens += e.OutputTokens
			ms.TokenCounts.CacheCreationTokens += e.CacheCreationTokens
			ms.TokenCounts.CacheReadTokens += e.CacheReadTokens
			ms.CostUSD += e.CostUSD
			ms.MessageCount++
			activeDays[mk][t.Day()] = struct{}{}
		}
	}

	result := make([]data.MonthlyStats, 0, len(byMonth))
	for mk, ms := range byMonth {
		ms.DayCount = len(activeDays[mk])
		result = append(result, *ms)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Date.Before(result[j].Date)
	})
	return result
}
