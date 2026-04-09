/**
 * LeaderboardPage.tsx — 排行榜落地页。
 *
 * 包含：排行榜 Tab（默认）、介绍 Tab（Hero / 功能 / 步骤 / CTA）。
 * 支持中英文切换（默认英文），通过 html[lang] CSS 属性控制显示。
 * 所有动态数据由 Hono JSX 运行时自动转义，无需手动 escapeHtml。
 */

import { Layout, GithubIcon } from './Layout';

/** 单条排行榜数据（来自 buildLeaderboard 查询结果）。 */
export interface LeaderboardRow {
  rank: number;
  github_login: string;
  avatar_url: string;
  total_cost_usd: number;
  total_tokens: number;
  device_count: number;
}

interface LeaderboardPageProps {
  rows: LeaderboardRow[];
  period: string;
  /** 默认激活的 tab，'about' 或 'leaderboard'，默认 'about' */
  defaultTab?: 'about' | 'leaderboard';
  /** 数据来源，驱动榜单 tab 初始状态：'claude' / 'codex' / 'all'。默认 'claude'。 */
  source?: 'claude' | 'codex' | 'all';
}

// ── 功能特性（双语） ──────────────────────────────────────────

const FEATURES = [
  {
    icon: '📊',
    tag: 'F1',
    en: { title: 'Live Overview', desc: 'Active session progress bar, token burn rate, and estimated remaining time — refreshes every 10 seconds.' },
    zh: { title: '实时概览', desc: '活跃会话进度条、Token 燃烧率、预计剩余时间，每 10 秒自动刷新。' },
  },
  {
    icon: '📋',
    tag: 'F2',
    en: { title: 'Session History', desc: 'Sortable history table — drill into any session to see per-message token breakdown.' },
    zh: { title: 'Session 历史', desc: '可排序的历史记录表，钻取任意 Session 查看逐条消息的 Token 拆解。' },
  },
  {
    icon: '📅',
    tag: 'F3',
    en: { title: 'Calendar Heatmap', desc: '52-week contribution graph showing which days you used Claude most, with daily cost summaries.' },
    zh: { title: '日历热力图', desc: '52 周贡献图，一眼看出你哪天用 Claude 最狠，带每日费用汇总。' },
  },
  {
    icon: '🌐',
    tag: 'F4',
    en: { title: 'Global Leaderboard', desc: 'Press u to voluntarily upload this month\'s aggregated data. Multi-device merge included.' },
    zh: { title: '全球排行榜', desc: '按 u 键自愿上传本月聚合数据，多设备自动合并，登上全球榜单。' },
  },
] as const;

// ── 使用步骤（双语） ──────────────────────────────────────────

const STEPS = [
  {
    num: '01',
    en: { title: 'Install the CLI', desc: 'No global install needed — run with npx. Reads your local ~/.claude/projects. Data stays on your machine.' },
    zh: { title: '安装 CLI', desc: '无需全局安装，直接用 npx 运行。读取本地 ~/.claude/projects 目录，数据不会自动外传。' },
    code: '$ npx @a2d2/claude-top',
    codeColor: 'var(--primary)',
    keys: null as readonly string[] | null,
  },
  {
    num: '02',
    en: { title: 'Explore Your Usage', desc: 'After launch, press 1 / 2 / 3 to switch between Overview, Sessions, and Daily calendar views.' },
    zh: { title: '查看用量', desc: '启动后进入终端 TUI 界面，用 1 / 2 / 3 切换概览、Session 列表、日历视图。' },
    code: null as string | null,
    codeColor: '',
    keys: ['1', '2', '3', '↑↓'] as readonly string[],
  },
  {
    num: '03',
    en: { title: 'Upload & Rank', desc: 'Press u to upload this month\'s aggregated stats (cost, tokens, device count). No prompts are shared.' },
    zh: { title: '上传并上榜', desc: '在主界面按 u，工具将本月聚合统计（费用、Token 数、设备数）上传至排行榜，仅含汇总数字，不含任何 prompt 内容。' },
    code: '✓ Uploaded! Global rank: #42',
    codeColor: 'var(--green)',
    keys: null as readonly string[] | null,
  },
] as const;

// ── CSS ──────────────────────────────────────────────────────

/** 页面所有样式。 */
const pageStyles = `
/* ── 语言切换 ── */
html[lang="en"] [data-lang="zh"] { display: none !important; }
html[lang="zh"] [data-lang="en"] { display: none !important; }

/* ── 顶部 Tab 栏 ── */
.page-tabs {
  position: relative; z-index: 10;
  display: flex; justify-content: center; gap: 0;
  border-bottom: 1px solid var(--border);
  background: var(--nav-bg);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
}
.tab-btn {
  display: flex; align-items: center; gap: 0.5rem;
  padding: 0.75rem 2rem;
  font-size: 0.875rem; font-weight: 500;
  color: var(--text-muted);
  background: none; border: none; border-bottom: 2px solid transparent;
  cursor: pointer; margin-bottom: -1px;
  transition: color 0.15s, border-color 0.15s;
  font-family: inherit;
}
.tab-btn:hover { color: var(--text); }
.tab-btn.active { color: var(--primary); border-bottom-color: var(--primary); }
.tab-panel { display: none; }
.tab-panel.active { display: block; }

/* ── 来源子 Tab（Claude / Codex / 总榜）── */
.source-tabs {
  display: flex; justify-content: center; gap: 0;
  border-bottom: 1px solid var(--border);
  background: hsl(220 20% 7%);
  padding: 0 1.5rem;
}
.source-tab {
  display: flex; align-items: center; gap: 0.4rem;
  padding: 0.55rem 1.5rem;
  font-size: 0.8rem; font-weight: 500;
  color: var(--text-muted);
  background: none; border: none; border-bottom: 2px solid transparent;
  cursor: pointer; margin-bottom: -1px;
  transition: color 0.15s, border-color 0.15s;
  font-family: inherit;
}
.source-tab:hover { color: var(--text); }
.source-tab.active-claude { color: var(--primary); border-bottom-color: var(--primary); }
.source-tab.active-codex  { color: #10A37F;        border-bottom-color: #10A37F; }
.source-tab.coming-soon   { color: var(--text-dim); cursor: default; font-size: 0.75rem; }
.source-panel { display: none; }
.source-panel.active { display: block; }

/* Codex 绿色主题覆盖 */
#sp-codex .cost-val { color: #10A37F; }
#sp-codex .lb-period { color: #10A37F; background: rgba(16,163,127,0.1); border-color: rgba(16,163,127,0.3); }
#sp-all .lb-empty { color: var(--text-muted); }

/* ── Hero ── */
.hero {
  position: relative; z-index: 1;
  max-width: 960px; margin: 0 auto;
  padding: 4rem 1.5rem 3rem;
  display: grid; grid-template-columns: 1fr 1fr;
  gap: 3rem; align-items: center;
}
@media (max-width: 640px) {
  .hero { grid-template-columns: 1fr; gap: 2rem; }
  .hero-terminal { display: none; }
}
.hero-glow {
  position: absolute; border-radius: 50%;
  background: hsl(198 93% 59% / 0.08); filter: blur(80px);
  pointer-events: none;
}
.hero-badge {
  display: inline-flex; align-items: center; gap: 0.5rem;
  font-family: 'Space Mono', monospace; font-size: 0.75rem;
  color: var(--primary);
  background: var(--primary-10); border: 1px solid var(--primary-30);
  padding: 0.25rem 0.75rem; border-radius: 999px;
  margin-bottom: 1.25rem;
}
.hero-badge-dot {
  width: 8px; height: 8px; border-radius: 50%; background: var(--primary);
  animation: pulse 2s infinite;
}
@keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.4; } }
.hero h1 {
  font-size: clamp(2rem, 5vw, 3rem); font-weight: 700;
  line-height: 1.15; letter-spacing: -0.03em; color: #fff;
  margin-bottom: 1rem;
}
.hero-sub {
  color: var(--text-muted); font-size: 1rem; line-height: 1.7;
  margin-bottom: 1.75rem; max-width: 440px;
}
.hero-actions { display: flex; gap: 0.75rem; flex-wrap: wrap; margin-bottom: 1.5rem; }
.hero-badges {
  display: flex; gap: 1.25rem;
  font-size: 0.8rem; color: var(--text-dim);
}
.hero-badges span {
  display: flex; align-items: center; gap: 0.35rem;
}
.hero-badges span::before {
  content: ''; width: 6px; height: 6px; border-radius: 50%;
  background: var(--green); display: inline-block;
}

/* ── 终端 TUI 仿真窗口 ── */
.terminal {
  background: hsl(220 25% 7%);
  border: 1px solid var(--border);
  border-radius: 12px; overflow: hidden;
  font-family: 'Space Mono', monospace; font-size: 0.72rem;
  box-shadow: var(--glow);
  line-height: 1.4;
}
.term-bar {
  background: hsl(220 18% 14%);
  padding: 0.55rem 1rem;
  display: flex; align-items: center; gap: 0.4rem;
  border-bottom: 1px solid var(--border);
}
.dot { width: 10px; height: 10px; border-radius: 50%; }
.dot-r { background: #ff5f57; }
.dot-y { background: #febc2e; }
.dot-g { background: #28c840; }
.term-title { margin-left: auto; color: var(--text-dim); font-size: 0.68rem; }
.term-tabs {
  display: flex; gap: 0;
  background: hsl(220 25% 9%);
  border-bottom: 1px solid var(--border);
  padding: 0 0.5rem;
}
.term-tab {
  padding: 0.35rem 0.85rem;
  color: var(--text-dim); font-size: 0.68rem;
  border-bottom: 1px solid transparent;
  cursor: default;
}
.term-tab-active {
  color: var(--primary);
  border-bottom-color: var(--primary);
  background: hsl(198 93% 59% / 0.06);
}
.term-section-hdr {
  padding: 0.45rem 1rem 0;
  color: hsl(270 80% 70%);
  font-size: 0.65rem;
}
.term-info { padding: 0.1rem 1rem; color: var(--text-dim); font-size: 0.65rem; white-space: nowrap; overflow: hidden; }
.term-model { padding: 0.25rem 1rem; }
.t-dim { color: hsl(215 16% 46%); }
.t-out { color: hsl(210 20% 72%); }
.t-ok  { color: hsl(142 70% 50%); }
.t-hi  { color: hsl(38 92% 60%); }
.t-cmd { color: var(--primary); }
.t-purple { color: hsl(270 80% 70%); }

/* Cost bar chart */
.term-chart-wrap { padding: 0.4rem 1rem 0; }
.term-chart-label { color: var(--text-dim); font-size: 0.65rem; margin-bottom: 0.25rem; }
.term-chart {
  display: flex; align-items: flex-end; gap: 2px;
  height: 44px; padding-bottom: 2px;
}
.chart-bar {
  width: 7px; border-radius: 1px 1px 0 0; flex-shrink: 0;
}
.bar-lo  { background: hsl(142 70% 45% / 0.7); }
.bar-md  { background: hsl(38 92% 50% / 0.8); }
.bar-hi  { background: hsl(0 80% 60% / 0.85); }
.term-chart-footer {
  font-size: 0.62rem; color: var(--text-dim);
  padding: 0.2rem 0;
  border-bottom: 1px solid hsl(215 19% 20%);
}

/* Messages table */
.term-msgs-hdr {
  padding: 0.35rem 1rem 0.15rem;
  color: var(--primary); font-size: 0.65rem;
  border-top: 1px solid hsl(215 19% 20%);
}
.term-msg-cols {
  display: flex; gap: 0.75rem;
  padding: 0 1rem 0.15rem;
  color: var(--text-dim); font-size: 0.62rem;
  border-bottom: 1px solid hsl(215 19% 18%);
}
.term-msg-row {
  display: flex; align-items: center;
  padding: 0.12rem 1rem;
  font-size: 0.65rem; color: var(--text-dim);
  gap: 0;
}
.term-msg-row.sel { background: hsl(198 93% 59% / 0.08); color: var(--text); }

/* 终端 Screen 切换 */
.term-screen { display: none; animation: fadeIn 0.35s ease; }
.term-screen.ts-active { display: block; }
@keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }

/* Overview screen */
.term-ov-row {
  display: flex; align-items: center; gap: 0.5rem;
  padding: 0.15rem 1rem; font-size: 0.65rem;
}
.term-bar-track {
  flex: 1; height: 5px; background: hsl(215 19% 20%);
  border-radius: 3px; overflow: hidden;
}
.term-bar-fill {
  height: 100%; border-radius: 3px;
  background: linear-gradient(90deg, var(--primary), hsl(142 70% 50%));
}
.term-stat-grid {
  display: grid; grid-template-columns: 1fr 1fr;
  gap: 0.1rem 0; padding: 0.2rem 1rem;
}
.term-stat-item { font-size: 0.65rem; }
.term-hint {
  padding: 0.35rem 1rem;
  font-size: 0.62rem; color: var(--text-dim);
  border-top: 1px solid hsl(215 19% 20%);
  display: flex; align-items: center; gap: 0.5rem;
}
.term-hint-key {
  display: inline-flex; align-items: center; justify-content: center;
  width: 18px; height: 14px; border-radius: 3px;
  background: hsl(215 19% 22%); border: 1px solid hsl(215 19% 30%);
  color: var(--amber); font-size: 0.58rem; flex-shrink: 0;
}

/* Daily calendar — horizontal layout matching real TUI */
.term-cal-h { padding: 0.3rem 0.75rem 0; }
.term-cal-month-row {
  display: grid; grid-template-columns: 1.6rem repeat(12, 1fr);
  gap: 2px; margin-bottom: 2px;
}
.cal-month-lbl {
  font-size: 0.58rem; color: var(--text-dim);
  grid-column: span 3; text-align: left;
  overflow: hidden; white-space: nowrap;
}
.term-cal-day-row {
  display: grid; grid-template-columns: 1.6rem repeat(12, 1fr);
  gap: 2px; margin-bottom: 2px; align-items: center;
}
.cal-day-lbl { font-size: 0.6rem; color: hsl(215 16% 46%); }
/* heatmap cell — green-only shades matching real TUI */
.cc {
  height: 8px; border-radius: 1px;
}
.cc-nil { background: hsl(215 19% 14%); }
.cc-lo  { background: hsl(142 45% 18%); }
.cc-md  { background: hsl(142 55% 33%); }
.cc-hi  { background: hsl(142 65% 48%); }
.cc-today { outline: 1px solid hsl(38 92% 50%); outline-offset: 1px; }
.term-cost-summary {
  padding: 0.3rem 0.75rem;
  border-top: 1px solid hsl(215 19% 20%);
  font-size: 0.62rem;
}
.term-daily-row {
  display: flex; align-items: center; gap: 0;
  padding: 0.25rem 0.75rem; font-size: 0.62rem;
  border-top: 1px solid hsl(215 19% 20%);
}
.daily-bar-wrap {
  flex: 1; height: 5px;
  background: hsl(215 19% 20%); border-radius: 2px; overflow: hidden;
  margin: 0 0.4rem;
}
.daily-bar-fill { height: 100%; background: hsl(142 55% 40%); border-radius: 2px; }

/* Copy 按钮 */
.btn-copy {
  cursor: pointer; border: none; font-family: 'Space Mono', monospace;
  font-size: 0.82rem; letter-spacing: 0;
  transition: background 0.15s, color 0.15s, transform 0.1s;
}
.btn-copy:active { transform: scale(0.97); }
.btn-copy-hint {
  font-size: 0.65rem; opacity: 0.7; margin-left: 0.4rem;
  font-family: inherit;
}

/* ── 分节 ── */
.section {
  position: relative; z-index: 1;
  max-width: 960px; margin: 0 auto; padding: 4.5rem 1.5rem;
}
.section-bg {
  background: linear-gradient(180deg, transparent, hsl(198 93% 59% / 0.04), transparent);
}
.section-header { text-align: center; margin-bottom: 3rem; }
.section-tag {
  display: inline-block;
  font-family: 'Space Mono', monospace; font-size: 0.7rem;
  color: var(--primary); letter-spacing: 0.12em; text-transform: uppercase;
  margin-bottom: 0.75rem;
}
.section-title {
  font-size: clamp(1.5rem, 3vw, 2.2rem); font-weight: 700;
  color: #fff; letter-spacing: -0.02em; margin-bottom: 0.6rem;
}
.section-sub {
  color: var(--text-muted); font-size: 0.95rem;
  max-width: 520px; margin: 0 auto; line-height: 1.65;
}
.section-divider {
  border: none; border-top: 1px solid var(--border); margin: 0;
  position: relative; z-index: 1;
}

/* ── 功能卡片 ── */
.features-grid {
  display: grid; grid-template-columns: repeat(auto-fit, minmax(210px, 1fr));
  gap: 1rem;
}
.feat-card {
  background: var(--bg-card); border: 1px solid var(--border);
  border-radius: 12px; padding: 1.4rem;
  transition: border-color 0.15s, box-shadow 0.15s;
  animation: fade-in-up 0.5s ease-out forwards; opacity: 0;
}
.feat-card:hover { border-color: var(--primary-30); box-shadow: var(--glow-card); }
.feat-icon {
  width: 40px; height: 40px;
  background: var(--primary-10); border: 1px solid var(--primary-20);
  border-radius: 9px;
  display: flex; align-items: center; justify-content: center;
  font-size: 1.1rem; margin-bottom: 1rem;
}
.feat-tag {
  float: right; margin-top: 0.15rem;
  font-family: 'Space Mono', monospace; font-size: 0.65rem;
  color: var(--text-dim); letter-spacing: 0.1em; text-transform: uppercase;
}
.feat-title { font-size: 0.95rem; font-weight: 600; color: #fff; margin-bottom: 0.5rem; }
.feat-desc  { font-size: 0.82rem; color: var(--text-muted); line-height: 1.6; }

/* ── 步骤 ── */
.steps { max-width: 680px; margin: 0 auto; }
.step  {
  position: relative; display: flex; gap: 1.5rem;
  padding-bottom: 2.5rem;
}
.step:last-child { padding-bottom: 0; }
.step-line {
  position: absolute; left: 23px; top: 48px; bottom: 0; width: 1px;
  background: linear-gradient(to bottom, var(--primary-30), transparent);
}
.step-num {
  flex-shrink: 0; width: 48px; height: 48px; border-radius: 50%;
  background: var(--primary-10); border: 1px solid var(--primary-30);
  display: flex; align-items: center; justify-content: center;
  font-family: 'Space Mono', monospace; font-size: 0.78rem;
  font-weight: 700; color: var(--primary); position: relative; z-index: 1;
}
.step-content { flex: 1; padding-top: 0.6rem; }
.step-title {
  font-size: 1.05rem; font-weight: 600; color: #fff;
  margin-bottom: 0.4rem;
}
.step-desc { color: var(--text-muted); font-size: 0.875rem; line-height: 1.65; margin-bottom: 0.75rem; }
.step-code {
  display: inline-block;
  font-family: 'Space Mono', monospace; font-size: 0.78rem;
  background: var(--bg-card); border: 1px solid var(--border);
  border-radius: 7px; padding: 0.55rem 1rem;
}
.step-keys { display: flex; gap: 0.4rem; flex-wrap: wrap; }
.step-key {
  font-family: 'Space Mono', monospace; font-size: 0.72rem;
  background: hsl(38 92% 50% / 0.1); border: 1px solid hsl(38 92% 50% / 0.3);
  color: var(--amber); padding: 0.15rem 0.5rem; border-radius: 4px;
}

/* ── CTA 横幅 ── */
.cta-strip {
  position: relative; z-index: 1;
  background: linear-gradient(135deg, hsl(220 20% 8%) 0%, hsl(220 20% 12%) 100%);
  border-top: 1px solid var(--border); border-bottom: 1px solid var(--border);
  overflow: hidden;
}
.cta-glow {
  position: absolute; bottom: -100px; left: 50%;
  transform: translateX(-50%);
  width: 600px; height: 300px;
  background: hsl(198 93% 59% / 0.12); border-radius: 50%; filter: blur(60px);
}
.cta-inner {
  position: relative; z-index: 1;
  max-width: 720px; margin: 0 auto;
  padding: 4rem 1.5rem; text-align: center;
}
.cta-badge {
  display: inline-flex; align-items: center; gap: 0.5rem;
  border: 1px solid var(--primary-30); background: var(--primary-10);
  padding: 0.35rem 1rem; border-radius: 999px;
  font-size: 0.82rem; color: var(--text); margin-bottom: 1.75rem;
}
.cta-inner h2 {
  font-size: clamp(1.6rem, 4vw, 2.4rem); font-weight: 700;
  color: #fff; line-height: 1.2; letter-spacing: -0.02em; margin-bottom: 0.75rem;
}
.cta-inner p { color: var(--text-muted); font-size: 0.95rem; line-height: 1.65; margin-bottom: 2rem; }
.cta-btns { display: flex; gap: 0.75rem; justify-content: center; flex-wrap: wrap; margin-bottom: 2.5rem; }
.cta-tools {
  border-top: 1px solid hsl(215 19% 34% / 0.5);
  padding-top: 1.75rem; color: var(--text-dim); font-size: 0.8rem;
}
.cta-tools-row {
  display: flex; justify-content: center; gap: 2rem; margin-top: 0.75rem;
  font-family: 'Space Mono', monospace; font-size: 0.78rem; color: var(--text-dim);
}

/* ── 排行榜表格 ── */
.lb-header {
  position: relative; z-index: 1;
  max-width: 960px; margin: 0 auto;
  padding: 2.5rem 1.5rem 1rem;
  display: flex; align-items: baseline; gap: 1rem;
}
.lb-title { font-size: 1.15rem; font-weight: 700; color: #fff; }
.lb-period {
  font-family: 'Space Mono', monospace; font-size: 0.72rem;
  color: var(--primary);
  background: var(--primary-10); border: 1px solid var(--primary-30);
  padding: 0.2rem 0.65rem; border-radius: 6px;
}
.table-wrap {
  position: relative; z-index: 1;
  max-width: 960px; margin: 0 auto 3rem; padding: 0 1.5rem;
}
.table-card {
  background: var(--bg-card); border: 1px solid var(--border);
  border-radius: 12px; overflow: hidden;
  box-shadow: var(--glow-card);
}
table { width: 100%; border-collapse: collapse; }
thead tr {
  background: hsl(198 93% 59% / 0.04);
  border-bottom: 1px solid var(--border);
}
th {
  text-align: left; color: var(--text-dim);
  font-size: 0.68rem; font-weight: 500;
  letter-spacing: 0.1em; text-transform: uppercase;
  padding: 0.75rem 1rem;
  font-family: 'Space Mono', monospace;
}
tbody tr { border-bottom: 1px solid hsl(215 19% 34% / 0.4); transition: background 0.12s; }
tbody tr:last-child { border-bottom: none; }
tbody tr:hover { background: var(--bg-card-hover); }
td { padding: 0.75rem 1rem; vertical-align: middle; }
.rank-badge { font-family: 'Space Mono', monospace; font-size: 0.85rem; color: var(--text-dim); }
.user-inner { display: flex; align-items: center; gap: 0.6rem; }
.user-inner img { border-radius: 50%; flex-shrink: 0; border: 1px solid var(--border); }
.user-link { color: var(--text); text-decoration: none; font-weight: 500; font-size: 0.9rem; }
.user-link:hover { color: var(--primary); }
.gh-link { color: var(--text-dim); display: flex; align-items: center; transition: color 0.12s; }
.gh-link:hover { color: var(--text-muted); }
.cost-val { font-family: 'Space Mono', monospace; font-size: 0.88rem; color: var(--green); font-weight: 700; }
.token-val { font-family: 'Space Mono', monospace; font-size: 0.88rem; color: hsl(198 93% 59%); }
td:last-child { color: var(--text-dim); font-family: 'Space Mono', monospace; font-size: 0.82rem; }
.lb-empty {
  text-align: center; padding: 4rem 1.5rem;
  color: var(--text-dim); font-size: 0.9rem; line-height: 1.8;
}
`;

/** 页面交互脚本：Tab 切换、来源 Tab、语言切换、终端自动轮播、复制命令。 */
const pageScript = `
(function() {
  // ── 页面 Tab（About / Leaderboard）──
  function showTab(id) {
    document.querySelectorAll('.tab-panel').forEach(function(p) { p.classList.remove('active'); });
    document.querySelectorAll('.tab-btn').forEach(function(b) { b.classList.remove('active'); });
    var panel = document.getElementById('tab-' + id);
    var btn   = document.querySelector('[data-tab="' + id + '"]');
    if (panel) panel.classList.add('active');
    if (btn)   btn.classList.add('active');
  }

  // ── 来源 Tab（Claude / Codex / 总榜）──
  // Switches the visible source panel AND updates the URL ?source= param.
  function showSource(src) {
    document.querySelectorAll('.source-panel').forEach(function(p) { p.classList.remove('active'); });
    document.querySelectorAll('.source-tab').forEach(function(b) {
      b.classList.remove('active-claude');
      b.classList.remove('active-codex');
    });
    var panel = document.getElementById('sp-' + src);
    var btn   = document.querySelector('[data-source="' + src + '"]');
    if (panel) panel.classList.add('active');
    if (btn) {
      if (src === 'codex') btn.classList.add('active-codex');
      else if (src === 'claude') btn.classList.add('active-claude');
      // 'all' coming-soon tab has no active highlight
    }
    // Update URL without full reload so share links work.
    try {
      var url = new URL(location.href);
      url.searchParams.set('source', src);
      history.replaceState(null, '', url.toString());
    } catch(e) {}
  }

  // ── 语言切换 ──
  function toggleLang() {
    var cur  = document.documentElement.lang || 'en';
    var next = cur === 'en' ? 'zh' : 'en';
    document.documentElement.lang = next;
    var btn = document.getElementById('lang-btn');
    if (btn) btn.textContent = next === 'zh' ? 'EN' : '中文';
    try { localStorage.setItem('lang', next); } catch(e) {}
  }

  // ── 终端 TUI 自动轮播 ──
  var termScreens = ['overview', 'sessions', 'daily'];
  var termIdx = 0;
  function showTermScreen(name) {
    document.querySelectorAll('.term-screen').forEach(function(s) { s.classList.remove('ts-active'); });
    document.querySelectorAll('.term-ttab').forEach(function(t) { t.classList.remove('term-tab-active'); });
    var s = document.getElementById('ts-' + name);
    var t = document.getElementById('tt-' + name);
    if (s) s.classList.add('ts-active');
    if (t) t.classList.add('term-tab-active');
  }
  function cycleTermScreen() {
    termIdx = (termIdx + 1) % termScreens.length;
    showTermScreen(termScreens[termIdx]);
  }

  // ── 复制命令 ──
  function copyCmd(el, cmd) {
    if (!navigator.clipboard) return;
    navigator.clipboard.writeText(cmd).then(function() {
      var orig = el.innerHTML;
      el.innerHTML = '&#x2713; Copied!';
      setTimeout(function() { el.innerHTML = orig; }, 2000);
    });
  }

  window.showTab     = showTab;
  window.showSource  = showSource;
  window.toggleLang  = toggleLang;
  window.copyCmd     = copyCmd;

  document.addEventListener('DOMContentLoaded', function() {
    // 语言
    var lang = 'en';
    try { lang = localStorage.getItem('lang') || 'en'; } catch(e) {}
    document.documentElement.lang = lang;
    var lbtn = document.getElementById('lang-btn');
    if (lbtn) lbtn.textContent = lang === 'zh' ? 'EN' : '中文';

    // 页面 Tab：hash 优先，否则使用服务端注入的默认值
    var hash      = location.hash.replace('#', '');
    var validTabs = ['leaderboard', 'about'];
    var tab = validTabs.indexOf(hash) !== -1 ? hash : __DEFAULT_TAB__;
    showTab(tab);

    // 来源 Tab：URL ?source= 参数优先，否则使用服务端注入的默认值
    var urlSource    = new URL(location.href).searchParams.get('source');
    var validSources = ['claude', 'codex', 'all'];
    var initSource = validSources.indexOf(urlSource) !== -1 ? urlSource : __DEFAULT_SOURCE__;
    showSource(initSource);

    // 终端轮播：先显示 overview，3.5 秒一循环
    showTermScreen('overview');
    setInterval(cycleTermScreen, 3500);
  });
})();
`;

// ── 子组件 ────────────────────────────────────────────────────

/**
 * TerminalBlock — 三屏自动轮播的 claude-top TUI 仿真窗口。
 * JS 每 3.5s 切换一次：Overview → Sessions → Daily → …
 */
const TerminalBlock = () => {
  const bars = [
    { h: 10, t: 'lo' }, { h: 8,  t: 'lo' }, { h: 15, t: 'lo' }, { h: 12, t: 'lo' },
    { h: 20, t: 'md' }, { h: 35, t: 'md' }, { h: 18, t: 'lo' }, { h: 28, t: 'md' },
    { h: 44, t: 'hi' }, { h: 32, t: 'md' }, { h: 22, t: 'md' }, { h: 12, t: 'lo' },
    { h: 25, t: 'md' }, { h: 16, t: 'lo' }, { h: 40, t: 'hi' }, { h: 30, t: 'md' },
    { h: 18, t: 'lo' }, { h: 36, t: 'md' }, { h: 44, t: 'hi' }, { h: 8,  t: 'lo' },
  ];

  // 水平热力图：每行=一个工作日，每列=一周（共12周，模拟最近3个月）
  // 0=无 1=低 2=中 3=高，行顺序：Mo/We/Fr/Su
  const calRows: Array<{ label: string; cells: number[] }> = [
    { label: 'Mo', cells: [0,0,0,1,2,0,2,3,2,3,2,3] },
    { label: 'We', cells: [0,0,0,1,2,1,3,3,3,3,2,1] },
    { label: 'Fr', cells: [0,0,0,0,1,1,2,2,3,3,3,0] },
    { label: 'Su', cells: [0,0,0,0,1,0,1,2,1,2,1,0] },
  ];
  const ccClass = ['cc-nil','cc-lo','cc-md','cc-hi'];

  return (
    <div class="hero-terminal terminal">
      {/* 窗口标题栏 */}
      <div class="term-bar">
        <span class="dot dot-r" /><span class="dot dot-y" /><span class="dot dot-g" />
        <span class="term-title">claude-top — zsh</span>
      </div>

      {/* Tab 栏（JS 切换 term-tab-active） */}
      <div class="term-tabs">
        <span id="tt-overview" class="term-tab term-ttab">Overview</span>
        <span id="tt-sessions" class="term-tab term-ttab">Sessions</span>
        <span id="tt-daily"    class="term-tab term-ttab">Daily</span>
        <span style="margin-left:auto;padding:0.35rem 0.85rem;color:hsl(215 16% 46%);font-size:0.62rem">08:42:30</span>
      </div>

      {/* ── Screen 1: Overview ── */}
      <div id="ts-overview" class="term-screen">
        <div class="term-section-hdr" style="padding-bottom:0.2rem">MONTHLY OVERVIEW — 2026-04</div>

        <div class="term-ov-row" style="padding-top:0.4rem">
          <span class="t-dim" style="min-width:4.5rem;font-size:0.62rem">Cost</span>
          <div class="term-bar-track"><div class="term-bar-fill" style="width:62%" /></div>
          <span class="t-ok" style="font-size:0.65rem;min-width:4rem;text-align:right">$12.40</span>
        </div>
        <div class="term-ov-row">
          <span class="t-dim" style="min-width:4.5rem;font-size:0.62rem">Tokens</span>
          <div class="term-bar-track"><div class="term-bar-fill" style="width:55%;background:var(--primary)" /></div>
          <span class="t-cmd" style="font-size:0.65rem;min-width:4rem;text-align:right">10.8M</span>
        </div>

        <div class="term-stat-grid" style="margin-top:0.4rem">
          <span class="term-stat-item t-dim">Sessions <span class="t-out">847</span></span>
          <span class="term-stat-item t-dim">Devices  <span class="t-out">3</span></span>
        </div>

        <div style="border-top:1px solid hsl(215 19% 20%);margin:0.4rem 0" />

        <div class="term-section-hdr" style="color:hsl(142 70% 50%)">● Active — claude-sonnet-4.6</div>
        <div class="term-info">
          Window: 23:00 → 00:50&nbsp;
          <span class="t-dim">(1h 50m)&nbsp;&nbsp;Dir: ~/backend</span>
        </div>
        <div class="term-info">
          <span class="t-ok">$4.4332</span>
          <span class="t-dim">&nbsp;&nbsp;10.8M tokens&nbsp;&nbsp;143 msgs</span>
        </div>

        <div class="term-hint">
          <span class="term-hint-key">u</span>
          <span class="t-dim">upload &amp; join global leaderboard</span>
        </div>
      </div>

      {/* ── Screen 2: Sessions ── */}
      <div id="ts-sessions" class="term-screen">
        <div class="term-section-hdr">SESSION DETAIL</div>
        <div class="term-info" style="color:hsl(210 20% 72%)">
          Window: 2026-04-08 23:00 → 00:50&nbsp;
          <span class="t-dim">(1h 50m)  Dir: ~/backend</span>
        </div>
        <div class="term-model">
          <span class="t-ok">● Sonnet 4.6</span>
          <span class="t-dim">  10.8M</span>
          <span class="t-ok">  $4.4332</span>
          <span class="t-dim">  100.0%  (143 msgs)</span>
        </div>

        <div class="term-chart-wrap">
          <div class="term-chart-label">Cost over time</div>
          <div class="term-chart">
            {bars.map((b) => (
              <span class={`chart-bar bar-${b.t}`} style={`height:${b.h}px`} />
            ))}
          </div>
          <div class="term-chart-footer">peak $0.4306 &nbsp;|&nbsp; 46s/col &nbsp;|&nbsp; 143 msgs</div>
        </div>

        <div class="term-msgs-hdr">Messages [106/143]</div>
        <div class="term-msg-row">
          <span class="t-dim" style="min-width:6.2rem">23:51:22</span>
          <span class="t-out" style="min-width:7rem">claude-sonnet..</span>
          <span class="t-dim" style="min-width:3.8rem">82.5k</span>
          <span class="t-ok">$0.0265</span>
        </div>
        <div class="term-msg-row">
          <span class="t-dim" style="min-width:6.2rem">00:15:05</span>
          <span class="t-out" style="min-width:7rem">claude-sonnet..</span>
          <span class="t-dim" style="min-width:3.8rem">78.5k</span>
          <span class="t-ok">$0.0263</span>
        </div>
        <div class="term-msg-row sel">
          <span style="color:var(--primary);min-width:0.8rem">▶</span>
          <span class="t-dim" style="min-width:5.4rem">23:59:37</span>
          <span class="t-out" style="min-width:7rem">claude-sonnet..</span>
          <span class="t-dim" style="min-width:3.8rem">55.0k</span>
          <span class="t-ok">$0.0213</span>
        </div>

        {/* 提示：可进入 session 详情 */}
        <div class="term-hint">
          <span class="term-hint-key">↵</span>
          <span class="t-dim">open session detail — messages, sourcing &amp; cost breakdown</span>
        </div>
      </div>

      {/* ── Screen 3: Daily ── */}
      <div id="ts-daily" class="term-screen">
        <div class="term-section-hdr" style="color:hsl(38 92% 60%)">DAILY &amp; STATS</div>

        {/* 水平热力图（月份跨列，工作日作行，与真实 TUI 一致） */}
        <div class="term-cal-h">
          {/* 月份标签行 */}
          <div class="term-cal-month-row">
            <span />{/* day-label spacer */}
            <span class="cal-month-lbl">Feb</span>
            <span /><span />
            <span class="cal-month-lbl">Mar</span>
            <span /><span />
            <span class="cal-month-lbl">Apr</span>
            <span /><span /><span /><span />
          </div>
          {/* 热力图行（Mo / We / Fr / Su） */}
          {calRows.map((row) => (
            <div class="term-cal-day-row" key={row.label}>
              <span class="cal-day-lbl">{row.label}</span>
              {row.cells.map((v, i) => (
                <div class={`cc ${ccClass[v]}${i === 11 ? ' cc-today' : ''}`} key={i} />
              ))}
            </div>
          ))}
          {/* Less → More 图例 */}
          <div style="display:flex;align-items:center;gap:3px;margin-top:3px;font-size:0.58rem;color:hsl(215 16% 46%)">
            <span style="margin-right:2px">Less</span>
            {['cc-nil','cc-lo','cc-md','cc-hi'].map((c) => (
              <div class={`cc ${c}`} style="width:10px;flex-shrink:0" key={c} />
            ))}
            <span style="margin-left:2px">More</span>
          </div>
        </div>

        {/* COST SUMMARY */}
        <div class="term-cost-summary" style="color:hsl(38 92% 60%);font-weight:700;margin-bottom:0.15rem">
          COST SUMMARY
        </div>
        <div class="term-cost-summary" style="padding-top:0;line-height:1.7">
          <div class="t-dim">
            Total cost: <span class="t-hi">$2,573.86</span>
            &nbsp;&nbsp;Active days: <span class="t-out">83</span>
            &nbsp;&nbsp;Avg/day: <span class="t-dim">$31.01</span>
          </div>
          <div class="t-dim">
            Peak day: <span class="t-hi">$121.56</span>
            <span style="color:hsl(215 16% 46%)"> (2026-04-05)</span>
          </div>
        </div>

        {/* DAILY 日明细表头 + 一行示例 */}
        <div class="term-daily-row" style="color:var(--primary)">
          DAILY&nbsp;<span class="t-dim">[1-42 / 83]</span>
        </div>
        <div class="term-daily-row" style="background:hsl(198 93% 59% / 0.08)">
          <span class="t-ok" style="min-width:5.5rem">04-09 Thu</span>
          <span class="t-dim" style="min-width:2.5rem">261</span>
          <span class="t-dim" style="min-width:3.2rem">26.7M</span>
          <span class="t-ok" style="min-width:3.5rem">$12.69</span>
          <div class="daily-bar-wrap"><div class="daily-bar-fill" style="width:10%" /></div>
          <span class="t-dim">0.5%</span>
        </div>

        <div class="term-hint">
          <span class="term-hint-key">3</span>
          <span class="t-dim">switch to daily view</span>
        </div>
      </div>
    </div>
  );
};

/** 单个功能卡片（双语）。 */
const FeatureCard = ({
  icon, tag, en, zh, delay,
}: {
  icon: string;
  tag: string;
  en: { title: string; desc: string };
  zh: { title: string; desc: string };
  delay: number;
}) => (
  <div class="feat-card" style={`animation-delay:${delay}ms`}>
    <span class="feat-tag">{tag}</span>
    <div class="feat-icon">{icon}</div>
    <div class="feat-title">
      <span data-lang="en">{en.title}</span>
      <span data-lang="zh">{zh.title}</span>
    </div>
    <p class="feat-desc">
      <span data-lang="en">{en.desc}</span>
      <span data-lang="zh">{zh.desc}</span>
    </p>
  </div>
);

/** 单个使用步骤（双语）。 */
const Step = ({
  num, en, zh, code, codeColor, keys, isLast,
}: {
  num: string;
  en: { title: string; desc: string };
  zh: { title: string; desc: string };
  code: string | null;
  codeColor: string;
  keys: readonly string[] | null;
  isLast: boolean;
}) => (
  <div class="step">
    {!isLast && <div class="step-line" />}
    <div class="step-num">{num}</div>
    <div class="step-content">
      <div class="step-title">
        <span data-lang="en">{en.title}</span>
        <span data-lang="zh">{zh.title}</span>
      </div>
      <p class="step-desc">
        <span data-lang="en">{en.desc}</span>
        <span data-lang="zh">{zh.desc}</span>
      </p>
      {code && (
        <span class="step-code" style={`color:${codeColor || 'var(--primary)'}`}>{code}</span>
      )}
      {keys && (
        <div class="step-keys">
          {keys.map((k) => <span class="step-key">{k}</span>)}
        </div>
      )}
    </div>
  </div>
);

/** 排行榜表格行。 */
const TableRow = ({ row }: { row: LeaderboardRow }) => {
  const avatarSrc = row.avatar_url.includes('?')
    ? `${row.avatar_url}&s=40`
    : `${row.avatar_url}?s=40`;
  const rankDisplay =
    row.rank === 1 ? '🥇' : row.rank === 2 ? '🥈' : row.rank === 3 ? '🥉' : `#${row.rank}`;
  return (
    <tr>
      <td><span class="rank-badge">{rankDisplay}</span></td>
      <td>
        <span class="user-inner">
          <img src={avatarSrc} alt={row.github_login} width="32" height="32" loading="lazy" />
          <a href={`/u/${row.github_login}`} class="user-link">@{row.github_login}</a>
          <a href={`https://github.com/${row.github_login}`} target="_blank" rel="noopener"
             class="gh-link" title="GitHub Profile">
            <GithubIcon />
          </a>
        </span>
      </td>
      <td><span class="cost-val">${row.total_cost_usd.toFixed(2)}</span></td>
      <td><span class="token-val">{(row.total_tokens / 1_000_000).toFixed(1)}M</span></td>
      <td>{row.device_count}</td>
    </tr>
  );
};

// ── 主组件 ────────────────────────────────────────────────────

/**
 * LeaderboardPage — 排行榜落地页根组件。
 *
 * @param rows   - 排行榜数据行
 * @param period - 当前周期（YYYY-MM）
 */
export const LeaderboardPage = ({ rows, period, defaultTab = 'about', source = 'claude' }: LeaderboardPageProps) => (
  <Layout
    title="claude-top · Claude API Usage Leaderboard"
    ogMeta={
      <>
        <meta property="og:title" content={`claude-top Global Leaderboard ${period}`} />
        <meta property="og:description" content="Monthly Claude API usage rankings — who uses Claude the most?" />
      </>
    }
    navRight={
      <div style="display:flex;align-items:center;gap:0.75rem">
        <span style="font-family:'Space Mono',monospace;font-size:0.75rem;color:var(--text-dim)">
          <span style="color:var(--green)">$</span> npx @a2d2/claude-top
        </span>
        <button id="lang-btn" onclick="toggleLang()"
          style="font-family:'Space Mono',monospace;font-size:0.75rem;color:var(--text-muted);background:none;border:1px solid var(--border);border-radius:6px;padding:0.25rem 0.6rem;cursor:pointer">
          中文
        </button>
      </div>
    }
  >
    <style dangerouslySetInnerHTML={{ __html: pageStyles }} />
    <script dangerouslySetInnerHTML={{ __html: pageScript
      .replace('__DEFAULT_TAB__', JSON.stringify(defaultTab))
      .replace('__DEFAULT_SOURCE__', JSON.stringify(source))
    }} />

    {/* ── Tab 导航栏 ── */}
    <div class="page-tabs">
      <button class="tab-btn" data-tab="leaderboard" onclick="showTab('leaderboard')">
        🏆&nbsp;
        <span data-lang="en">Leaderboard</span>
        <span data-lang="zh">排行榜</span>
      </button>
      <button class="tab-btn" data-tab="about" onclick="showTab('about')">
        📦&nbsp;
        <span data-lang="en">About</span>
        <span data-lang="zh">介绍</span>
      </button>
    </div>

    {/* ══════════════════════════════════════════════════
        Tab 1: 排行榜（默认）
    ══════════════════════════════════════════════════ */}
    <div id="tab-leaderboard" class="tab-panel">

      {/* ── 来源子 Tab 栏（Claude Code / Codex CLI / 总榜预留）── */}
      <div class="source-tabs">
        <button class="source-tab" data-source="claude" onclick="showSource('claude')">
          🤖&nbsp;Claude Code
        </button>
        <button class="source-tab" data-source="codex" onclick="showSource('codex')"
                style="--codex-accent:#10A37F">
          ✦&nbsp;Codex CLI
        </button>
        <button class="source-tab coming-soon" data-source="all" onclick="showSource('all')">
          <span data-lang="en">All&nbsp;(coming soon)</span>
          <span data-lang="zh">总榜（即将上线）</span>
        </button>
      </div>

      {/* ── Source Panel: Claude Code ── */}
      <div id="sp-claude" class="source-panel">
        <div class="lb-header">
          <span class="lb-title">
            <span data-lang="en">Claude Code Rankings</span>
            <span data-lang="zh">Claude Code 排行榜</span>
          </span>
          <span class="lb-period">{period}</span>
        </div>
        <div class="table-wrap">
          {rows.length === 0 ? (
            <div class="lb-empty">
              <span data-lang="en">
                No data yet for this period.<br />
                Install claude-top and press <span style="color:var(--amber);font-family:'Space Mono',monospace">u</span> to be the first on the board!
              </span>
              <span data-lang="zh">
                本周期暂无数据。<br />
                安装 claude-top 后按 <span style="color:var(--amber);font-family:'Space Mono',monospace">u</span> 键，成为第一个上榜的人！
              </span>
            </div>
          ) : (
            <div class="table-card">
              <table>
                <thead>
                  <tr>
                    <th style="width:4rem">
                      <span data-lang="en">Rank</span>
                      <span data-lang="zh">排名</span>
                    </th>
                    <th>
                      <span data-lang="en">User</span>
                      <span data-lang="zh">用户</span>
                    </th>
                    <th>
                      <span data-lang="en">Monthly Cost</span>
                      <span data-lang="zh">月度费用</span>
                    </th>
                    <th>Tokens</th>
                    <th>
                      <span data-lang="en">Devices</span>
                      <span data-lang="zh">设备数</span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {rows.map((row) => <TableRow key={row.github_login} row={row} />)}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>

      {/* ── Source Panel: Codex CLI（空榜占位，数据由 URL ?source=codex 服务端注入）── */}
      <div id="sp-codex" class="source-panel">
        <div class="lb-header">
          <span class="lb-title" style="color:#10A37F">
            <span data-lang="en">Codex CLI Rankings</span>
            <span data-lang="zh">Codex CLI 排行榜</span>
          </span>
          <span class="lb-period">{period}</span>
        </div>
        <div class="table-wrap">
          {source === 'codex' && rows.length > 0 ? (
            <div class="table-card">
              <table>
                <thead>
                  <tr>
                    <th style="width:4rem">
                      <span data-lang="en">Rank</span>
                      <span data-lang="zh">排名</span>
                    </th>
                    <th>
                      <span data-lang="en">User</span>
                      <span data-lang="zh">用户</span>
                    </th>
                    <th>
                      <span data-lang="en">Monthly Cost</span>
                      <span data-lang="zh">月度费用</span>
                    </th>
                    <th>Tokens</th>
                    <th>
                      <span data-lang="en">Devices</span>
                      <span data-lang="zh">设备数</span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {rows.map((row) => <TableRow key={row.github_login} row={row} />)}
                </tbody>
              </table>
            </div>
          ) : (
            <div class="lb-empty">
              <span data-lang="en">
                No Codex CLI data yet.<br />
                Run claude-top with <span style="color:var(--amber);font-family:'Space Mono',monospace">--source codex</span> and press <span style="color:var(--amber);font-family:'Space Mono',monospace">u</span> to upload!
              </span>
              <span data-lang="zh">
                暂无 Codex CLI 数据。<br />
                使用 <span style="color:var(--amber);font-family:'Space Mono',monospace">--source codex</span> 启动 claude-top，按 <span style="color:var(--amber);font-family:'Space Mono',monospace">u</span> 键上传！
              </span>
            </div>
          )}
        </div>
      </div>

      {/* ── Source Panel: 总榜（即将上线占位）── */}
      <div id="sp-all" class="source-panel">
        <div class="lb-header">
          <span class="lb-title">
            <span data-lang="en">All-Source Rankings</span>
            <span data-lang="zh">总榜</span>
          </span>
          <span class="lb-period">{period}</span>
        </div>
        <div class="table-wrap">
          <div class="lb-empty">
            <span style="font-size:2rem">🚧</span><br />
            <span data-lang="en">
              Combined leaderboard — coming soon.<br />
              <span style="color:var(--text-dim);font-size:0.8rem">Claude Code + Codex CLI usage in one ranking.</span>
            </span>
            <span data-lang="zh">
              总榜功能即将上线。<br />
              <span style="color:var(--text-dim);font-size:0.8rem">Claude Code + Codex CLI 合并排名，敬请期待。</span>
            </span>
          </div>
        </div>
      </div>

    </div>

    {/* ══════════════════════════════════════════════════
        Tab 2: 产品介绍
    ══════════════════════════════════════════════════ */}
    <div id="tab-about" class="tab-panel">

      {/* ── Hero ── */}
      <section class="hero">
        <div class="hero-glow" style="width:400px;height:400px;top:-100px;left:-50px" />
        <div>
          <div class="hero-badge">
            <span class="hero-badge-dot" />
            <span data-lang="en">Live · Global Rankings</span>
            <span data-lang="zh">实时更新 · 全球排行</span>
          </div>
          <h1>
            <span data-lang="en">
              Claude API<br />
              <span class="grad-text">Usage Leaderboard</span>
            </span>
            <span data-lang="zh">
              Claude API<br />
              <span class="grad-text">消费排行榜</span>
            </span>
          </h1>
          <p class="hero-sub">
            <span data-lang="en">
              Who uses Claude the most?<br />
              Install claude-top, track your usage in real time, and compete with developers worldwide.
            </span>
            <span data-lang="zh">
              谁在用 Claude 最猛？<br />
              安装 claude-top，查看实时用量，上传数据和全球开发者一起卷。
            </span>
          </p>
          <div class="hero-actions">
            <button class="btn-primary btn-copy"
                    onclick="copyCmd(this,'npx @a2d2/claude-top')">
              <span data-lang="en">$ npx @a2d2/claude-top</span>
              <span data-lang="zh">$ npx @a2d2/claude-top</span>
              <span class="btn-copy-hint" data-lang="en">click to copy</span>
              <span class="btn-copy-hint" data-lang="zh">点击复制</span>
            </button>
            <a class="btn-ghost" href="https://github.com/a2d2-dev/claude-top"
               target="_blank" rel="noopener">
              <GithubIcon /> GitHub
            </a>
          </div>
          <div class="hero-badges">
            <span>npm Ready</span>
            <span>CLI First</span>
            <span>Privacy First</span>
          </div>
        </div>
        <TerminalBlock />
      </section>

      {/* ── 功能特性 ── */}
      <hr class="section-divider" />
      <div class="section-bg">
        <section class="section">
          <div class="section-header">
            <div class="section-tag">
              <span data-lang="en">Features</span>
              <span data-lang="zh">功能特性</span>
            </div>
            <h2 class="section-title">
              <span data-lang="en">Track Your <span class="grad-text">Claude Usage</span></span>
              <span data-lang="zh">掌握你的 <span class="grad-text">Claude 用量</span></span>
            </h2>
            <p class="section-sub">
              <span data-lang="en">
                claude-top is a terminal TUI tool that visualizes your Claude API consumption
                and lets you voluntarily join the global leaderboard.
              </span>
              <span data-lang="zh">
                claude-top 是一个终端 TUI 工具，实时展示 Claude API 消费，并支持自愿上传参与全球排行榜。
              </span>
            </p>
          </div>
          <div class="features-grid">
            {FEATURES.map((f, i) => (
              <FeatureCard key={f.tag} {...f} delay={(i + 1) * 100} />
            ))}
          </div>
        </section>
      </div>

      {/* ── 使用步骤 ── */}
      <hr class="section-divider" />
      <section class="section">
        <div class="section-header">
          <div class="section-tag">
            <span data-lang="en">How It Works</span>
            <span data-lang="zh">使用流程</span>
          </div>
          <h2 class="section-title">
            <span data-lang="en">3 Steps to the <span class="grad-text">Leaderboard</span></span>
            <span data-lang="zh">三步<span class="grad-text">上榜</span></span>
          </h2>
          <p class="section-sub">
            <span data-lang="en">
              No account required — one command gets you started.
            </span>
            <span data-lang="zh">
              从安装到上榜，无需注册账号，一条命令搞定。
            </span>
          </p>
        </div>
        <div class="steps">
          {STEPS.map((s, i) => (
            <Step key={s.num} {...s} isLast={i === STEPS.length - 1} />
          ))}
        </div>
      </section>

      {/* ── CTA 横幅 ── */}
      <div class="cta-strip">
        <div class="cta-glow" />
        <div class="cta-inner">
          <div class="cta-badge">
            <span style="color:var(--amber)">★</span>
            <span data-lang="en">Open Source &amp; Free</span>
            <span data-lang="zh">开源免费</span>
          </div>
          <h2>
            <span data-lang="en">
              How much did you spend?<br />
              <span class="grad-text">Find out and rank up</span>
            </span>
            <span data-lang="zh">
              你用了多少？<br />
              <span class="grad-text">现在就来比一比</span>
            </span>
          </h2>
          <p>
            <span data-lang="en">
              Install claude-top, see how much you've spent this month, then press{' '}
              <span style="color:var(--amber);font-family:'Space Mono',monospace">u</span>
              {' '}to upload and see your global rank.
            </span>
            <span data-lang="zh">
              安装 claude-top，查看你这个月花了多少，然后按{' '}
              <span style="color:var(--amber);font-family:'Space Mono',monospace">u</span>
              {' '}键上传，看看自己排第几。
            </span>
          </p>
          <div class="cta-btns">
            <button class="btn-primary btn-copy"
                    onclick="copyCmd(this,'npx @a2d2/claude-top')">
              $ npx @a2d2/claude-top
              <span class="btn-copy-hint">click to copy</span>
            </button>
            <a class="btn-ghost" href="https://github.com/a2d2-dev/claude-top"
               target="_blank" rel="noopener">
              <GithubIcon /> GitHub
            </a>
          </div>
          <div class="cta-tools">
            <div data-lang="en">Compatible with</div>
            <div data-lang="zh">兼容以下平台</div>
            <div class="cta-tools-row">
              <span>macOS</span><span>Linux</span><span>Windows</span><span>Node.js</span>
            </div>
          </div>
        </div>
      </div>

    </div>{/* end tab-about */}
  </Layout>
);
