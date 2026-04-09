/**
 * GET /api/leaderboard?period=YYYY-MM
 *   Returns the TOP 100 users for the given period, served from KV cache.
 *   Falls back to D1 aggregation if cache is missing.
 *
 * GET /api/user/:login
 *   Returns an individual user's stats for the current period.
 */

import { Hono } from 'hono';
import type { Bindings } from '../index';

export const leaderboardRoutes = new Hono<{ Bindings: Bindings }>();

/** One row in the leaderboard response. */
export interface LeaderboardEntry {
  rank: number;
  github_login: string;
  avatar_url: string;
  total_cost_usd: number;
  total_tokens: number;
  device_count: number;
}

/** KV cache TTL in seconds (5 minutes). */
const KV_TTL = 300;

/**
 * KV cache key for leaderboard data.
 * Includes source prefix to avoid collision between claude/codex caches.
 * @param period - YYYY-MM period string
 * @param source - "claude" or "codex"
 */
const kvKey = (period: string, source: string) => `leaderboard:${source}:${period}`;

/**
 * Validates a source parameter string.
 * Returns "claude" for missing/invalid values (backward compatibility).
 */
function normalizeSource(raw: string | undefined): 'claude' | 'codex' {
  if (raw === 'codex') return 'codex';
  return 'claude'; // default to claude for backward compat
}

leaderboardRoutes.get('/leaderboard', async (c) => {
  const period = c.req.query('period') ?? new Date().toISOString().slice(0, 7);
  if (!/^\d{4}-\d{2}$/.test(period)) {
    return c.json({ error: 'period must be YYYY-MM' }, 400);
  }

  const source = normalizeSource(c.req.query('source'));
  const key = kvKey(period, source);

  // Try KV cache first.
  const cached = await c.env.LEADERBOARD.get(key, 'json') as LeaderboardEntry[] | null;
  if (cached) {
    return c.json({ period, source, data: cached, cache: 'hit' });
  }

  // Cache miss: aggregate from D1.
  const data = await buildLeaderboard(c.env.DB, period, source);

  // Write to KV cache asynchronously.
  c.executionCtx.waitUntil(
    c.env.LEADERBOARD.put(key, JSON.stringify(data), { expirationTtl: KV_TTL }),
  );

  return c.json({ period, source, data, cache: 'miss' });
});

leaderboardRoutes.get('/user/:login', async (c) => {
  const login = c.req.param('login');
  const period = c.req.query('period') ?? new Date().toISOString().slice(0, 7);

  // Find github_id for this login.
  const device = await c.env.DB.prepare(
    `SELECT github_id, avatar_url FROM devices WHERE github_login = ? LIMIT 1`,
  )
    .bind(login)
    .first<{ github_id: number; avatar_url: string }>();

  if (!device) {
    return c.json({ error: 'user not found' }, 404);
  }

  // Fetch per-source stats for this user.
  const claudeStats = await queryUserStats(c.env.DB, login, period, 'claude');
  const codexStats = await queryUserStats(c.env.DB, login, period, 'codex');

  if (!claudeStats && !codexStats) {
    return c.json({ error: 'no data for this period' }, 404);
  }

  // Compute combined total cost for display.
  const totalCostUsd = (claudeStats?.total_cost_usd ?? 0) + (codexStats?.total_cost_usd ?? 0);

  return c.json({
    period,
    github_login: login,
    avatar_url: device.avatar_url,
    total_cost_usd: totalCostUsd,
    claude: claudeStats ?? null,
    codex: codexStats ?? null,
  });
});

/** Full stats for a single user for a given period. */
export interface UserStats {
  github_login: string;
  avatar_url: string;
  rank: number;
  period: string;
  total_cost_usd: number;
  total_tokens: number;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
  session_count: number;
  device_count: number;
}

/**
 * queryUserStats fetches aggregated stats for a single github_login from D1.
 * Returns null if the user has no data for the given period and source.
 *
 * @param db     - D1 database binding
 * @param login  - GitHub login name
 * @param period - YYYY-MM period string
 * @param source - "claude" or "codex" (default: "claude")
 */
export async function queryUserStats(
  db: D1Database,
  login: string,
  period: string,
  source: 'claude' | 'codex' = 'claude',
): Promise<UserStats | null> {
  const device = await db
    .prepare(`SELECT github_id, avatar_url FROM devices WHERE github_login = ? LIMIT 1`)
    .bind(login)
    .first<{ github_id: number; avatar_url: string }>();

  if (!device) return null;

  const agg = await db
    .prepare(
      `SELECT
         SUM(total_cost_usd)     AS total_cost_usd,
         SUM(total_tokens)       AS total_tokens,
         SUM(input_tokens)       AS input_tokens,
         SUM(output_tokens)      AS output_tokens,
         SUM(cache_read_tokens)  AS cache_read_tokens,
         SUM(cache_write_tokens) AS cache_write_tokens,
         SUM(session_count)      AS session_count,
         COUNT(*)                AS device_count
       FROM uploads
       WHERE github_id = ? AND period = ? AND source = ?`,
    )
    .bind(device.github_id, period, source)
    .first<{
      total_cost_usd: number | null;
      total_tokens: number;
      input_tokens: number;
      output_tokens: number;
      cache_read_tokens: number;
      cache_write_tokens: number;
      session_count: number;
      device_count: number;
    }>();

  if (!agg || agg.total_cost_usd === null) return null;

  const rankResult = await db
    .prepare(
      `WITH totals AS (
         SELECT github_id, SUM(total_cost_usd) AS cost
         FROM uploads WHERE period = ? AND source = ?
         GROUP BY github_id
       )
       SELECT COUNT(*) + 1 AS rank
       FROM totals
       WHERE cost > (SELECT cost FROM totals WHERE github_id = ?)`,
    )
    .bind(period, source, device.github_id)
    .first<{ rank: number }>();

  return {
    github_login: login,
    avatar_url: device.avatar_url,
    rank: rankResult?.rank ?? 1,
    period,
    total_cost_usd: agg.total_cost_usd,
    total_tokens: agg.total_tokens,
    input_tokens: agg.input_tokens,
    output_tokens: agg.output_tokens,
    cache_read_tokens: agg.cache_read_tokens,
    cache_write_tokens: agg.cache_write_tokens,
    session_count: agg.session_count,
    device_count: agg.device_count,
  };
}

/**
 * buildLeaderboard queries D1 to produce the TOP 100 leaderboard for a period and source.
 * Aggregates across all devices per github_id, filtered by source.
 *
 * @param db     - D1 database binding
 * @param period - YYYY-MM period string
 * @param source - "claude" or "codex" (default: "claude")
 */
export async function buildLeaderboard(
  db: D1Database,
  period: string,
  source: 'claude' | 'codex' = 'claude',
): Promise<LeaderboardEntry[]> {
  const rows = await db
    .prepare(
      `SELECT
         d.github_login,
         d.avatar_url,
         SUM(u.total_cost_usd) AS total_cost_usd,
         SUM(u.total_tokens)   AS total_tokens,
         COUNT(u.device_id)    AS device_count
       FROM uploads u
       JOIN (
         SELECT github_id, github_login, avatar_url
         FROM devices
         GROUP BY github_id
       ) d ON d.github_id = u.github_id
       WHERE u.period = ? AND u.source = ?
       GROUP BY u.github_id
       ORDER BY total_cost_usd DESC
       LIMIT 100`,
    )
    .bind(period, source)
    .all<{
      github_login: string;
      avatar_url: string;
      total_cost_usd: number;
      total_tokens: number;
      device_count: number;
    }>();

  return (rows.results ?? []).map((row, idx) => ({
    rank: idx + 1,
    github_login: row.github_login,
    avatar_url: row.avatar_url,
    total_cost_usd: row.total_cost_usd,
    total_tokens: row.total_tokens,
    device_count: row.device_count,
  }));
}

/**
 * refreshLeaderboardCache rebuilds and writes the KV leaderboard for a period and source.
 * Called from upload handler after each successful upload.
 *
 * @param db     - D1 database binding
 * @param kv     - KV namespace binding
 * @param period - YYYY-MM period string
 * @param source - "claude" or "codex" (default: "claude")
 */
export async function refreshLeaderboardCache(
  db: D1Database,
  kv: KVNamespace,
  period: string,
  source: 'claude' | 'codex' = 'claude',
): Promise<void> {
  const data = await buildLeaderboard(db, period, source);
  await kv.put(kvKey(period, source), JSON.stringify(data), { expirationTtl: KV_TTL });
}
