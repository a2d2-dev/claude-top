// Package data provides JSONL file reading and entry parsing for Claude usage data.
package data

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// rawEntry is the JSON structure of an entry in a Claude JSONL file.
type rawEntry struct {
	Type       string          `json:"type"`
	Timestamp  string          `json:"timestamp"`
	SessionID  string          `json:"sessionId"`
	UUID       string          `json:"uuid"`
	ParentUUID string          `json:"parentUuid"`
	CostUSD    float64         `json:"costUSD"`
	CWD        string          `json:"cwd"`
	Message    *rawMessage     `json:"message"`
}

// rawMessage holds the nested message object for both user and assistant entries.
type rawMessage struct {
	// Assistant fields
	ID    string `json:"id"`
	Model string `json:"model"`
	Role  string `json:"role"`
	Usage *struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	} `json:"usage"`
	// User fields: content can be a plain string or an array of content blocks.
	Content json.RawMessage `json:"content"`
}

// LoadEntries reads all JSONL files under dataPath and returns sorted usage entries.
// dataPath defaults to ~/.claude/projects if empty.
// Results are cached in ~/.cache/a2d2/claude-usage-monitor/entries.cache so
// only files whose modification time has changed are re-parsed.
func LoadEntries(dataPath string) ([]UsageEntry, error) {
	if dataPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("cannot determine home directory: %w", err)
		}
		dataPath = filepath.Join(home, ".claude", "projects")
	}

	files, err := findJSONLFiles(dataPath)
	if err != nil {
		return nil, fmt.Errorf("finding JSONL files: %w", err)
	}

	cachePath := defaultCachePath()
	store := loadCache(cachePath)

	// Build a set of all currently known paths for cache pruning.
	knownPaths := make(map[string]bool, len(files))
	for _, f := range files {
		knownPaths[f] = true
	}
	changed := pruneCache(&store, knownPaths)

	// Split files into cached (no re-parse needed) and stale/new.
	type cachedFile struct {
		entries []UsageEntry
	}
	type parsedFile struct {
		path    string
		modTime time.Time
		entries []UsageEntry
	}

	var (
		fromCache []cachedFile
		toparse   []string
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

	// Parse stale/new files in parallel using a worker pool.
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
					fe, err := parseFile(j.path)
					if err != nil {
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

// LoadCached reads only the on-disk gob cache and returns whatever was stored there,
// without touching any JSONL files. Returns nil entries (no error) when no cache exists.
// This is intentionally fast (~80ms) for use as a "preliminary" data load on startup.
func LoadCached() ([]UsageEntry, error) {
	store := loadCache(defaultCachePath())
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

// mergeEntries appends src entries into dst, skipping duplicates tracked by seen.
func mergeEntries(src []UsageEntry, dst *[]UsageEntry, seen map[string]bool) {
	for _, e := range src {
		key := deduplicationKey(e)
		if key == "" || !seen[key] {
			if key != "" {
				seen[key] = true
			}
			*dst = append(*dst, e)
		}
	}
}

// findJSONLFiles returns all .jsonl file paths under root recursively.
func findJSONLFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}
		if !d.IsDir() && filepath.Ext(path) == ".jsonl" {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// parseFile reads a JSONL file and returns all valid usage entries.
// It performs two passes: first collecting user prompts by UUID, then mapping
// each assistant entry to its parent user message for context.
func parseFile(path string) ([]UsageEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	// Read all lines into memory for two-pass processing.
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	var lines [][]byte
	for scanner.Scan() {
		b := scanner.Bytes()
		if len(b) == 0 {
			continue
		}
		cp := make([]byte, len(b))
		copy(cp, b)
		lines = append(lines, cp)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Pass 1: collect user prompt text keyed by UUID.
	prompts := make(map[string]string) // uuid → truncated prompt
	for _, line := range lines {
		var raw rawEntry
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}
		if raw.Type != "user" || raw.UUID == "" || raw.Message == nil {
			continue
		}
		text := extractUserPrompt(raw.Message.Content)
		if text != "" {
			prompts[raw.UUID] = text
		}
	}

	// Pass 2: extract assistant usage entries and link to parent prompt.
	var entries []UsageEntry
	for _, line := range lines {
		var raw rawEntry
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}
		entry, ok := mapToEntry(raw, prompts)
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// extractUserPrompt extracts plain text from a user message content field.
// Content can be a JSON string or a JSON array of content blocks.
// Returns up to 200 characters of text.
func extractUserPrompt(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	// Try as a plain string first.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return truncatePrompt(s)
	}

	// Try as an array of content blocks (e.g. [{type:"text", text:"..."}]).
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				return truncatePrompt(b.Text)
			}
		}
	}

	return ""
}

// truncatePrompt returns the first 200 runes of s, appending "…" if longer.
func truncatePrompt(s string) string {
	// Collapse leading whitespace/newlines.
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= 200 {
		return s
	}
	return string(runes[:199]) + "…"
}

// mapToEntry converts a rawEntry to a UsageEntry.
// prompts maps parent UUIDs to their user prompt text.
// Returns false if the entry doesn't have the required token data.
func mapToEntry(raw rawEntry, prompts map[string]string) (UsageEntry, bool) {
	// Only process assistant messages with usage data.
	if raw.Type != "assistant" || raw.Message == nil || raw.Message.Usage == nil {
		return UsageEntry{}, false
	}

	u := raw.Message.Usage
	// Skip entries with no meaningful token counts.
	if u.InputTokens == 0 && u.OutputTokens == 0 &&
		u.CacheCreationInputTokens == 0 && u.CacheReadInputTokens == 0 {
		return UsageEntry{}, false
	}

	ts, err := parseTimestamp(raw.Timestamp)
	if err != nil {
		return UsageEntry{}, false
	}

	return UsageEntry{
		Timestamp:           ts,
		Model:               raw.Message.Model,
		InputTokens:         u.InputTokens,
		OutputTokens:        u.OutputTokens,
		CacheCreationTokens: u.CacheCreationInputTokens,
		CacheReadTokens:     u.CacheReadInputTokens,
		CostUSD:             raw.CostUSD,
		SessionID:           raw.SessionID,
		MessageID:           raw.Message.ID,
		CWD:                 raw.CWD,
		UserPrompt:          prompts[raw.ParentUUID],
	}, true
}

// parseTimestamp parses an ISO 8601 timestamp string.
func parseTimestamp(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999Z",
		"2006-01-02T15:04:05Z",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse timestamp: %s", s)
}

// deduplicationKey builds a key for deduplicating entries across files.
func deduplicationKey(e UsageEntry) string {
	if e.MessageID == "" || e.SessionID == "" {
		return ""
	}
	return e.MessageID + ":" + e.SessionID
}
