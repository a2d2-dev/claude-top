/**
 * Layout.tsx — 共享页面骨架，包含 <head>、导航栏和 footer。
 *
 * 所有插入 JSX 的变量都由 Hono 运行时自动 HTML 转义，
 * 不需要手动调用 escapeHtml。
 */

import type { Child } from 'hono/jsx';

/** Layout 组件 props */
interface LayoutProps {
  title: string;
  /** 可选 OG / Twitter 等额外 meta 标签 */
  ogMeta?: Child;
  /** 导航栏右侧内容（徽章、安装命令等） */
  navRight?: Child;
  children: Child;
}

/** 共享字体与 CSS 变量（a2d2.lovable.app 设计系统移植）。 */
const sharedStyles = `
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

:root {
  /* ── 颜色 ── */
  --bg:          hsl(222 47% 11%);
  --bg-hero:     linear-gradient(135deg, hsl(220 20% 6%) 0%, hsl(220 25% 12%) 50%, hsl(220 20% 6%) 100%);
  --bg-card:     hsl(217 32% 17%);
  --bg-card-hover: hsl(217 32% 20%);
  --primary:     hsl(198 93% 59%);
  --primary-dim: hsl(198 93% 59% / 0.7);
  --primary-10:  hsl(198 93% 59% / 0.1);
  --primary-20:  hsl(198 93% 59% / 0.2);
  --primary-30:  hsl(198 93% 59% / 0.3);
  --border:      hsl(215 19% 34%);
  --border-glow: hsl(198 93% 59% / 0.15);
  --green:       hsl(142 70% 50%);
  --amber:       hsl(38 92% 50%);
  --text:        hsl(210 40% 98%);
  --text-muted:  hsl(215 20% 65%);
  --text-dim:    hsl(215 16% 46%);
  --nav-bg:      hsl(222 47% 11% / 0.85);
  /* ── 发光效果 ── */
  --glow:        0 0 40px hsl(187 100% 42% / 0.3);
  --glow-card:   0 0 20px hsl(198 93% 59% / 0.08);
  /* ── 渐变文字 ── */
  --grad-accent: linear-gradient(135deg, hsl(187 100% 42%) 0%, hsl(200 100% 50%) 100%);
}

html { scroll-behavior: smooth; }

body {
  background: var(--bg);
  color: var(--text);
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
  min-height: 100vh;
}

/* 背景网格纹理（来自 a2d2 grid-pattern） */
body::before {
  content: '';
  position: fixed;
  inset: 0;
  background-image:
    linear-gradient(hsl(215 19% 34% / 0.25) 1px, transparent 1px),
    linear-gradient(90deg, hsl(215 19% 34% / 0.25) 1px, transparent 1px);
  background-size: 60px 60px;
  pointer-events: none;
  z-index: 0;
}

/* ── 导航 ── */
nav {
  position: sticky; top: 0; z-index: 50;
  background: var(--nav-bg);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border-bottom: 1px solid var(--border);
  padding: 0 2rem; height: 56px;
  display: flex; align-items: center; justify-content: space-between;
}
.nav-logo {
  display: flex; align-items: center; gap: 0.5rem;
  font-family: 'Space Mono', monospace;
  font-size: 0.95rem; font-weight: 700;
  color: var(--primary); text-decoration: none;
}
.nav-logo-icon {
  width: 28px; height: 28px;
  background: var(--primary-10);
  border: 1px solid var(--primary-30);
  border-radius: 6px;
  display: flex; align-items: center; justify-content: center;
  font-size: 0.8rem;
}

/* ── 公共按钮 ── */
.btn-primary {
  display: inline-flex; align-items: center; gap: 0.4rem;
  background: var(--primary);
  color: hsl(204 80% 15%);
  font-weight: 700; font-size: 0.875rem;
  padding: 0.6rem 1.4rem; border-radius: 8px;
  text-decoration: none; transition: opacity 0.15s;
  box-shadow: var(--glow);
}
.btn-primary:hover { opacity: 0.85; }
.btn-ghost {
  display: inline-flex; align-items: center; gap: 0.4rem;
  border: 1px solid var(--border);
  color: var(--text-muted); font-size: 0.875rem;
  padding: 0.6rem 1.25rem; border-radius: 8px;
  text-decoration: none; transition: border-color 0.15s, color 0.15s;
}
.btn-ghost:hover { border-color: var(--primary-30); color: var(--text); }

/* ── 渐变文字 ── */
.grad-text {
  background-image: var(--grad-accent);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

/* ── Footer ── */
footer {
  position: relative; z-index: 1;
  text-align: center; color: var(--text-dim);
  font-size: 0.75rem; padding: 2rem 1rem;
  border-top: 1px solid var(--border);
  font-family: 'Space Mono', monospace;
}
footer a { color: inherit; }

/* ── 入场动画 ── */
@keyframes fade-in-up {
  from { opacity: 0; transform: translateY(20px); }
  to   { opacity: 1; transform: translateY(0); }
}
.fade-in { animation: fade-in-up 0.5s ease-out forwards; }
.fade-in-1 { animation-delay: 100ms; opacity: 0; }
.fade-in-2 { animation-delay: 200ms; opacity: 0; }
.fade-in-3 { animation-delay: 300ms; opacity: 0; }
.fade-in-4 { animation-delay: 400ms; opacity: 0; }
`;

/** 共享 GitHub SVG 图标 */
export const GithubIcon = () => (
  <svg width="15" height="15" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
    <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
  </svg>
);

/**
 * 页面根 Layout：注入字体、CSS 变量、导航栏、footer。
 *
 * @param title      - <title> 文本
 * @param ogMeta     - 可选的 OG/Twitter meta 标签
 * @param navRight   - 导航右侧内容（徽章、安装命令等）
 * @param children   - 页面主体内容
 */
export const Layout = ({ title, ogMeta, navRight, children }: LayoutProps) => (
  <html lang="zh">
    <head>
      <meta charset="UTF-8" />
      <meta name="viewport" content="width=device-width, initial-scale=1" />
      <title>{title}</title>
      {ogMeta}
      <link rel="preconnect" href="https://fonts.googleapis.com" />
      <link
        href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=Space+Mono:wght@400;700&display=swap"
        rel="stylesheet"
      />
      <style dangerouslySetInnerHTML={{ __html: sharedStyles }} />
      {/* Cloudflare Web Analytics */}
      <script
        defer
        src="https://static.cloudflareinsights.com/beacon.min.js"
        data-cf-beacon='{"token": "19c63d74703a4d69aa5642a289748139"}'
      />
    </head>
    <body>
      <nav>
        <a class="nav-logo" href="/">
          <span class="nav-logo-icon">&gt;_</span>
          claude-top
        </a>
        {navRight}
      </nav>
      {children}
      <footer>
        claude-top ·{' '}
        <span data-lang="en">Data uploaded voluntarily · Aggregated stats only ·{' '}</span>
        <span data-lang="zh">数据由用户自愿上传 · 仅含聚合统计 ·{' '}</span>
        <a href="https://github.com/a2d2-dev/claude-top" target="_blank" rel="noopener">
          GitHub
        </a>
      </footer>
    </body>
  </html>
);

