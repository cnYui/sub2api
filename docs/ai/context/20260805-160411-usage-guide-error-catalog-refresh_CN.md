# 使用方法错误编号参考更新

## 背景

旧“错误编号参考”栏目把 `S2A-xxxx` 目录和 `X-Sub2API-Error-ID` 响应头描述成当前稳定契约，但当前 `main` 分支没有统一的 error contract 实现。后端实际仍按端点输出通用响应、OpenAI/Anthropic 兼容错误或 Gemini Google 风格错误。

## 实现决策

- 通用 REST 错误以 `{code, message, reason, metadata}` 解释；`code` 是 HTTP 状态，`reason` 是端点/业务错误代码，可能为空。
- OpenAI/Claude 兼容端点按 `invalid_request_error`、`rate_limit_error`、`insufficient_quota`、`upstream_error`、`content_policy_violation` 等当前代码实际使用的协议字段说明。
- API Key 鉴权与计费错误按当前中间件实际返回的 `API_KEY_REQUIRED`、`INVALID_API_KEY`、`API_KEY_EXPIRED`、`INSUFFICIENT_BALANCE`、`USAGE_LIMIT_EXCEEDED` 等列出；不再把不存在于当前 main 的 S2A 编号当作用户应复制的错误码。
- 保留 S2A 术语作为迁移提示，明确当前 main 尚未全量统一到 `X-Sub2API-Error-ID` / `S2A-*`，避免客户端提前绑定未落地的字段。
- 本栏目更新时间改为 2026-08-05，并继续参与使用方法页按日期倒序排序。

## 变更范围

- 页面：`frontend/src/views/user/UsageGuideView.vue`
- 测试：`frontend/src/views/user/__tests__/UsageGuideView.spec.ts`
- 项目记忆：`AGENTS.md`

## 验证口径

- 使用方法页单测通过。
- 页面内容不再把 S2A 目录描述为当前稳定响应头契约。
- 未启动公网服务、未重建应用容器、未修改数据库或运行态配置。
