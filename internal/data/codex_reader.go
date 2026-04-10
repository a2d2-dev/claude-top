// Package data provides JSONL file reading and entry parsing for Codex CLI usage data.
package data

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// codexEvent represents a single line in a Codex JSONL session file.
type codexEvent struct {
	// Timestamp is an ISO 8601 timestamp string at the top level.
	Timestamp string          `json:"timestamp"`
	// Type is the event type: "session_meta", "turn_context", "event_msg", "response_item".
	Type      string          `json:"type"`
	// Payload holds the event-specific data as raw JSON for flexible parsing.
	Payload   json.RawMessage `json:"payload"`
}

// codexSessionMeta is the payload for "session_meta" events.
type codexSessionMeta struct {
	// ID is the unique session identifier.
	ID string `json:"id"`
	// CWD is the working directory where Codex was launched.
	CWD string `json:"cwd"`
}

// codexTurnContext is the payload for "turn_context" events.
type codexTurnContext struct {
	// Model is the AI model used for this turn.
	Model string `json:"model"`
}

// codexEventMsgPayload is the payload for "event_msg" wrapper events.
// The inner SubType field distinguishes user_message, token_count, etc.
type codexEventMsgPayload struct {
	// SubType is the inner event type: "user_message", "token_count", etc.
	SubType string `json:"type"`
	// Message holds the user text for "user_message" sub-type.
	Message string `json:"message"`
	// Info holds token usage for "token_count" sub-type.
	Info json.RawMessage `json:"info"`
}

// codexLastTokenUsage holds token usage totals from a token_count event.
type codexLastTokenUsage struct {
	// InputTokens is the number of prompt/input tokens.
	InputTokens int `json:"input_tokens"`
	// CachedInputTokens is the number of cached prompt tokens.
	CachedInputTokens int `json:"cached_input_tokens"`
	// OutputTokens is the number of generated output tokens.
	OutputTokens int `json:"output_tokens"`
	// ReasoningOutputTokens is the number of reasoning tokens (counted as output).
	ReasoningOutputTokens int `json:"reasoning_output_tokens"`
}

// codexTokenCountInfo is the "info" field inside a token_count event.
type codexTokenCountInfo struct {
	// LastTokenUsage is the cumulative token usage snapshot for deduplication.
	LastTokenUsage *codexLastTokenUsage `json:"last_token_usage"`
}

// codexParseState holds parser state while processing a single Codex JSONL file.
// It implements a state machine over the sequence of events.
type codexParseState struct {
	// currentModel is the model name from the most recent turn_context event.
	currentModel string
	// currentSession is the session ID from the most recent session_meta event.
	currentSession string
	// currentCWD is the working directory from session_meta (where Codex was launched).
	currentCWD string
	// currentPrompt is the user prompt from the most recent user_message event (truncated).
	currentPrompt string
	// pendingEntry holds the latest token_count snapshot for the current AI turn.
	// It is flushed (emitted) when the next user_message arrives or at end-of-file.
	// This ensures only the FINAL snapshot per turn is recorded, not every streaming update.
	pendingEntry *UsageEntry
	// fileDateFallback is midnight UTC derived from the YYYY/MM/DD directory structure.
	fileDateFallback time.Time
}

// parseCodexFile reads a single Codex JSONL file and returns usage entries.
// Only the final token_count snapshot per AI turn is emitted (not streaming intermediates).
// Returns an empty slice (not an error) if the file contains no usable entries.
func parseCodexFile(path string) ([]UsageEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	state := &codexParseState{
		fileDateFallback: deriveDateFromPath(path),
	}

	var entries []UsageEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var evt codexEvent
		if err := json.Unmarshal(line, &evt); err != nil {
			continue
		}
		entry, ok := processCodexEvent(evt, state, path)
		if ok {
			entries = append(entries, entry)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Flush any pending entry from the last AI turn in the file.
	if state.pendingEntry != nil {
		entries = append(entries, *state.pendingEntry)
	}

	return entries, nil
}

// processCodexEvent updates the parse state based on the event type.
//
// Event ordering in a Codex session file:
//   user_message → turn_context → token_count (repeating, streaming) → user_message → …
//
// Strategy: buffer the latest token_count snapshot in state.pendingEntry (overwriting
// each intermediate value). Flush (emit) the pending entry only when a new user_message
// arrives — that signals the previous AI turn is complete. The final turn in the file
// is flushed by parseCodexFile after the scanner loop ends.
func processCodexEvent(evt codexEvent, state *codexParseState, filePath string) (UsageEntry, bool) {
	switch evt.Type {
	case "session_meta":
		var meta codexSessionMeta
		if err := json.Unmarshal(evt.Payload, &meta); err == nil {
			if meta.ID != "" {
				state.currentSession = meta.ID
			}
			if meta.CWD != "" {
				state.currentCWD = meta.CWD
			}
		}

	case "turn_context":
		var ctx codexTurnContext
		if err := json.Unmarshal(evt.Payload, &ctx); err == nil && ctx.Model != "" {
			state.currentModel = ctx.Model
		}

	case "event_msg":
		// event_msg is a wrapper; dispatch on the inner sub-type.
		var inner codexEventMsgPayload
		if err := json.Unmarshal(evt.Payload, &inner); err != nil {
			return UsageEntry{}, false
		}
		switch inner.SubType {
		case "user_message":
			// A new user turn begins. Flush the previous AI turn's pending entry first.
			flushed := state.pendingEntry
			state.pendingEntry = nil
			if inner.Message != "" {
				state.currentPrompt = truncateCodexPrompt(inner.Message)
			}
			if flushed != nil {
				return *flushed, true
			}
		case "token_count":
			// Overwrite pending entry with the latest snapshot for this AI turn.
			// All streaming intermediates are discarded; only the last one will be emitted.
			updatePendingEntry(inner.Info, evt.Timestamp, state, filePath)
		}
	}

	return UsageEntry{}, false
}

// updatePendingEntry parses a token_count event and stores it as state.pendingEntry,
// overwriting any previous intermediate value. Non-final streaming snapshots are silently
// discarded when the next snapshot arrives.
func updatePendingEntry(infoRaw json.RawMessage, timestamp string, state *codexParseState, filePath string) {
	if len(infoRaw) == 0 {
		return
	}
	var info codexTokenCountInfo
	if err := json.Unmarshal(infoRaw, &info); err != nil || info.LastTokenUsage == nil {
		return
	}
	usage := info.LastTokenUsage

	// Skip all-zero snapshots (no actual token activity).
	if usage.InputTokens == 0 && usage.OutputTokens == 0 &&
		usage.CachedInputTokens == 0 && usage.ReasoningOutputTokens == 0 {
		return
	}

	// Parse ISO 8601 timestamp; fall back to directory-derived date.
	ts, err := parseTimestamp(timestamp)
	if err != nil || ts.IsZero() {
		ts = state.fileDateFallback
	}

	sessionID := state.currentSession
	if sessionID == "" {
		sessionID = filepath.Base(filePath)
	}

	// Use the launch directory from session_meta; fall back to the JSONL file's parent dir.
	cwd := state.currentCWD
	if cwd == "" {
		cwd = filepath.Dir(filePath)
	}

	// OpenAI's input_tokens INCLUDES cached tokens, unlike Anthropic where
	// input_tokens excludes cache_read. Subtract cached portion so downstream
	// cost calculation charges non-cached input at full price and cached input
	// at the lower cache-read price (matching OpenAI billing semantics).
	nonCachedInput := usage.InputTokens - usage.CachedInputTokens
	if nonCachedInput < 0 {
		nonCachedInput = 0
	}

	state.pendingEntry = &UsageEntry{
		Timestamp:           ts,
		Model:               state.currentModel,
		InputTokens:         nonCachedInput,
		OutputTokens:        usage.OutputTokens + usage.ReasoningOutputTokens,
		CacheCreationTokens: 0,
		CacheReadTokens:     usage.CachedInputTokens,
		SessionID:           sessionID,
		MessageID:           buildCodexMessageID(sessionID, ts),
		CWD:                 cwd,
		UserPrompt:          state.currentPrompt,
		Source:              "codex",
	}
}

// deriveDateFromPath extracts midnight UTC from a YYYY/MM/DD directory structure.
// Returns zero time if the path doesn't match the expected pattern.
func deriveDateFromPath(path string) time.Time {
	parts := strings.Split(filepath.ToSlash(path), "/")
	// Look for three consecutive numeric segments that could be YYYY/MM/DD.
	for i := 0; i+2 < len(parts); i++ {
		candidate := parts[i] + "-" + parts[i+1] + "-" + parts[i+2]
		t, err := time.Parse("2006-01-02", candidate)
		if err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// buildCodexMessageID constructs a deterministic message ID from session and timestamp.
// Used to enable deduplication via deduplicationKey().
func buildCodexMessageID(sessionID string, ts time.Time) string {
	return "codex:" + sessionID + ":" + ts.Format(time.RFC3339Nano)
}

// truncateCodexPrompt returns the first 200 runes of s, appending "…" if longer.
func truncateCodexPrompt(s string) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= 200 {
		return s
	}
	return string(runes[:199]) + "…"
}

// LoadCodexEntries reads all Codex JSONL files under dataPath and returns sorted usage entries.
// dataPath defaults to ~/.codex/sessions if empty.
// Results are cached in ~/.cache/a2d2/claude-usage-monitor/codex.cache so only
// files whose modification time has changed are re-parsed.
// Returns (nil, nil) if the directory does not exist (graceful no-op).
func LoadCodexEntries(dataPath string) ([]UsageEntry, error) {
	if dataPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dataPath = filepath.Join(home, ".codex", "sessions")
	}

	// Gracefully return nil if the directory doesn't exist.
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		return nil, nil
	}

	files, err := findJSONLFiles(dataPath)
	if err != nil {
		return nil, err
	}

	cachePath := codexCachePath()
	store := loadCache(cachePath)

	// Build set of known paths for cache pruning.
	knownPaths := make(map[string]bool, len(files))
	for _, f := range files {
		knownPaths[f] = true
	}
	changed := pruneCache(&store, knownPaths)

	type cachedFile struct {
		entries []UsageEntry
	}
	type parsedFile struct {
		path    string
		modTime time.Time
		entries []UsageEntry
	}

	var (
		fromCache   []cachedFile
		toparse     []string
		toparseInfo = make(map[string]os.FileInfo)
	)

	for _, filePath := range files {
		info, statErr := os.Stat(filePath)
		if statErr != nil {
			continue
		}
		if cached, ok := store.Files[filePath]; ok && cached.ModTime.Equal(info.ModTime()) {
			fromCache = append(fromCache, cachedFile{cached.Entries})
		} else {
			toparse = append(toparse, filePath)
			toparseInfo[filePath] = info
		}
	}

	// Parse stale/new files in parallel using a worker pool (mirrors LoadEntries pattern).
	results := make([]parsedFile, len(toparse))
	if len(toparse) > 0 {
		workers := runtime.NumCPU()
		if workers > len(toparse) {
			workers = len(toparse)
		}
		type job struct {
			idx  int
			path string
		}
		jobs := make(chan job, len(toparse))
		for i, p := range toparse {
			jobs <- job{i, p}
		}
		close(jobs)

		var wg sync.WaitGroup
		for range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range jobs {
					fe, parseErr := parseCodexFile(j.path)
					if parseErr != nil {
						continue
					}
					results[j.idx] = parsedFile{
						path:    j.path,
						modTime: toparseInfo[j.path].ModTime(),
						entries: fe,
					}
				}
			}()
		}
		wg.Wait()
	}

	// Update cache with newly parsed files.
	for _, r := range results {
		if r.path == "" {
			continue // parse failed
		}
		store.Files[r.path] = fileCache{ModTime: r.modTime, Entries: r.entries}
		changed = true
	}

	if changed {
		saveCache(cachePath, store)
	}

	// Merge all entries and deduplicate.
	var entries []UsageEntry
	seen := make(map[string]bool)
	for _, c := range fromCache {
		mergeEntries(c.entries, &entries, seen)
	}
	for _, r := range results {
		if r.path != "" {
			mergeEntries(r.entries, &entries, seen)
		}
	}

	// Sort chronologically.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	return entries, nil
}

// LoadCodexCached reads only the on-disk gob cache for Codex entries and returns
// whatever was stored there, without touching any JSONL files.
// Returns nil entries (no error) when no cache exists.
func LoadCodexCached() ([]UsageEntry, error) {
	store := loadCache(codexCachePath())
	if len(store.Files) == 0 {
		return nil, nil
	}
	var entries []UsageEntry
	seen := make(map[string]bool)
	for _, fc := range store.Files {
		mergeEntries(fc.Entries, &entries, seen)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})
	return entries, nil
}
