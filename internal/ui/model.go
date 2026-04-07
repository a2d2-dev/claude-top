package ui

import (
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/a2d2-dev/claude-usage-monitor/internal/core"
	"github.com/a2d2-dev/claude-usage-monitor/internal/data"
)

// refreshInterval is how often the monitor reloads data from disk.
const refreshInterval = 10 * time.Second

// ── Tab IDs ───────────────────────────────────────────────────────────────────

// tabID identifies the active top-level tab.
type tabID int

const (
	tabOverview tabID = iota
	tabSessions
	tabDaily
	tabCount
)

var tabNames = []string{"Overview", "Sessions", "Daily"}

// ── View / sort enums (Sessions tab) ─────────────────────────────────────────

// viewMode controls whether we show the session list or a session detail.
type viewMode int

const (
	viewList   viewMode = iota
	viewDetail // detail for the row under cursor
)

// sortCol selects which column to sort the sessions table by.
type sortCol int

const (
	sortByStart   sortCol = iota
	sortByUpdated         // ActualEndTime
	sortByMsgs
	sortByTokens
	sortByCost
	sortByDir
	sortColCount
)

var sortColNames = []string{"Start", "Updated", "Msgs", "Tokens", "Cost", "Dir"}

// detailSortCol selects which column to sort the messages table in detail view.
type detailSortCol int

const (
	detailSortCost    detailSortCol = iota // default: highest cost first
	detailSortTokens
	detailSortTime
	detailSortModel
	detailSortColCount
)


// ── Message types ─────────────────────────────────────────────────────────────

// tickMsg is sent on each refresh tick.
type tickMsg time.Time

// loadedMsg carries session data from either a quick cache read or a full refresh.
type loadedMsg struct {
	blocks    []data.SessionBlock
	err       error
	fromCache bool // true = preliminary data from gob cache; full refresh still pending
}

// ── Per-tab state ─────────────────────────────────────────────────────────────

// sessionsState holds all UI state for the Sessions tab.
type sessionsState struct {
	cursor          int
	sortColumn      sortCol
	sortAsc         bool
	view            viewMode
	detailMsgCursor int           // selected message index in detail view
	detailSort      detailSortCol // sort column for message table in detail view
	detailSortAsc   bool          // sort direction for message table
}

// ── Model ─────────────────────────────────────────────────────────────────────

// Model is the bubbletea application model.
type Model struct {
	blocks  []data.SessionBlock
	daily   []data.DailyStats
	plan    core.Plan
	dataPath string
	width   int
	height  int
	loading bool
	err     error

	// refreshing is true while a full disk refresh runs in the background.
	refreshing  bool
	lastRefresh time.Time

	// Tab navigation.
	tab tabID

	// Per-tab state.
	sessions sessionsState
	dailyCur int // cursor row in Daily tab
}

// NewModel creates a Model with the given plan and data path.
func NewModel(planName, dataPath string) Model {
	return Model{
		plan:     core.GetPlan(planName),
		dataPath: dataPath,
		loading:  true,
		width:    120,
		height:   40,
		tab:      tabOverview,
		sessions: sessionsState{sortColumn: sortByStart, sortAsc: false},
	}
}

// ── bubbletea lifecycle ───────────────────────────────────────────────────────

// Init kicks off two concurrent loads:
//   - loadCached: reads only the on-disk gob, returns in ~80 ms.
//   - loadData: full stat+parse cycle, delivers up-to-date data once done.
func (m Model) Init() tea.Cmd {
	return tea.Batch(loadCached(), loadData(m.dataPath), tick())
}

// Update handles incoming messages and user input.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		m.refreshing = true
		return m, tea.Batch(loadData(m.dataPath), tick())

	case loadedMsg:
		if msg.fromCache {
			if msg.err == nil && len(msg.blocks) > 0 {
				m.blocks = msg.blocks
				m.daily = core.BuildDailyStats(m.blocks)
				m.loading = false
			}
			return m, nil
		}
		// Full refresh completed.
		m.refreshing = false
		m.loading = false
		m.lastRefresh = time.Now()
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.blocks = msg.blocks
			m.daily = core.BuildDailyStats(m.blocks)
			m.err = nil
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg.String())
	}
	return m, nil
}

// handleKey routes key presses to the appropriate handler.
func (m Model) handleKey(key string) (tea.Model, tea.Cmd) {
	// Global keys.
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "r":
		m.refreshing = true
		return m, loadData(m.dataPath)
	// Tab switching.
	case "1":
		m.tab = tabOverview
		return m, nil
	case "2":
		m.tab = tabSessions
		return m, nil
	case "3":
		m.tab = tabDaily
		return m, nil
	case "tab":
		m.tab = (m.tab + 1) % tabCount
		return m, nil
	case "shift+tab":
		m.tab = (m.tab + tabCount - 1) % tabCount
		return m, nil
	}

	// Tab-specific keys.
	switch m.tab {
	case tabSessions:
		return m.handleSessionsKey(key)
	case tabDaily:
		return m.handleDailyKey(key)
	}
	return m, nil
}

// handleSessionsKey processes keys when the Sessions tab is active.
func (m Model) handleSessionsKey(key string) (tea.Model, tea.Cmd) {
	// Detail view: navigate messages, sort, or go back.
	if m.sessions.view == viewDetail {
		sel := m.selectedSession()
		msgCount := 0
		if sel != nil {
			msgCount = len(sel.Entries)
		}
		switch key {
		case "esc", "backspace":
			m.sessions.view = viewList
			m.sessions.detailMsgCursor = 0
		case "up", "k":
			if m.sessions.detailMsgCursor > 0 {
				m.sessions.detailMsgCursor--
			}
		case "down", "j":
			if m.sessions.detailMsgCursor < msgCount-1 {
				m.sessions.detailMsgCursor++
			}
		case "g", "home":
			m.sessions.detailMsgCursor = 0
		case "G", "end":
			if msgCount > 0 {
				m.sessions.detailMsgCursor = msgCount - 1
			}
		case "s":
			m.sessions.detailSort = (m.sessions.detailSort + 1) % detailSortColCount
			m.sessions.detailMsgCursor = 0
		case "S":
			m.sessions.detailSort = (m.sessions.detailSort + detailSortColCount - 1) % detailSortColCount
			m.sessions.detailMsgCursor = 0
		case "/":
			m.sessions.detailSortAsc = !m.sessions.detailSortAsc
			m.sessions.detailMsgCursor = 0
		}
		return m, nil
	}

	rows := m.sessionRows()
	visible := m.sessionsVisibleRows()

	switch key {
	case "up", "k":
		if m.sessions.cursor > 0 {
			m.sessions.cursor--
		}
	case "down", "j":
		if m.sessions.cursor < len(rows)-1 {
			m.sessions.cursor++
		}
	case "pgup":
		m.sessions.cursor -= visible
		if m.sessions.cursor < 0 {
			m.sessions.cursor = 0
		}
	case "pgdown":
		m.sessions.cursor += visible
		if m.sessions.cursor >= len(rows) {
			m.sessions.cursor = len(rows) - 1
		}
	case "g", "home":
		m.sessions.cursor = 0
	case "G", "end":
		if len(rows) > 0 {
			m.sessions.cursor = len(rows) - 1
		}
	case "s":
		m.sessions.sortColumn = (m.sessions.sortColumn + 1) % sortColCount
		m.sessions.cursor = 0
	case "S":
		m.sessions.sortColumn = (m.sessions.sortColumn + sortColCount - 1) % sortColCount
		m.sessions.cursor = 0
	case "/":
		m.sessions.sortAsc = !m.sessions.sortAsc
		m.sessions.cursor = 0
	case "enter":
		if len(rows) > 0 && m.sessions.cursor < len(rows) {
			m.sessions.view = viewDetail
			m.sessions.detailMsgCursor = 0
		}
	}
	return m, nil
}

// handleDailyKey processes keys when the Daily tab is active.
func (m Model) handleDailyKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.dailyCur > 0 {
			m.dailyCur--
		}
	case "down", "j":
		if m.dailyCur < len(m.daily)-1 {
			m.dailyCur++
		}
	case "g", "home":
		m.dailyCur = 0
	case "G", "end":
		if len(m.daily) > 0 {
			m.dailyCur = len(m.daily) - 1
		}
	}
	return m, nil
}

// ── Derived data ──────────────────────────────────────────────────────────────

// sessionRows returns all historical (non-active, non-gap) blocks sorted per current settings.
func (m Model) sessionRows() []data.SessionBlock {
	var rows []data.SessionBlock
	for i := range m.blocks {
		if !m.blocks[i].IsGap && !m.blocks[i].IsActive {
			rows = append(rows, m.blocks[i])
		}
	}
	sortSessionRows(rows, m.sessions.sortColumn, m.sessions.sortAsc)
	return rows
}

// sortSessionRows sorts session blocks in-place.
func sortSessionRows(rows []data.SessionBlock, col sortCol, asc bool) {
	sort.SliceStable(rows, func(i, j int) bool {
		var less bool
		switch col {
		case sortByStart:
			less = rows[i].StartTime.Before(rows[j].StartTime)
		case sortByUpdated:
			ti, tj := rows[i].StartTime, rows[j].StartTime
			if rows[i].ActualEndTime != nil {
				ti = *rows[i].ActualEndTime
			}
			if rows[j].ActualEndTime != nil {
				tj = *rows[j].ActualEndTime
			}
			less = ti.Before(tj)
		case sortByMsgs:
			less = rows[i].MessageCount < rows[j].MessageCount
		case sortByTokens:
			less = rows[i].TokenCounts.TotalTokens() < rows[j].TokenCounts.TotalTokens()
		case sortByCost:
			less = rows[i].CostUSD < rows[j].CostUSD
		case sortByDir:
			less = rows[i].Directory < rows[j].Directory
		}
		if asc {
			return less
		}
		return !less
	})
}

// selectedSession returns the session block under the Sessions cursor, or nil.
func (m Model) selectedSession() *data.SessionBlock {
	rows := m.sessionRows()
	if m.sessions.cursor < 0 || m.sessions.cursor >= len(rows) {
		return nil
	}
	s := rows[m.sessions.cursor]
	return &s
}

// sessionsScrollOffset computes the scroll offset to keep the cursor visible.
func (m Model) sessionsScrollOffset() int {
	visible := m.sessionsVisibleRows()
	if m.sessions.cursor < visible {
		return 0
	}
	return m.sessions.cursor - visible + 1
}

// sessionsVisibleRows returns the number of data rows that fit in the Sessions panel.
func (m Model) sessionsVisibleRows() int {
	// Tab header(1) + content border(2) + col header(1) + divider(1) + footer(1) = 6
	inner := m.height - 6
	if inner < 1 {
		return 1
	}
	return inner
}

// activeBlock returns the currently active session block, or nil.
func (m Model) activeBlock() *data.SessionBlock {
	for i := range m.blocks {
		if m.blocks[i].IsActive {
			return &m.blocks[i]
		}
	}
	return nil
}

// ── View ──────────────────────────────────────────────────────────────────────

// View renders the current model state to a string.
func (m Model) View() string {
	return RenderDashboard(m)
}

// ── Commands ──────────────────────────────────────────────────────────────────

// tick returns a command that fires a tickMsg after refreshInterval.
func tick() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// loadCached reads only the on-disk gob cache (fast preliminary load).
func loadCached() tea.Cmd {
	return func() tea.Msg {
		entries, err := data.LoadCached()
		if err != nil || len(entries) == 0 {
			return loadedMsg{fromCache: true}
		}
		blocks := core.BuildSessionBlocks(entries)
		return loadedMsg{blocks: blocks, fromCache: true}
	}
}

// loadData reads all JSONL files (full refresh with cache validation).
func loadData(dataPath string) tea.Cmd {
	return func() tea.Msg {
		entries, err := data.LoadEntries(dataPath)
		if err != nil {
			return loadedMsg{err: err}
		}
		blocks := core.BuildSessionBlocks(entries)
		return loadedMsg{blocks: blocks}
	}
}
