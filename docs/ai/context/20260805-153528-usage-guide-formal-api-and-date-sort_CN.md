# 使用方法规范 API 与栏目日期排序

## 背景

当前“规范使用”栏目仍沿用 2026-07 的旧说明，把无 `/v1` 路径统一描述为 `400 INVALID_BASE_URL`。核对当前后端 `backend/internal/server/routes/gateway.go` 与 `backend/internal/handler/endpoint.go` 后，确认网关对部分无版本路径保留了兼容注册，旧文案会误导用户排查。

## 实现决策

- OpenAI、Claude 及兼容客户端统一推荐 `https://api.aaccx.pw/v1`；Gemini 原生客户端使用 `https://api.aaccx.pw/v1beta`。
- 认证说明以当前中间件为准：首选 `Authorization: Bearer <API Key>`，兼容 `x-api-key`，Gemini 兼容 `x-goog-api-key`；不再展示不存在的 `INVALID_BASE_URL` 迁移结论。
- 规范接口表补充 Responses compact、Claude Messages、Count Tokens、Alpha Search 和 Grok 视频，并强调 `/v1/models` 是当前 API Key 可用模型的事实来源。
- 无 `/v1` 路径表改为“兼容入口 / 当前行为 / 建议配置”：明确 Responses、Chat Completions、Embeddings、图片、Models 和 Codex 直连的兼容别名，同时指出 `/messages`、`/usage` 等未注册裸路径不能省略版本前缀。
- 在所有主题数据上增加 `updatedAt`，可见主题由 `guideTopics` 按日期倒序排序；导航、移动标签和当前栏目标题均显示更新时间。已下架的“生图方法”保留日期和源码数据，但继续隐藏。

## 变更范围

- 页面：`frontend/src/views/user/UsageGuideView.vue`
- 测试：`frontend/src/views/user/__tests__/UsageGuideView.spec.ts`
- 项目记忆：`AGENTS.md`

## 验证口径

- 页面源码不再包含旧的 `400 INVALID_BASE_URL` 结论。
- 每个主题均有日期，排序表达式按 `updatedAt` 从新到旧排列。
- 前端单测、类型检查、Lint 和生产构建需通过；启动本地 Vite 服务后检查 `/usage-guide` 返回 200。
