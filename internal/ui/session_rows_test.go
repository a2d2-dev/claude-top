package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/a2d2-dev/claude-usage-monitor/internal/data"
)

func TestSessionDetailMetadataRendering(t *testing.T) {
	block := data.SessionBlock{
		StartTime:    time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC),
		Directory:    "/Users/alice/projects/claude-top",
		MessageCount: 1,
		Entries: []data.UsageEntry{{
			Timestamp: time.Date(2026, 7, 12, 10, 1, 0, 0, time.UTC),
			SessionID: "session-1234567890",
			CWD:       "/Users/alice/projects/claude-top",
		}},
	}

	head := renderDetailHead(&block, 100)
	if !containsAll(head, []string{"Session ID:", "session-1234567890", "Directory:", "claude-top"}) {
		t.Fatalf("session detail metadata missing from render:\n%s", head)
	}

	copyText := formatMsgForCopy(block.Entries[0], 0)
	if !containsAll(copyText, []string{"Session ID:    session-1234567890", "Directory:     /Users/alice/projects/claude-top"}) {
		t.Fatalf("message copy metadata missing:\n%s", copyText)
	}
}

func BenchmarkSessionRowsCachedNavigation(b *testing.B) {
	m := Model{
		sessions: sessionsState{sortColumn: sortByStart, sortAsc: false, showClaude: true, showCodex: true},
		blocks:   make([]data.SessionBlock, 10000),
	}
	base := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	for i := range m.blocks {
		source := "claude"
		if i%2 == 0 {
			source = "codex"
		}
		m.blocks[i] = data.SessionBlock{
			StartTime:    base.Add(time.Duration(i) * time.Minute),
			MessageCount: i % 100,
			Directory:    fmt.Sprintf("/tmp/project-%04d", i%250),
			Source:       source,
		}
	}
	m.refreshSessionRows()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows := m.sessionRows()
		m.sessions.cursor = i % len(rows)
		_ = rows[m.sessions.cursor]
	}
}

func containsAll(s string, needles []string) bool {
	for _, needle := range needles {
		if !strings.Contains(s, needle) {
			return false
		}
	}
	return true
}
