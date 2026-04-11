package data

import (
	"encoding/gob"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ── in-memory cache layer ──────────────────────────────────────────────────────
//
// On each 10-second refresh tick the TUI calls LoadEntries / LoadCodexEntries.
// Both call loadCache, which gob-decodes the entire cache file (~60-200 ms for
// 10 k sessions). If nothing changed on disk, this work is wasted.
//
// inMemStore keeps the last decoded store per cache file together with the file's
// mtime at decode time. loadCache checks the mtime first: if it hasn't changed,
// the in-memory copy is returned directly (zero decode overhead).

type inMemEntry struct {
	mtime time.Time
	store cacheStore
}

// inMemCache maps cachePath → last decoded store.
var (
	inMemCacheMu sync.Mutex
	inMemCache   = make(map[string]inMemEntry)
)

// cacheVersion must be bumped whenever UsageEntry or fileCache change shape,
// or when the parsing logic changes in a way that alters stored field values.
// v3: added Source field to UsageEntry and SessionBlock
// v4: fixed Codex CWD to use session_meta.payload.cwd instead of file path
// v5: fixed Codex InputTokens to subtract CacheReadTokens (avoid double billing)
// v6: fixed Codex streaming dedup — only emit final token_count per turn
const cacheVersion = 6

// cacheFilename is the name of the cache file on disk.
const cacheFilename = "entries.cache"

// fileCache holds the parsed entries for a single JSONL file along with
// the file's modification time used for invalidation.
type fileCache struct {
	ModTime time.Time
	Entries []UsageEntry
}

// cacheStore is the top-level structure written to disk via gob encoding.
// Files maps an absolute file path to its cached data.
type cacheStore struct {
	// Version guards against reading stale caches after a struct change.
	Version int
	// Files maps absolute file path → cached parse result.
	Files map[string]fileCache
}

// defaultCachePath returns ~/.cache/a2d2/claude-usage-monitor/entries.cache.
func defaultCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", "a2d2", "claude-usage-monitor", cacheFilename)
}

// codexCachePath returns ~/.cache/a2d2/claude-usage-monitor/codex.cache.
func codexCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", "a2d2", "claude-usage-monitor", "codex.cache")
}

// loadCache reads the cache from disk. Returns an empty store on any error
// (missing file, version mismatch, corrupt data) so the caller can rebuild.
//
// An in-memory layer avoids repeated gob decodes on consecutive 10-second
// refresh ticks when the cache file has not been updated.
func loadCache(cachePath string) cacheStore {
	empty := cacheStore{Version: cacheVersion, Files: make(map[string]fileCache)}
	if cachePath == "" {
		return empty
	}

	// Check current mtime before acquiring the lock to minimise contention.
	currentMtime := cacheFileModTime(cachePath)

	inMemCacheMu.Lock()
	if mem, ok := inMemCache[cachePath]; ok && !currentMtime.IsZero() && mem.mtime.Equal(currentMtime) {
		hit := mem.store
		inMemCacheMu.Unlock()
		return hit
	}
	inMemCacheMu.Unlock()

	// Cache miss — decode from disk.
	f, err := os.Open(cachePath)
	if err != nil {
		return empty
	}
	defer f.Close()

	var store cacheStore
	if err := gob.NewDecoder(f).Decode(&store); err != nil {
		return empty
	}
	if store.Version != cacheVersion {
		return empty
	}

	// Store in memory for subsequent calls.
	if !currentMtime.IsZero() {
		inMemCacheMu.Lock()
		inMemCache[cachePath] = inMemEntry{mtime: currentMtime, store: store}
		inMemCacheMu.Unlock()
	}

	return store
}

// saveCache writes store to cachePath, creating parent directories as needed.
// Errors are silently ignored; a missing cache just means a full parse next time.
// The in-memory cache entry is invalidated so the next loadCache re-reads from disk.
func saveCache(cachePath string, store cacheStore) {
	if cachePath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return
	}
	f, err := os.Create(cachePath)
	if err != nil {
		return
	}
	defer f.Close()
	if gob.NewEncoder(f).Encode(store) != nil {
		return
	}

	// Invalidate in-memory entry; the file's mtime has just changed.
	inMemCacheMu.Lock()
	delete(inMemCache, cachePath)
	inMemCacheMu.Unlock()
}

// cacheFileModTime returns the modification time of the cache file at cachePath,
// or zero time if the file cannot be stat'd. Used to skip unnecessary gob decodes
// when the cache file hasn't changed since the last load.
func cacheFileModTime(cachePath string) time.Time {
	info, err := os.Stat(cachePath)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// pruneCache removes entries for files that no longer exist in knownPaths.
// Returns true if any entries were removed.
func pruneCache(store *cacheStore, knownPaths map[string]bool) bool {
	pruned := false
	for path := range store.Files {
		if !knownPaths[path] {
			delete(store.Files, path)
			pruned = true
		}
	}
	return pruned
}
