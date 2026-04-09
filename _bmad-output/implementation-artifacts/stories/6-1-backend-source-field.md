# Story 6.1: Backend Multi-Source Support (source field)

Status: in-progress

## Story

As a system,
I want upload and leaderboard APIs to support the `source` parameter,
So that Claude Code and Codex CLI data are completely isolated in storage and queries.

## Acceptance Criteria

1. POST /api/upload accepts `source: "claude" | "codex"` in body; defaults to "claude".
2. D1 `uploads` table has `source` column with index.
3. GET /api/leaderboard?source=claude|codex filters by source; default is "claude".
4. KV cache keys include source prefix: `leaderboard:claude:2026-04`.
5. GET /api/user/:login returns per-source stats.

## Implementation

- schema.sql: add `source TEXT NOT NULL DEFAULT 'claude'` to uploads table
- upload.ts: accept and store source field; pass to leaderboard refresh
- leaderboard.ts: source-aware KV key, buildLeaderboard/queryUserStats accept source
