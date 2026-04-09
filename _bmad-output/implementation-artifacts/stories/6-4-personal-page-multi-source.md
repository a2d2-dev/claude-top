# Story 6-4: 个人统计页多源展示

**Status:** done

## Story

As a 用户,
I want 我的个人统计页 `/u/:login` 展示所有 AI 工具的数据,
So that 访客看到我完整的 AI 使用概况.

## Acceptance Criteria

- [x] 用户同时上传了 Claude 和 Codex 数据时，页面分两个 section 展示，顶部显示合计总费用
- [x] 仅上传了 Claude 数据时，仅显示 Claude section，无 Codex section
- [x] 仅上传了 Codex 数据时，仅显示 Codex section，无 Claude section

## Implementation

### Files Changed

- `backend/src/pages/UserPage.tsx`
  - Added `MultiSourceUserData` interface with `claude: UserStats | null`, `codex: UserStats | null`, `total_cost_usd`
  - Updated `UserPageProps` to use `MultiSourceUserData` instead of single `UserStats`
  - Added `SourceSection` sub-component for per-source stats grid
  - Added `total-banner` for combined cost (only shown when both sources have data)
  - Added CSS: `.source-section`, `.source-badge`, `.badge-claude`, `.badge-codex`, `.total-banner`, `.v-codex`
  - Claude section uses cyan/primary accent; Codex section uses #10A37F green

- `backend/src/routes/web.tsx`
  - `GET /u/:login` now fetches both `queryUserStats(..., 'claude')` and `queryUserStats(..., 'codex')` in parallel
  - Builds `MultiSourceUserData` object with both results; returns 404 only if both are null
  - `GET /og/:login` also fetches both sources and uses combined cost + best rank for OG image

## Tasks Completed

- [x] `backend/src/pages/UserPage.tsx`: 按 source 分 section 渲染
- [x] `backend/src/routes/web.tsx`: `/u/:login` 返回并渲染多 source 数据
