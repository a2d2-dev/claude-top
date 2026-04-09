# Story 5.2: Sessions Tab Multi-Source Display

Status: in-progress

## Story

As a multi-source data user,
I want Sessions list each row to show `[C]`/`[X]` source prefix,
So that I can distinguish Claude Code and Codex CLI sessions at a glance.

## Acceptance Criteria

1. **AC1 – Mixed mode prefix:** Given `--source all` (default), when viewing Sessions tab, then each row shows `[C]` (Claude) or `[X]` (Codex) prefix; rows mixed chronologically.

2. **AC2 – Claude-only backward compat:** Given `--source claude`, when viewing Sessions tab, then behavior is identical to old version — no `[C]` prefix shown.

3. **AC3 – Codex-only mode:** Given `--source codex`, when viewing Sessions tab, then only Codex sessions shown with `[X]` prefix.

4. **AC4 – Source flag wired to data loading:** Given `--source` flag passed, when app starts, then data is loaded via `LoadAllEntries` with the given source filter.

## Tasks

- [x] **Task 1: Add source/codexPath to Model** (AC: 4)
  - Add `source string` and `codexPath string` to `Model` struct in `model.go`
  - Update `NewModel` to accept source and codexPath params
  - Update `loadData` and `loadCached` to use `LoadAllEntries`

- [x] **Task 2: Update main.go** (AC: 4)
  - Add `--source` (default "all") and `--codex-path` (default "") flags
  - Pass to `NewModel`

- [x] **Task 3: Update Sessions tab header** (AC: 1, 2, 3)
  - In `tab_sessions.go`: add 4-char prefix column when source != "claude"
  - Update `histColWidths` call to account for prefix

- [x] **Task 4: Update historyDataRow** (AC: 1, 2, 3)
  - In `render.go`: emit `[C]` or `[X]` prefix based on `SessionBlock.Source` when showSource=true

## Dev Notes

- `historyDataRow` signature: add `showSource bool` parameter
- When `showSource=false` (claude-only mode): prefix is `"  "` (2 spaces), same as before
- When `showSource=true`: prefix is `[C] ` or `[X] ` (4 chars total including space)
- `histColWidths(innerW)` receives `innerW` = `m.width - 4 - 2` (existing prefix accounted separately)
- The 2-char cursor prefix `"▶ "` / `"  "` stays; source prefix is added before col data
