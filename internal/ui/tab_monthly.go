package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderMonthly renders the Monthly tab:
// top section = cost summary, bottom section = scrollable per-month table.
func renderMonthly(m Model, height int) string {
	if m.loading {
		content := padToHeight(
			sectionTitleStyle.Render("  MONTHLY")+"\n"+mutedStyle.Render("  Loading…"),
			height-2,
		)
		return cardStyle.Width(m.width - 2).Height(height - 2).Render(content)
	}

	if len(m.monthly) == 0 {
		content := padToHeight(
			sectionTitleStyle.Render("  MONTHLY")+"\n"+mutedStyle.Render("  No data."),
			height-2,
		)
		return cardStyle.Width(m.width - 2).Height(height - 2).Render(content)
	}

	innerW := m.width - 4

	// ── Fixed top: cost summary ───────────────────────────────────────────────
	var fixed []string
	fixed = append(fixed, renderMonthlySummary(m)...)
	fixed = append(fixed, "")

	// ── Monthly table header ──────────────────────────────────────────────────
	// Columns: Month(8) Days(5) Msgs(6) Tokens(10) Cost(10) Avg/Day(10)
	const (
		colMonth  = 8
		colDays   = 5
		colMsgs   = 6
		colTok    = 10
		colCost   = 10
		colAvgDay = 10
	)
	divider := mutedStyle.Render(strings.Repeat("─", min(innerW, m.width-6)))
	header := fmt.Sprintf("  %s %s %s %s %s %s",
		labelStyle.Width(colMonth).Render("Month"),
		labelStyle.Width(colDays).Render("Days"),
		labelStyle.Width(colMsgs).Render("Msgs"),
		labelStyle.Width(colTok).Render("Tokens"),
		labelStyle.Width(colCost).Render("Cost"),
		labelStyle.Width(colAvgDay).Render("Avg/Day"),
	)

	// Show newest first.
	n := len(m.monthly)
	months := make([]int, n)
	for i := range months {
		months[i] = n - 1 - i
	}

	// Compute visible rows and scroll offset.
	fixedH := len(fixed) + 3 // header + divider + title
	availH := (height - 2) - fixedH
	if availH < 1 {
		availH = 1
	}
	scroll := 0
	if m.monthlyCur >= availH {
		scroll = m.monthlyCur - availH + 1
	}
	end := min(scroll+availH, n)

	progressInfo := mutedStyle.Render(
		fmt.Sprintf(" [%d-%d / %d]", scroll+1, min(scroll+availH, n), n))

	fixed = append(fixed,
		sectionTitleStyle.Render("  MONTHLY")+progressInfo,
		header,
		divider,
	)

	// ── Scrollable rows ───────────────────────────────────────────────────────
	var lines []string
	lines = append(lines, fixed...)

	for i := scroll; i < end; i++ {
		ms := m.monthly[months[i]]
		isCursor := i == m.monthlyCur

		avgDay := 0.0
		if ms.DayCount > 0 {
			avgDay = ms.CostUSD / float64(ms.DayCount)
		}

		row := fmt.Sprintf("  %s %s %s %s %s %s",
			ms.Date.Local().Format("2006-01"),
			fmt.Sprintf("%*d", colDays, ms.DayCount),
			fmt.Sprintf("%*d", colMsgs, ms.MessageCount),
			lipgloss.NewStyle().Width(colTok).Render(formatInt(ms.TokenCounts.TotalTokens())),
			lipgloss.NewStyle().Width(colCost).Render(fmt.Sprintf("$%.3f", ms.CostUSD)),
			mutedStyle.Width(colAvgDay).Render(fmt.Sprintf("$%.3f", avgDay)),
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

// renderMonthlySummary renders cost summary statistics for the monthly view.
func renderMonthlySummary(m Model) []string {
	if len(m.monthly) == 0 {
		return nil
	}

	totalCost := 0.0
	maxCost, minCost := 0.0, math.MaxFloat64
	var maxMonth, minMonth string
	activeMonths := 0

	for _, ms := range m.monthly {
		totalCost += ms.CostUSD
		if ms.CostUSD > 0 {
			activeMonths++
			if ms.CostUSD > maxCost {
				maxCost = ms.CostUSD
				maxMonth = ms.Date.Format("2006-01")
			}
			if ms.CostUSD < minCost {
				minCost = ms.CostUSD
				minMonth = ms.Date.Format("2006-01")
			}
		}
	}
	if activeMonths == 0 {
		minCost = 0
	}
	avgCost := 0.0
	if activeMonths > 0 {
		avgCost = totalCost / float64(activeMonths)
	}

	innerW := m.width - 4
	sep := mutedStyle.Render(strings.Repeat("─", min(innerW, 60)))
	lines := []string{
		sectionTitleStyle.Render("  COST SUMMARY"),
		sep,
		fmt.Sprintf("  %s %s   %s %s   %s %s",
			labelStyle.Render("Total cost:"), accentValueStyle.Render(fmt.Sprintf("$%.2f", totalCost)),
			labelStyle.Render("Months:"), valueStyle.Render(fmt.Sprintf("%d", activeMonths)),
			labelStyle.Render("Avg/month:"), mutedStyle.Render(fmt.Sprintf("$%.2f", avgCost)),
		),
		fmt.Sprintf("  %s %s (%s)   %s %s (%s)",
			labelStyle.Render("Peak month:"), accentValueStyle.Render(fmt.Sprintf("$%.2f", maxCost)),
			mutedStyle.Render(maxMonth),
			labelStyle.Render("Min month:"), mutedStyle.Render(fmt.Sprintf("$%.2f", minCost)),
			mutedStyle.Render(minMonth),
		),
	}
	return lines
}
