package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// renderOverview renders the Overview tab: active session card + totals + cost chart.
// The chart dynamically fills the remaining vertical space.
func renderOverview(m Model, height int) string {
	now := time.Now().UTC()
	active := m.activeBlock()
	innerW := m.width - 4

	var fixedLines []string

	// ── Active session ────────────────────────────────────────────────────────
	if m.loading && active == nil {
		fixedLines = append(fixedLines,
			sectionTitleStyle.Render("● SESSION"),
			mutedStyle.Render("  Loading…"),
		)
	} else if active != nil {
		fixedLines = append(fixedLines, renderSessionCard(m, active, now, innerW))
	} else {
		fixedLines = append(fixedLines,
			sectionTitleStyle.Render("○ NO ACTIVE SESSION"),
			mutedStyle.Render("  Start Claude Code to begin tracking."),
		)
	}

	fixedLines = append(fixedLines, "")

	// ── All-time totals ───────────────────────────────────────────────────────
	var totalTokens int
	var totalCost float64
	var totalMessages int
	for i := range m.blocks {
		if m.blocks[i].IsGap {
			continue
		}
		totalTokens += m.blocks[i].TokenCounts.TotalTokens()
		totalCost += m.blocks[i].CostUSD
		totalMessages += m.blocks[i].MessageCount
	}

	fixedLines = append(fixedLines,
		sectionTitleStyle.Render("  ALL-TIME TOTALS"),
		fmt.Sprintf("  %s %s   %s %s   %s %s   %s %s",
			labelStyle.Render("Tokens:"), accentValueStyle.Render(formatInt(totalTokens)),
			labelStyle.Render("Cost:"), accentValueStyle.Render(fmt.Sprintf("$%.2f", totalCost)),
			labelStyle.Render("Messages:"), accentValueStyle.Render(fmt.Sprintf("%d", totalMessages)),
			labelStyle.Render("Sessions:"), accentValueStyle.Render(fmt.Sprintf("%d", len(m.daily))),
		),
	)

	// ── Per-source breakdown (only when both sources have data) ────────────────
	var (
		claudeTokens, codexTokens     int
		claudeCost, codexCost         float64
		claudeBlocks, codexBlocks     int
	)
	for i := range m.blocks {
		b := &m.blocks[i]
		if b.IsGap {
			continue
		}
		switch b.Source {
		case "codex":
			codexTokens += b.TokenCounts.TotalTokens()
			codexCost += b.CostUSD
			codexBlocks++
		default: // "claude" or empty defaults to claude
			claudeTokens += b.TokenCounts.TotalTokens()
			claudeCost += b.CostUSD
			claudeBlocks++
		}
	}
	// Only show per-source rows when both sources have data.
	if claudeBlocks > 0 && codexBlocks > 0 {
		fixedLines = append(fixedLines,
			fmt.Sprintf("  %s  %s %s   %s %s   %s %s",
				mutedStyle.Render("● Claude Code "),
				labelStyle.Render("Tokens:"), mutedStyle.Render(formatInt(claudeTokens)),
				labelStyle.Render("Cost:"), mutedStyle.Render(fmt.Sprintf("$%.2f", claudeCost)),
				labelStyle.Render("Sessions:"), mutedStyle.Render(fmt.Sprintf("%d", claudeBlocks)),
			),
			fmt.Sprintf("  %s  %s %s   %s %s   %s %s",
				mutedStyle.Render("✦ Codex CLI   "),
				labelStyle.Render("Tokens:"), mutedStyle.Render(formatInt(codexTokens)),
				labelStyle.Render("Cost:"), mutedStyle.Render(fmt.Sprintf("$%.2f", codexCost)),
				labelStyle.Render("Sessions:"), mutedStyle.Render(fmt.Sprintf("%d", codexBlocks)),
			),
		)
	}

	// ── Cost chart: fixed height like tokscale ───────────────────────────────
	const chartH = 10

	var lines []string
	lines = append(lines, fixedLines...)

	if len(m.daily) > 0 {
		lines = append(lines, "")
		lines = append(lines, sectionTitleStyle.Render("  COST HISTORY"))
		lines = append(lines, renderDailyCostChart(m, innerW-2, chartH))
	}

	content := padToHeight(strings.Join(lines, "\n"), height-2)
	return cardStyle.Width(m.width - 2).Height(height - 2).Render(content)
}

// renderDailyCostChart renders a bar chart of all daily costs with a time-proportional
// X-axis. Each column represents a fixed time slice of the total span; days with no
// activity are empty. This prevents sparse old data from producing equal-width fat bars.
// chartH controls how many rows tall the bar chart area is (min 4).
func renderDailyCostChart(m Model, width int, chartH int) string {
	if chartH < 4 {
		chartH = 4
	}

	days := m.daily
	if len(days) == 0 || width < 10 {
		return mutedStyle.Render("  No data")
	}

	// Find max for scaling.
	maxCost := 0.0
	for _, d := range days {
		if d.CostUSD > maxCost {
			maxCost = d.CostUSD
		}
	}
	if maxCost == 0 {
		return mutedStyle.Render("  No cost data")
	}

	chartW := width - 10 // leave space for y-axis label

	// Build a lookup: date → cost.
	const day = 24 * time.Hour
	costByDate := make(map[time.Time]float64, len(days))
	for _, d := range days {
		key := d.Date.UTC().Truncate(day)
		costByDate[key] = d.CostUSD
	}

	// Compute time span from first data day to today.
	startDate := days[0].Date.UTC().Truncate(day)
	endDate := time.Now().UTC().Truncate(day)
	if days[len(days)-1].Date.After(endDate) {
		endDate = days[len(days)-1].Date.UTC().Truncate(day)
	}
	totalDays := int(endDate.Sub(startDate).Hours()/24) + 1

	// Map each chart column to the cost of the day it represents.
	// Multiple days that fall in the same column are summed.
	sampled := make([]float64, chartW)
	for col := range chartW {
		// Which day does this column start at?
		dayOffset := int(float64(col) * float64(totalDays) / float64(chartW))
		date := startDate.Add(time.Duration(dayOffset) * day)
		sampled[col] = costByDate[date]
	}

	// Render rows.
	rows := make([]string, chartH)
	for row := range chartH {
		rowTop := float64(chartH-row) / float64(chartH)
		rowBot := float64(chartH-row-1) / float64(chartH)

		yLabel := ""
		switch row {
		case 0:
			yLabel = fmt.Sprintf("$%5.2f", maxCost)
		case chartH - 1:
			yLabel = fmt.Sprintf("%6s", "0")
		}
		axis := fmt.Sprintf("%s│", lipgloss.NewStyle().Width(7).Render(yLabel))

		var sb strings.Builder
		sb.WriteString(axis)
		for _, c := range sampled {
			frac := c / maxCost
			var cell string
			switch {
			case frac >= rowTop:
				cell = "█"
			case frac > rowBot:
				partial := (frac - rowBot) / (rowTop - rowBot)
				lvl := int(partial * float64(len(blockLevels)))
				if lvl >= len(blockLevels) {
					lvl = len(blockLevels) - 1
				}
				cell = string(blockLevels[lvl])
			default:
				cell = " "
			}
			if c > 0 {
				sb.WriteString(lipgloss.NewStyle().Foreground(costColor(frac)).Render(cell))
			} else {
				sb.WriteString(cell)
			}
		}
		rows[row] = sb.String()
	}

	// X-axis baseline.
	baseline := strings.Repeat(" ", 7) + "└" + strings.Repeat("─", chartW)

	// Time labels: start, intermediate ticks (monthly or yearly), and end.
	// Build a rune slice so we can place labels at exact column positions.
	timeBuf := make([]rune, chartW)
	for i := range timeBuf {
		timeBuf[i] = ' '
	}

	// Place a label at a given column, clamping to stay within the buffer.
	placeLabel := func(col int, label string) {
		// Center the label on col; don't overwrite the start label.
		start := col - len(label)/2
		if start < 0 {
			start = 0
		}
		if start+len(label) > chartW {
			start = chartW - len(label)
		}
		for i, r := range label {
			if start+i < chartW {
				timeBuf[start+i] = r
			}
		}
	}

	// Always place start and end labels.
	startLabel := startDate.Format("01/02")
	endLabel := endDate.Format("01/02")
	placeLabel(0, startLabel)
	placeLabel(chartW-len(endLabel), endLabel)

	// Choose intermediate tick interval: years if span > 18 months, else months.
	spanMonths := (endDate.Year()-startDate.Year())*12 + int(endDate.Month()) - int(startDate.Month())
	tickByYear := spanMonths > 18

	// Walk from the first tick boundary after startDate to just before endDate.
	var tickDate time.Time
	if tickByYear {
		tickDate = time.Date(startDate.Year()+1, 1, 1, 0, 0, 0, 0, time.UTC)
	} else {
		y, m, _ := startDate.Date()
		if m == 12 {
			y++
			m = 1
		} else {
			m++
		}
		tickDate = time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
	}
	for tickDate.Before(endDate) {
		// Column for this tick.
		col := int(float64(tickDate.Sub(startDate).Hours()/24) / float64(totalDays) * float64(chartW))
		var label string
		if tickByYear {
			label = tickDate.Format("2006")
		} else if tickDate.Month() == 1 {
			// January: show year to make year transitions obvious.
			label = tickDate.Format("Jan 2006")
		} else {
			label = tickDate.Format("01/02")
		}
		// Only place the tick if it has enough breathing room from start/end.
		if col > len(startLabel)+1 && col < chartW-len(endLabel)-len(label)-1 {
			placeLabel(col, label)
		}
		// Advance to next tick.
		if tickByYear {
			tickDate = tickDate.AddDate(1, 0, 0)
		} else {
			tickDate = tickDate.AddDate(0, 1, 0)
		}
	}

	timeLine := strings.Repeat(" ", 8) + string(timeBuf)

	parts := append(rows, baseline, mutedStyle.Render(timeLine))
	return strings.Join(parts, "\n")
}
