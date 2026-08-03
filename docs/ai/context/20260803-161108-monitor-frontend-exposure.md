# /monitor 前端展示验证

时间：2026-08-03 16:11:08（Asia/Tokyo）

## 背景

用户要求在前端打开并展示渠道状态页面 `/monitor`。当前工作区已包含用户端渠道状态视图、路由和侧边栏入口，因此本次不重复改写现有实现。

## 现状与边界

- 路由 `frontend/src/router/index.ts` 已注册 `/monitor`，页面组件为 `ChannelStatusView.vue`。
- 登录后的侧边栏已提供“渠道状态”入口；公开设置 `channel_monitor_enabled` 为 `true` 时默认展示。
- 渠道状态接口 `/api/v1/channel-monitors` 受登录鉴权保护。保持该边界，避免将渠道、分组和模型状态在未授权情况下公开。

## 本地展示

- 前端开发服务：`http://localhost:3001/monitor`
- 代理后端：`http://127.0.0.1:18082`
- 未登录访问接口返回 `401`，登录后页面会加载渠道状态数据。

## 验证

- `pnpm typecheck` 通过。
- `/monitor` 返回 `200`。
- `/api/v1/settings/public` 通过 Vite 代理返回当前后端公开设置。
