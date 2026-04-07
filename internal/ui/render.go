package ui

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/a2d2-dev/claude-usage-monitor/internal/data"
)

// ── Dashboard orchestrator ────────────────────────────────────────────────────

// RenderDashboard assembles header + active tab content + footer.
func RenderDashboard(m Model) string {
	header := renderHeader(m)
	tabBar := renderTabBar(m)
	footer := renderFooter(m)

	// Content height = total – header(1) – tabBar(1) – footer(1)
	contentH := m.height - 3
	if contentH < 4 {
		contentH = 4
	}

	var content string
	switch m.tab {
	case tabOverview:
		content = renderOverview(m, contentH)
	case tabSessions:
		content = renderSessions(m, contentH)
	case tabDaily:
		content = renderDaily(m, contentH)
	default:
		content = renderOverview(m, contentH)
	}

	return strings.Join([]string{header, tabBar, content, footer}, "\n")
}

// ── Header ────────────────────────────────────────────────────────────────────

func renderHeader(m Model) string {
	title := "  Claude Usage Monitor"
	ts := time.Now().Local().Format("2006-01-02 15:04:05")
	badge := fmt.Sprintf(" Plan: %s ", m.plan.DisplayName)

	spinner := ""
	if m.loading || m.refreshing {
		spinner = mutedStyle.Render(" ↻")
	}

	pad := strings.Repeat(" ", max(0,
		m.width-lipgloss.Width(title)-lipgloss.Width(spinner)-len(ts)-len(badge)-4))
	return headerStyle.Width(m.width).Render(title + spinner + pad + badge + "  " + ts)
}

// ── Tab bar ───────────────────────────────────────────────────────────────────

func renderTabBar(m Model) string {
	var tabs []string
	for i, name := range tabNames {
		if tabID(i) == m.tab {
			tabs = append(tabs, tabActiveStyle.Render("● "+name))
		} else {
			tabs = append(tabs, tabInactiveStyle.Render("○ "+name))
		}
	}
	bar := strings.Join(tabs, "")
	// Fill remaining width with border color underline.
	fill := strings.Repeat("─", max(0, m.width-lipgloss.Width(bar)))
	return bar + mutedStyle.Render(fill)
}

// ── Footer ────────────────────────────────────────────────────────────────────

func renderFooter(m Model) string {
	var hint string
	switch m.tab {
	case tabSessions:
		if m.sessions.view == viewDetail {
			hint = "" // hint is rendered inside the card
		} else {
			hint = fmt.Sprintf("  ↑↓ cursor  Enter detail  s/S sort(%s)  / direction  Tab switch  q quit",
				sortColNames[m.sessions.sortColumn])
		}
	case tabDaily:
		hint = "  ↑↓ cursor  Tab switch  q quit"
	default:
		hint = "  1-3 tabs  Tab switch  r refresh  q quit"
	}
	return mutedStyle.Render(hint)
}

// ── Session card (shared by Overview and Sessions tabs) ───────────────────────

func renderSessionCard(m Model, b *data.SessionBlock, now time.Time, innerW int) string {
	totalTokens := b.TokenCounts.TotalTokens()
	tokenPct := clampPct(float64(totalTokens) / float64(m.plan.TokenLimit) * 100)

	elapsed := now.Sub(b.StartTime)
	totalWindow := b.EndTime.Sub(b.StartTime)
	timePct := clampPct(elapsed.Seconds() / totalWindow.Seconds() * 100)
	timeLeft := b.EndTime.Sub(now)
	if timeLeft < 0 {
		timeLeft = 0
	}

	elapsedMin := elapsed.Minutes()
	burnTPM, burnCPH := 0.0, 0.0
	if elapsedMin > 1 {
		burnTPM = float64(totalTokens) / elapsedMin
		burnCPH = b.CostUSD / elapsedMin * 60
	}

	barW := min(40, innerW-12)
	lines := []string{
		sectionTitleStyle.Render("● ACTIVE SESSION"),
		fmt.Sprintf("%s  %s → %s  (resets in %s)",
			labelStyle.Render("Window:"),
			b.StartTime.Local().Format("15:04"),
			b.EndTime.Local().Format("15:04"),
			formatDuration(timeLeft),
		),
		"",
		progressRow("Tokens", tokenPct, barW),
		fmt.Sprintf("  %s %s / %s  (%s)",
			labelStyle.Render("Used:"),
			valueStyle.Render(formatInt(totalTokens)),
			mutedStyle.Render(formatInt(m.plan.TokenLimit)),
			lipgloss.NewStyle().Foreground(colorForPercent(tokenPct)).Bold(true).
				Render(fmt.Sprintf("%.1f%%", tokenPct)),
		),
		"",
		progressRow("Time", timePct, barW),
		fmt.Sprintf("  %s %s elapsed  /  %s remaining",
			labelStyle.Render("Time:"),
			valueStyle.Render(formatDuration(elapsed)),
			mutedStyle.Render(formatDuration(timeLeft)),
		),
		"",
		fmt.Sprintf("  %s %s   %s %s/hr   %s %s   %s %s",
			labelStyle.Render("Cost:"),
			accentValueStyle.Render(fmt.Sprintf("$%.4f", b.CostUSD)),
			labelStyle.Render("Burn:"),
			mutedStyle.Render(fmt.Sprintf("$%.4f", burnCPH)),
			labelStyle.Render("Tok/min:"),
			mutedStyle.Render(fmt.Sprintf("%.0f", burnTPM)),
			labelStyle.Render("Msgs:"),
			valueStyle.Render(fmt.Sprintf("%d", b.MessageCount)),
		),
		"",
		renderModelBreakdown(b, innerW),
	}
	return strings.Join(lines, "\n")
}

func progressRow(label string, pct float64, barW int) string {
	if barW < 1 {
		barW = 1
	}
	filled := int(math.Round(float64(barW) * pct / 100))
	if filled > barW {
		filled = barW
	}
	c := colorForPercent(pct)
	bar := lipgloss.NewStyle().Foreground(c).Render(strings.Repeat("█", filled)) +
		mutedStyle.Render(strings.Repeat("░", barW-filled))
	return fmt.Sprintf("  %s [%s]", labelStyle.Width(8).Render(label), bar)
}

func renderModelBreakdown(b *data.SessionBlock, innerW int) string {
	if len(b.PerModelStats) == 0 {
		return ""
	}
	total := b.TokenCounts.TotalTokens()
	barW := min(20, innerW-46)
	if barW < 2 {
		barW = 2
	}
	var lines []string
	lines = append(lines, labelStyle.Render("  Models:"))
	for model, stats := range b.PerModelStats {
		pct := 0.0
		if total > 0 {
			pct = float64(stats.TotalTokens()) / float64(total) * 100
		}
		c := modelColor(model)
		filled := int(math.Round(float64(barW) * pct / 100))
		bar := lipgloss.NewStyle().Foreground(c).Render(strings.Repeat("▪", filled)) +
			mutedStyle.Render(strings.Repeat("·", barW-filled))
		lines = append(lines, fmt.Sprintf("    %s [%s] %5.1f%%  $%.4f",
			lipgloss.NewStyle().Foreground(c).Bold(true).Width(14).Render(model),
			bar, pct, stats.CostUSD,
		))
	}
	return strings.Join(lines, "\n")
}

// ── History table row (shared by Sessions tab) ────────────────────────────────

// histColWidths computes column widths given available content width (excl. 2-char prefix).
// Columns: Start(14), Updated(11), Msgs(6), Tokens(9), Cost(8), Directory(rest)
func histColWidths(innerW int) [6]int {
	fixed := 14 + 11 + 6 + 9 + 8 + 5 // columns + 5 inter-column gaps
	dirW := innerW - fixed
	if dirW < 10 {
		dirW = 10
	}
	return [6]int{14, 11, 6, 9, 8, dirW}
}

// historyDataRow renders one history table row.
func historyDataRow(s data.SessionBlock, colW [6]int, cursor bool) string {
	updatedAt := s.StartTime
	if s.ActualEndTime != nil {
		updatedAt = *s.ActualEndTime
	}

	dirStr := shortenPath(s.Directory, colW[5])
	cols := []string{
		s.StartTime.Local().Format("01-02 15:04"),
		updatedAt.Local().Format("01-02 15:04"),
		fmt.Sprintf("%d", s.MessageCount),
		formatInt(s.TokenCounts.TotalTokens()),
		fmt.Sprintf("$%.3f", s.CostUSD),
		dirStr,
	}

	rowStyle := mutedStyle
	if cursor {
		rowStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorText).
			Background(lipgloss.Color("#374151"))
	}

	parts := make([]string, 6)
	for i, c := range cols {
		parts[i] = rowStyle.Width(colW[i]).Render(truncateStr(c, colW[i]))
	}

	prefix := "  "
	if cursor {
		prefix = "▶ "
	}
	return prefix + strings.Join(parts, " ")
}

// ── Detail panel (shared by Sessions tab) ─────────────────────────────────────

// ── Message table column widths ───────────────────────────────────────────────
// Prefix(2) + Time(11) + Model(14) + Tokens(7) + Cost(9) + Pct(6) + gaps(5) = 54
// Prompt fills the rest of innerW.
const (
	msgColTime   = 11
	msgColModel  = 14
	msgColTokens = 7
	msgColCost   = 9
	msgColPct    = 6
	msgFixedW    = 2 + msgColTime + 1 + msgColModel + 1 + msgColTokens + 1 + msgColCost + 1 + msgColPct + 1
)

// sortedEntries returns all entries sorted by the given column and direction.
func sortedEntries(entries []data.UsageEntry, col detailSortCol, asc bool) []data.UsageEntry {
	sorted := make([]data.UsageEntry, len(entries))
	copy(sorted, entries)
	sort.SliceStable(sorted, func(i, j int) bool {
		var less bool
		switch col {
		case detailSortCost:
			less = sorted[i].CostUSD < sorted[j].CostUSD
		case detailSortTokens:
			ti := sorted[i].InputTokens + sorted[i].OutputTokens + sorted[i].CacheCreationTokens + sorted[i].CacheReadTokens
			tj := sorted[j].InputTokens + sorted[j].OutputTokens + sorted[j].CacheCreationTokens + sorted[j].CacheReadTokens
			less = ti < tj
		case detailSortTime:
			less = sorted[i].Timestamp.Before(sorted[j].Timestamp)
		case detailSortModel:
			less = sorted[i].Model < sorted[j].Model
		}
		if asc {
			return less
		}
		return !less
	})
	return sorted
}

// msgTableHeader renders the column header row for the messages table.
// sortCol and asc control which column shows the sort indicator.
func msgTableHeader(col detailSortCol, asc bool, promptW int) string {
	indicator := func(c detailSortCol) string {
		if c != col {
			return ""
		}
		if asc {
			return " ↑"
		}
		return " ↓"
	}
	dir := func(c detailSortCol, label string, w int) string {
		text := label + indicator(c)
		return labelStyle.Width(w).Render(text)
	}
	return "  " + strings.Join([]string{
		dir(detailSortTime, "HH:MM:SS", msgColTime),
		dir(detailSortModel, "Model", msgColModel),
		dir(detailSortTokens, "Tokens", msgColTokens),
		dir(detailSortCost, "Cost", msgColCost),
		labelStyle.Width(msgColPct).Render("%"),
		labelStyle.Render("Prompt"),
	}, " ")
}

// renderMsgRow renders one message as a single table row.
func renderMsgRow(e data.UsageEntry, sessionCost float64, promptW int, isCursor bool) string {
	tok := e.InputTokens + e.OutputTokens + e.CacheCreationTokens + e.CacheReadTokens
	costPct := 0.0
	if sessionCost > 0 {
		costPct = e.CostUSD / sessionCost * 100
	}
	c := modelColor(e.Model)

	prefix := "  "
	rowStyle := mutedStyle
	if isCursor {
		prefix = lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render("▶ ")
		rowStyle = lipgloss.NewStyle().Bold(true).Foreground(colorText)
	}

	prompt := e.UserPrompt
	if prompt == "" {
		prompt = mutedStyle.Render("(no prompt)")
	}

	// Collapse newlines in prompt so it fits on one line.
	prompt = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		return r
	}, prompt)

	return prefix + strings.Join([]string{
		rowStyle.Width(msgColTime).Render(e.Timestamp.Local().Format("15:04:05")),
		lipgloss.NewStyle().Foreground(c).Width(msgColModel).Render(truncateStr(e.Model, msgColModel)),
		rowStyle.Width(msgColTokens).Render(formatInt(tok)),
		accentValueStyle.Width(msgColCost).Render(fmt.Sprintf("$%.4f", e.CostUSD)),
		mutedStyle.Width(msgColPct).Render(fmt.Sprintf("%.1f%%", costPct)),
		// MaxWidth clips CJK/wide characters correctly (double-width runes count as 2).
		rowStyle.MaxWidth(promptW).Render(prompt),
	}, " ")
}

// sessionUpdatedAt returns the effective end time of a session block.
func sessionUpdatedAt(s *data.SessionBlock) time.Time {
	if s.ActualEndTime != nil {
		return *s.ActualEndTime
	}
	return s.StartTime
}

// renderDetailHead renders the head box content: window, token summary, per-model.
func renderDetailHead(s *data.SessionBlock, innerW int) string {
	updatedAt := sessionUpdatedAt(s)
	tc := s.TokenCounts
	totalTok := tc.TotalTokens()

	lines := []string{
		sectionTitleStyle.Render("  SESSION DETAIL"),
		fmt.Sprintf("  %s %s → %s  (%s)    %s %s",
			labelStyle.Render("Window:"),
			s.StartTime.Local().Format("2006-01-02 15:04"),
			updatedAt.Local().Format("15:04"),
			formatDuration(updatedAt.Sub(s.StartTime)),
			labelStyle.Render("Dir:"),
			mutedStyle.Render(shortenPath(s.Directory, innerW-50)),
		),
		fmt.Sprintf("  %s %s · %s %s · %s %s · %s %s  =  %s  %s  %s",
			labelStyle.Render("In:"), mutedStyle.Render(formatInt(tc.InputTokens)),
			labelStyle.Render("Out:"), mutedStyle.Render(formatInt(tc.OutputTokens)),
			labelStyle.Render("CR:"), mutedStyle.Render(formatInt(tc.CacheReadTokens)),
			labelStyle.Render("CW:"), mutedStyle.Render(formatInt(tc.CacheCreationTokens)),
			valueStyle.Render(formatInt(totalTok)),
			accentValueStyle.Render(fmt.Sprintf("$%.4f", s.CostUSD)),
			mutedStyle.Render(fmt.Sprintf("(%d msgs)", s.MessageCount)),
		),
	}

	// Per-model on one line each, sorted by token count desc.
	if len(s.PerModelStats) > 0 {
		type modelRow struct {
			name string
			ms   *data.ModelStats
		}
		var mrows []modelRow
		for name, ms := range s.PerModelStats {
			mrows = append(mrows, modelRow{name, ms})
		}
		sort.Slice(mrows, func(i, j int) bool {
			return mrows[i].ms.TotalTokens() > mrows[j].ms.TotalTokens()
		})
		for _, r := range mrows {
			pct := 0.0
			if totalTok > 0 {
				pct = float64(r.ms.TotalTokens()) / float64(totalTok) * 100
			}
			c := modelColor(r.name)
			lines = append(lines, fmt.Sprintf("  %s %s  %s  $%.4f  %5.1f%%",
				lipgloss.NewStyle().Foreground(c).Render("●"),
				lipgloss.NewStyle().Foreground(c).Bold(true).Width(16).Render(r.name),
				mutedStyle.Render(formatInt(r.ms.TotalTokens())),
				r.ms.CostUSD, pct,
			))
		}
	}

	return strings.Join(lines, "\n")
}

// renderDetailMsgsContent renders the content for the messages box.
// contentH is the available height inside the box (excluding border).
func renderDetailMsgsContent(m Model, s *data.SessionBlock, msgs []data.UsageEntry, innerW, contentH int) string {
	promptW := max(10, innerW-msgFixedW)
	divider := mutedStyle.Render("  " + strings.Repeat("─", min(innerW-2, m.width-6)))
	msgCount := len(msgs)

	progress := ""
	if msgCount > 0 {
		progress = mutedStyle.Render(fmt.Sprintf(" [%d/%d]", m.sessions.detailMsgCursor+1, msgCount))
	}

	header := []string{
		"  " + sectionTitleStyle.Render("Messages") + progress,
		msgTableHeader(m.sessions.detailSort, m.sessions.detailSortAsc, promptW),
		divider,
	}

	// Rows available after header (3 lines) inside the box.
	availRows := contentH - len(header)
	if availRows < 1 {
		availRows = 1
	}

	scroll := 0
	if m.sessions.detailMsgCursor >= availRows {
		scroll = m.sessions.detailMsgCursor - availRows + 1
	}

	var lines []string
	lines = append(lines, header...)
	end := min(scroll+availRows, msgCount)
	for i := scroll; i < end; i++ {
		lines = append(lines, renderMsgRow(msgs[i], s.CostUSD, promptW, i == m.sessions.detailMsgCursor))
	}

	return padToHeight(strings.Join(lines, "\n"), contentH)
}

// renderDetailPanel renders the session detail view as three stacked boxes:
// head (session info), chart (cost over time), messages (sortable table).
// Each box has a fixed height; messages fills the remaining space.
func renderDetailPanel(m Model, s *data.SessionBlock, height int) string {
	msgs := sortedEntries(s.Entries, m.sessions.detailSort, m.sessions.detailSortAsc)

	// Chart highlight: timestamp of the selected message.
	var highlightTime *time.Time
	if len(msgs) > 0 && m.sessions.detailMsgCursor < len(msgs) {
		t := msgs[m.sessions.detailMsgCursor].Timestamp
		highlightTime = &t
	}

	updatedAt := sessionUpdatedAt(s)
	// boxW = m.width - 2 → total rendered box width = m.width (border adds 2).
	// innerW = boxW - 2 (padding each side) = m.width - 4.
	boxW := m.width - 2
	innerW := m.width - 4

	// ── Head box ──────────────────────────────────────────────────────────────
	headBox := cardStyle.Width(boxW).Render(renderDetailHead(s, innerW))
	headH := strings.Count(headBox, "\n") + 1

	// ── Chart box ─────────────────────────────────────────────────────────────
	chartContent := "  " + sectionTitleStyle.Render("Cost over time") + "\n" +
		RenderCostChart(s.Entries, s.StartTime, updatedAt, innerW-2, highlightTime)
	chartBox := cardStyle.Width(boxW).Render(chartContent)
	chartH := strings.Count(chartBox, "\n") + 1

	// ── Messages box ──────────────────────────────────────────────────────────
	// hint line sits below the messages box.
	hint := mutedStyle.Render("  ↑↓ select  s/S sort  / dir  ESC back")
	msgsBoxH := height - headH - chartH - 1
	if msgsBoxH < 5 {
		msgsBoxH = 5
	}
	// Content height inside the box = total height - 2 border lines.
	msgsContentH := msgsBoxH - 2
	msgsContent := renderDetailMsgsContent(m, s, msgs, innerW, msgsContentH)
	msgsBox := activeCardStyle.Width(boxW).Height(msgsContentH).Render(msgsContent)

	return strings.Join([]string{headBox, chartBox, msgsBox, hint}, "\n")
}

// RenderDetailPanelForTest renders the session detail panel at the given
// terminal width/height. Used by cmd/bench for visual layout verification.
func RenderDetailPanelForTest(block data.SessionBlock, width, height int) string {
	m := NewModel("pro", "")
	m.width = width
	m.height = height
	return renderDetailPanel(m, &block, height-3)
}

// ── Shared helpers ────────────────────────────────────────────────────────────

// padToHeight pads (or clips) content to exactly h lines.
func padToHeight(content string, h int) string {
	lines := strings.Split(content, "\n")
	for len(lines) < h {
		lines = append(lines, "")
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	return strings.Join(lines, "\n")
}

func clampPct(pct float64) float64 {
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return pct
}

// truncateStr clips s to at most maxW runes, appending "…" if truncated.
func truncateStr(s string, maxW int) string {
	runes := []rune(s)
	if len(runes) <= maxW {
		return s
	}
	if maxW <= 1 {
		return "…"
	}
	return string(runes[:maxW-1]) + "…"
}

// shortenPath trims a file path to fit within maxW characters.
func shortenPath(path string, maxW int) string {
	if len(path) <= maxW {
		return path
	}
	parts := strings.Split(filepath.ToSlash(path), "/")
	result := parts[len(parts)-1]
	for i := len(parts) - 2; i >= 0; i-- {
		candidate := parts[i] + "/" + result
		if len(candidate)+2 > maxW {
			return "…/" + result
		}
		result = candidate
	}
	return result
}

// formatInt formats a large integer with k/M suffixes.
func formatInt(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// formatDuration formats a duration into a human-readable string like "2h 15m".
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0m"
	}
	h := int(d.Hours())
	mn := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, mn)
	}
	return fmt.Sprintf("%dm", mn)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
