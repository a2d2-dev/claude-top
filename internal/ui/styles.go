// Package ui provides terminal UI rendering for the Claude usage monitor.
package ui

import "github.com/charmbracelet/lipgloss"

// ── Base palette ──────────────────────────────────────────────────────────────

var (
	colorPrimary = lipgloss.Color("#7C3AED") // violet  – header bg
	colorAccent  = lipgloss.Color("#F59E0B") // amber   – active / highlight
	colorSuccess = lipgloss.Color("#10B981") // emerald – low cost
	colorWarning = lipgloss.Color("#F97316") // orange  – medium cost
	colorDanger  = lipgloss.Color("#EF4444") // red     – high cost
	colorMuted   = lipgloss.Color("#6B7280") // gray    – secondary text
	colorText    = lipgloss.Color("#F3F4F6") // near-white
	colorSubtext = lipgloss.Color("#9CA3AF") // light gray
	colorBorder  = lipgloss.Color("#374151") // border gray
)

// ── Model / provider colors (matches tokscale semantics) ─────────────────────

var (
	colorOpus    = lipgloss.Color("#A78BFA") // purple
	colorSonnet  = lipgloss.Color("#60A5FA") // blue
	colorHaiku   = lipgloss.Color("#34D399") // green
	colorUnknown = lipgloss.Color("#9CA3AF") // gray
)

// ── Contribution graph intensity grades (tokscale-inspired) ──────────────────
// Index 0 = no activity, 4 = peak activity.

var contribGrades = [5]lipgloss.Color{
	lipgloss.Color("#1F2937"), // 0 – empty
	lipgloss.Color("#064E3B"), // 1 – very low
	lipgloss.Color("#065F46"), // 2 – low
	lipgloss.Color("#059669"), // 3 – medium
	lipgloss.Color("#34D399"), // 4 – high
}

// ── Style presets ─────────────────────────────────────────────────────────────

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorText).
			Background(colorPrimary).
			Padding(0, 2)

	labelStyle = lipgloss.NewStyle().
			Foreground(colorSubtext)

	valueStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorText)

	accentValueStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorAccent)

	mutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	activeCardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(0, 1)

	sectionTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorAccent)

	// Tab bar styles.
	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Padding(0, 2)

	tabActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			Padding(0, 2)
)

// ── Color helpers ─────────────────────────────────────────────────────────────

// modelColor returns a consistent color for a given normalized model name.
func modelColor(model string) lipgloss.Color {
	switch {
	case len(model) >= 4 && model[:4] == "Opus":
		return colorOpus
	case len(model) >= 6 && model[:6] == "Sonnet":
		return colorSonnet
	case len(model) >= 5 && model[:5] == "Haiku":
		return colorHaiku
	default:
		return colorUnknown
	}
}

// colorForPercent returns a color scaled from green → amber → orange → red.
func colorForPercent(pct float64) lipgloss.Color {
	switch {
	case pct >= 95:
		return colorDanger
	case pct >= 80:
		return colorWarning
	case pct >= 60:
		return colorAccent
	default:
		return colorSuccess
	}
}

// costColor maps a normalised cost fraction [0,1] to a colour gradient.
func costColor(frac float64) lipgloss.Color {
	switch {
	case frac >= 0.8:
		return colorDanger
	case frac >= 0.5:
		return colorWarning
	case frac >= 0.25:
		return colorAccent
	default:
		return colorSuccess
	}
}
