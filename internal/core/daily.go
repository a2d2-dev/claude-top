package core

import (
	"sort"
	"time"

	"github.com/a2d2-dev/claude-usage-monitor/internal/data"
)

// dayKey uniquely identifies a calendar day in local time.
type dayKey struct {
	year  int
	month time.Month
	day   int
}

// BuildDailyStats aggregates all session block entries into per-day statistics,
// sorted chronologically (oldest first).
func BuildDailyStats(blocks []data.SessionBlock) []data.DailyStats {
	byDay := make(map[dayKey]*data.DailyStats)

	for i := range blocks {
		if blocks[i].IsGap {
			continue
		}
		for _, e := range blocks[i].Entries {
			t := e.Timestamp.Local()
			k := dayKey{t.Year(), t.Month(), t.Day()}
			ds, ok := byDay[k]
			if !ok {
				ds = &data.DailyStats{
					Date: time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()),
				}
				byDay[k] = ds
			}
			ds.TokenCounts.InputTokens += e.InputTokens
			ds.TokenCounts.OutputTokens += e.OutputTokens
			ds.TokenCounts.CacheCreationTokens += e.CacheCreationTokens
			ds.TokenCounts.CacheReadTokens += e.CacheReadTokens
			ds.CostUSD += e.CostUSD
			ds.MessageCount++
		}
	}

	result := make([]data.DailyStats, 0, len(byDay))
	for _, ds := range byDay {
		result = append(result, *ds)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Date.Before(result[j].Date)
	})
	return result
}
