package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/a2d2-dev/claude-usage-monitor/internal/data"
)

// renderChat renders the Chat tab: flat message list with search, or message detail.
func renderChat(m Model, height int) string {
	if m.chat.showDetail {
		return renderChatDetail(m, height)
	}
	return renderChatList(m, height)
}

// ── Search bar ───────────────────────────────────────────────────────────────

// renderChatSearchBar renders the search input with a visible bordered box.
func renderChatSearchBar(m Model) string {
	innerW := m.width - 8 // card padding + prefix
	if innerW < 20 {
		innerW = 20
	}

	term := m.chat.searchTerm
	if m.chat.searchInput {
		// Active input: bright border, cursor indicator.
		inputContent := searchInputStyle.Render(term) + searchCursorStyle.Render("▎")
		box := searchBoxActiveStyle.Width(innerW).Render("🔍 " + inputContent)
		return "  " + box
	}
	if term != "" {
		// Filter active but not typing.
		inputContent := searchInputStyle.Render(term) + mutedStyle.Render("  (f edit / Esc clear)")
		box := searchBoxStyle.Width(innerW).Render("🔍 " + inputContent)
		return "  " + box
	}
	// Idle state.
	box := searchBoxStyle.Width(innerW).Render("🔍 " + mutedStyle.Render("Press f to search…"))
	return "  " + box
}

// ── Message list ─────────────────────────────────────────────────────────────

// renderChatList renders all messages in a flat chronological table with search.
func renderChatList(m Model, height int) string {
	if m.loading {
		content := padToHeight(
			sectionTitleStyle.Render("  CHAT")+"\n"+mutedStyle.Render("  Loading…"),
			height-2,
		)
		return cardStyle.Width(m.width - 2).Height(height - 2).Render(content)
	}

	msgs := m.chatMessages()
	innerW := m.width - 4

	// Column widths: Time(18) + Model(14) + Cost(9) + Prompt(rest)
	timeW, modelW, costW := 18, 14, 9
	promptW := innerW - 2 - timeW - modelW - costW - 3 // 2=prefix, 3=gaps
	if promptW < 10 {
		promptW = 10
	}

	// Header.
	header := "  " + strings.Join([]string{
		labelStyle.Width(timeW).Render("Time"),
		labelStyle.Width(modelW).Render("Model"),
		labelStyle.Width(costW).Render("Cost"),
		labelStyle.Render("Prompt"),
	}, " ")
	divider := mutedStyle.Render(strings.Repeat("─", min(innerW, m.width-6)))
	searchBar := renderChatSearchBar(m)

	// Visible rows.
	visibleRows := height - 9 // border(2) + title(1) + searchBox(3) + header(1) + divider(1) + padding
	if visibleRows < 1 {
		visibleRows = 1
	}

	scroll := 0
	if m.chat.cursor >= visibleRows {
		scroll = m.chat.cursor - visibleRows + 1
	}
	end := scroll + visibleRows
	if end > len(msgs) {
		end = len(msgs)
	}

	// Progress info.
	total := len(msgs)
	progressInfo := ""
	if total > 0 {
		shown := min(scroll+visibleRows, total)
		if m.chat.searchTerm != "" {
			progressInfo = mutedStyle.Render(fmt.Sprintf(" [%d-%d / %d matches]", scroll+1, shown, total))
		} else {
			progressInfo = mutedStyle.Render(fmt.Sprintf(" [%d-%d / %d]", scroll+1, shown, total))
		}
	} else if m.chat.searchTerm != "" {
		progressInfo = mutedStyle.Render(" [0 matches]")
	}

	lines := []string{
		sectionTitleStyle.Render("  CHAT") + progressInfo,
		searchBar,
		header,
		divider,
	}

	for i := scroll; i < end; i++ {
		isCursor := i == m.chat.cursor
		lines = append(lines, renderChatMsgRow(msgs[i], timeW, modelW, costW, promptW, isCursor, m.chat.searchTerm))
	}

	content := padToHeight(strings.Join(lines, "\n"), height-2)
	return cardStyle.Width(m.width - 2).Height(height - 2).Render(content)
}

// ── Detail view ──────────────────────────────────────────────────────────────

// renderChatDetail renders the full message content for the selected entry.
func renderChatDetail(m Model, height int) string {
	innerW := m.width - 4
	contentH := height - 4 // border(2) + title(1) + hint(1)
	if contentH < 5 {
		contentH = 5
	}

	if m.chat.selectedEntry == nil {
		content := padToHeight(mutedStyle.Render("  No message selected"), contentH)
		return cardStyle.Width(m.width - 2).Height(height - 2).Render(content)
	}

	e := *m.chat.selectedEntry
	detail := m.chat.msgDetail
	loading := m.chat.msgDetailLoading

	// Reuse the existing renderMsgDetailContent from render.go.
	content := renderMsgDetailContent(e, 0, detail, loading, innerW, contentH)
	return cardStyle.Width(m.width - 2).Height(height - 2).Render(content)
}

// ── Row renderer ─────────────────────────────────────────────────────────────

// renderChatMsgRow renders one message row in the Chat tab.
// Highlights search term in the prompt column when non-empty.
func renderChatMsgRow(e data.UsageEntry, timeW, modelW, costW, promptW int, isCursor bool, searchTerm string) string {
	c := modelColor(e.Model)

	prefix := "  "
	rowStyle := mutedStyle
	if isCursor {
		prefix = lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render("▶ ")
		rowStyle = lipgloss.NewStyle().Bold(true).Foreground(colorText)
	}

	prompt := collapseWhitespace(e.UserPrompt)
	if prompt == "" {
		prompt = mutedStyle.Render("(no prompt)")
	}

	var renderedPrompt string
	if searchTerm != "" && e.UserPrompt != "" {
		renderedPrompt = highlightMatch(prompt, searchTerm, rowStyle, promptW)
	} else {
		renderedPrompt = rowStyle.MaxWidth(promptW).Render(prompt)
	}

	return prefix + strings.Join([]string{
		rowStyle.Width(timeW).Render(e.Timestamp.Local().Format("01-02 15:04:05")),
		lipgloss.NewStyle().Foreground(c).Width(modelW).Render(truncateStr(e.Model, modelW)),
		accentValueStyle.Width(costW).Render(fmt.Sprintf("$%.4f", e.CostUSD)),
		renderedPrompt,
	}, " ")
}
