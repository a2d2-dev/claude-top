package data

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadEntries_RealData(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot get home dir: %v", err)
	}
	dataPath := filepath.Join(home, ".claude", "projects")
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		t.Skip("no Claude projects data found")
	}

	entries, err := LoadEntries(dataPath)
	if err != nil {
		t.Fatalf("LoadEntries error: %v", err)
	}

	t.Logf("Loaded %d entries", len(entries))

	if len(entries) == 0 {
		t.Log("No entries found (may be normal if no assistant messages exist)")
		return
	}

	// Verify basic entry validity.
	for i, e := range entries {
		if e.Timestamp.IsZero() {
			t.Errorf("entry %d has zero timestamp", i)
		}
		if e.Timestamp.Location() != time.UTC {
			t.Errorf("entry %d timestamp not UTC: %v", i, e.Timestamp.Location())
		}
		if e.InputTokens < 0 || e.OutputTokens < 0 {
			t.Errorf("entry %d has negative tokens: in=%d out=%d", i, e.InputTokens, e.OutputTokens)
		}
	}

	// Verify sorted order.
	for i := 1; i < len(entries); i++ {
		if entries[i].Timestamp.Before(entries[i-1].Timestamp) {
			t.Errorf("entries not sorted: %v > %v", entries[i-1].Timestamp, entries[i].Timestamp)
		}
	}

	// Log sample stats.
	last := entries[len(entries)-1]
	t.Logf("Latest entry: %s model=%s input=%d output=%d",
		last.Timestamp.Format(time.RFC3339), last.Model, last.InputTokens, last.OutputTokens)
}

func TestParseTimestamp(t *testing.T) {
	cases := []struct {
		input   string
		wantErr bool
	}{
		{"2026-03-26T13:16:25.380Z", false},
		{"2026-03-26T13:16:25Z", false},
		{"2026-01-18T17:13:40.558Z", false},
		{"", true},
		{"not-a-date", true},
	}

	for _, tc := range cases {
		_, err := parseTimestamp(tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("parseTimestamp(%q): wantErr=%v got err=%v", tc.input, tc.wantErr, err)
		}
	}
}

// TestLoadCodexEntries_MissingDir verifies that LoadCodexEntries returns (nil, nil)
// when the Codex sessions directory does not exist.
func TestLoadCodexEntries_MissingDir(t *testing.T) {
	dir := t.TempDir()
	missingPath := filepath.Join(dir, "does-not-exist")

	entries, err := LoadCodexEntries(missingPath)
	if err != nil {
		t.Fatalf("expected nil error for missing dir, got: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries for missing dir, got %d entries", len(entries))
	}
}

// TestParseCodexFile_TokenDedup verifies streaming deduplication in parseCodexFile.
// It creates a JSONL file with 3 token_count events where the last two are identical.
// Only 2 UsageEntry values should be emitted (the first unique and the second unique).
func TestParseCodexFile_TokenDedup(t *testing.T) {
	dir := t.TempDir()
	sessionDir := filepath.Join(dir, "2026", "04", "10")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(sessionDir, "test-session.jsonl")

	// Build JSONL lines: session_meta, turn_context, user_message,
	// then 3 token_count events: first unique, second unique, third = duplicate of second.
	lines := []string{
		`{"timestamp":"2026-04-10T00:00:00Z","type":"session_meta","payload":{"id":"sess-test-001"}}`,
		`{"timestamp":"2026-04-10T00:00:01Z","type":"turn_context","payload":{"model":"codex-mini-latest"}}`,
		`{"timestamp":"2026-04-10T00:00:02Z","type":"event_msg","payload":{"type":"user_message","message":"hello codex","images":[]}}`,
		// First token_count (unique)
		`{"timestamp":"2026-04-10T00:00:03Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"cached_input_tokens":0,"output_tokens":50,"reasoning_output_tokens":0}}}}`,
		// Second token_count (different from first — final streamed value)
		`{"timestamp":"2026-04-10T00:00:04Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":200,"cached_input_tokens":10,"output_tokens":80,"reasoning_output_tokens":5}}}}`,
		// Third token_count — identical to second (streaming duplicate, should be skipped)
		`{"timestamp":"2026-04-10T00:00:05Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":200,"cached_input_tokens":10,"output_tokens":80,"reasoning_output_tokens":5}}}}`,
	}

	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := parseCodexFile(filePath)
	if err != nil {
		t.Fatalf("parseCodexFile error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (dedup should skip identical third), got %d", len(entries))
	}

	// Verify first entry tokens.
	e0 := entries[0]
	if e0.InputTokens != 100 {
		t.Errorf("entry[0] InputTokens: want 100, got %d", e0.InputTokens)
	}
	if e0.OutputTokens != 50 {
		t.Errorf("entry[0] OutputTokens: want 50, got %d", e0.OutputTokens)
	}
	if e0.Source != "codex" {
		t.Errorf("entry[0] Source: want 'codex', got %q", e0.Source)
	}
	if e0.Model != "codex-mini-latest" {
		t.Errorf("entry[0] Model: want 'codex-mini-latest', got %q", e0.Model)
	}

	// Verify second entry tokens (output = 80 + 5 reasoning = 85).
	e1 := entries[1]
	if e1.InputTokens != 200 {
		t.Errorf("entry[1] InputTokens: want 200, got %d", e1.InputTokens)
	}
	if e1.OutputTokens != 85 {
		t.Errorf("entry[1] OutputTokens: want 85 (80+5 reasoning), got %d", e1.OutputTokens)
	}
	if e1.CacheReadTokens != 10 {
		t.Errorf("entry[1] CacheReadTokens: want 10, got %d", e1.CacheReadTokens)
	}
	if e1.UserPrompt != "hello codex" {
		t.Errorf("entry[1] UserPrompt: want 'hello codex', got %q", e1.UserPrompt)
	}
}

// TestLoadCodexEntries_WithData verifies that LoadCodexEntries reads and returns
// entries from a valid JSONL directory structure.
func TestLoadCodexEntries_WithData(t *testing.T) {
	dir := t.TempDir()
	sessionDir := filepath.Join(dir, "2026", "04", "10")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(sessionDir, "session-abc.jsonl")

	content := `{"timestamp":"2026-04-10T00:00:00Z","type":"session_meta","payload":{"id":"sess-abc"}}
{"timestamp":"2026-04-10T00:00:01Z","type":"turn_context","payload":{"model":"codex-mini-latest"}}
{"timestamp":"2026-04-10T00:00:02Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":500,"cached_input_tokens":0,"output_tokens":100,"reasoning_output_tokens":0}}}}
`
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := LoadCodexEntries(dir)
	if err != nil {
		t.Fatalf("LoadCodexEntries error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Source != "codex" {
		t.Errorf("Source: want 'codex', got %q", e.Source)
	}
	if e.SessionID != "sess-abc" {
		t.Errorf("SessionID: want 'sess-abc', got %q", e.SessionID)
	}
	if e.InputTokens != 500 {
		t.Errorf("InputTokens: want 500, got %d", e.InputTokens)
	}
}

// TestParseCodexFile_NullInfo verifies that a token_count event with null info is skipped.
// This is a real-world occurrence: the first token_count in a session often has info=null.
func TestParseCodexFile_NullInfo(t *testing.T) {
	f := writeCodexFile(t, []string{
		`{"timestamp":"2026-04-10T00:00:00Z","type":"session_meta","payload":{"id":"sess-null"}}`,
		`{"timestamp":"2026-04-10T00:00:01Z","type":"turn_context","payload":{"model":"gpt-5-codex"}}`,
		// null info — should be silently skipped
		`{"timestamp":"2026-04-10T00:00:02Z","type":"event_msg","payload":{"type":"token_count","info":null,"rate_limits":{"primary":null}}}`,
		// non-null info — should produce one entry
		`{"timestamp":"2026-04-10T00:00:03Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":42,"cached_input_tokens":0,"output_tokens":7,"reasoning_output_tokens":0}},"rate_limits":{"primary":null}}}`,
	})

	entries, err := parseCodexFile(f)
	if err != nil {
		t.Fatalf("parseCodexFile error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (null info skipped), got %d", len(entries))
	}
	if entries[0].InputTokens != 42 {
		t.Errorf("InputTokens: want 42, got %d", entries[0].InputTokens)
	}
}

// TestParseCodexFile_AllZeroTokens verifies that token_count events with all-zero
// token counts are skipped (they represent empty/aborted turns).
func TestParseCodexFile_AllZeroTokens(t *testing.T) {
	f := writeCodexFile(t, []string{
		`{"timestamp":"2026-04-10T00:00:00Z","type":"session_meta","payload":{"id":"sess-zero"}}`,
		// all zeros — should be skipped
		`{"timestamp":"2026-04-10T00:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":0,"cached_input_tokens":0,"output_tokens":0,"reasoning_output_tokens":0}}}}`,
		// non-zero — should produce one entry
		`{"timestamp":"2026-04-10T00:00:02Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":5,"reasoning_output_tokens":0}}}}`,
	})

	entries, err := parseCodexFile(f)
	if err != nil {
		t.Fatalf("parseCodexFile error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (all-zero skipped), got %d", len(entries))
	}
}

// TestParseCodexFile_NoTokenEvents verifies that a file with no token_count events
// returns an empty slice without error.
func TestParseCodexFile_NoTokenEvents(t *testing.T) {
	f := writeCodexFile(t, []string{
		`{"timestamp":"2026-04-10T00:00:00Z","type":"session_meta","payload":{"id":"sess-notokens"}}`,
		`{"timestamp":"2026-04-10T00:00:01Z","type":"turn_context","payload":{"model":"gpt-5-codex"}}`,
		`{"timestamp":"2026-04-10T00:00:02Z","type":"event_msg","payload":{"type":"user_message","message":"hi","images":[]}}`,
	})

	entries, err := parseCodexFile(f)
	if err != nil {
		t.Fatalf("parseCodexFile error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

// TestParseCodexFile_TimestampParsed verifies that the top-level ISO 8601 timestamp
// is parsed correctly and stored as a UTC time on the entry.
func TestParseCodexFile_TimestampParsed(t *testing.T) {
	f := writeCodexFile(t, []string{
		`{"timestamp":"2026-03-15T10:30:00.500Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":5,"reasoning_output_tokens":0}}}}`,
	})

	entries, err := parseCodexFile(f)
	if err != nil {
		t.Fatalf("parseCodexFile error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	want := time.Date(2026, 3, 15, 10, 30, 0, 500_000_000, time.UTC)
	if !entries[0].Timestamp.Equal(want) {
		t.Errorf("Timestamp: want %v, got %v", want, entries[0].Timestamp)
	}
}

// TestParseCodexFile_SessionIDFallback verifies that the file basename is used as
// the session ID when no session_meta event is present.
func TestParseCodexFile_SessionIDFallback(t *testing.T) {
	f := writeCodexFile(t, []string{
		// No session_meta
		`{"timestamp":"2026-04-10T00:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":5,"reasoning_output_tokens":0}}}}`,
	})

	entries, err := parseCodexFile(f)
	if err != nil {
		t.Fatalf("parseCodexFile error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	wantID := filepath.Base(f)
	if entries[0].SessionID != wantID {
		t.Errorf("SessionID fallback: want %q, got %q", wantID, entries[0].SessionID)
	}
}

// TestParseCodexFile_CWDFromSessionMeta verifies that the CWD is read from
// session_meta.payload.cwd (the user's project directory), not the JSONL file path.
func TestParseCodexFile_CWDFromSessionMeta(t *testing.T) {
	f := writeCodexFile(t, []string{
		`{"timestamp":"2026-04-10T00:00:00Z","type":"session_meta","payload":{"id":"sess-cwd","cwd":"/Users/user/myproject"}}`,
		`{"timestamp":"2026-04-10T00:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":5,"reasoning_output_tokens":0}}}}`,
	})

	entries, err := parseCodexFile(f)
	if err != nil {
		t.Fatalf("parseCodexFile error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// CWD should be the project dir from session_meta, not the sessions storage dir.
	if entries[0].CWD != "/Users/user/myproject" {
		t.Errorf("CWD: want '/Users/user/myproject', got %q", entries[0].CWD)
	}
}

// TestParseCodexFile_CWDFallbackToFilePath verifies that when no session_meta CWD
// is present, the JSONL file's parent directory is used as the fallback.
func TestParseCodexFile_CWDFallbackToFilePath(t *testing.T) {
	f := writeCodexFile(t, []string{
		// No session_meta, no CWD info
		`{"timestamp":"2026-04-10T00:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":5,"reasoning_output_tokens":0}}}}`,
	})

	entries, err := parseCodexFile(f)
	if err != nil {
		t.Fatalf("parseCodexFile error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	want := filepath.Dir(f)
	if entries[0].CWD != want {
		t.Errorf("CWD fallback: want %q, got %q", want, entries[0].CWD)
	}
}

// TestParseCodexFile_MalformedLines verifies that malformed JSON lines are skipped
// without returning an error, and valid lines are still parsed.
func TestParseCodexFile_MalformedLines(t *testing.T) {
	f := writeCodexFile(t, []string{
		`{"timestamp":"2026-04-10T00:00:00Z","type":"session_meta","payload":{"id":"sess-malformed"}}`,
		`not valid json {{{`,
		`{"broken":`,
		`{"timestamp":"2026-04-10T00:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":5,"reasoning_output_tokens":0}}}}`,
	})

	entries, err := parseCodexFile(f)
	if err != nil {
		t.Fatalf("parseCodexFile error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (malformed lines skipped), got %d", len(entries))
	}
}

// TestLoadCodexEntries_CWDSurvivedCacheRoundTrip verifies that the CWD from
// session_meta is stored in the cache and correctly retrieved on a second load.
// This guards against silent regressions where the cache stores wrong values.
func TestLoadCodexEntries_CWDSurvivedCacheRoundTrip(t *testing.T) {
	// Create a temp Codex directory with one session file containing a known CWD.
	codexDir := t.TempDir()
	sessionDir := filepath.Join(codexDir, "2026", "04", "10")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"timestamp":"2026-04-10T10:00:00Z","type":"session_meta","payload":{"id":"sess-rtt","cwd":"/Users/user/myproject"}}
{"timestamp":"2026-04-10T10:00:01Z","type":"turn_context","payload":{"model":"gpt-5-codex"}}
{"timestamp":"2026-04-10T10:00:02Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":5,"reasoning_output_tokens":0}}}}
`
	filePath := filepath.Join(sessionDir, "session.jsonl")
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// First load: parses from JSONL and writes cache.
	entries1, err := LoadCodexEntries(codexDir)
	if err != nil {
		t.Fatalf("first LoadCodexEntries: %v", err)
	}
	if len(entries1) != 1 {
		t.Fatalf("first load: expected 1 entry, got %d", len(entries1))
	}
	if entries1[0].CWD != "/Users/user/myproject" {
		t.Errorf("first load CWD: want '/Users/user/myproject', got %q", entries1[0].CWD)
	}

	// Second load: served from cache (file mod time unchanged).
	entries2, err := LoadCodexEntries(codexDir)
	if err != nil {
		t.Fatalf("second LoadCodexEntries: %v", err)
	}
	if len(entries2) != 1 {
		t.Fatalf("second load: expected 1 entry, got %d", len(entries2))
	}
	if entries2[0].CWD != "/Users/user/myproject" {
		t.Errorf("cache round-trip CWD: want '/Users/user/myproject', got %q (stale cache bug)", entries2[0].CWD)
	}
}

// TestParseCodexFile_RealWorldFormat verifies parsing of the full real-world event
// structure including extra fields (rate_limits, total_token_usage, model_context_window).
func TestParseCodexFile_RealWorldFormat(t *testing.T) {
	// This mirrors what Codex CLI actually writes to disk.
	f := writeCodexFile(t, []string{
		`{"timestamp":"2026-04-09T14:08:39.172Z","type":"session_meta","payload":{"id":"019d7292-d6d4-71d3-82f5-284cdaea1043","cwd":"/Users/user/project","originator":"codex_cli_rs","cli_version":"0.50.0"}}`,
		`{"timestamp":"2026-04-09T14:08:42.223Z","type":"event_msg","payload":{"type":"user_message","message":"hello","images":[]}}`,
		`{"timestamp":"2026-04-09T14:08:43.000Z","type":"turn_context","payload":{"cwd":"/","model":"gpt-5.1-codex-mini","effort":"low","summary":"auto"}}`,
		`{"timestamp":"2026-04-09T15:25:11.657Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":6465,"cached_input_tokens":0,"output_tokens":52,"reasoning_output_tokens":0,"total_tokens":6517},"last_token_usage":{"input_tokens":6465,"cached_input_tokens":0,"output_tokens":52,"reasoning_output_tokens":0,"total_tokens":6517},"model_context_window":258400},"rate_limits":{"primary":null,"secondary":null}}}`,
	})

	entries, err := parseCodexFile(f)
	if err != nil {
		t.Fatalf("parseCodexFile error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.SessionID != "019d7292-d6d4-71d3-82f5-284cdaea1043" {
		t.Errorf("SessionID: want '019d7292-...', got %q", e.SessionID)
	}
	if e.Model != "gpt-5.1-codex-mini" {
		t.Errorf("Model: want 'gpt-5.1-codex-mini', got %q", e.Model)
	}
	if e.InputTokens != 6465 {
		t.Errorf("InputTokens: want 6465, got %d", e.InputTokens)
	}
	if e.OutputTokens != 52 {
		t.Errorf("OutputTokens: want 52, got %d", e.OutputTokens)
	}
	if e.UserPrompt != "hello" {
		t.Errorf("UserPrompt: want 'hello', got %q", e.UserPrompt)
	}
	if e.Source != "codex" {
		t.Errorf("Source: want 'codex', got %q", e.Source)
	}
}

// writeCodexFile creates a temp JSONL file with the given lines and returns the path.
// It creates YYYY/MM/DD directory structure within t.TempDir().
func writeCodexFile(t *testing.T, lines []string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "2026", "04", "10")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "test-session.jsonl")
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestLoadAllEntries_Sources verifies that LoadAllEntries correctly filters by source.
func TestLoadAllEntries_Sources(t *testing.T) {
	// Create a temp codex directory with one entry.
	codexDir := t.TempDir()
	sessionDir := filepath.Join(codexDir, "2026", "04", "10")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf(`{"timestamp":"2026-04-10T00:00:00Z","type":"session_meta","payload":{"id":"sess-load-all-%d"}}
{"timestamp":"2026-04-10T00:00:01Z","type":"turn_context","payload":{"model":"codex-mini-latest"}}
{"timestamp":"2026-04-10T00:00:02.%03dZ","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":300,"cached_input_tokens":0,"output_tokens":50,"reasoning_output_tokens":0}}}}
`, 1712345678, 680)
	if err := os.WriteFile(filepath.Join(sessionDir, "s.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// No claude path — use nonexistent dir so it doesn't try real ~/.claude.
	claudeDir := filepath.Join(t.TempDir(), "no-claude")

	// sources="codex" should return 1 codex entry.
	entries, err := LoadAllEntries(claudeDir, codexDir, "codex")
	if err != nil {
		t.Fatalf("LoadAllEntries(codex) error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("LoadAllEntries(codex): expected 1 entry, got %d", len(entries))
	}
	if entries[0].Source != "codex" {
		t.Errorf("Source: want 'codex', got %q", entries[0].Source)
	}

	// sources="claude" with missing dir should return 0 entries (LoadEntries errors on missing dir).
	// We use an empty temp dir (exists but no JSONL files).
	emptyClaudeDir := t.TempDir()
	entries, err = LoadAllEntries(emptyClaudeDir, codexDir, "claude")
	if err != nil {
		t.Fatalf("LoadAllEntries(claude) error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("LoadAllEntries(claude) with no claude data: expected 0, got %d", len(entries))
	}
}
