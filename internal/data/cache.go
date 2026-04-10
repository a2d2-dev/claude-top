package data

import (
	"encoding/gob"
	"os"
	"path/filepath"
	"time"
)

// cacheVersion must be bumped whenever UsageEntry or fileCache change shape,
// or when the parsing logic changes in a way that alters stored field values.
// v3: added Source field to UsageEntry and SessionBlock
// v4: fixed Codex CWD to use session_meta.payload.cwd instead of file path
// v5: fixed Codex InputTokens to subtract CacheReadTokens (avoid double billing)
const cacheVersion = 5

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
func loadCache(cachePath string) cacheStore {
	empty := cacheStore{Version: cacheVersion, Files: make(map[string]fileCache)}

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
	return store
}

// saveCache writes store to cachePath, creating parent directories as needed.
// Errors are silently ignored; a missing cache just means a full parse next time.
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
	_ = gob.NewEncoder(f).Encode(store)
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
