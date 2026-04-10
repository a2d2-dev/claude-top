package ui

import (
	"context"
	"fmt"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/a2d2-dev/claude-usage-monitor/internal/auth"
	"github.com/a2d2-dev/claude-usage-monitor/internal/core"
	"github.com/a2d2-dev/claude-usage-monitor/internal/data"
	"github.com/a2d2-dev/claude-usage-monitor/internal/upload"
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
	viewList      viewMode = iota
	viewDetail    // detail for the row under cursor
	viewMsgDetail // full detail for the selected message
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


// ── Auth state ────────────────────────────────────────────────────────────────

// authPhase tracks the stage of the GitHub Device Flow.
type authPhase int

const (
	authIdle        authPhase = iota // not in auth flow
	authRequesting                   // requesting device code from GitHub
	authShowingCode                  // displaying code to user, polling in background
	authVerifying                    // exchanging GitHub token for backend JWT
	authSuccess                      // auth complete
	authError                        // auth failed
)

// authState holds all state for the GitHub Device Flow overlay.
type authState struct {
	phase           authPhase
	userCode        string // e.g. "WDJB-MJHT"
	verificationURI string // e.g. "https://github.com/login/device"
	deviceCode      string // opaque code used to poll for the token
	pollInterval    int    // seconds between polls
	login           string // populated on success
	errMsg          string // populated on error
}

// ── Upload state ──────────────────────────────────────────────────────────────

// uploadPhase tracks the stage of the upload flow.
type uploadPhase int

const (
	uploadIdle       uploadPhase = iota // not in upload flow
	uploadConfirm                       // showing confirmation dialog
	uploadInProgress                    // upload in progress
	uploadSuccess                       // upload complete
	uploadError                         // upload failed
)

// uploadState holds all state for the upload overlay.
type uploadState struct {
	phase       uploadPhase
	stats       *upload.MonthlyStats // populated in uploadConfirm (combined for "all")
	claudeStats *upload.MonthlyStats // non-nil when source=all, for per-source display
	codexStats  *upload.MonthlyStats // non-nil when source=all and codex data exists
	rank        int                  // populated on uploadSuccess
	total       int                  // total users, populated on uploadSuccess
	shareURL    string               // populated on uploadSuccess
	errMsg      string               // populated on uploadError
}

// ── Upload tea messages ────────────────────────────────────────────────────────

// uploadResultMsg is sent when the upload API call completes.
type uploadResultMsg struct {
	Rank     int
	Total    int
	ShareURL string
	Err      error
}

// ── Auth tea messages ──────────────────────────────────────────────────────────

// authCodeMsg is sent when the device code has been received from GitHub.
type authCodeMsg struct {
	UserCode        string
	VerificationURI string
	DeviceCode      string
	Interval        int
	Err             error
}

// authPollMsg is sent after each poll of the GitHub token endpoint.
type authPollMsg struct {
	AccessToken string // non-empty = user has authorised
	Pending     bool   // true = "authorization_pending" or "slow_down"
	SlowDown    bool   // true = must increase interval
	Err         error  // non-nil = unrecoverable error
}

// authJWTMsg is sent when the backend /auth/verify call completes.
type authJWTMsg struct {
	Login string
	Err   error
}

// ── Message types ─────────────────────────────────────────────────────────────

// tickMsg is sent on each refresh tick.
type tickMsg time.Time

// loadedMsg carries session data from either a quick cache read or a full refresh.
type loadedMsg struct {
	blocks    []data.SessionBlock
	err       error
	fromCache bool // true = preliminary data from gob cache; full refresh still pending
}

// msgDetailLoadedMsg is sent when on-demand message detail loading completes.
type msgDetailLoadedMsg struct {
	detail *data.MessageDetail
}

// ── Per-tab state ─────────────────────────────────────────────────────────────

// sessionsState holds all UI state for the Sessions tab.
type sessionsState struct {
	cursor           int
	sortColumn       sortCol
	sortAsc          bool
	view             viewMode
	detailMsgCursor  int           // selected message index in detail view
	detailSort       detailSortCol // sort column for message table in detail view
	detailSortAsc    bool          // sort direction for message table
	copyFeedback     string        // set briefly after clipboard copy ("Copied!" or "Copy failed")
	msgDetail        *data.MessageDetail // loaded on-demand when entering viewMsgDetail
	msgDetailLoading bool          // true while async detail load is in flight
}

// ── Model ─────────────────────────────────────────────────────────────────────

// Model is the bubbletea application model.
type Model struct {
	blocks    []data.SessionBlock
	daily     []data.DailyStats
	plan      core.Plan
	dataPath  string
	codexPath string // path to ~/.codex/sessions; empty = use default
	source    string // "all", "claude", or "codex"
	width     int
	height    int
	loading   bool
	err       error

	// refreshing is true while a full disk refresh runs in the background.
	refreshing  bool
	lastRefresh time.Time

	// Tab navigation.
	tab tabID

	// Per-tab state.
	sessions sessionsState
	dailyCur int // cursor row in Daily tab

	// Auth overlay — active when auth.phase != authIdle.
	authOverlay authState

	// Upload overlay — active when upload.phase != uploadIdle.
	uploadOverlay uploadState

	// Settings overlay — active when settings.phase != settingsIdle.
	settings settingsState
}

// NewModel creates a Model with the given plan, data path, source filter, and codex path.
// source: "all" | "claude" | "codex" — controls which data sources are loaded.
// codexPath: path to Codex sessions dir; empty uses the default ~/.codex/sessions.
func NewModel(planName, dataPath, source, codexPath string) Model {
	if source == "" {
		source = "all"
	}
	return Model{
		plan:      core.GetPlan(planName),
		dataPath:  dataPath,
		codexPath: codexPath,
		source:    source,
		loading:   true,
		width:     120,
		height:    40,
		tab:       tabOverview,
		sessions:  sessionsState{sortColumn: sortByStart, sortAsc: false},
	}
}

// ── bubbletea lifecycle ───────────────────────────────────────────────────────

// Init kicks off two concurrent loads:
//   - loadCached: reads only the on-disk gob, returns in ~80 ms.
//   - loadData: full stat+parse cycle, delivers up-to-date data once done.
func (m Model) Init() tea.Cmd {
	return tea.Batch(loadCached(m.source), loadData(m.dataPath, m.codexPath, m.source), tick())
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
		return m, tea.Batch(loadData(m.dataPath, m.codexPath, m.source), tick())

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

	case authCodeMsg:
		return m.handleAuthCodeMsg(msg)

	case authPollMsg:
		return m.handleAuthPollMsg(msg)

	case authJWTMsg:
		return m.handleAuthJWTMsg(msg)

	case uploadResultMsg:
		return m.handleUploadResult(msg)

	case msgDetailLoadedMsg:
		m.sessions.msgDetail = msg.detail
		m.sessions.msgDetailLoading = false
		m.sessions.view = viewMsgDetail
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg.String())
	}
	return m, nil
}

// handleKey routes key presses to the appropriate handler.
func (m Model) handleKey(key string) (tea.Model, tea.Cmd) {
	// Auth overlay captures all keys while active.
	if m.authOverlay.phase != authIdle {
		return m.handleAuthKey(key)
	}

	// Upload overlay captures all keys while active.
	if m.uploadOverlay.phase != uploadIdle {
		return m.handleUploadKey2(key)
	}

	// Settings overlay captures all keys while active.
	if m.settings.phase != settingsIdle {
		return m.handleSettingsKeyWrapper(key)
	}

	// Global keys.
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "r":
		m.refreshing = true
		return m, loadData(m.dataPath, m.codexPath, m.source)
	case ",", "o":
		// Open settings modal.
		m = m.openSettings()
		return m, nil
	case "u":
		// Upload / auth: check existing auth before starting Device Flow.
		return m.handleUploadKey()
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

// handleSettingsKeyWrapper bridges settings key handling to bubbletea.
// It triggers a data reload when settings are saved.
func (m Model) handleSettingsKeyWrapper(key string) (tea.Model, tea.Cmd) {
	newModel, reloadMsg := m.handleSettingsKey(key)
	if reloadMsg.reload {
		newModel.loading = true
		return newModel, loadData(newModel.dataPath, newModel.codexPath, newModel.source)
	}
	return newModel, nil
}

// handleSessionsKey processes keys when the Sessions tab is active.
func (m Model) handleSessionsKey(key string) (tea.Model, tea.Cmd) {
	sel := m.selectedSession()

	// Message detail view: full token breakdown for one message.
	if m.sessions.view == viewMsgDetail {
		var msgs []data.UsageEntry
		if sel != nil {
			msgs = sortedEntries(sel.Entries, m.sessions.detailSort, m.sessions.detailSortAsc)
		}
		switch key {
		case "esc", "backspace":
			m.sessions.view = viewDetail
			m.sessions.copyFeedback = ""
		case "y", "c":
			if sel != nil && len(msgs) > 0 && m.sessions.detailMsgCursor < len(msgs) {
				text := formatMsgForCopy(msgs[m.sessions.detailMsgCursor], sel.CostUSD)
				if err := copyToClipboard(text); err == nil {
					m.sessions.copyFeedback = "Copied!"
				} else {
					m.sessions.copyFeedback = "Copy failed: " + err.Error()
				}
			}
		default:
			m.sessions.copyFeedback = ""
		}
		return m, nil
	}

	// Detail view: navigate messages table, sort, left/right by time, or go back.
	if m.sessions.view == viewDetail {
		var msgs []data.UsageEntry
		if sel != nil {
			msgs = sortedEntries(sel.Entries, m.sessions.detailSort, m.sessions.detailSortAsc)
		}
		msgCount := len(msgs)
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
		case "left":
			// Navigate to the previous message chronologically.
			m.sessions.detailMsgCursor = chronologicalAdjacent(msgs, m.sessions.detailMsgCursor, false)
		case "right":
			// Navigate to the next message chronologically.
			m.sessions.detailMsgCursor = chronologicalAdjacent(msgs, m.sessions.detailMsgCursor, true)
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
		case "enter":
			if msgCount > 0 {
				entry := msgs[m.sessions.detailMsgCursor]
				m.sessions.msgDetail = nil
				m.sessions.msgDetailLoading = true
				m.sessions.copyFeedback = ""
				return m, loadMsgDetail(m.dataPath, entry)
			}
		}
		return m, nil
	}

	// List view: navigate sessions.
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

// chronologicalAdjacent returns the index in msgs of the message immediately
// before (forward=false) or after (forward=true) msgs[cursor] by timestamp.
// Returns cursor unchanged if there is no adjacent message in that direction.
func chronologicalAdjacent(msgs []data.UsageEntry, cursor int, forward bool) int {
	if len(msgs) == 0 || cursor < 0 || cursor >= len(msgs) {
		return cursor
	}
	curTS := msgs[cursor].Timestamp
	var targetTS time.Time
	found := false

	for _, e := range msgs {
		if forward {
			if e.Timestamp.After(curTS) && (!found || e.Timestamp.Before(targetTS)) {
				targetTS = e.Timestamp
				found = true
			}
		} else {
			if e.Timestamp.Before(curTS) && (!found || e.Timestamp.After(targetTS)) {
				targetTS = e.Timestamp
				found = true
			}
		}
	}
	if !found {
		return cursor
	}
	for i, e := range msgs {
		if e.Timestamp.Equal(targetTS) {
			return i
		}
	}
	return cursor
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
// source controls which cache(s) are read: "all", "claude", or "codex".
func loadCached(source string) tea.Cmd {
	return func() tea.Msg {
		var entries []data.UsageEntry

		if source == "all" || source == "claude" {
			ce, err := data.LoadCached()
			if err == nil {
				entries = append(entries, ce...)
			}
		}
		if source == "all" || source == "codex" {
			xe, err := data.LoadCodexCached()
			if err == nil {
				entries = append(entries, xe...)
			}
		}

		if len(entries) == 0 {
			return loadedMsg{fromCache: true}
		}
		blocks := core.BuildSessionBlocks(entries)
		return loadedMsg{blocks: blocks, fromCache: true}
	}
}

// loadData reads all JSONL files (full refresh with cache validation).
// dataPath is the Claude projects dir; codexPath is the Codex sessions dir.
// loadMsgDetail returns a command that reads the full message content from disk.
// It finds the JSONL source file via sessionID and extracts the assistant turn.
func loadMsgDetail(dataPath string, entry data.UsageEntry) tea.Cmd {
	return func() tea.Msg {
		if entry.Source == "codex" {
			return msgDetailLoadedMsg{detail: &data.MessageDetail{
				LoadErr: fmt.Errorf("full message content is not yet available for Codex sessions"),
			}}
		}
		sourceFile := data.FindClaudeSessionFile(dataPath, entry.SessionID)
		if sourceFile == "" {
			return msgDetailLoadedMsg{detail: &data.MessageDetail{
				LoadErr: fmt.Errorf("session file not found (id: %s)", entry.SessionID),
			}}
		}
		detail, err := data.ReadMessageDetail(sourceFile, entry.MessageID)
		if err != nil {
			return msgDetailLoadedMsg{detail: &data.MessageDetail{LoadErr: err}}
		}
		return msgDetailLoadedMsg{detail: detail}
	}
}

// source: "all" | "claude" | "codex".
func loadData(dataPath, codexPath, source string) tea.Cmd {
	return func() tea.Msg {
		entries, err := data.LoadAllEntries(dataPath, codexPath, source)
		if err != nil {
			return loadedMsg{err: err}
		}
		blocks := core.BuildSessionBlocks(entries)
		return loadedMsg{blocks: blocks}
	}
}

// ── Auth handlers ─────────────────────────────────────────────────────────────


// handleAuthKey handles keys while the auth overlay is showing.
func (m Model) handleAuthKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "q":
		// Cancel auth and return to normal UI.
		m.authOverlay = authState{phase: authIdle}
		return m, nil
	case "enter":
		// Dismiss success / error screens.
		if m.authOverlay.phase == authSuccess || m.authOverlay.phase == authError {
			m.authOverlay = authState{phase: authIdle}
		}
		return m, nil
	}
	return m, nil
}

// handleAuthCodeMsg processes the result of requesting a device code.
func (m Model) handleAuthCodeMsg(msg authCodeMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.authOverlay = authState{phase: authError, errMsg: msg.Err.Error()}
		return m, nil
	}
	m.authOverlay = authState{
		phase:           authShowingCode,
		userCode:        msg.UserCode,
		verificationURI: msg.VerificationURI,
		deviceCode:      msg.DeviceCode,
		pollInterval:    msg.Interval,
	}
	return m, pollTokenCmd(msg.DeviceCode, msg.Interval)
}

// handleAuthPollMsg processes the result of one GitHub token poll.
func (m Model) handleAuthPollMsg(msg authPollMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.authOverlay = authState{phase: authError, errMsg: msg.Err.Error()}
		return m, nil
	}
	if msg.Pending {
		// Still waiting — keep polling. Increase interval on slow_down.
		interval := m.authOverlay.pollInterval
		if msg.SlowDown && interval < 30 {
			interval += 5
		}
		m.authOverlay.pollInterval = interval
		return m, pollTokenCmd(m.authOverlay.deviceCode, interval)
	}
	// Got the access token — verify with backend.
	m.authOverlay.phase = authVerifying
	return m, verifyWithBackendCmd(msg.AccessToken)
}

// handleAuthJWTMsg processes the result of the backend /auth/verify call.
// On success, immediately transitions to the upload confirmation dialog.
func (m Model) handleAuthJWTMsg(msg authJWTMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.authOverlay = authState{phase: authError, errMsg: msg.Err.Error()}
		return m, nil
	}
	// Auth succeeded — clear overlay and open upload confirm.
	m.authOverlay = authState{phase: authIdle}
	info, _ := auth.LoadAuth()
	return m.startUploadConfirm(info)
}

// ── Auth commands ─────────────────────────────────────────────────────────────

// requestDeviceCodeCmd asks GitHub for a device code (runs in background).
func requestDeviceCodeCmd() tea.Cmd {
	return func() tea.Msg {
		resp, err := auth.RequestDeviceCode(context.Background())
		if err != nil {
			return authCodeMsg{Err: err}
		}
		return authCodeMsg{
			UserCode:        resp.UserCode,
			VerificationURI: resp.VerificationURI,
			DeviceCode:      resp.DeviceCode,
			Interval:        resp.Interval,
		}
	}
}

// pollTokenCmd waits interval seconds then polls GitHub for the access token.
func pollTokenCmd(deviceCode string, interval int) tea.Cmd {
	return func() tea.Msg {
		resp, err := auth.PollToken(context.Background(), deviceCode, interval)
		if err != nil {
			return authPollMsg{Err: err}
		}
		switch resp.Error {
		case "":
			// Success.
			return authPollMsg{AccessToken: resp.AccessToken}
		case "authorization_pending":
			return authPollMsg{Pending: true}
		case "slow_down":
			return authPollMsg{Pending: true, SlowDown: true}
		default:
			return authPollMsg{Err: fmt.Errorf("github: %s — %s", resp.Error, resp.ErrorDescription)}
		}
	}
}

// verifyWithBackendCmd exchanges a GitHub access_token for a backend JWT.
func verifyWithBackendCmd(accessToken string) tea.Cmd {
	return func() tea.Msg {
		device, err := auth.EnsureDevice()
		if err != nil {
			return authJWTMsg{Err: fmt.Errorf("device setup: %w", err)}
		}
		resp, err := auth.VerifyWithBackend(context.Background(), device.DeviceID, accessToken)
		if err != nil {
			return authJWTMsg{Err: err}
		}
		// Persist auth info locally.
		info := &auth.AuthInfo{
			JWT:         resp.JWT,
			GitHubID:    resp.GitHubID,
			GitHubLogin: resp.GitHubLogin,
			AvatarURL:   resp.AvatarURL,
			ExpiresAt:   resp.ExpiresAt,
		}
		if saveErr := auth.SaveAuth(info); saveErr != nil {
			return authJWTMsg{Err: fmt.Errorf("save auth: %w", saveErr)}
		}
		return authJWTMsg{Login: resp.GitHubLogin}
	}
}

// ── Upload handlers ────────────────────────────────────────────────────────────

// handleUploadKey processes `u` when no overlay is active.
// Redirects to Device Flow if not authenticated, otherwise shows confirm dialog.
func (m Model) handleUploadKey() (tea.Model, tea.Cmd) {
	info, _ := auth.LoadAuth()
	if !auth.IsAuthValid(info) {
		// Start Device Flow.
		m.authOverlay = authState{phase: authRequesting}
		return m, requestDeviceCodeCmd()
	}
	// Already authenticated: show upload confirmation.
	return m.startUploadConfirm(info)
}

// startUploadConfirm aggregates the current month's stats and shows the
// upload confirmation dialog. Called after successful auth or on 'u' if
// already authenticated.
func (m Model) startUploadConfirm(info *auth.AuthInfo) (tea.Model, tea.Cmd) {
	// Aggregate combined stats for the confirm dialog total.
	stats, err := upload.AggregateCurrentMonth(m.blocks, m.source)
	if err != nil {
		m.uploadOverlay = uploadState{
			phase:  uploadError,
			errMsg: "数据聚合失败: " + err.Error(),
		}
		return m, nil
	}

	overlay := uploadState{
		phase: uploadConfirm,
		stats: stats,
	}

	// When source=all, also compute per-source breakdown for the dialog.
	if m.source == "all" {
		claudeStats, _ := upload.AggregateCurrentMonth(m.blocks, "claude")
		codexStats, _ := upload.AggregateCurrentMonth(m.blocks, "codex")
		overlay.claudeStats = claudeStats
		overlay.codexStats = codexStats
	}

	m.uploadOverlay = overlay
	return m, nil
}

// handleUploadKey2 handles keys while the upload overlay is showing.
func (m Model) handleUploadKey2(key string) (tea.Model, tea.Cmd) {
	switch m.uploadOverlay.phase {
	case uploadConfirm:
		switch key {
		case "enter", "y":
			// Start upload.
			info, _ := auth.LoadAuth()
			if !auth.IsAuthValid(info) {
				m.uploadOverlay = uploadState{
					phase:  uploadError,
					errMsg: "JWT 已过期，请重新认证（u 键）",
				}
				return m, nil
			}
			m.uploadOverlay.phase = uploadInProgress
			return m, doUploadCmd(info.JWT, m.blocks, m.source)
		case "esc", "n", "q":
			m.uploadOverlay = uploadState{phase: uploadIdle}
			return m, nil
		}
	case uploadSuccess, uploadError:
		switch key {
		case "esc", "enter", "q":
			m.uploadOverlay = uploadState{phase: uploadIdle}
			return m, nil
		}
	}
	return m, nil
}

// handleUploadResult processes the uploadResultMsg from the HTTP call.
func (m Model) handleUploadResult(msg uploadResultMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.uploadOverlay = uploadState{
			phase:  uploadError,
			errMsg: msg.Err.Error(),
		}
		return m, nil
	}
	m.uploadOverlay = uploadState{
		phase:    uploadSuccess,
		rank:     msg.Rank,
		total:    msg.Total,
		shareURL: msg.ShareURL,
	}
	return m, nil
}

// ── Upload command ─────────────────────────────────────────────────────────────

// doUploadCmd runs the upload HTTP call in the background.
// When source="all", it uploads claude and codex data separately,
// returning the combined rank from the claude upload (primary source).
func doUploadCmd(jwt string, blocks []data.SessionBlock, modelSource string) tea.Cmd {
	return func() tea.Msg {
		device, err := auth.EnsureDevice()
		if err != nil {
			return uploadResultMsg{Err: fmt.Errorf("读取设备信息失败: %w", err)}
		}

		ctx := context.Background()

		// Determine which sources to upload.
		sources := []string{modelSource}
		if modelSource == "all" {
			sources = []string{"claude", "codex"}
		}

		var lastResp *upload.UploadResponse
		for _, src := range sources {
			stats, aggErr := upload.AggregateCurrentMonth(blocks, src)
			if aggErr != nil {
				return uploadResultMsg{Err: fmt.Errorf("数据聚合失败 (%s): %w", src, aggErr)}
			}
			// Skip upload if there's no data for this source.
			if stats.SessionCount == 0 && stats.TotalCostUSD == 0 {
				continue
			}
			resp, uploadErr := upload.Upload(ctx, jwt, device, stats, src)
			if uploadErr != nil {
				return uploadResultMsg{Err: uploadErr}
			}
			lastResp = resp
		}

		if lastResp == nil {
			return uploadResultMsg{Err: fmt.Errorf("当前月没有可上传的数据")}
		}
		return uploadResultMsg{
			Rank:     lastResp.Rank,
			Total:    lastResp.TotalUsers,
			ShareURL: lastResp.ShareURL,
		}
	}
}
