package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// dailyBarW is the fixed width of the mini percentage bar in the daily table.
const dailyBarW = 20

// renderDaily renders the merged Daily + Stats tab:
// top section = 52-week contribution graph + cost summary,
// bottom section = scrollable per-day table.
func renderDaily(m Model, height int) string {
	if m.loading {
		content := padToHeight(
			sectionTitleStyle.Render("  DAILY")+"\n"+mutedStyle.Render("  Loading…"),
			height-2,
		)
		return cardStyle.Width(m.width - 2).Height(height - 2).Render(content)
	}

	if len(m.daily) == 0 {
		content := padToHeight(
			sectionTitleStyle.Render("  DAILY")+"\n"+mutedStyle.Render("  No data."),
			height-2,
		)
		return cardStyle.Width(m.width - 2).Height(height - 2).Render(content)
	}

	innerW := m.width - 4

	// ── Fixed top: contribution graph + cost summary ───────────────────────────
	var fixed []string
	fixed = append(fixed, sectionTitleStyle.Render("  DAILY & STATS"), "")
	fixed = append(fixed, renderContribGraph(m.daily, innerW)...)
	fixed = append(fixed, "")
	fixed = append(fixed, renderStatsTable(m.daily, innerW)...)
	fixed = append(fixed, "")

	// ── Daily table header ─────────────────────────────────────────────────────
	// Columns: Date(10) Msgs(5) Tokens(10) Cost(9) [bar] pct(6)
	// Row:  2-prefix + 10 + 1 + 5 + 1 + 10 + 1 + 9 + 1 + [dailyBarW+2] + 1 + 6
	const (
		colDate   = 10
		colMsgs   = 5
		colTok    = 10
		colCost   = 9
		colPct    = 6
	)
	divider := mutedStyle.Render(strings.Repeat("─", min(innerW, m.width-6)))
	header := fmt.Sprintf("  %s %s %s %s  %s  %s",
		labelStyle.Width(colDate).Render("Date"),
		labelStyle.Width(colMsgs).Render("Msgs"),
		labelStyle.Width(colTok).Render("Tokens"),
		labelStyle.Width(colCost).Render("Cost"),
		labelStyle.Width(dailyBarW).Render("% of total"),
		labelStyle.Width(colPct).Render(""),
	)

	// Compute total cost for % column.
	totalCost := 0.0
	for _, d := range m.daily {
		totalCost += d.CostUSD
	}

	// Show newest first.
	n := len(m.daily)
	days := make([]int, n) // each entry = index into m.daily
	for i := range days {
		days[i] = n - 1 - i
	}

	// Compute visible rows and scroll offset.
	fixedH := len(fixed) + 3 // header + divider + title
	availH := (height - 2) - fixedH
	if availH < 1 {
		availH = 1
	}
	scroll := 0
	if m.dailyCur >= availH {
		scroll = m.dailyCur - availH + 1
	}
	end := min(scroll+availH, n)

	progressInfo := mutedStyle.Render(
		fmt.Sprintf(" [%d-%d / %d]", scroll+1, min(scroll+availH, n), n))

	fixed = append(fixed,
		sectionTitleStyle.Render("  DAILY")+progressInfo,
		header,
		divider,
	)

	// ── Scrollable rows ────────────────────────────────────────────────────────
	var lines []string
	lines = append(lines, fixed...)

	for i := scroll; i < end; i++ {
		d := m.daily[days[i]]
		isCursor := i == m.dailyCur

		pct := 0.0
		if totalCost > 0 {
			pct = d.CostUSD / totalCost * 100
		}

		filled := int(pct / 100 * float64(dailyBarW))
		if filled > dailyBarW {
			filled = dailyBarW
		}
		bar := lipgloss.NewStyle().Foreground(costColor(pct/100)).Render(strings.Repeat("█", filled)) +
			mutedStyle.Render(strings.Repeat("░", dailyBarW-filled))

		row := fmt.Sprintf("  %s %s %s %s [%s] %s",
			d.Date.Local().Format("01-02 Mon"),
			fmt.Sprintf("%*d", colMsgs, d.MessageCount),
			lipgloss.NewStyle().Width(colTok).Render(formatInt(d.TokenCounts.TotalTokens())),
			lipgloss.NewStyle().Width(colCost).Render(fmt.Sprintf("$%.3f", d.CostUSD)),
			bar,
			mutedStyle.Render(fmt.Sprintf("%5.1f%%", pct)),
		)

		if isCursor {
			row = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorText).
				Background(lipgloss.Color("#374151")).
				Render(row)
		} else {
			row = mutedStyle.Render(row)
		}
		lines = append(lines, row)
	}

	content := padToHeight(strings.Join(lines, "\n"), height-2)
	return cardStyle.Width(m.width - 2).Height(height - 2).Render(content)
}
