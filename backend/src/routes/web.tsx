/**
 * web.tsx — HTML 页面路由。
 *
 * GET /           — 排行榜落地页（含产品介绍）
 * GET /u/:login   — 用户统计页（含 OG meta）
 * GET /og/:login  — OG 图片 SVG（1200×630）
 *
 * 页面 HTML 由 src/pages/ 下的 JSX 组件生成，
 * Hono JSX 运行时自动转义所有插值，无需手动 escapeHtml。
 * OG SVG 仍用字符串模板，保留 escapeHtml 防注入。
 */

import { Hono } from 'hono';
import { html } from 'hono/html';
import type { Bindings } from '../index';
import { buildLeaderboard, queryUserStats, type LeaderboardEntry } from './leaderboard';
import { LeaderboardPage } from '../pages/LeaderboardPage';
import { UserPage, type MultiSourceUserData } from '../pages/UserPage';

export const webRoutes = new Hono<{ Bindings: Bindings }>();

/** 返回当前 YYYY-MM 周期字符串。 */
function currentPeriod(): string {
  return new Date().toISOString().slice(0, 7);
}

/**
 * escapeHtml — 仅用于 OG SVG 字符串模板中的文本内容。
 * JSX 组件内的插值由 Hono 运行时自动转义，无需调用此函数。
 *
 * @param str - 原始字符串
 * @returns HTML 安全字符串
 */
function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

// ── 排行榜页 ──────────────────────────────────────────────────

/**
 * Validates the ?source= query parameter.
 * Returns 'claude' for missing/invalid values (backward compatibility).
 * 'all' is allowed as a placeholder for the future combined leaderboard.
 */
function normalizePageSource(raw: string | undefined): 'claude' | 'codex' | 'all' {
  if (raw === 'codex') return 'codex';
  if (raw === 'all') return 'all';
  return 'claude';
}

webRoutes.get('/', async (c) => {
  const period = c.req.query('period') ?? currentPeriod();
  const source = normalizePageSource(c.req.query('source'));
  // For the landing page default to 'about' tab; load claude data for the leaderboard panel.
  const rows = await buildLeaderboard(c.env.DB, period, source === 'all' ? 'claude' : source);
  return c.html(html`<!DOCTYPE html>${(
    <LeaderboardPage rows={rows} period={period} defaultTab="about" source={source} />
  )}`);
});

webRoutes.get('/leaderboard', async (c) => {
  const period = c.req.query('period') ?? currentPeriod();
  const source = normalizePageSource(c.req.query('source'));
  // Fetch rows for the requested source; 'all' shows placeholder (no DB query needed).
  const rows: LeaderboardEntry[] = source === 'all'
    ? []
    : await buildLeaderboard(c.env.DB, period, source);
  return c.html(html`<!DOCTYPE html>${(
    <LeaderboardPage rows={rows} period={period} defaultTab="leaderboard" source={source} />
  )}`);
});

// ── 用户统计页 ────────────────────────────────────────────────

webRoutes.get('/u/:login', async (c) => {
  const login = c.req.param('login');
  const period = currentPeriod();

  // Fetch stats for each source independently.
  const [claudeStats, codexStats] = await Promise.all([
    queryUserStats(c.env.DB, login, period, 'claude'),
    queryUserStats(c.env.DB, login, period, 'codex'),
  ]);

  if (!claudeStats && !codexStats) {
    return c.html('<h1>用户不存在或暂无数据</h1>', 404);
  }

  // Use avatar from whichever source has data.
  const avatarUrl = claudeStats?.avatar_url ?? codexStats?.avatar_url ?? '';
  const totalCostUsd = (claudeStats?.total_cost_usd ?? 0) + (codexStats?.total_cost_usd ?? 0);

  const userData: MultiSourceUserData = {
    github_login: login,
    avatar_url: avatarUrl,
    total_cost_usd: totalCostUsd,
    claude: claudeStats ?? null,
    codex: codexStats ?? null,
  };

  const origin = new URL(c.req.url).origin;
  const ogImg = `${origin}/og/${encodeURIComponent(login)}`;
  const shareUrl = `${origin}/u/${encodeURIComponent(login)}`;

  return c.html(html`<!DOCTYPE html>${(<UserPage user={userData} period={period} ogImg={ogImg} shareUrl={shareUrl} />)}`);
});

// ── OG 图片（SVG 字符串，保留手动转义）────────────────────────

webRoutes.get('/og/:login', async (c) => {
  const login = c.req.param('login');
  const period = currentPeriod();

  // Combine both sources for OG image; show total cost and best rank.
  const [claudeStats, codexStats] = await Promise.all([
    queryUserStats(c.env.DB, login, period, 'claude'),
    queryUserStats(c.env.DB, login, period, 'codex'),
  ]);
  const totalCostUsd = (claudeStats?.total_cost_usd ?? 0) + (codexStats?.total_cost_usd ?? 0);
  const bestRank = claudeStats?.rank ?? codexStats?.rank ?? 0;
  const user = { rank: bestRank, total_cost_usd: totalCostUsd, total_tokens: 0 };

  // SVG 文本内容需手动转义（不走 JSX 运行时）。
  const safeLogin = escapeHtml(login);
  const safePeriod = escapeHtml(period);

  const svg = `<svg width="1200" height="630" viewBox="0 0 1200 630" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="1200" y2="630" gradientUnits="userSpaceOnUse">
      <stop offset="0%" stop-color="#0b0e14"/>
      <stop offset="100%" stop-color="#0f172a"/>
    </linearGradient>
    <linearGradient id="acc" x1="0" y1="0" x2="400" y2="0" gradientUnits="userSpaceOnUse">
      <stop offset="0%" stop-color="#00c7d4"/>
      <stop offset="100%" stop-color="#00aaff"/>
    </linearGradient>
  </defs>
  <rect width="1200" height="630" fill="url(#bg)"/>
  <rect x="0" y="0" width="1200" height="6" fill="url(#acc)"/>
  <text x="80" y="100" font-family="monospace" font-size="28" fill="#38bdf8">claude-top</text>
  <text x="80" y="200" font-family="monospace" font-size="72" font-weight="bold" fill="#ffffff">@${safeLogin}</text>
  <text x="80" y="255" font-family="monospace" font-size="26" fill="#64748b">${safePeriod}</text>
  <text x="80" y="375" font-family="monospace" font-size="34" fill="#94a3b8">全球排名</text>
  <text x="80" y="460" font-family="monospace" font-size="96" font-weight="bold" fill="#fbbf24">#${user.rank}</text>
  <text x="680" y="375" font-family="monospace" font-size="34" fill="#94a3b8">月度消费</text>
  <text x="680" y="460" font-family="monospace" font-size="72" font-weight="bold" fill="#4ade80">$${user.total_cost_usd.toFixed(2)}</text>
  <text x="80" y="590" font-family="monospace" font-size="22" fill="#374151">claude-top.a2d2.dev · npx @a2d2/claude-top</text>
</svg>`;

  return new Response(svg, {
    headers: {
      'Content-Type': 'image/svg+xml',
      'Cache-Control': 'public, max-age=3600',
    },
  });
});
