package data

import (
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
