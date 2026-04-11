// Package data benchmark tests simulate large-scale session datasets to measure
// performance of parsing, caching, and aggregation pipelines under load.
//
// Run with:
//
//	go test -bench=. -benchmem -benchtime=3s ./internal/data/...
//	go test -bench=BenchmarkLoadEntries -benchmem -count=3 ./internal/data/...
package data

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ─── Synthetic data generators ────────────────────────────────────────────────

// syntheticConfig controls the shape of the generated test dataset.
type syntheticConfig struct {
	// numSessions is the number of JSONL files (one per session) to generate.
	numSessions int
	// msgsPerSession is the number of assistant messages per session.
	msgsPerSession int
	// startTime is the earliest session timestamp.
	startTime time.Time
}

// models is a representative sample of Claude model names.
var models = []string{
	"claude-opus-4-6",
	"claude-sonnet-4-6",
	"claude-haiku-4-5-20251001",
	"claude-opus-4-5",
}

// buildClaudeJSONLLine returns one newline-terminated JSON line for a Claude assistant entry.
func buildClaudeJSONLLine(rng *rand.Rand, sessionID, msgID string, ts time.Time, cwd string) string {
	model := models[rng.Intn(len(models))]
	inputToks := 1000 + rng.Intn(50_000)
	outputToks := 100 + rng.Intn(5_000)
	cacheCreate := rng.Intn(8_000)
	cacheRead := rng.Intn(20_000)

	type usage struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	}
	type msg struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Role  string `json:"role"`
		Usage usage  `json:"usage"`
	}
	type entry struct {
		Type      string  `json:"type"`
		Timestamp string  `json:"timestamp"`
		SessionID string  `json:"sessionId"`
		UUID      string  `json:"uuid"`
		CostUSD   float64 `json:"costUSD"`
		CWD       string  `json:"cwd"`
		Message   msg     `json:"message"`
	}

	cost := float64(inputToks)*0.000003 + float64(outputToks)*0.000015
	e := entry{
		Type:      "assistant",
		Timestamp: ts.Format(time.RFC3339Nano),
		SessionID: sessionID,
		UUID:      msgID,
		CostUSD:   cost,
		CWD:       cwd,
		Message: msg{
			ID:    msgID,
			Model: model,
			Role:  "assistant",
			Usage: usage{
				InputTokens:              inputToks,
				OutputTokens:             outputToks,
				CacheCreationInputTokens: cacheCreate,
				CacheReadInputTokens:     cacheRead,
			},
		},
	}

	b, _ := json.Marshal(e)
	return string(b) + "\n"
}

// writeSyntheticClaudeDir creates a temporary directory tree mimicking ~/.claude/projects.
// Each "session" maps to one .jsonl file under a subdirectory.
// Returns the root path and a cleanup func.
func writeSyntheticClaudeDir(t testing.TB, cfg syntheticConfig) string {
	t.Helper()
	root := t.TempDir()
	rng := rand.New(rand.NewSource(42))

	dirs := []string{
		"github.com-user-projectA", "github.com-user-projectB",
		"github.com-user-projectC", "home-work-service",
	}
	cwdBase := "/Users/bench/projects"

	for s := range cfg.numSessions {
		sessionID := fmt.Sprintf("sess-%08d", s)
		dir := filepath.Join(root, dirs[s%len(dirs)])
		_ = os.MkdirAll(dir, 0o755)
		fPath := filepath.Join(dir, sessionID+".jsonl")

		f, err := os.Create(fPath)
		if err != nil {
			t.Fatalf("create session file: %v", err)
		}

		// Session start: spread evenly over one year.
		sessionStart := cfg.startTime.Add(time.Duration(s) * (365 * 24 * time.Hour / time.Duration(cfg.numSessions)))
		cwd := cwdBase + "/" + dirs[s%len(dirs)]

		for m := range cfg.msgsPerSession {
			ts := sessionStart.Add(time.Duration(m) * 2 * time.Minute)
			msgID := fmt.Sprintf("msg-%08d-%04d", s, m)
			line := buildClaudeJSONLLine(rng, sessionID, msgID, ts, cwd)
			_, _ = f.WriteString(line)
		}
		f.Close()
	}
	return root
}

// ─── Benchmarks ───────────────────────────────────────────────────────────────

// BenchmarkParseFile measures how fast a single JSONL file with 50 messages parses.
func BenchmarkParseFile(b *testing.B) {
	cfg := syntheticConfig{numSessions: 1, msgsPerSession: 50, startTime: time.Now().Add(-24 * time.Hour)}
	root := writeSyntheticClaudeDir(b, cfg)

	// Find the one file created.
	var filePath string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Ext(p) == ".jsonl" {
			filePath = p
			return filepath.SkipAll
		}
		return nil
	})
	if filePath == "" {
		b.Fatal("no file generated")
	}

	b.ResetTimer()
	for range b.N {
		_, err := parseFile(filePath)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLoadEntries_100 measures LoadEntries with 100 sessions (cold cache each run).
func BenchmarkLoadEntries_100(b *testing.B) {
	benchLoadEntries(b, 100, 20)
}

// BenchmarkLoadEntries_1k measures LoadEntries with 1 000 sessions.
func BenchmarkLoadEntries_1k(b *testing.B) {
	benchLoadEntries(b, 1_000, 20)
}

// BenchmarkLoadEntries_5k measures LoadEntries with 5 000 sessions.
func BenchmarkLoadEntries_5k(b *testing.B) {
	benchLoadEntries(b, 5_000, 20)
}

// BenchmarkLoadEntries_10k measures LoadEntries with 10 000 sessions (~200k entries).
// This is the stress-test scenario reported to saturate all CPU cores.
func BenchmarkLoadEntries_10k(b *testing.B) {
	benchLoadEntries(b, 10_000, 20)
}

// benchLoadEntries is the shared implementation for LoadEntries benchmarks.
// It wipes the cache before each b.N iteration to force a full re-parse
// (worst-case: first startup with no valid cache).
func benchLoadEntries(b *testing.B, sessions, msgsPerSession int) {
	b.Helper()
	cfg := syntheticConfig{
		numSessions:    sessions,
		msgsPerSession: msgsPerSession,
		startTime:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	root := writeSyntheticClaudeDir(b, cfg)

	// Use a temp cache file so we don't pollute the real cache.
	cacheDir := b.TempDir()
	cachePath := filepath.Join(cacheDir, "bench.cache")

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		// Delete cache to simulate cold start each iteration.
		_ = os.Remove(cachePath)

		store := loadCache(cachePath)
		files, err := findJSONLFiles(root)
		if err != nil {
			b.Fatal(err)
		}

		knownPaths := make(map[string]bool, len(files))
		for _, f := range files {
			knownPaths[f] = true
		}
		changed := pruneCache(&store, knownPaths)

		var toparse []string
		toparseInfo := make(map[string]os.FileInfo)
		for _, fp := range files {
			info, statErr := os.Stat(fp)
			if statErr != nil {
				continue
			}
			if cached, ok := store.Files[fp]; ok && cached.ModTime.Equal(info.ModTime()) {
				// cache hit — skip
			} else {
				toparse = append(toparse, fp)
				toparseInfo[fp] = info
			}
		}

		type parsedFile struct {
			path    string
			modTime time.Time
			entries []UsageEntry
		}

		results := make([]parsedFile, len(toparse))
		// Single-threaded parse to match old code behavior for comparison.
		for i, fp := range toparse {
			fe, parseErr := parseFile(fp)
			if parseErr != nil {
				continue
			}
			results[i] = parsedFile{path: fp, modTime: toparseInfo[fp].ModTime(), entries: fe}
		}

		for _, r := range results {
			if r.path != "" {
				store.Files[r.path] = fileCache{ModTime: r.modTime, Entries: r.entries}
				changed = true
			}
		}
		if changed {
			saveCache(cachePath, store)
		}

		_ = len(results) // prevent optimization
	}

	b.ReportMetric(float64(sessions), "sessions")
	b.ReportMetric(float64(sessions*msgsPerSession), "entries")
}

// BenchmarkLoadEntries_WarmCache measures a warm-cache LoadEntries (cache hit, no re-parse).
// This models the steady-state 10-second refresh tick.
func BenchmarkLoadEntries_WarmCache_1k(b *testing.B) {
	benchLoadEntriesWarm(b, 1_000, 20)
}

// BenchmarkLoadEntries_WarmCache_10k measures warm-cache with 10k sessions.
func BenchmarkLoadEntries_WarmCache_10k(b *testing.B) {
	benchLoadEntriesWarm(b, 10_000, 20)
}

// benchLoadEntriesWarm builds a warm cache once, then benchmarks only cache reads.
func benchLoadEntriesWarm(b *testing.B, sessions, msgsPerSession int) {
	b.Helper()
	cfg := syntheticConfig{
		numSessions:    sessions,
		msgsPerSession: msgsPerSession,
		startTime:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	root := writeSyntheticClaudeDir(b, cfg)

	cacheDir := b.TempDir()
	cachePath := filepath.Join(cacheDir, "bench-warm.cache")

	// Build the cache once (outside the benchmark loop).
	store := loadCache(cachePath)
	files, _ := findJSONLFiles(root)
	knownPaths := make(map[string]bool, len(files))
	for _, f := range files {
		knownPaths[f] = true
	}
	pruneCache(&store, knownPaths)
	for _, fp := range files {
		info, err := os.Stat(fp)
		if err != nil {
			continue
		}
		if _, ok := store.Files[fp]; !ok {
			fe, _ := parseFile(fp)
			store.Files[fp] = fileCache{ModTime: info.ModTime(), Entries: fe}
		}
	}
	saveCache(cachePath, store)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		// Load from warm cache — this is what happens on every 10s tick.
		warm := loadCache(cachePath)
		var entries []UsageEntry
		seen := make(map[string]bool)
		for _, fc := range warm.Files {
			mergeEntries(fc.Entries, &entries, seen)
		}
		_ = len(entries)
	}

	b.ReportMetric(float64(sessions), "sessions")
	b.ReportMetric(float64(sessions*msgsPerSession), "entries")
}

// BenchmarkMergeEntries measures deduplication overhead at 200k entries.
func BenchmarkMergeEntries_200k(b *testing.B) {
	const count = 200_000
	rng := rand.New(rand.NewSource(7))
	src := make([]UsageEntry, count)
	for i := range src {
		src[i] = UsageEntry{
			Timestamp:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Minute),
			Model:        "claude-sonnet-4-6",
			InputTokens:  1000 + rng.Intn(10_000),
			OutputTokens: 100 + rng.Intn(2_000),
			SessionID:    fmt.Sprintf("sess-%06d", i/20),
			MessageID:    fmt.Sprintf("msg-%08d", i),
		}
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		var dst []UsageEntry
		seen := make(map[string]bool, count)
		mergeEntries(src, &dst, seen)
		_ = dst
	}
}

// BenchmarkCacheSave measures gob encoding time for various entry counts.
func BenchmarkCacheSave_10k_entries(b *testing.B) {
	benchCacheSave(b, 10_000)
}

func BenchmarkCacheSave_200k_entries(b *testing.B) {
	benchCacheSave(b, 200_000)
}

func benchCacheSave(b *testing.B, numEntries int) {
	b.Helper()
	rng := rand.New(rand.NewSource(99))
	store := cacheStore{
		Version: cacheVersion,
		Files:   make(map[string]fileCache),
	}

	// Distribute entries across 100 "files".
	batchSize := numEntries / 100
	for f := range 100 {
		entries := make([]UsageEntry, batchSize)
		for i := range entries {
			entries[i] = UsageEntry{
				Timestamp:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(f*batchSize+i) * time.Minute),
				Model:        models[rng.Intn(len(models))],
				InputTokens:  1000 + rng.Intn(50_000),
				OutputTokens: 100 + rng.Intn(5_000),
				SessionID:    fmt.Sprintf("sess-%04d", f),
				MessageID:    fmt.Sprintf("msg-%08d", f*batchSize+i),
				CWD:          "/Users/bench/project",
				Source:       "claude",
			}
		}
		key := fmt.Sprintf("/home/bench/.claude/projects/proj-%04d/session.jsonl", f)
		store.Files[key] = fileCache{
			ModTime: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
			Entries: entries,
		}
	}

	cachePath := filepath.Join(b.TempDir(), "save-bench.cache")

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		saveCache(cachePath, store)
	}

	b.ReportMetric(float64(numEntries), "entries")
}

// BenchmarkCacheLoad measures gob decoding time for various entry counts.
func BenchmarkCacheLoad_10k_entries(b *testing.B) {
	benchCacheLoad(b, 10_000)
}

func BenchmarkCacheLoad_200k_entries(b *testing.B) {
	benchCacheLoad(b, 200_000)
}

func benchCacheLoad(b *testing.B, numEntries int) {
	b.Helper()
	// Build and save the cache once.
	rng := rand.New(rand.NewSource(11))
	store := cacheStore{
		Version: cacheVersion,
		Files:   make(map[string]fileCache),
	}

	batchSize := numEntries / 100
	for f := range 100 {
		entries := make([]UsageEntry, batchSize)
		for i := range entries {
			entries[i] = UsageEntry{
				Timestamp:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(f*batchSize+i) * time.Minute),
				Model:       models[rng.Intn(len(models))],
				InputTokens: 1000 + rng.Intn(50_000),
				SessionID:   fmt.Sprintf("sess-%04d", f),
				MessageID:   fmt.Sprintf("msg-%08d", f*batchSize+i),
				Source:      "claude",
			}
		}
		key := fmt.Sprintf("/home/bench/.claude/projects/proj-%04d/session.jsonl", f)
		store.Files[key] = fileCache{
			ModTime: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
			Entries: entries,
		}
	}

	cachePath := filepath.Join(b.TempDir(), "load-bench.cache")
	saveCache(cachePath, store)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		loaded := loadCache(cachePath)
		_ = len(loaded.Files)
	}

	b.ReportMetric(float64(numEntries), "entries")
}

// BenchmarkFindJSONLFiles measures directory walk time with many files.
func BenchmarkFindJSONLFiles_1k(b *testing.B) {
	root := writeSyntheticClaudeDir(b, syntheticConfig{
		numSessions:    1_000,
		msgsPerSession: 1, // file content doesn't matter for walk
		startTime:      time.Now(),
	})
	b.ResetTimer()
	for range b.N {
		files, err := findJSONLFiles(root)
		if err != nil {
			b.Fatal(err)
		}
		_ = len(files)
	}
}

func BenchmarkFindJSONLFiles_10k(b *testing.B) {
	root := writeSyntheticClaudeDir(b, syntheticConfig{
		numSessions:    10_000,
		msgsPerSession: 1,
		startTime:      time.Now(),
	})
	b.ResetTimer()
	for range b.N {
		files, err := findJSONLFiles(root)
		if err != nil {
			b.Fatal(err)
		}
		_ = len(files)
	}
}
