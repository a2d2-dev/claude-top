package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/a2d2-dev/claude-usage-monitor/internal/data"
)

// blockLevels maps a fraction [0,1] → one of 8 Unicode block elements.
var blockLevels = []rune("▁▂▃▄▅▆▇█")

// chartYAxisW is the fixed width of the y-axis label column (e.g. "$1.23│").
const chartYAxisW = 8

// RenderCostChart builds a terminal bar chart showing cost-per-bucket over time.
// If highlightTime is non-nil, the bar at that timestamp is rendered in bright
// white and a ▲ marker row is shown below the x-axis labels.
//
// Layout (innerW = available content width):
//
//	$x.xx │ ████ ▇ ▄ ▂                 (chartH rows)
//	      └─────────────────────────
//	      HH:MM          HH:MM
//	           ▲                        (marker row, only when highlightTime set)
func RenderCostChart(entries []data.UsageEntry, start, end time.Time, innerW int, highlightTime *time.Time) string {
	if len(entries) == 0 {
		return mutedStyle.Render("  No entries to chart.")
	}

	chartW := innerW - chartYAxisW - 1 // -1 for the "│" separator
	if chartW < 10 {
		chartW = 10
	}
	const chartH = 6

	// ── Bucket entries by time ────────────────────────────────────────────────
	duration := end.Sub(start)
	if duration <= 0 {
		duration = time.Minute
	}
	bucketDur := duration / time.Duration(chartW)
	if bucketDur < time.Second {
		bucketDur = time.Second
	}

	buckets := make([]float64, chartW)
	for _, e := range entries {
		idx := int(e.Timestamp.Sub(start) / bucketDur)
		if idx < 0 {
			idx = 0
		}
		if idx >= chartW {
			idx = chartW - 1
		}
		buckets[idx] += e.CostUSD
	}

	// Compute which bucket to highlight.
	highlightBucket := -1
	if highlightTime != nil && !highlightTime.Before(start) && !highlightTime.After(end) {
		idx := int(highlightTime.Sub(start) / bucketDur)
		if idx < 0 {
			idx = 0
		}
		if idx >= chartW {
			idx = chartW - 1
		}
		highlightBucket = idx
	}

	maxCost := 0.0
	for _, c := range buckets {
		if c > maxCost {
			maxCost = c
		}
	}
	if maxCost == 0 {
		return mutedStyle.Render("  All costs are zero.")
	}

	// ── Render rows top → bottom ──────────────────────────────────────────────
	// Row 0 = top of chart, row chartH-1 = bottom.
	rows := make([]string, chartH)
	for row := 0; row < chartH; row++ {
		// The normalised threshold this row sits at (1.0 = top, 1/chartH = bottom).
		rowTop := float64(chartH-row) / float64(chartH)
		rowBot := float64(chartH-row-1) / float64(chartH)

		// Y-axis label: print on first, middle, and last row.
		yLabel := ""
		switch row {
		case 0:
			yLabel = fmt.Sprintf("$%5.2f", maxCost)
		case chartH / 2:
			yLabel = fmt.Sprintf("$%5.2f", maxCost/2)
		case chartH - 1:
			yLabel = fmt.Sprintf("%6s", "0")
		}
		axis := fmt.Sprintf("%s│", lipgloss.NewStyle().Width(chartYAxisW-1).Render(yLabel))

		var sb strings.Builder
		sb.WriteString(axis)
		for bIdx, c := range buckets {
			frac := c / maxCost
			isHL := bIdx == highlightBucket
			barColor := costColor(frac)
			if isHL {
				barColor = colorText // bright white for highlighted bar
			}
			switch {
			case frac >= rowTop:
				// Full block — bar fills this row entirely.
				sb.WriteString(lipgloss.NewStyle().Foreground(barColor).Render("█"))
			case frac > rowBot:
				// Partial block — bar crosses this row boundary.
				partial := (frac - rowBot) / (rowTop - rowBot)
				lvl := int(partial * float64(len(blockLevels)))
				if lvl >= len(blockLevels) {
					lvl = len(blockLevels) - 1
				}
				sb.WriteString(lipgloss.NewStyle().Foreground(barColor).Render(string(blockLevels[lvl])))
			default:
				if isHL {
					// Show a dim marker on empty highlighted column.
					sb.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render("│"))
				} else {
					sb.WriteString(" ")
				}
			}
		}
		rows[row] = sb.String()
	}

	// ── X-axis baseline ───────────────────────────────────────────────────────
	baseline := strings.Repeat(" ", chartYAxisW-1) + "└" + strings.Repeat("─", chartW)

	// ── Time labels ───────────────────────────────────────────────────────────
	startLabel := start.Local().Format("15:04")
	endLabel := end.Local().Format("15:04")
	gap := chartW - len(startLabel) - len(endLabel)
	timeLine := strings.Repeat(" ", chartYAxisW) + startLabel
	if gap > 0 {
		timeLine += strings.Repeat(" ", gap) + endLabel
	}

	// ── Highlight marker row (▲ at selected message position) ─────────────────
	markerLine := strings.Repeat(" ", chartYAxisW)
	if highlightBucket >= 0 {
		markerLine += strings.Repeat(" ", highlightBucket) +
			lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render("▲") +
			strings.Repeat(" ", max(0, chartW-highlightBucket-1))
	} else {
		markerLine += strings.Repeat(" ", chartW)
	}

	// ── Bucket legend ─────────────────────────────────────────────────────────
	mins := bucketDur.Minutes()
	var bucketLabel string
	switch {
	case mins < 1:
		bucketLabel = fmt.Sprintf("%.0fs/col", bucketDur.Seconds())
	case mins < 60:
		bucketLabel = fmt.Sprintf("%.0fmin/col", mins)
	default:
		bucketLabel = fmt.Sprintf("%.1fh/col", bucketDur.Hours())
	}
	legend := mutedStyle.Render(fmt.Sprintf("  peak $%.4f  │  %s  │  %d msgs",
		maxCost, bucketLabel, len(entries)))

	parts := append(rows, baseline, mutedStyle.Render(timeLine), markerLine, legend)
	return strings.Join(parts, "\n")
}

