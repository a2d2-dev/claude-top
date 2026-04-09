# Story 5.4: Settings Panel

Status: done

## Story

As a user,
I want to press `s` in the TUI to open a Settings modal that persists my data source preference,
So that each startup uses my preferred source without command-line arguments.

## Acceptance Criteria

1. Press `s` → Settings modal opens with source options and codex path field.
2. ↑↓ cycle source options; Enter saves; Esc cancels.
3. Settings saved to ~/.claude-top/config.json.
4. On next startup, config is loaded as fallback (CLI --source overrides).
5. Confirming settings triggers immediate data reload.

## Implementation

- `internal/config/config.go`: Config struct, Load(), Save()
- `internal/ui/settings.go`: settingsState, handleSettingsKey, renderSettingsOverlay, openSettings
- `internal/ui/model.go`: Added settings field, handleSettingsKeyWrapper, `s` global key → openSettings
- `internal/ui/render.go`: Settings overlay in render pipeline, footer hint
- `main.go`: Load config as fallback for --source/--codex-path flags
