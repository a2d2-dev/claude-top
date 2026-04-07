---
id: 2-1
title: 上传确认 UI
status: ready-for-dev
epic: 2
---

# Story 2.1 + 2.2: 上传功能（数据聚合 + 确认 UI）

As a 认证用户,
I want 按 `u` 后看到上传确认框（含数据预览），确认后上传当月数据，
So that 我在上传前清楚知道将分享的内容，并在上传成功后看到排名和分享链接.

## Acceptance Criteria

**AC1: 已认证用户按 u 看到确认框**
Given 用户已认证（JWT 有效）
When 按 `u` 键
Then 显示确认框：当前周期 YYYY-MM / 设备名 / 总费用 / token 数 / session 数
And 按 Enter 确认上传，ESC 取消

**AC2: 上传中状态**
Given 用户按 Enter 确认
When 上传进行中
Then 显示"正在上传…"状态

**AC3: 上传成功**
Given 上传 API 返回成功
When 收到响应
Then 显示：全球排名 #N / 分享链接 claude-top.a2d2.dev/u/{login}
And 按 Enter/ESC 关闭

**AC4: 上传失败**
Given 上传 API 失败（网络/401 等）
When 收到错误
Then 显示具体错误信息，可 ESC 关闭，核心 TUI 继续正常运行

**AC5: 数据聚合正确**
Given 当月有 N 个 sessions
When 准备 payload
Then payload 包含当月合计：cost / input_tokens / output_tokens / cache_read / cache_write / session_count / model_breakdown
And 不含 prompt 文本或文件路径

## Tasks

- [x] internal/auth/ 包（已完成 Story 1.1/1.2）
- [ ] internal/upload/aggregator.go：从 SessionBlocks 计算月度聚合
- [ ] internal/upload/client.go：POST /api/upload HTTP 调用
- [ ] 新增 uploadState + uploadPhase 到 Model
- [ ] 认证成功后自动进入 viewUploadConfirm
- [ ] 渲染确认框 / 上传中 / 成功 / 失败面板
