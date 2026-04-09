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
