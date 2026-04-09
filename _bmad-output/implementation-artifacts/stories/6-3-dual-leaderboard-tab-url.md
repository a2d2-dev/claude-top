# Story 6-3: 排行榜双 Tab + 独立 URL

**Status:** done

## Story

As a 开发者,
I want 排行榜页面有 Claude Code 和 Codex CLI 两个独立 tab，且每个 tab 有独立 URL,
So that 我可以直接分享对应榜单链接，朋友打开即看到正确的榜单.

## Acceptance Criteria

- [x] 访问 `/leaderboard` 或 `/leaderboard?source=claude` 默认显示 Claude Code 榜，tab 高亮 Claude
- [x] 访问 `/leaderboard?source=codex` 直接显示 Codex CLI 榜，tab 高亮 Codex，使用 OpenAI 绿色系（#10A37F）
- [x] 用户点击 tab 时 URL 随之更新（?source=claude / ?source=codex），刷新保持状态
- [x] 访问 `/leaderboard?source=all` 显示"总榜（即将上线）"占位提示

## Implementation

### Files Changed

- `backend/src/pages/LeaderboardPage.tsx`
  - Added `source` prop to `LeaderboardPageProps` interface
  - Added `.source-tabs`, `.source-tab`, `.source-panel` CSS classes with Codex green theme
  - Added `showSource(src)` JS function that switches panels and updates `?source=` URL param
  - Replaced single leaderboard table with three `source-panel` divs: `sp-claude`, `sp-codex`, `sp-all`
  - Codex panel uses `#10A37F` accent color; all-source panel shows coming-soon placeholder
  - Script initialization reads `?source=` URL param and activates correct panel on load

- `backend/src/routes/web.tsx`
  - Added `normalizePageSource()` helper to validate source query param
  - `GET /` and `GET /leaderboard` routes now read `?source=` param and pass to `buildLeaderboard` and `LeaderboardPage`
  - For `source=all`, skips DB query and passes empty rows (placeholder shown)

## Tasks Completed

- [x] `backend/src/pages/LeaderboardPage.tsx`: 新增来源 tab 切换组件，URL 参数驱动
- [x] Codex tab 样式：绿色主题（#10A37F accent）
- [x] 总榜 tab 预留占位 UI（"即将上线"）
- [x] 更新 leaderboard API 调用，传入 source 参数
