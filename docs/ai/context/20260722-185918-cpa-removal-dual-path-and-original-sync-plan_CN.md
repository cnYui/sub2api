# CPA 临时去除双方案与 original 同步计划

## 当前结论

- 本轮只做只读调查与方案准备，未修改运行态、未合并 upstream 代码、未改数据库。
- 当前本地 `origin` 实际指向 `https://github.com/cnYui/sub2api.git`，不是记忆里的 `Wei-Shaw/sub2api`。已临时抓取 `https://github.com/Wei-Shaw/sub2api.git main` 到 `refs/remotes/original/main` 用于对比。
- 本地 `main`：`aa046495a`，`backend/cmd/server/VERSION=0.1.138`。
- original/main：`60013c5f1`，`backend/cmd/server/VERSION=0.1.163`，最近标签到 `v0.1.163`。
- 共同祖先：`5f022663a`；original 比本地多 1152 个提交，本地比 original 多 294 个提交。
- 这是长期分叉，禁止直接 `merge original/main` 或用 original 覆盖本地。original 会删除/覆盖本地关键计费改造，包括 `AGENTS.md`、`usage_facts`、流量卡 reservation、订阅权益周期、周滚动额度迁移等。

## original 值得同步的能力

优先同步“能力补丁”，不要同步整套表结构和 UI。

1. OpenAI/Codex 协议能力
   - Codex CLI User-Agent 升级到 `0.144.1`。
   - GPT-5.6 系列模型、`responses`/`chat_completions` 桥接、compact/body-signal、function tool、image tool、`count_tokens` 兼容持续补强。
   - `response.failed`、SSE 紧凑格式、流式错误透传和 failover 边界更完整。

2. OpenAI 账号调度
   - `OpenAIAccountScheduleRequest` 新增平台、上一轮 response 粘性账号、是否允许 previous response 迁移、订阅优先、上游 token cost 调度等字段。
   - 高级调度器支持 Top-K 覆盖、quota headroom、低上游成本优先、订阅优先、粘性加权。
   - 近期修复包括 model-scoped transient cooldown、调度缓存异常时间、LastUsedAt 独立写入、无可用账号排除原因统计、调度快照账号载荷复用。

3. 账号凭证与导入
   - Agent Identity 认证、Team 隔离、按 `chatgpt_user_id` 匹配导入、避免 access-only 导入误合并。
   - API Key 账号请求头覆写、账号级自定义上游地址、上游计费倍率探测。
   - 账号凭证错误 failover 和缺失 refresh token 的调度隔离更完整。

4. 用量与计费解析
   - 图片输入 token、hosted `image_generation` 工具 usage、长上下文计费、mapped billing model、alpha/search 按次计费等补丁。
   - 这些要接入本地 `usage_facts + OpenAIBillingAuthorization + UsageBillingRepository`，不能回退到 original 的普通 `usage_logs` 直写路径。

5. 运维与安全
   - 审计日志、敏感操作 step-up 2FA、入口拒绝日志、Redis ACL username、依赖安全升级可择优移植。
   - 前端 i18n 拆分、Grok/Batch Image/Prompt Audit 等与当前“去 CPA”目标弱相关，后置。

## 同步策略

1. 新建独立分支，例如 `codex/original-sync-openai-cpa-removal-prep`。
2. 固定 original 基线为 `60013c5f1`，不要继续浮动同步，避免计划执行时目标漂移。
3. 先做迁移号审计：本地 `164-175` 已用于 `usage_facts`、流量卡 reservation、订阅权益周期、周额度；original 同号迁移用途不同。所有 original SQL 必须在本地重新编号并手工改名，不能 cherry-pick 迁移文件。
4. 按能力移植，而不是按 PR 整体 cherry-pick：
   - 第一批：Codex 协议兼容、错误透传、SSE/compact 修复。
   - 第二批：OpenAI 账号调度和 scheduler cache 修复。
   - 第三批：Agent Identity / OAuth 导入 / 凭证持久化。
   - 第四批：usage/token 解析进入本地 durable billing。
5. 每批必须跑：
   - `go test ./...`
   - 前端只在触碰前端时跑 `pnpm typecheck && pnpm lint:check && pnpm test:run && pnpm build`
   - 本地 `/v1/models`、`/v1/responses`、`/v1/chat/completions`、图片端点 smoke。

## 方案 A：维持现有 CPA 链路

目标：保持当前公网链路不变，让 CPA 继续承担账号池、OAuth、协议转换与账号切换；Sub2API 仍是唯一公网入口、用户 Key、计费事实源。

链路：

```text
用户 -> Sub2API /v1/* -> Sub2API 预授权 -> Sub2API account(cliproxy upstream) -> CPA -> 上游账号池 -> OpenAI/ChatGPT
```

必须保持：

- Sub2API 对用户请求先做 `OpenAIBillingAuthorization`，按“订阅额度 -> 流量卡”确定唯一来源。
- CPA usage event 不能独立创建计费事实；当前 `InternalUsageEventService.RecordCLIProxyUsageEvent` 返回 `Skipped=true` 是正确保护，避免 CPA 使用独立 request_id 或硬编码余额导致错源/双计费。
- Sub2API account 里的 CPA 上游入口继续使用 `credentials.pool_mode=true`，让 Sub2API 不把 CPA 当单个静态 OpenAI Key。
- 401/403/429/5xx 的真实上游语义要从 CPA 透回 Sub2API，Sub2API 再按 `S2A-*` 错误契约渲染。
- 运行态只需要准备 CPA 可回滚：保留 `host.docker.internal:8317` 或共享 Docker bridge、TLS CA、内网 token、nginx 当前指向。

优点：

- 改动最小，公网风险低。
- CPA 的账号池/OAuth/协议转换能力保留，适合短期业务稳定。

缺点：

- 调度事实割裂：Sub2API 只看到一个 CPA 上游账号，真实 OAuth 账号状态在 CPA 内。
- 错误语义和 usage 事实跨进程，仍需维护 HMAC、TLS、内部 token、网络、双日志排查。
- 无法从 Sub2API 管理台直接精确管理每个真实 Codex 凭证。

适合：近期保公网稳定、边同步 original 账号调度能力边准备切换。

## 方案 B：停止 CPA，仅使用 Sub2API

目标：真实 OpenAI/Codex 账号凭证直接存入 Sub2API `accounts.credentials`，Sub2API 独立完成用户计费、账号选择、token refresh、failover、usage fact 持久化和 dashboard 展示。

链路：

```text
用户 -> Sub2API /v1/* -> Sub2API 预授权 -> Sub2API OpenAIAccountScheduler -> 真实 OpenAI/Codex 账号 -> usage_facts -> billing worker
```

必须实现的最小闭环：

1. 凭证模型
   - `accounts.platform=openai`。
   - OAuth 账号：`type=oauth`，`credentials` 保存 `access_token`、`refresh_token`、`expires_at`、`chatgpt_user_id`、Team/plan/agent identity 等必要字段。
   - PAT/API Key 账号：`type=api_key` 或现有等价类型，保留 `base_url`、请求头覆写、模型能力、rate multiplier。
   - 不再需要 CPA 的内部转发密钥；但仍要保持密钥脱敏、导入校验和 refresh 失败隔离。

2. 账号调度
   - 使用本地 `OpenAIAccountScheduler`、`scheduler_cache`、`scheduler_outbox`。
   - 移植 original 的 model-scoped cooldown、quota headroom、上游成本优先、previous response 粘性、无账号排除原因统计。
   - 每次选择返回真实 `account_id`，`usage_facts.account_id` 不再恒等于 CPA 入口账号。

3. 计费预授权
   - 请求前仍由 `OpenAIBillingAuthorizationService` 做唯一来源判断。
   - 订阅路径必须绑定本地 `subscription_entitlement_periods` 和周滚动窗口。
   - 流量卡路径必须 reservation，响应成功前必须已经有可追踪 reservation。
   - 成功后只允许 `UsageFactSettlementService` 按预授权结果结算，禁止响应后重新选择来源。

4. 请求转发
   - `/v1/responses`、`/v1/chat/completions`、`/v1/models`、图片端点都走 Sub2API 内部 OpenAI gateway。
   - OAuth 账号使用 `OpenAITokenProvider` 刷新 access token。
   - failover 只在可重试错误发生，并保留请求体缓存、流式边界、已输出后不切号等语义。

5. 用量事实
   - `BuildUsageFact/PersistUsageFact` 是唯一成功计费入口。
   - 图片 usage、cache token、reasoning token、hosted image_generation、alpha/search、长上下文倍率都要落入本地 `UsageBillingCommand`。
   - CPA 的 `/internal/usage-events` 可以保留但关闭，作为回滚后 CPA 链路使用。

6. 观测与回滚
   - dashboard/ops 页面展示真实账号选择、排除原因、上游 token/quota、最后使用、临时不可调度原因。
   - 保留一键回滚：启用单个 CPA upstream account，禁用真实 OpenAI 账号调度。

优点：

- 单事实源：用户计费、真实账号调度、usage、错误归因都在 Sub2API。
- 排障简单，公网链路少一层网络/TLS/HMAC/日志系统。
- 可以按真实 account 维度做额度、成本、质量和 failover。

缺点：

- 初期实现风险高；Sub2API 必须补齐 CPA 现在成熟承担的协议和账号池细节。
- 切换时真实凭证迁移、token refresh、风控指纹、模型能力探测必须非常谨慎。

适合：完成 original 关键 OpenAI 同步后，先本地/候选环境灰度，再停 CPA。

## 推荐执行顺序

1. 先保持方案 A 生产不变，建立 `original/main@60013c5f1` 同步分支。
2. 移植 original 的 OpenAI/Codex 协议与调度修复，但全部适配本地 durable billing。
3. 增加 Sub2API 真实 OpenAI 凭证导入/管理的最小后台能力，先只在本地环境启用。
4. 本地用 2-3 个真实账号跑完整 smoke：`models`、`responses`、`chat_completions`、图片、429/failover、usage_facts settled、dashboard quota。
5. 候选环境并行配置两组账号：
   - CPA upstream account 保留并可调度。
   - 真实 OpenAI accounts 分组隔离，只给测试 Key 使用。
6. 通过配置开关按用户/Key/group 灰度到 Sub2API 单体链路。
7. 灰度稳定后禁用 CPA upstream account，但不删除 CPA 容器和配置，保留至少 48 小时回滚窗口。

## 当前风险点

- original 与本地迁移号冲突严重，直接同步会破坏数据库演进。
- 本地已有未提交文档、deploy 和 AGENTS 修改；后续动代码前要先确认这些不是待保留运行态记录。
- `AGENTS.md` 里 remote 记忆与实际 `git remote -v` 不一致，需要单独修正文档或远端配置。
- 单体链路不是“删 CPA 配置”这么简单；必须先让真实凭证、调度、failover、usage fact 形成闭环。
