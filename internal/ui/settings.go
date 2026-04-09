package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/a2d2-dev/claude-usage-monitor/internal/config"
	"github.com/a2d2-dev/claude-usage-monitor/internal/core"
	"github.com/a2d2-dev/claude-usage-monitor/internal/data"
)

// ── Settings overlay state ────────────────────────────────────────────────────

// settingsPhase tracks whether the Settings modal is open or closed.
type settingsPhase int

const (
	settingsIdle   settingsPhase = iota // settings modal not shown
	settingsOpen                        // settings modal visible
)

// settingsSourceOption holds a source filter option displayed in the modal.
type settingsSourceOption struct {
	value string // "all", "claude", "codex"
	label string // display label
}

var settingsSourceOptions = []settingsSourceOption{
	{"all", "All sources (Claude Code + Codex CLI)"},
	{"claude", "Claude Code only"},
	{"codex", "Codex CLI only"},
}

// settingsState holds the UI state for the Settings modal.
type settingsState struct {
	phase    settingsPhase
	cursor   int    // index into settingsSourceOptions
	selected string // current source value
	codexPath string // editable codex path
	editing  bool   // true when the codex path text field is focused
}

// selectedSourceIndex returns the cursor index for a given source string.
func selectedSourceIndex(source string) int {
	for i, opt := range settingsSourceOptions {
		if opt.value == source {
			return i
		}
	}
	return 0 // default to "all"
}

// ── Settings key handler ──────────────────────────────────────────────────────

// handleSettingsKey processes key events when the Settings modal is open.
func (m Model) handleSettingsKey(key string) (Model, settingsReloadMsg) {
	switch key {
	case "esc", "q":
		// Close without saving.
		m.settings.phase = settingsIdle
		return m, settingsReloadMsg{}

	case "up", "k":
		if !m.settings.editing && m.settings.cursor > 0 {
			m.settings.cursor--
		}

	case "down", "j":
		if !m.settings.editing && m.settings.cursor < len(settingsSourceOptions)-1 {
			m.settings.cursor++
		}

	case "tab":
		// Toggle between source list and codex path field.
		m.settings.editing = !m.settings.editing

	case "enter":
		if m.settings.editing {
			// Confirm codex path edit, return to source list.
			m.settings.editing = false
		} else {
			// Save and apply selected source.
			selected := settingsSourceOptions[m.settings.cursor].value
			m.settings.selected = selected
			m.source = selected
			m.codexPath = m.settings.codexPath
			m.settings.phase = settingsIdle
			// Persist to config.
			cfg := config.Config{Source: selected, CodexPath: m.settings.codexPath}
			_ = config.Save(cfg)
			// Signal caller to reload data.
			return m, settingsReloadMsg{reload: true}
		}

	default:
		// Text input for codex path when editing.
		if m.settings.editing {
			switch key {
			case "backspace":
				if len(m.settings.codexPath) > 0 {
					runes := []rune(m.settings.codexPath)
					m.settings.codexPath = string(runes[:len(runes)-1])
				}
			default:
				// Append printable characters.
				if len(key) == 1 {
					m.settings.codexPath += key
				}
			}
		}
	}
	return m, settingsReloadMsg{}
}

// settingsReloadMsg signals that settings were saved and data should reload.
type settingsReloadMsg struct {
	reload bool
}

// ── Settings rendering ────────────────────────────────────────────────────────

// renderSettingsOverlay renders the Settings modal, replacing the content area.
func renderSettingsOverlay(m Model, height int) string {
	innerW := m.width - 4
	if innerW < 40 {
		innerW = 40
	}

	lines := []string{
		sectionTitleStyle.Render("  SETTINGS"),
		"",
		labelStyle.Render("  Data Source:"),
	}

	// Source options list.
	for i, opt := range settingsSourceOptions {
		line := renderSettingsOption(opt.label, i == m.settings.cursor, i == m.settings.cursor && !m.settings.editing)
		lines = append(lines, line)
	}

	lines = append(lines, "")
	// Codex path field.
	codexPathLabel := labelStyle.Render("  Codex Path:")
	codexPathValue := m.settings.codexPath
	if codexPathValue == "" {
		codexPathValue = mutedStyle.Render("~/.codex/sessions (default)")
	}
	if m.settings.editing {
		codexPathValue = accentValueStyle.Render(codexPathValue + "█")
	}
	lines = append(lines, codexPathLabel)
	lines = append(lines, "    "+codexPathValue)
	lines = append(lines, "")

	// Footer hints.
	if m.settings.editing {
		lines = append(lines, mutedStyle.Render("  Type path  Tab switch  Enter confirm  Esc cancel"))
	} else {
		lines = append(lines, mutedStyle.Render("  ↑↓ select  Tab codex path  Enter save  Esc cancel"))
	}

	content := padToHeight(strings.Join(lines, "\n"), height-2)
	boxStyle := cardStyle.Width(innerW).Height(height - 2)
	return boxStyle.Render(content)
}

// renderSettingsOption renders one source option row in the settings modal.
func renderSettingsOption(label string, isCursor bool, isActive bool) string {
	indicator := "  ○ "
	style := mutedStyle
	if isCursor {
		indicator = "  ● "
		style = lipgloss.NewStyle().Bold(true).Foreground(colorText)
	}
	if isActive {
		indicator = "  ▶ "
		style = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	}
	return fmt.Sprintf("%s%s", indicator, style.Render(label))
}

// openSettings opens the Settings modal, pre-populated with current settings.
func (m Model) openSettings() Model {
	m.settings = settingsState{
		phase:     settingsOpen,
		cursor:    selectedSourceIndex(m.source),
		selected:  m.source,
		codexPath: m.codexPath,
	}
	return m
}

// buildBlocks converts usage entries to session blocks using the core package.
func buildBlocks(entries []data.UsageEntry) []data.SessionBlock {
	return core.BuildSessionBlocks(entries)
}
