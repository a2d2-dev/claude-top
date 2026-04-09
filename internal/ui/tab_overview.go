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
		lines = append(lines, sectionTitleStyle.Render("  RECENT COST  (30 days)"))
		lines = append(lines, renderDailyCostChart(m, innerW-2, chartH))
	}

	content := padToHeight(strings.Join(lines, "\n"), height-2)
	return cardStyle.Width(m.width - 2).Height(height - 2).Render(content)
}

// renderDailyCostChart renders a compact bar chart of daily costs for the last 30 days.
// chartH controls how many rows tall the bar chart area is (min 4).
func renderDailyCostChart(m Model, width int, chartH int) string {
	if chartH < 4 {
		chartH = 4
	}
	// Take the last N days that fit.
	n := min(30, len(m.daily))
	days := m.daily[len(m.daily)-n:]

	if n == 0 || width < 10 {
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

	chartW := width - 10 // leave space for y-axis
	// Each day gets barW chars; spread days across available width.
	barW := max(1, chartW/n)
	// Recompute how many days actually fit.
	fitDays := min(n, chartW/barW)
	if fitDays < 1 {
		fitDays = 1
	}
	days = days[n-fitDays:]
	n = fitDays
	chartW = n * barW

	// Sample days to fit chart width (one entry per bar slot).
	sampled := make([]float64, n)
	for i := range n {
		sampled[i] = days[i].CostUSD
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
				cell = strings.Repeat("█", barW)
			case frac > rowBot:
				partial := (frac - rowBot) / (rowTop - rowBot)
				lvl := int(partial * float64(len(blockLevels)))
				if lvl >= len(blockLevels) {
					lvl = len(blockLevels) - 1
				}
				// Fill left portion with full block, rightmost char with partial.
				cell = strings.Repeat("█", barW-1) + string(blockLevels[lvl])
			default:
				cell = strings.Repeat(" ", barW)
			}
			sb.WriteString(lipgloss.NewStyle().Foreground(costColor(frac)).Render(cell))
		}
		rows[row] = sb.String()
	}

	// X-axis.
	baseline := strings.Repeat(" ", 7) + "└" + strings.Repeat("─", chartW)

	// Time labels.
	startLabel := days[0].Date.Format("01/02")
	endLabel := days[len(days)-1].Date.Format("01/02")
	gap := chartW - len(startLabel) - len(endLabel)
	timeLine := strings.Repeat(" ", 8) + startLabel
	if gap > 0 {
		timeLine += strings.Repeat(" ", gap) + endLabel
	}

	parts := append(rows, baseline, mutedStyle.Render(timeLine))
	return strings.Join(parts, "\n")
}
