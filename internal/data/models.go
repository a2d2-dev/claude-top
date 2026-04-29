// Package data defines core data structures for Claude usage tracking.
package data

import "time"

// UsageEntry represents a single assistant message with token usage info.
type UsageEntry struct {
	// Timestamp is when the message was sent.
	Timestamp time.Time
	// Model is the Claude model used (e.g., "claude-opus-4-6").
	Model string
	// InputTokens is the number of input tokens consumed.
	InputTokens int
	// OutputTokens is the number of output tokens produced.
	OutputTokens int
	// CacheCreationTokens is tokens written to cache.
	CacheCreationTokens int
	// CacheReadTokens is tokens read from cache.
	CacheReadTokens int
	// CostUSD is the cost in USD (may be 0 if not provided).
	CostUSD float64
	// SessionID is the ID of the session this entry belongs to.
	SessionID string
	// MessageID is the unique message identifier.
	MessageID string
	// CWD is the working directory when the message was sent.
	CWD string
	// UserPrompt is the text of the user message that triggered this response.
	// Truncated to 200 chars. Empty if not available.
	UserPrompt string
	// Source identifies which tool produced this entry: "claude" or "codex".
	Source string
}

// TokenCounts aggregates token usage across multiple entries.
type TokenCounts struct {
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
}

// TotalTokens returns the sum of all token types.
func (t TokenCounts) TotalTokens() int {
	return t.InputTokens + t.OutputTokens + t.CacheCreationTokens + t.CacheReadTokens
}

// SessionBlock represents a 5-hour session window aggregating usage entries.
type SessionBlock struct {
	// ID uniquely identifies this block (start time ISO string).
	ID string
	// StartTime is when the block begins.
	StartTime time.Time
	// EndTime is when the block expires (StartTime + 5h).
	EndTime time.Time
	// ActualEndTime is the timestamp of the last entry in this block.
	ActualEndTime *time.Time
	// Entries holds all usage entries in this block.
	Entries []UsageEntry
	// TokenCounts aggregates tokens across all entries.
	TokenCounts TokenCounts
	// CostUSD is the total cost for this block.
	CostUSD float64
	// IsActive indicates the block window is still open.
	IsActive bool
	// IsGap indicates this is a synthetic gap block (no activity).
	IsGap bool
	// Models lists unique models used in this block.
	Models []string
	// PerModelStats maps model name to per-model aggregated stats.
	PerModelStats map[string]*ModelStats
	// MessageCount is the number of entries/messages in this block.
	MessageCount int
	// Directory is the primary working directory for this session block.
	// Computed as the most frequently seen CWD across all entries.
	Directory string
	// Source is the dominant data source among this block's entries ("claude" or "codex").
	Source string
}

// ModelStats holds per-model token and cost aggregates within a session.
type ModelStats struct {
	// InputTokens for this model in the session.
	InputTokens int
	// OutputTokens for this model.
	OutputTokens int
	// CacheCreationTokens for this model.
	CacheCreationTokens int
	// CacheReadTokens for this model.
	CacheReadTokens int
	// CostUSD for this model.
	CostUSD float64
	// MessageCount is the number of messages using this model.
	MessageCount int
}

// TotalTokens returns the total token count for this model.
func (m *ModelStats) TotalTokens() int {
	return m.InputTokens + m.OutputTokens + m.CacheCreationTokens + m.CacheReadTokens
}

// DailyStats aggregates token usage and cost for a single calendar day.
type DailyStats struct {
	// Date is truncated to midnight local time.
	Date         time.Time
	TokenCounts  TokenCounts
	CostUSD      float64
	MessageCount int
}

// MonthlyStats aggregates token usage and cost for a single calendar month.
type MonthlyStats struct {
	// Date is the first day of the month, truncated to midnight local time.
	Date         time.Time
	TokenCounts  TokenCounts
	CostUSD      float64
	MessageCount int
	// DayCount is the number of distinct active days within the month.
	DayCount int
}

// DurationMinutes returns the block's duration in minutes.
func (b *SessionBlock) DurationMinutes() float64 {
	end := b.EndTime
	if b.ActualEndTime != nil {
		end = *b.ActualEndTime
	}
	d := end.Sub(b.StartTime).Minutes()
	if d < 1 {
		return 1
	}
	return d
}
