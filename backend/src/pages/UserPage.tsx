/**
 * UserPage.tsx — 单用户统计页 /u/:login。
 *
 * 支持多源展示：同时有 Claude Code 和 Codex CLI 数据时分两个 section；
 * 仅有一种数据时只显示对应 section。
 * 所有动态数据由 Hono JSX 运行时自动转义，无需手动 escapeHtml。
 */

import { Layout, GithubIcon } from './Layout';

/** 单来源用量统计（来自 queryUserStats）。 */
export interface UserStats {
  github_login: string;
  avatar_url: string;
  rank: number;
  total_cost_usd: number;
  total_tokens: number;
  session_count: number;
  device_count: number;
}

/** 多源用户数据（来自 web.tsx 聚合）。 */
export interface MultiSourceUserData {
  /** GitHub 登录名 */
  github_login: string;
  /** 头像 URL */
  avatar_url: string;
  /** 合计总费用（所有来源之和）*/
  total_cost_usd: number;
  /** Claude Code 数据，无数据为 null */
  claude: UserStats | null;
  /** Codex CLI 数据，无数据为 null */
  codex: UserStats | null;
}

interface UserPageProps {
  user: MultiSourceUserData;
  period: string;
  ogImg: string;
  shareUrl: string;
}

/** 页面私有样式。 */
const pageStyles = `
main {
  position: relative; z-index: 1;
  flex: 1; display: flex;
  align-items: flex-start; justify-content: center;
  padding: 3rem 1.5rem;
  min-height: calc(100vh - 56px - 60px);
}

/* 背景光晕 */
.user-glow {
  position: fixed; top: 20%; left: 50%;
  transform: translateX(-50%);
  width: 600px; height: 400px;
  background: hsl(198 93% 59% / 0.06);
  border-radius: 50%; filter: blur(80px);
  pointer-events: none; z-index: 0;
}

.profile { width: 100%; max-width: 560px; position: relative; z-index: 1; }

/* ── 头部 ── */
.profile-header {
  display: flex; align-items: flex-start; gap: 1.25rem;
  margin-bottom: 2rem;
}
.avatar-wrap { position: relative; flex-shrink: 0; }
.avatar {
  width: 72px; height: 72px; border-radius: 50%;
  border: 2px solid hsl(198 93% 59% / 0.4);
  box-shadow: 0 0 20px hsl(198 93% 59% / 0.2);
}
.rank-chip {
  position: absolute; bottom: -4px; right: -6px;
  background: hsl(38 92% 50%);
  color: hsl(220 20% 6%);
  font-family: 'Space Mono', monospace;
  font-size: 0.62rem; font-weight: 700;
  padding: 0.1rem 0.4rem;
  border-radius: 999px;
  border: 1.5px solid hsl(222 47% 11%);
}
.profile-name {
  font-size: 1.6rem; font-weight: 700; color: #fff;
  margin-bottom: 0.15rem;
}
.profile-period {
  font-family: 'Space Mono', monospace;
  font-size: 0.72rem; color: hsl(215 20% 65%);
  margin-bottom: 0.75rem;
}
.profile-actions { display: flex; gap: 0.6rem; flex-wrap: wrap; }
.btn-sm {
  display: inline-flex; align-items: center; gap: 0.4rem;
  border: 1px solid hsl(215 19% 34%);
  color: hsl(215 20% 65%); font-size: 0.8rem;
  padding: 0.35rem 0.85rem; border-radius: 7px;
  text-decoration: none; transition: border-color 0.12s, color 0.12s;
}
.btn-sm:hover { border-color: hsl(198 93% 59% / 0.4); color: hsl(210 40% 98%); }

/* ── 合计总费用横幅 ── */
.total-banner {
  background: hsl(217 32% 17%); border: 1px solid hsl(215 19% 34%);
  border-radius: 10px; padding: 0.9rem 1.25rem;
  margin-bottom: 1.5rem;
  display: flex; align-items: center; justify-content: space-between;
}
.total-banner-label {
  font-size: 0.72rem; color: hsl(215 16% 46%);
  text-transform: uppercase; letter-spacing: 0.08em;
  font-family: 'Space Mono', monospace;
}
.total-banner-val {
  font-family: 'Space Mono', monospace;
  font-size: 1.35rem; font-weight: 700; color: hsl(142 70% 50%);
}

/* ── 来源 section ── */
.source-section {
  margin-bottom: 1.5rem;
}
.source-section-hdr {
  display: flex; align-items: center; gap: 0.5rem;
  margin-bottom: 0.75rem;
}
.source-badge {
  font-family: 'Space Mono', monospace; font-size: 0.7rem; font-weight: 700;
  padding: 0.2rem 0.65rem; border-radius: 6px;
  letter-spacing: 0.06em;
}
.badge-claude {
  color: var(--primary);
  background: var(--primary-10); border: 1px solid var(--primary-30);
}
.badge-codex {
  color: #10A37F;
  background: rgba(16,163,127,0.1); border: 1px solid rgba(16,163,127,0.3);
}
.source-section-title {
  font-size: 0.9rem; font-weight: 600; color: var(--text);
}

/* ── 统计卡片网格 ── */
.stats-grid {
  display: grid; grid-template-columns: 1fr 1fr;
  gap: 0.75rem; margin-bottom: 1rem;
}
.stat-card {
  background: hsl(217 32% 17%); border: 1px solid hsl(215 19% 34%);
  border-radius: 10px; padding: 1rem 1.25rem;
  transition: border-color 0.12s;
}
.stat-card:hover { border-color: hsl(198 93% 59% / 0.3); }
.stat-label {
  font-size: 0.68rem; color: hsl(215 16% 46%);
  letter-spacing: 0.08em; text-transform: uppercase;
  font-family: 'Space Mono', monospace; margin-bottom: 0.4rem;
}
.stat-val {
  font-family: 'Space Mono', monospace;
  font-size: 1.55rem; font-weight: 700; line-height: 1;
}
.v-amber  { color: hsl(38 92% 50%); }
.v-green  { color: hsl(142 70% 50%); }
.v-codex  { color: #10A37F; }
.v-cyan   { color: hsl(198 93% 59%); }
.v-dim    { color: hsl(215 20% 65%); }

/* ── 分割线 ── */
.source-divider {
  border: none; border-top: 1px solid hsl(215 19% 34% / 0.6);
  margin: 0.5rem 0 1.5rem;
}

/* ── 分享区 ── */
.share-card {
  background: hsl(217 32% 17%); border: 1px solid hsl(215 19% 34%);
  border-radius: 10px; padding: 1rem 1.25rem;
  margin-bottom: 1.25rem;
}
.share-label {
  font-size: 0.68rem; color: hsl(215 16% 46%);
  letter-spacing: 0.08em; text-transform: uppercase;
  font-family: 'Space Mono', monospace; margin-bottom: 0.5rem;
}
.share-url {
  font-family: 'Space Mono', monospace; font-size: 0.78rem;
  color: hsl(198 93% 59%);
  background: hsl(198 93% 59% / 0.06);
  border: 1px solid hsl(198 93% 59% / 0.18);
  padding: 0.5rem 0.75rem; border-radius: 6px;
  word-break: break-all; display: block;
}

/* ── 回到排行榜 ── */
.back-link {
  display: flex; justify-content: center;
}
`;

/**
 * SourceSection — 渲染单个来源（Claude Code 或 Codex CLI）的统计卡片块。
 *
 * @param label     - 来源显示名称
 * @param badgeClass - CSS class for badge color
 * @param stats     - 该来源的统计数据
 * @param isCodex   - 是否为 Codex（影响费用颜色）
 */
const SourceSection = ({
  label,
  badgeClass,
  stats,
  isCodex,
}: {
  label: string;
  badgeClass: string;
  stats: UserStats;
  isCodex: boolean;
}) => (
  <div class="source-section">
    <div class="source-section-hdr">
      <span class={`source-badge ${badgeClass}`}>{label}</span>
      <span class="source-section-title">全球第 {stats.rank} 名</span>
    </div>
    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-label">全球排名</div>
        <div class="stat-val v-amber">#{stats.rank}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">月度费用</div>
        <div class={`stat-val ${isCodex ? 'v-codex' : 'v-green'}`}>
          ${stats.total_cost_usd.toFixed(2)}
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-label">总 Token 数</div>
        <div class="stat-val v-cyan">{(stats.total_tokens / 1_000_000).toFixed(1)}M</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">Session 数</div>
        <div class="stat-val v-dim">{stats.session_count}</div>
      </div>
    </div>
  </div>
);

/**
 * UserPage — 单用户统计页根组件。
 *
 * @param user     - 多源用户统计数据
 * @param period   - 当前周期（YYYY-MM）
 * @param ogImg    - OG 图片 URL
 * @param shareUrl - 分享 URL
 */
export const UserPage = ({ user, period, ogImg, shareUrl }: UserPageProps) => {
  // Determine display rank: prefer Claude if available, else Codex.
  const displayRank = user.claude?.rank ?? user.codex?.rank ?? 0;
  const hasMultiple = user.claude !== null && user.codex !== null;

  return (
    <Layout
      title={`@${user.github_login} 的 AI 用量统计`}
      ogMeta={
        <>
          <meta property="og:title" content={`@${user.github_login} 的 AI 用量 ${period}`} />
          <meta property="og:description" content={`月消费 $${user.total_cost_usd.toFixed(2)} · 全球排名 #${displayRank}`} />
          <meta property="og:image" content={ogImg} />
          <meta property="og:url" content={shareUrl} />
          <meta name="twitter:card" content="summary_large_image" />
          <meta name="twitter:image" content={ogImg} />
        </>
      }
      navRight={
        <a href="/" style="font-size:0.82rem;color:hsl(215 20% 65%);text-decoration:none;display:flex;align-items:center;gap:0.35rem">
          ← 排行榜
        </a>
      }
    >
      <style dangerouslySetInnerHTML={{ __html: pageStyles }} />
      <div class="user-glow" />
      <main>
        <div class="profile">
          {/* 头部：头像 + 名字 + GitHub 按钮 */}
          <div class="profile-header">
            <div class="avatar-wrap">
              <img class="avatar" src={user.avatar_url} alt={user.github_login} />
              {displayRank > 0 && <span class="rank-chip">#{displayRank}</span>}
            </div>
            <div>
              <div class="profile-name">@{user.github_login}</div>
              <div class="profile-period">{period}</div>
              <div class="profile-actions">
                <a class="btn-sm" href={`https://github.com/${user.github_login}`} target="_blank" rel="noopener">
                  <GithubIcon /> GitHub 主页
                </a>
              </div>
            </div>
          </div>

          {/* 多源时显示合计总费用横幅 */}
          {hasMultiple && (
            <div class="total-banner">
              <span class="total-banner-label">合计月度费用</span>
              <span class="total-banner-val">${user.total_cost_usd.toFixed(2)}</span>
            </div>
          )}

          {/* Claude Code section */}
          {user.claude && (
            <SourceSection
              label="Claude Code"
              badgeClass="badge-claude"
              stats={user.claude}
              isCodex={false}
            />
          )}

          {/* 两种数据都有时加分割线 */}
          {hasMultiple && <hr class="source-divider" />}

          {/* Codex CLI section */}
          {user.codex && (
            <SourceSection
              label="Codex CLI"
              badgeClass="badge-codex"
              stats={user.codex}
              isCodex={true}
            />
          )}

          {/* 分享链接 */}
          <div class="share-card">
            <div class="share-label">分享链接</div>
            <span class="share-url">{shareUrl}</span>
          </div>

          {/* 返回 */}
          <div class="back-link">
            <a class="btn-ghost" href="/">← 查看排行榜</a>
          </div>
        </div>
      </main>
    </Layout>
  );
};
