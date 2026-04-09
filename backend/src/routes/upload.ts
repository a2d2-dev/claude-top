/**
 * POST /api/upload
 *
 * Receives monthly usage statistics from an authenticated device.
 * Upserts the `uploads` table (one row per device+period).
 * Refreshes the KV leaderboard cache for the given period.
 * Returns the user's current global rank for the period.
 */

import { Hono } from 'hono';
import type { Bindings } from '../index';
import { verifyJWT } from '../jwt';
import { refreshLeaderboardCache } from './leaderboard';

export const uploadRoutes = new Hono<{ Bindings: Bindings }>();

/** Upload payload sent by the CLI client. */
interface UploadPayload {
  period: string;
  device_id: string;
  device_name?: string;
  /** Data origin: "claude" or "codex". Defaults to "claude" for backward compat. */
  source?: string;
  total_cost_usd: number;
  total_tokens: number;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
  session_count: number;
  model_breakdown: Record<string, unknown>;
}

uploadRoutes.post('/upload', async (c) => {
  // Authenticate via Bearer JWT.
  const authHeader = c.req.header('Authorization') ?? '';
  if (!authHeader.startsWith('Bearer ')) {
    return c.json({ error: 'missing Authorization header' }, 401);
  }
  const token = authHeader.slice(7);

  let claims;
  try {
    claims = await verifyJWT(c.env.JWT_SECRET, token);
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : 'unknown';
    return c.json({ error: `unauthorized: ${msg}` }, 401);
  }

  let body: UploadPayload;
  try {
    body = await c.req.json();
  } catch {
    return c.json({ error: 'invalid JSON body' }, 400);
  }

  // Basic schema validation.
  if (!body.period || !/^\d{4}-\d{2}$/.test(body.period)) {
    return c.json({ error: 'period must be YYYY-MM' }, 400);
  }
  if (typeof body.total_cost_usd !== 'number' || body.total_cost_usd < 0) {
    return c.json({ error: 'invalid total_cost_usd' }, 400);
  }

  const githubId = parseInt(claims.sub, 10);

  // Normalize source field — default to "claude" for backward compat.
  const source: 'claude' | 'codex' = body.source === 'codex' ? 'codex' : 'claude';

  // Upsert upload record. Repeated uploads for the same device+period+source overwrite.
  await c.env.DB.prepare(
    `INSERT INTO uploads
      (github_id, device_id, period, source, total_cost_usd, total_tokens,
       input_tokens, output_tokens, cache_read_tokens, cache_write_tokens,
       session_count, model_breakdown)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
     ON CONFLICT (device_id, period, source)
     DO UPDATE SET
       github_id          = excluded.github_id,
       total_cost_usd     = excluded.total_cost_usd,
       total_tokens       = excluded.total_tokens,
       input_tokens       = excluded.input_tokens,
       output_tokens      = excluded.output_tokens,
       cache_read_tokens  = excluded.cache_read_tokens,
       cache_write_tokens = excluded.cache_write_tokens,
       session_count      = excluded.session_count,
       model_breakdown    = excluded.model_breakdown,
       uploaded_at        = datetime('now')`,
  )
    .bind(
      githubId,
      body.device_id,
      body.period,
      source,
      body.total_cost_usd,
      body.total_tokens,
      body.input_tokens,
      body.output_tokens,
      body.cache_read_tokens,
      body.cache_write_tokens,
      body.session_count,
      JSON.stringify(body.model_breakdown ?? {}),
    )
    .run();

  // Update device name if provided.
  if (body.device_name) {
    await c.env.DB.prepare(
      `UPDATE devices SET device_name = ? WHERE github_id = ? AND device_id = ?`,
    )
      .bind(body.device_name, githubId, body.device_id)
      .run();
  }

  // Compute this user's rank for the period and source (by aggregated cost across devices).
  const rank = await computeUserRank(c.env.DB, githubId, body.period, source);
  const totalUsers = await countUsers(c.env.DB, body.period, source);

  // Refresh KV leaderboard cache in the background (don't await).
  c.executionCtx.waitUntil(
    refreshLeaderboardCache(c.env.DB, c.env.LEADERBOARD, body.period, source),
  );

  return c.json({
    rank,
    total_users: totalUsers,
    share_url: `https://claude-top.a2d2.dev/u/${claims.login}`,
  });
});

/**
 * computeUserRank returns the rank (1-based) of a github_id for a given period and source.
 * Rank is determined by descending aggregated total_cost_usd across all devices.
 *
 * @param db       - D1 database binding
 * @param githubId - Numeric GitHub user ID
 * @param period   - YYYY-MM period string
 * @param source   - "claude" or "codex"
 */
async function computeUserRank(
  db: D1Database,
  githubId: number,
  period: string,
  source: 'claude' | 'codex',
): Promise<number> {
  const result = await db
    .prepare(
      `WITH totals AS (
         SELECT github_id, SUM(total_cost_usd) AS cost
         FROM uploads
         WHERE period = ? AND source = ?
         GROUP BY github_id
       )
       SELECT COUNT(*) + 1 AS rank
       FROM totals
       WHERE cost > (SELECT cost FROM totals WHERE github_id = ?)`,
    )
    .bind(period, source, githubId)
    .first<{ rank: number }>();
  return result?.rank ?? 1;
}

/**
 * countUsers returns the number of distinct github_ids with uploads for period and source.
 *
 * @param db     - D1 database binding
 * @param period - YYYY-MM period string
 * @param source - "claude" or "codex"
 */
async function countUsers(
  db: D1Database,
  period: string,
  source: 'claude' | 'codex',
): Promise<number> {
  const result = await db
    .prepare(`SELECT COUNT(DISTINCT github_id) AS cnt FROM uploads WHERE period = ? AND source = ?`)
    .bind(period, source)
    .first<{ cnt: number }>();
  return result?.cnt ?? 0;
}
