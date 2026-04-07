package ui

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/a2d2-dev/claude-usage-monitor/internal/data"
)

// renderContribGraph builds a GitHub-style 52-week activity graph.
// Each cell = "██" (2 chars) colored by cost intensity grade (0–4).
func renderContribGraph(daily []data.DailyStats, innerW int) []string {
	// ── Build date → cost index ───────────────────────────────────────────────
	costByDate := make(map[string]float64, len(daily))
	for _, d := range daily {
		costByDate[d.Date.Format("2006-01-02")] = d.CostUSD
	}

	// ── Compute intensity thresholds (percentile-based) ───────────────────────
	var nonZero []float64
	for _, c := range costByDate {
		if c > 0 {
			nonZero = append(nonZero, c)
		}
	}
	sort.Float64s(nonZero)
	thresholds := contribThresholds(nonZero)

	// ── Layout: how many weeks fit? ───────────────────────────────────────────
	// Each week column = 2 chars (cell) + no gap between cols.
	// Left margin for day labels = 3 chars.
	maxWeeks := (innerW - 3) / 2
	if maxWeeks > 52 {
		maxWeeks = 52
	}
	if maxWeeks < 4 {
		maxWeeks = 4
	}

	// ── Start date: Monday, maxWeeks weeks ago ────────────────────────────────
	today := time.Now().Local()
	// Find last Monday (or today if Monday).
	weekday := int(today.Weekday()) // Sun=0
	daysToMon := (weekday + 6) % 7  // days since last Monday
	lastMon := today.AddDate(0, 0, -daysToMon)
	startMon := lastMon.AddDate(0, 0, -(maxWeeks-1)*7)

	// ── Month label row ───────────────────────────────────────────────────────
	monthBuf := make([]string, maxWeeks*2+3)
	for i := range monthBuf {
		monthBuf[i] = " "
	}
	prevMonth := -1
	for w := range maxWeeks {
		weekMon := startMon.AddDate(0, 0, w*7)
		if int(weekMon.Month()) != prevMonth {
			label := weekMon.Format("Jan")
			col := 3 + w*2
			for ci, ch := range label {
				if col+ci < len(monthBuf) {
					monthBuf[col+ci] = string(ch)
				}
			}
			prevMonth = int(weekMon.Month())
		}
	}
	monthLine := "   " + strings.Join(monthBuf[3:], "")

	// ── Day-of-week labels ────────────────────────────────────────────────────
	// ISO week: Mon(0)...Sun(6)
	dayLabels := []string{"Mo", "  ", "We", "  ", "Fr", "  ", "Su"}

	// ── Grid rows ─────────────────────────────────────────────────────────────
	gridRows := make([]string, 7)
	for dow := range 7 {
		var row strings.Builder
		row.WriteString(dayLabels[dow] + " ")
		for w := range maxWeeks {
			date := startMon.AddDate(0, 0, w*7+dow)
			if date.After(today) {
				row.WriteString("  ")
				continue
			}
			cost := costByDate[date.Format("2006-01-02")]
			grade := contribGrade(cost, thresholds)
			if cost == 0 {
				row.WriteString(mutedStyle.Render("· "))
			} else {
				cell := lipgloss.NewStyle().Foreground(contribGrades[grade]).Render("██")
				row.WriteString(cell)
			}
		}
		gridRows[dow] = row.String()
	}

	// ── Legend ────────────────────────────────────────────────────────────────
	legendParts := []string{mutedStyle.Render("   Less")}
	for g := 1; g <= 4; g++ {
		legendParts = append(legendParts,
			lipgloss.NewStyle().Foreground(contribGrades[g]).Render("██"))
	}
	legendParts = append(legendParts, mutedStyle.Render("More"))
	legend := strings.Join(legendParts, " ")

	out := []string{mutedStyle.Render(monthLine)}
	out = append(out, gridRows...)
	out = append(out, legend)
	return out
}

// renderStatsTable renders a small summary table below the contribution graph.
func renderStatsTable(daily []data.DailyStats, innerW int) []string {
	if len(daily) == 0 {
		return nil
	}

	// Collect totals.
	totalCost := 0.0
	maxDayCost, minDayCost := 0.0, math.MaxFloat64
	var maxDay, minDay time.Time
	activeDays := 0
	for _, d := range daily {
		totalCost += d.CostUSD
		if d.CostUSD > 0 {
			activeDays++
			if d.CostUSD > maxDayCost {
				maxDayCost = d.CostUSD
				maxDay = d.Date
			}
			if d.CostUSD < minDayCost {
				minDayCost = d.CostUSD
				minDay = d.Date
			}
		}
	}
	if activeDays == 0 {
		minDayCost = 0
	}
	avgCost := 0.0
	if activeDays > 0 {
		avgCost = totalCost / float64(activeDays)
	}

	sep := mutedStyle.Render(strings.Repeat("─", min(innerW, 60)))
	lines := []string{
		sectionTitleStyle.Render("  COST SUMMARY"),
		sep,
		fmt.Sprintf("  %s %s   %s %s   %s %s",
			labelStyle.Render("Total cost:"), accentValueStyle.Render(fmt.Sprintf("$%.2f", totalCost)),
			labelStyle.Render("Active days:"), valueStyle.Render(fmt.Sprintf("%d", activeDays)),
			labelStyle.Render("Avg/day:"), mutedStyle.Render(fmt.Sprintf("$%.2f", avgCost)),
		),
		fmt.Sprintf("  %s %s (%s)   %s %s (%s)",
			labelStyle.Render("Peak day:"), accentValueStyle.Render(fmt.Sprintf("$%.2f", maxDayCost)),
			mutedStyle.Render(maxDay.Format("2006-01-02")),
			labelStyle.Render("Min day:"), mutedStyle.Render(fmt.Sprintf("$%.2f", minDayCost)),
			mutedStyle.Render(minDay.Format("2006-01-02")),
		),
	}
	return lines
}

// ── Contribution graph helpers ────────────────────────────────────────────────

// contribThresholds returns cost boundaries for grades 1-4 based on percentiles
// of the non-zero daily costs slice (must be pre-sorted ascending).
func contribThresholds(sorted []float64) [4]float64 {
	n := len(sorted)
	if n == 0 {
		return [4]float64{0, 0, 0, 0}
	}
	pct := func(p int) float64 {
		idx := n * p / 100
		if idx >= n {
			idx = n - 1
		}
		return sorted[idx]
	}
	return [4]float64{pct(25), pct(50), pct(75), math.MaxFloat64}
}

// contribGrade maps a daily cost to a grade 0-4.
func contribGrade(cost float64, t [4]float64) int {
	if cost <= 0 {
		return 0
	}
	if cost < t[0] {
		return 1
	}
	if cost < t[1] {
		return 2
	}
	if cost < t[2] {
		return 3
	}
	return 4
}
