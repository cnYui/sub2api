# 2026-07-07 代码冗余治理 P0/P1/P2/P3 设计

## 来源

- 输入审查：`docs/ai/context/20260707-090748-code-redundancy-review_CN.md`
- 本文目标：把静态冗余审查整理成可拆分、可验证、可回滚的治理设计。
- 本文范围：只写设计，不改业务代码，不连接运行态服务，不重启或替换公网容器。

## 约束

- Sub2API 仍是唯一公网 API、用户 Key、计费和用量事实源。
- 不改变当前公网链路：`Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> sub2api-candidate 127.0.0.1:18084 -> CLIProxyAPI 127.0.0.1:8317`。
- 不把完整 API Key、内部 token、HMAC secret、SMTP 密码写入文档、日志或提交。
- 已发布 migration 不能为了“变整洁”回写修改；只能新增 migration 或新增生成工具。
- 重构优先选择行为保持型、小步提交；凡是改变用户可见行为的地方必须先写回归测试。

## 优先级定义

- P0：会让新增核心能力语义不稳定，或让用户在正常入口得到错误行为的问题。必须最先收口。
- P1：不一定立即造成线上故障，但一旦继续堆叠会放大安全、计费、发布风险的问题。
- P2：重复明显、收益高、可以在不改主链路语义的前提下逐步抽取的问题。
- P3：结构性技术债，适合在高优先级收口后分批治理，不应阻塞近期业务修复。

## 总体方案

推荐采用“风险优先、按领域切片”的方案，不做一次性大重构。

备选方案：

1. 一次性大拆所有重复代码：短期看似彻底，但会同时触碰认证、计费、路由、前端、部署脚本，回归面太大，不采用。
2. 只补文档不改代码：能降低认知成本，但不能解决自动 Key、认证漂移、窗口语义分叉这类会复发的问题，不采用。
3. 按 P0/P1/P2/P3 分层治理：先收口产品语义和事实源，再抽重复模板，最后做结构拆分。推荐采用。

横向原则：

- 产品语义先显式化，再抽公共代码。
- 协议差异只能留在边界层，例如错误响应 writer、Key 提取方式、协议专属路由。
- 计费事实源只能在 Sub2API 的服务层收口，不能让 handler、middleware、前端各自推导。
- 迁移、部署和公网操作必须可审计、可 dry-run、可回滚。

## P0：自动 API Key 语义和路由边界

### 问题

当前 `group_id = NULL` 同时表达“自动 Key”和“未分组 Key”。`ResolveEffectiveGroup` 只挂在 `/v1`、裸 `/responses`、裸 `/chat/completions`、裸 `/embeddings`、裸 `/images/*`、`/backend-api/codex` 等入口；`/v1beta` 和 `/antigravity/v1beta` 仍走 Google 认证与 `RequireGroupAssignment`。这会让同一把自动 Key 在不同入口表现为不同产品语义。

相关文件：

- `backend/internal/service/effective_group_resolver.go`
- `backend/internal/server/middleware/effective_group.go`
- `backend/internal/server/routes/gateway.go`
- `backend/internal/server/middleware/middleware.go`
- `backend/internal/handler/api_key_handler.go`
- `frontend/src/api/keys.ts`
- `frontend/src/views/user/KeysView.vue`
- `backend/migrations/159_auto_api_key_effective_group.sql`

### 设计目标

- 普通用户新建 Key 的产品语义明确为“自动选择 OpenAI 可用权益”，而不是传统未分组 Key。
- 固定分组 Key 继续保留，管理员能力不降级。
- 未支持自动解析的协议入口要返回明确错误，不能落入旧的“未分组 Key”错误。
- 新增协议入口时，必须显式选择是否支持自动 Key。

### 设计

1. 建立路由策略表

   在路由或中间件层新增单一策略函数，表达入口是否支持自动 Key：

   - OpenAI/Claude 兼容入口：`/v1/messages`、`/v1/responses`、`/v1/chat/completions`、`/v1/embeddings`、`/v1/images/*`、裸 `/responses`、裸 `/chat/completions`、裸 `/embeddings`、裸 `/images/*`、`/backend-api/codex/*`。
   - 强制 antigravity 入口：不走 OpenAI 自动 Key。
   - Gemini 原生 `/v1beta`：默认不支持自动 Key，除非后续明确设计 Gemini/Antigravity 流量包权益。

   当前 `inferEffectiveGroupPlatform(path)` 的字符串包含判断只适合临时实现，后续应由路由策略显式传入 platform 和支持状态。

2. 自动 Key 未支持入口给明确错误

   对 `group_id = NULL` 的 Key：

   - 若当前路由支持自动 Key，执行 effective group resolver。
   - 若当前路由不支持自动 Key，返回 `AUTO_KEY_UNSUPPORTED_ENDPOINT` 或等价的协议化错误消息。
   - 不再让请求继续进入 `RequireGroupAssignment` 的传统未分组分支。

3. 短期显式化 `binding_mode`

   先在 service/API response 层派生：

   - `fixed`：`group_id != NULL`
   - `automatic`：普通用户 `group_id == NULL`

   前端用户 Key 创建接口移除 `groupId` 参数；管理员固定分组接口保留。这样可以先降低 API 层歧义，不急于加表字段。

4. 中期物理建模

   若后续出现多种自动模式，再新增 DB 字段 `binding_mode = automatic | fixed`。新增 migration 只能向前兼容：已有 `group_id != NULL` 为 `fixed`，已有普通用户 `group_id IS NULL` 为 `automatic`，管理员历史未分组数据需要按实际用途单独处理。

### 验收标准

- 自动 Key 请求 OpenAI 兼容入口时，能按 active OpenAI 套餐优先、GPT/OpenAI 流量包兜底解析 effective group。
- 自动 Key 请求 `/v1beta` 或 `/antigravity/v1beta` 时，得到明确“不支持该入口”的协议化错误。
- 固定分组 Key 行为不变。
- 前端普通用户创建 Key 不再提交 `group_id`。
- 单测覆盖 route policy、自动 Key 成功、自动 Key 不支持入口、固定 Key 跳过 resolver。

## P1：API Key 认证核心去重

### 问题

`api_key_auth.go` 与 `api_key_auth_google.go` 复制了 API Key 提取、Key 状态、用户状态、订阅加载等流程，但行为已经漂移。Google 版本缺少 IP ACL，`skipBilling`、Key 过期/额度耗尽、订阅兜底等细节也不完全一致。

相关文件：

- `backend/internal/server/middleware/api_key_auth.go`
- `backend/internal/server/middleware/api_key_auth_google.go`
- `backend/internal/server/middleware/api_key_auth_test.go`
- `backend/internal/server/middleware/api_key_auth_google_test.go`

### 设计目标

- 身份认证、安全限制、Key 状态和订阅加载只实现一套。
- Anthropic/OpenAI 风格与 Google 风格只保留错误响应格式和 Key 提取策略差异。
- 后续计费兜底或安全规则修复只改一处。

### 设计

1. 抽 `authenticateAPIKeyCore`

   核心函数负责：

   - 禁止或允许 query key 的策略判断。
   - 从已抽取的 key string 加载 API Key。
   - 写入 Ops fallback API Key。
   - 校验 Key disabled、用户存在、用户状态、IP ACL、group 可用性、group 允许策略。
   - SimpleMode 上下文写入。
   - 订阅型 group 的 active subscription 加载。
   - `/v1/usage` 这类只鉴权不计费的路径判断。

2. 抽协议配置

   中间件只传入配置：

   - `ExtractAPIKey(c)`：普通入口支持 Bearer、`x-api-key`、`x-goog-api-key`，Google 入口优先 `x-goog-api-key` 并允许特定 `/v1beta` query `key`。
   - `WriteError(c, AuthError)`：普通错误 writer 与 Google 错误 writer。
   - `SkipBilling(path)`：先保留 `/v1/usage` 规则，后续若有新只读用量入口统一加这里。

3. 保持计费职责边界

   认证核心不判断订阅额度是否足够，不扣余额，不扣流量卡。额度、余额、流量卡兜底仍由 `BillingCacheService.CheckBillingEligibility()` 统一决定。

### 验收标准

- 普通入口与 Google 入口都执行 IP ACL。
- 订阅不存在但有 GPT/OpenAI 流量卡的路径不会被认证层硬拒绝。
- `/v1/usage` 保持只鉴权不计费。
- 现有 Google 错误 JSON 格式保持兼容。
- 单测覆盖普通/Google 两种 writer、query key 策略、IP ACL、订阅不存在兜底、Key expired/quota_exhausted。

## P1：订阅额度窗口统一

### 问题

订阅窗口规则分散在 `BillingCacheService`、`UserSubscription`、`SubscriptionService`、repository SQL、quota view helper。日窗口同时存在自然日刷新和 `DailyWindowStart + 24h` 刷新两种口径，展示、Retry-After、实际扣费入口容易不一致。

相关文件：

- `backend/internal/service/billing_cache_service.go`
- `backend/internal/service/user_subscription.go`
- `backend/internal/service/subscription_service.go`
- `backend/internal/repository/user_subscription_repo.go`
- `backend/internal/handler/quotaview/helpers.go`

### 设计目标

- 订阅型套餐统一使用全局时区自然日、自然周、30 天滚动月窗口。
- “一天有效期日卡不刷新日额度”保留为显式特殊规则，不再藏在实体方法里。
- 展示、Retry-After、DB 刷新、列表归一化复用同一套窗口策略。

### 设计

1. 新增窗口策略包

   建议新增 `backend/internal/service/usagewindow`，只放纯函数：

   - `CurrentDailyStart(now)`
   - `CurrentWeeklyStart(now)`
   - `CurrentMonthlyStart(existingStart, now)`
   - `DailyExpired(start, now, policy)`
   - `WeeklyExpired(start, now)`
   - `MonthlyExpired(start, now)`
   - `NextDailyReset(start, now, policy)`
   - `NextWeeklyReset(now)`
   - `NextMonthlyReset(start, now)`

2. 明确 nil 语义

   - `nil window` 表示窗口尚未激活或旧缓存缺字段。
   - 计费入口遇到旧缓存缺字段时回源 DB。
   - 展示层遇到 nil 不自行推断历史过期，只展示当前可理解的窗口状态和下次刷新时间。

3. 替换调用点

   - `BillingCacheService.refreshExpiredSubscriptionWindowsIfNeeded` 用策略包判断是否刷新。
   - `UserSubscription.NeedsDailyResetAt/NeedsWeeklyReset/NeedsMonthlyReset` 改为薄封装或逐步下线。
   - `SubscriptionService.normalizeExpiredWindows` 和 `CheckAndResetWindows` 复用策略包。
   - `quotaview/helpers.go` 删除重复窗口判断，复用策略包。
   - `user_subscription_repo.RefreshExpiredUsageWindows` 的 SQL 参数仍由 service 层策略生成。

### 验收标准

- 同一订阅在计费入口、管理列表、quota view 上对 daily/weekly/monthly 是否过期给出同一结论。
- stale 日窗口不会因为旧缓存复发。
- 日卡跨自然日不刷新日额度，多日套餐按自然日刷新。
- 单测覆盖自然日、自然周、30 天月窗口、nil window、日卡特殊规则、Retry-After。

## P1：部署脚本目标收口

### 问题

审查文件把部署脚本列为 P3，但当前历史上下文里 18080/18084 多次切换，且曾出现候选预演误停公网容器的事故。`promote-sub2api-candidate.sh` 仍含旧 `sub2api`、`18080`、`weishaw/sub2api:latest` 假设，这不只是整洁问题，而是生产操作风险。

相关文件：

- `deploy/restart-sub2api.sh`
- `deploy/redeploy-sub2api-image.sh`
- `deploy/rehearse-sub2api-candidate.sh`
- `deploy/promote-sub2api-candidate.sh`

### 设计目标

- 当前公网标准目标只有一个：`sub2api-candidate` / `127.0.0.1:18084` / 保留候选 Postgres 与 Redis。
- 旧 18080 路径必须显式标为 legacy/emergency，不能作为默认发布目标。
- 所有脚本复用同一套日志、命令执行、Docker 路径、health check、target 展示逻辑。

### 设计

1. 抽公共库

   新增 `deploy/lib/common.sh`，集中：

   - `log/warn/die`
   - `run_cmd/format_cmd`
   - Docker CLI 解析
   - 仓库根目录解析
   - HTTP health check
   - dry-run 与确认提示

2. 抽 target 配置

   新增 target 文件或 shell 变量入口：

   - `public_candidate_18084`
   - `legacy_18080`

   默认 target 必须是 `public_candidate_18084`。使用 legacy target 时脚本必须打印强提示，并要求显式环境变量确认。

3. 所有破坏性操作先展示 target 摘要

   摘要至少包含：

   - app container
   - postgres container
   - redis container
   - published port
   - health URL
   - image tag
   - 是否会重建 DB/Redis

### 验收标准

- `bash -n deploy/*.sh deploy/lib/*.sh` 通过。
- dry-run 输出清楚展示当前 target。
- 默认脚本不会指向 18080。
- legacy 路径没有显式确认时拒绝执行。

## P2：Handler 计费准入模板去重

### 问题

多个 gateway handler 重复执行 `GetSubscriptionFromContext -> SetOpsLatencyMs -> AcquireUserSlot -> CheckBillingEligibility -> billingErrorDetails -> Retry-After -> 协议错误响应`。这些流程顺序应该一致，但协议错误格式不同。

相关文件：

- `backend/internal/handler/gateway_handler.go`
- `backend/internal/handler/gateway_handler_chat_completions.go`
- `backend/internal/handler/gateway_handler_responses.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/handler/openai_chat_completions.go`
- `backend/internal/handler/openai_embeddings.go`
- `backend/internal/handler/openai_images.go`
- `backend/internal/handler/gemini_v1beta_handler.go`

### 设计

新增小 helper，不改 handler 主流程：

- `prepareGatewayRequestContext`：读取 API Key、user、group、subscription、request id、ops metadata。
- `acquireUserAndCheckBilling`：封装用户并发槽和计费准入，返回 release 函数和结构化错误。
- 协议响应由 callback 注入，例如 Anthropic/OpenAI/Gemini writer。

先抽最重复的 Responses 和 Chat Completions，再扩到 Embeddings/Images/Gemini。

### 验收标准

- 抽取后 handler 仍保留清晰的协议主流程。
- 计费拒绝、Retry-After、Ops 标记在 OpenAI/Anthropic/Gemini 路径保持原行为。
- 单测至少覆盖一个成功路径、一个额度拒绝路径、一个并发拒绝路径。

## P2：OpenAI-only 路由判断去重

### 问题

`/embeddings`、`/images/generations`、`/images/edits` 在 `/v1` 与裸路径重复检查 platform 并重复写 404 JSON。

相关文件：

- `backend/internal/server/routes/gateway.go`

### 设计

抽 `openAIOnly(featureName, handler)` 或 `requireOpenAIPlatform(c, featureName)`：

- 统一读取 `getGroupPlatform(c)`。
- 非 OpenAI 时统一写 not found 响应和 Ops 标记。
- 支持不同 feature message：Embeddings、Images。

### 验收标准

- `/v1/embeddings` 与裸 `/embeddings` 非 OpenAI 返回一致。
- `/v1/images/generations`、裸 `/images/generations`、`/images/edits` 非 OpenAI 返回一致。
- 新增 OpenAI-only 端点时只需声明 feature 和 handler。

## P2：前端账号测试/统计弹窗去重

### 问题

用户侧和管理侧 `AccountTestModal`、`AccountStatsModal` 高度同构，差异是 test mode、事件状态、请求 body、是否展示 `userBilled` 等。副本中已经出现格式变形，维护成本外显。

相关文件：

- `frontend/src/components/account/AccountTestModal.vue`
- `frontend/src/components/admin/account/AccountTestModal.vue`
- `frontend/src/components/account/AccountStatsModal.vue`
- `frontend/src/components/admin/account/AccountStatsModal.vue`

### 设计

1. 抽 `useAccountTestStream`

   统一：

   - fetch/SSE 流读取
   - 测试状态机
   - 模型加载状态
   - 图片预览状态
   - 错误归一化

   差异通过参数传入：endpoint、request body builder、event adapter、是否支持 OpenAI test mode。

2. 抽 `AccountStatsContent`

   统一统计展示结构，通过 props 控制：

   - 是否展示用户计费行
   - 图标或标题样式
   - 管理端额外字段

### 验收标准

- 用户侧和管理侧弹窗交互保持一致。
- 管理端特有 OpenAI test mode 不丢失。
- 组件测试覆盖 stream 成功、stream 错误、图片预览、`userBilled` 开关。

## P3：SettingsView 分域拆分

### 问题

`frontend/src/views/admin/SettingsView.vue` 约一万行，一个 SFC 同时维护安全、注册、OAuth、WeChat、支付 provider、Affiliate、网关、多段策略配置。继续在此文件上堆功能会放大冲突和回归成本。

### 设计

分批拆，不做一次性搬家：

1. 第一批拆支付 provider 子域：`PaymentProviderSettingsPanel`
2. 第二批拆 Affiliate 子域：`AffiliateSettingsPanel`
3. 同步抽 `useOAuthRedirectUrls`
4. 抽 `buildSettingsPayload` 或 `useSettingsSavePayload`

每次只移动一个子域，移动前后用测试锁住保存 payload 和关键交互。

### 验收标准

- `SettingsView.vue` 行数逐步下降，且每次拆分后行为不变。
- 支付 provider 与 Affiliate 的接口调用、分页、弹窗状态保持现有行为。
- 保存设置 payload 与拆分前一致。

## P3：Gateway service 工具层收口

### 问题

`gateway_service.go`、`openai_gateway_service.go`、`antigravity_gateway_service.go`、`openai_embeddings.go` 里存在响应头过滤、Content-Type 回写、上游错误提取、SSE terminal event/usage incomplete 判定重复。

相关文件：

- `backend/internal/service/gateway_service.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/antigravity_gateway_service.go`
- `backend/internal/service/openai_embeddings.go`

### 设计

只抽无争议工具，不碰账号调度、failover、计费主链路：

- passthrough response header writer
- upstream error message/code extractor
- SSE terminal/usage 判定工具

这些工具应是纯函数或小接口，先在一个 service 中替换并测试，再扩到其他 service。

### 验收标准

- 响应头透传规则一致。
- 上游错误消息提取一致。
- SSE 缺 terminal event 的判定和日志保持当前语义。

## P3：Settings schema/DTO 收口

### 问题

settings 字段在后端 service 解析、handler DTO、前端 type、保存 payload 中手工维护。新增字段容易只改部分位置，导致读写不对称或 secret 处理遗漏。

相关文件：

- `backend/internal/service/setting_service.go`
- `backend/internal/handler/admin/setting_handler.go`
- `frontend/src/api/admin/settings.ts`
- `frontend/src/views/admin/SettingsView.vue`

### 设计

短期：

- 先把前端 payload builder 从 `SettingsView.vue` 拆出，并按子域分组。
- 给每个子域写 payload snapshot 或字段级测试。

中期：

- 引入 settings schema registry，集中声明 key、默认值、读写可见性、secret 处理、前端展示分组。
- handler DTO 和前端类型可以逐步从 schema 生成或至少共享字段清单。

### 验收标准

- 新增 settings 字段时有单一入口提醒必须配置默认值、读写策略和 secret 策略。
- 前端保存 payload 不再散落在大 SFC 内。

## P3：套餐 seed migration 模板化

### 问题

新增套餐 migration 复制大量 group/subscription plan 字段，历史上已经出现 156/157 这类 seed 修正。migration 不可回写，不能整理历史文件，但可以改后续生成方式。

相关文件：

- `backend/migrations/154_seed_codex_99_subscription_plan.sql`
- `backend/migrations/155_seed_codex_subscription_plans_baseline.sql`
- `backend/migrations/156_seed_codex_79_subscription_plan.sql`
- `backend/migrations/157_fix_codex_79_subscription_plan_base_price.sql`
- `backend/migrations/159_auto_api_key_effective_group.sql`

### 设计

- 不改已发布 migration。
- 新增套餐时使用脚本或 SQL 模板生成 migration。
- 模板输入只保留套餐名、售价、group name、日额度、有效期、是否售卖、是否支持图片等必要参数。
- 生成结果仍提交为普通 SQL migration，避免运行态依赖生成器。

### 验收标准

- 新套餐 migration 由模板生成，减少手抄字段。
- 生成脚本有 dry-run 输出。
- migration 测试覆盖新套餐 group、subscription plan、account group 绑定。

## 建议执行顺序

1. P0 自动 Key 路由策略和错误语义。
2. P1 API Key 认证核心去重。
3. P1 usage window policy 统一。
4. P1 部署脚本 target 收口。
5. P2 handler 计费准入 helper 与 OpenAI-only route helper。
6. P2 前端账号弹窗 composable/content 抽取。
7. P3 SettingsView 分域拆分。
8. P3 gateway service 工具层、settings schema、套餐 migration 模板。

## 验证矩阵

后端：

- `go test -count=1 -tags=unit ./internal/server/middleware`
- `go test -count=1 -tags=unit ./internal/server/routes`
- `go test -count=1 -tags=unit ./internal/service ./internal/repository`
- `go test -count=1 -tags=unit ./internal/handler/...`

前端：

- `cd frontend && pnpm test --run`
- 对拆分组件补最小交互测试，避免只做 snapshot。

部署脚本：

- `bash -n deploy/*.sh deploy/lib/*.sh`
- dry-run 验证默认 target 是 `public_candidate_18084`。

公网验收：

- 本设计阶段不做公网操作。
- 后续若实现并发布，必须另写发布计划，先备份 Postgres/Redis，再只替换应用容器。

## 风险与回滚

- P0 若加入 DB `binding_mode` 字段，需要单独 migration 和发布前 DB 备份；短期建议先用派生字段降低风险。
- P1 认证核心抽取要先做行为锁定测试，再删重复分支。
- P1 窗口策略会影响扣费、展示、Retry-After，必须按自然日、自然周、30 天窗口写完整测试。
- 部署脚本重构不应在同一批次执行真实发布；先通过 dry-run 和语法测试。
- P2/P3 主要是行为保持型重构，每批只改一个领域，失败时可以直接 revert 对应提交。

## 自查

- 无未完成占位项。
- 未写入任何密钥、token、SMTP 密码或完整用户 API Key。
- 已按 P0/P1/P2/P3 覆盖原审查文件全部 11 类问题，并将部署脚本旧拓扑风险从 P3 提升为 P1。
- 本文是设计文档，不是实现计划；进入实现前还需要按具体批次另写 implementation plan。
