package ui

import (
	"fmt"
	"strings"

	"github.com/a2d2-dev/claude-usage-monitor/internal/data"
)

// renderSessions renders the Sessions tab: scrollable history table or detail view.
func renderSessions(m Model, height int) string {
	if m.sessions.view == viewDetail {
		sel := m.selectedSession()
		if sel != nil {
			return renderDetailPanel(m, sel, height)
		}
	}

	if m.loading {
		content := padToHeight(
			sectionTitleStyle.Render("  SESSIONS")+"\n"+mutedStyle.Render("  Loading…"),
			height-2,
		)
		return cardStyle.Width(m.width - 2).Height(height - 2).Render(content)
	}

	rows := m.sessionRows()
	innerW := m.width - 4
	colW := histColWidths(innerW - 2) // -2 for prefix

	// ── Column header ─────────────────────────────────────────────────────────
	colNames := []string{"Start", "Updated", "Msgs", "Tokens", "Cost", "Directory"}
	headerCols := make([]string, 6)
	for i, name := range colNames {
		indicator := ""
		if sortCol(i) == m.sessions.sortColumn {
			if m.sessions.sortAsc {
				indicator = " ↑"
			} else {
				indicator = " ↓"
			}
		}
		headerCols[i] = labelStyle.Width(colW[i]).Render(name + indicator)
	}
	header := "  " + strings.Join(headerCols, " ")
	divider := mutedStyle.Render(strings.Repeat("─", min(innerW, m.width-6)))

	// ── Visible rows ──────────────────────────────────────────────────────────
	visibleRows := height - 5 // 2 border + title + header + divider
	if visibleRows < 1 {
		visibleRows = 1
	}
	scroll := m.sessionsScrollOffset()
	end := scroll + visibleRows
	if end > len(rows) {
		end = len(rows)
	}
	var visible []data.SessionBlock
	if scroll < len(rows) {
		visible = rows[scroll:end]
	}

	// ── Progress indicator in title ───────────────────────────────────────────
	total := len(rows)
	progressInfo := ""
	if total > 0 {
		shown := min(scroll+visibleRows, total)
		progressInfo = mutedStyle.Render(fmt.Sprintf(" [%d-%d / %d]", scroll+1, shown, total))
	}

	lines := []string{
		sectionTitleStyle.Render("  SESSIONS") + progressInfo,
		header,
		divider,
	}

	for i, s := range visible {
		rowIdx := scroll + i
		isCursor := rowIdx == m.sessions.cursor
		lines = append(lines, historyDataRow(s, colW, isCursor))
	}

	content := padToHeight(strings.Join(lines, "\n"), height-2)
	return cardStyle.Width(m.width - 2).Height(height - 2).Render(content)
}
