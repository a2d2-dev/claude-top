# Story 5.1: Codex CLI Data Parsing & Cache

Status: ready-for-dev

## Story

As a developer who uses both Claude Code and Codex CLI,
I want claude-top to automatically read and parse my Codex CLI session data,
so that I can see all my AI tool spending in one place without any extra configuration.

## Acceptance Criteria

1. **AC1 – Codex JSONL parsing:** Given `~/.codex/sessions/` exists with `.jsonl` files, when the app loads, then all Codex session files under `YYYY/MM/DD/*.jsonl` are parsed and returned as `[]UsageEntry` with `Source: "codex"`.

2. **AC2 – Token field mapping:** Given a Codex token_count event with `last_token_usage`, then:
   - `input_tokens` → `InputTokens`
   - `cached_input_tokens` → `CacheReadTokens`
   - `output_tokens + reasoning_output_tokens` → `OutputTokens`
   - `CacheCreationTokens` = 0

3. **AC3 – Streaming dedup:** Given multiple `token_count` events for the same turn (streaming), then only the final one (where `last_token_usage` changes from previous) is emitted as a `UsageEntry`.

4. **AC4 – Model detection:** Given a `turn_context` event with `payload.model`, then subsequent `token_count` entries for that turn use that model string.

5. **AC5 – File-level cache:** Given Codex JSONL files that haven't changed since last run, then they are served from gob cache at `~/.cache/a2d2/claude-usage-monitor/codex.cache` without re-parsing.

6. **AC6 – Cache version bump:** Given an existing Claude entries cache at v2, when the app starts, then the old cache is automatically invalidated and rebuilt (cacheVersion bumped to 3).

7. **AC7 – Graceful missing dir:** Given `~/.codex/sessions/` does not exist, when `LoadCodexEntries` is called, then it returns `(nil, nil)` — no error, no crash.

8. **AC8 – OpenAI pricing:** Given a Codex entry with model `codex-mini-latest` or `codex-latest`, then `CalculateCost` returns a non-zero cost using the correct OpenAI per-million-token rates.

9. **AC9 – Source field propagation:** Given any `UsageEntry` loaded via `LoadCodexEntries`, then `entry.Source == "codex"`. Given any entry loaded via existing `LoadEntries`, then `entry.Source == "claude"`.

10. **AC10 – SessionBlock.Source:** Given `finalizeBlock()` is called on a block, then `block.Source` is set to the dominant source among its entries ("claude" or "codex").

11. **AC11 – LoadAllEntries merge:** Given `LoadAllEntries("", "", "all")` is called, then it returns entries from both sources merged and sorted chronologically.

12. **AC12 – Existing tests pass:** `go test ./...` passes without modification.

## Tasks / Subtasks

- [ ] **Task 1: Update data models** (AC: 9, 10)
  - [ ] 1.1 Add `Source string` field to `UsageEntry` in `internal/data/models.go`
  - [ ] 1.2 Add `Source string` field to `SessionBlock` in `internal/data/models.go`

- [ ] **Task 2: Update cache layer** (AC: 5, 6)
  - [ ] 2.1 Bump `cacheVersion` from `2` → `3` in `internal/data/cache.go`
  - [ ] 2.2 Add `codexCachePath()` returning `~/.cache/a2d2/claude-usage-monitor/codex.cache`

- [ ] **Task 3: Add OpenAI pricing** (AC: 8)
  - [ ] 3.1 Add `openAIPricing` map to `internal/core/pricing.go` with entries for `codex-mini`, `codex` (full)
  - [ ] 3.2 Update `pricingForModel()` to detect OpenAI models (contains "gpt" or "codex") before falling through to Anthropic logic

- [ ] **Task 4: Implement Codex reader** (AC: 1, 2, 3, 4, 7)
  - [ ] 4.1 Create `internal/data/codex_reader.go`
  - [ ] 4.2 Implement `parseCodexFile(path string) ([]UsageEntry, error)` — single-pass line-by-line parser
  - [ ] 4.3 Implement `LoadCodexEntries(dataPath string) ([]UsageEntry, error)` with cache + worker pool (mirrors `LoadEntries`)
  - [ ] 4.4 Implement `LoadCodexCached() ([]UsageEntry, error)` (mirrors `LoadCached`)

- [ ] **Task 5: Update reader.go** (AC: 11)
  - [ ] 5.1 Add `LoadAllEntries(claudePath, codexPath, sources string) ([]UsageEntry, error)` to `internal/data/reader.go`
  - [ ] 5.2 Set `Source: "claude"` on all entries returned by `LoadEntries` (backfill)

- [ ] **Task 6: Update session finalizer** (AC: 10)
  - [ ] 6.1 In `internal/core/session.go`, update `finalizeBlock()` to count entry sources and set `block.Source`

- [ ] **Task 7: Tests** (AC: 12)
  - [ ] 7.1 Add `TestLoadCodexEntries_MissingDir` — verify graceful no-op
  - [ ] 7.2 Add `TestParseCodexFile_TokenDedup` — verify streaming dedup logic with synthetic JSONL
  - [ ] 7.3 Add `TestCalculateCost_OpenAIModels` — verify codex-mini-latest and codex-latest pricing
  - [ ] 7.4 Run `go test ./...` to confirm no regressions

## Dev Notes

### Codex JSONL Format

Each line in a Codex session file is a JSON object. Key fields:

```json
// event_msg values and their relevant payload fields:

// "session_meta" — session ID
{ "event_msg": "session_meta", "payload": { "id": "sess_abc123" } }

// "user_message" — user prompt
{ "event_msg": "user_message", "payload": { "text": "...", "timestamp": 1712345678000 } }

// "turn_context" — model for this turn
{ "event_msg": "turn_context", "payload": { "model": "codex-mini-latest", "timestamp": 1712345679000 } }

// "token_count" — token usage (fires multiple times per turn during streaming)
{
  "event_msg": "token_count",
  "payload": {
    "timestamp": 1712345680000,
    "last_token_usage": {
      "input_tokens": 1234,
      "cached_input_tokens": 100,
      "output_tokens": 456,
      "reasoning_output_tokens": 50
    }
  }
}
```

**Turn parsing state machine** (per file):
```
state:
  currentModel    string
  currentSession  string
  currentPrompt   string
  lastTokenUsage  *lastTokenUsage  // previous token_count snapshot

on "session_meta"  → set currentSession = payload.id
on "turn_context"  → set currentModel = payload.model
on "user_message"  → set currentPrompt = payload.text (truncate to 200)
on "token_count"   → compare last_token_usage with lastTokenUsage
                     if CHANGED (or lastTokenUsage == nil) AND not all zeros → emit UsageEntry, update lastTokenUsage
                     if SAME → skip (streaming duplicate)
on end of file     → no special action
```

**Timestamp derivation**: Use `payload.timestamp` (unix ms → divide by 1000 → `time.Unix`). If absent, fall back to file path date (`YYYY/MM/DD` directories → midnight UTC).

**Session ID**: Use `session_meta.payload.id`. Fall back to file basename if not found.

**Directory**: Use the file path's parent dir (`filepath.Dir(path)`) as a reasonable CWD approximation — Codex doesn't store CWD in events.

### pricing.go Changes

Add **before** the existing `knownPricing` map and **update** `pricingForModel`:

```go
// openAIPricing maps normalized OpenAI model names to pricing tiers.
// Prices per million tokens in USD. Source: OpenAI pricing page (2026).
// CacheCreation = 0 (OpenAI cache is read-only from Codex CLI perspective).
var openAIPricing = map[string]modelPricing{
    "codex-mini": {Input: 1.50, Output: 6.00, CacheCreation: 0, CacheRead: 0.375},
    "codex":      {Input: 3.00, Output: 12.00, CacheCreation: 0, CacheRead: 0.750},
}

// In pricingForModel(), add BEFORE the Anthropic checks:
lower := strings.ToLower(model)
if strings.Contains(lower, "gpt") || strings.Contains(lower, "codex") {
    if strings.Contains(lower, "mini") {
        return openAIPricing["codex-mini"]
    }
    return openAIPricing["codex"]
}
```

### cache.go Changes

```go
const cacheVersion = 3  // was 2 — bump invalidates existing caches after Source field added

// Add after defaultCachePath():
func codexCachePath() string {
    home, err := os.UserHomeDir()
    if err != nil {
        return ""
    }
    return filepath.Join(home, ".cache", "a2d2", "claude-usage-monitor", "codex.cache")
}
```

### models.go Changes

```go
type UsageEntry struct {
    // ... existing fields ...
    // Source identifies which tool produced this entry: "claude" or "codex".
    Source string
}

type SessionBlock struct {
    // ... existing fields ...
    // Source is the dominant data source among this block's entries.
    Source string
}
```

### session.go — finalizeBlock Changes

```go
func finalizeBlock(block *data.SessionBlock, cwdFreq map[string]int) {
    // existing logic unchanged ...
    if len(block.Entries) > 0 {
        t := block.Entries[len(block.Entries)-1].Timestamp
        block.ActualEndTime = &t
    }
    block.MessageCount = len(block.Entries)
    block.Directory = modalCWD(cwdFreq)
    // NEW: dominant source
    block.Source = dominantSource(block.Entries)
}

// dominantSource returns "claude" or "codex" based on majority of entries.
// Defaults to "claude" on tie or empty.
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
```

### LoadAllEntries Signature

```go
// LoadAllEntries merges Claude and Codex entries based on sources filter.
// sources: "all" | "claude" | "codex"
// claudePath: defaults to ~/.claude/projects if empty
// codexPath:  defaults to ~/.codex/sessions if empty
func LoadAllEntries(claudePath, codexPath, sources string) ([]UsageEntry, error)
```

### reader.go — Source Backfill

After parsing Claude entries in `LoadEntries`, set `Source: "claude"` on every entry before returning. Add this just before the sort in `LoadEntries`:

```go
for i := range entries {
    entries[i].Source = "claude"
}
```

Do the same in `LoadCached` for backward-compat.

### Code Structure

```
internal/
  data/
    models.go          ← Add Source to UsageEntry, SessionBlock
    cache.go           ← cacheVersion 2→3, add codexCachePath()
    reader.go          ← Add LoadAllEntries(), backfill Source="claude"
    codex_reader.go    ← NEW: parseCodexFile, LoadCodexEntries, LoadCodexCached
    reader_test.go     ← Add codex tests
  core/
    pricing.go         ← Add openAIPricing, update pricingForModel()
    session.go         ← Update finalizeBlock(), add dominantSource()
```

### Worker Pool Pattern

`LoadCodexEntries` must use the **identical** parallel worker pool pattern as `LoadEntries` in `reader.go:102-138`. Copy the structure exactly — `runtime.NumCPU()` workers, channel-based job dispatch, `sync.WaitGroup`. Do NOT invent a different concurrency pattern.

### Testing Approach

Tests in this repo use `package data` (same package, not `data_test`). Test helpers create temp dirs with synthetic `.jsonl` files. See `reader_test.go` for the pattern — no mocks, use real file I/O with `t.TempDir()`.

For the streaming-dedup test, create a JSONL file with 3 `token_count` events for one turn where the last two are identical — verify only 2 `UsageEntry` values are emitted (the second unique one being the final).

### Project Structure Notes

- Go module: `github.com/a2d2-dev/claude-usage-monitor` (note: repo is `claude-top` but module is `claude-usage-monitor` — use existing import paths)
- No external dependencies to add — standard library only (`encoding/json`, `encoding/gob`, `os`, `path/filepath`, `sync`, `sort`, `time`, `bufio`)
- Build constraint: none needed — the new file is always compiled

### References

- Existing reader pattern: [Source: internal/data/reader.go]
- Cache pattern: [Source: internal/data/cache.go]
- Pricing pattern: [Source: internal/core/pricing.go]
- Session block pattern: [Source: internal/core/session.go#finalizeBlock]
- Token field mapping: [Source: _bmad-output/prd.md#Epic-3-Section-3.1]
- Codex JSONL spec: [Source: _bmad-output/implementation-artifacts/epics.md#Story-5.1]

## Dev Agent Record

### Agent Model Used

claude-sonnet-4-6

### Debug Log References

### Completion Notes List

### File List
