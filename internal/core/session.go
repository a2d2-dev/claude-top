package core

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/a2d2-dev/claude-usage-monitor/internal/data"
)

const sessionDuration = 5 * time.Hour

// BuildSessionBlocks groups usage entries into 5-hour session blocks.
// Entries are first partitioned by (CWD, source) so each project gets its own
// independent block sequence, then all blocks are merged and sorted by StartTime.
// Entries must be sorted chronologically before calling this function.
func BuildSessionBlocks(entries []data.UsageEntry) []data.SessionBlock {
	if len(entries) == 0 {
		return nil
	}

	// Pre-compute cost for every entry before grouping (entries are copied into
	// per-group slices, so set CostUSD on the originals first).
	for i := range entries {
		entries[i].CostUSD = computeEntryCost(&entries[i])
	}

	// Group entries by (CWD, source) so each project gets its own rows.
	type groupKey struct {
		cwd    string
		source string
	}
	groups := make(map[groupKey][]data.UsageEntry)
	// groupOrder preserves first-seen order for deterministic output.
	var groupOrder []groupKey
	seen := make(map[groupKey]bool)
	for _, e := range entries {
		key := groupKey{cwd: e.CWD, source: e.Source}
		if !seen[key] {
			seen[key] = true
			groupOrder = append(groupOrder, key)
		}
		groups[key] = append(groups[key], e)
	}

	// Build independent block sequences for each (CWD, source) group.
	var allBlocks []data.SessionBlock
	for _, key := range groupOrder {
		allBlocks = append(allBlocks, buildProjectBlocks(groups[key])...)
	}

	// Sort merged blocks chronologically for display.
	sort.Slice(allBlocks, func(i, j int) bool {
		return allBlocks[i].StartTime.Before(allBlocks[j].StartTime)
	})

	markActiveBlocks(allBlocks)
	return allBlocks
}

// buildProjectBlocks builds 5-hour session blocks for a single (CWD, source) group.
// Entries must be sorted chronologically.
func buildProjectBlocks(entries []data.UsageEntry) []data.SessionBlock {
	if len(entries) == 0 {
		return nil
	}

	var blocks []data.SessionBlock
	var current *data.SessionBlock
	cwdFreq := make(map[string]int)

	for i := range entries {
		entry := &entries[i]

		if current == nil || needsNewBlock(current, entry) {
			if current != nil {
				finalizeBlock(current, cwdFreq)
				blocks = append(blocks, *current)
				cwdFreq = make(map[string]int)

				// Insert a gap block between pauses within the same project.
				if gap := buildGap(current, entry); gap != nil {
					blocks = append(blocks, *gap)
				}
			}
			current = newBlock(entry)
		}

		addEntryToBlock(current, entry)
		if entry.CWD != "" {
			cwdFreq[entry.CWD]++
		}
	}

	if current != nil {
		finalizeBlock(current, cwdFreq)
		blocks = append(blocks, *current)
	}

	return blocks
}

// computeEntryCost calculates cost for an entry from its token counts.
func computeEntryCost(e *data.UsageEntry) float64 {
	return CalculateCost(e.Model, e.InputTokens, e.OutputTokens, e.CacheCreationTokens, e.CacheReadTokens)
}

// needsNewBlock returns true when entry falls outside current block's time window,
// or when the entry comes from a different source tool (Claude vs Codex).
func needsNewBlock(block *data.SessionBlock, entry *data.UsageEntry) bool {
	if entry.Timestamp.After(block.EndTime) || entry.Timestamp.Equal(block.EndTime) {
		return true
	}
	if len(block.Entries) > 0 {
		last := block.Entries[len(block.Entries)-1]
		if entry.Timestamp.Sub(last.Timestamp) >= sessionDuration {
			return true
		}
		// Different source tools (claude vs codex) never share a block.
		if last.Source != entry.Source {
			return true
		}
	}
	return false
}

// newBlock creates a new SessionBlock starting at the rounded hour of the entry.
func newBlock(entry *data.UsageEntry) *data.SessionBlock {
	start := roundToHour(entry.Timestamp)
	end := start.Add(sessionDuration)
	// Include source and CWD in ID so same-hour blocks from different projects
	// or tools are distinct and don't collide.
	id := start.Format(time.RFC3339)
	if entry.Source != "" {
		id += ":" + entry.Source
	}
	if entry.CWD != "" {
		id += ":" + entry.CWD
	}
	return &data.SessionBlock{
		ID:            id,
		StartTime:     start,
		EndTime:       end,
		PerModelStats: make(map[string]*data.ModelStats),
	}
}

// addEntryToBlock aggregates an entry's tokens and cost into the block.
func addEntryToBlock(block *data.SessionBlock, entry *data.UsageEntry) {
	block.Entries = append(block.Entries, *entry)

	model := normalizeModel(entry.Model)
	stats := block.PerModelStats[model]
	if stats == nil {
		stats = &data.ModelStats{}
		block.PerModelStats[model] = stats
		block.Models = append(block.Models, model)
	}

	stats.InputTokens += entry.InputTokens
	stats.OutputTokens += entry.OutputTokens
	stats.CacheCreationTokens += entry.CacheCreationTokens
	stats.CacheReadTokens += entry.CacheReadTokens
	stats.CostUSD += entry.CostUSD
	stats.MessageCount++

	block.TokenCounts.InputTokens += entry.InputTokens
	block.TokenCounts.OutputTokens += entry.OutputTokens
	block.TokenCounts.CacheCreationTokens += entry.CacheCreationTokens
	block.TokenCounts.CacheReadTokens += entry.CacheReadTokens
	block.CostUSD += entry.CostUSD
	block.MessageCount++
}

// finalizeBlock sets the actual end time, primary directory, and dominant source on the block.
func finalizeBlock(block *data.SessionBlock, cwdFreq map[string]int) {
	if len(block.Entries) > 0 {
		t := block.Entries[len(block.Entries)-1].Timestamp
		block.ActualEndTime = &t
	}
	block.MessageCount = len(block.Entries)
	block.Directory = modalCWD(cwdFreq)
	block.Source = dominantSource(block.Entries)
}

// dominantSource returns "claude" or "codex" based on the majority of entries.
// Defaults to "claude" on tie or empty slice.
func dominantSource(entries []data.UsageEntry) string {
	counts := map[string]int{}
	for _, e := range entries {
		counts[e.Source]++
	}
	if counts["codex"] > counts["claude"] {
		return "codex"
	}
	return "claude"
}

// modalCWD returns the most frequently seen working directory from cwdFreq.
func modalCWD(freq map[string]int) string {
	var best string
	var bestN int
	for cwd, n := range freq {
		if n > bestN {
			bestN = n
			best = cwd
		}
	}
	return best
}

// buildGap creates a synthetic gap block between two real blocks if needed.
func buildGap(last *data.SessionBlock, nextEntry *data.UsageEntry) *data.SessionBlock {
	if last.ActualEndTime == nil {
		return nil
	}
	gap := nextEntry.Timestamp.Sub(*last.ActualEndTime)
	if gap < sessionDuration {
		return nil
	}
	gapEnd := nextEntry.Timestamp
	return &data.SessionBlock{
		ID:            fmt.Sprintf("gap-%s", last.ActualEndTime.Format(time.RFC3339)),
		StartTime:     *last.ActualEndTime,
		EndTime:       gapEnd,
		IsGap:         true,
		PerModelStats: make(map[string]*data.ModelStats),
	}
}

// markActiveBlocks flags session blocks whose window has not yet expired.
func markActiveBlocks(blocks []data.SessionBlock) {
	now := time.Now().UTC()
	for i := range blocks {
		if !blocks[i].IsGap && blocks[i].EndTime.After(now) {
			blocks[i].IsActive = true
		}
	}
}

// roundToHour truncates a UTC time to the whole hour.
func roundToHour(t time.Time) time.Time {
	return t.UTC().Truncate(time.Hour)
}

// normalizeModel maps a raw model string to a short canonical name.
func normalizeModel(model string) string {
	lower := strings.ToLower(model)
	switch {
	case strings.Contains(lower, "opus"):
		return extractModelShortName(lower, "opus")
	case strings.Contains(lower, "sonnet"):
		return extractModelShortName(lower, "sonnet")
	case strings.Contains(lower, "haiku"):
		return extractModelShortName(lower, "haiku")
	default:
		if model == "" {
			return "unknown"
		}
		return model
	}
}

// capitalise uppercases the first letter of a string (ASCII only).
func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// extractModelShortName returns a readable short name like "Opus 4.6" or "Haiku 3.5".
func extractModelShortName(lower, tier string) string {
	title := capitalise(tier)
	// Claude 4 family: claude-opus-4-6, claude-sonnet-4-6, claude-haiku-4-5
	if strings.Contains(lower, tier+"-4-6") || strings.Contains(lower, "4.6") {
		return title + " 4.6"
	}
	if strings.Contains(lower, tier+"-4-5") || strings.Contains(lower, "4.5") {
		return title + " 4.5"
	}
	if strings.Contains(lower, tier+"-4-") {
		return title + " 4"
	}
	// Claude 3.5 family
	if strings.Contains(lower, "3-5") || strings.Contains(lower, "3.5") {
		return title + " 3.5"
	}
	// Claude 3 family
	if strings.Contains(lower, "3-") || strings.Contains(lower, "3.") {
		return title + " 3"
	}
	return title
}
