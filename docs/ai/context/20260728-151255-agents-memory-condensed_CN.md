# Sub2API AGENTS 压缩记忆

> 压缩时间：2026-07-28 15:12:55（Asia/Tokyo）。本文合并根 `AGENTS.md`、上一版压缩记忆和 2026-07-17～2026-07-28 的新增定论。根 `AGENTS.md` 只保留执行入口；单次操作细节以对应 `docs/ai/context/` 原始文档为准。

## 协作与文档规则

- 默认使用中文；文档、说明、总结、计划、回复和代码注释均使用中文，除非用户明确要求英文。
- 代码注释写原因，不写过程；表达简洁直接，不写重复总结。
- 函数式优先，组合优于继承；TS/JS 避免 OOP。新功能优先复用或重构现有代码，遵循 KISS、DRY 和 ai-coding-discipline。
- 从第一性原理确认真正问题，警惕 XY 问题，解决根因，不做 workaround。小设计问题直接重构；大设计问题原地加 TODO 并说明原因。
- 架构设计参考 ddia-principles 和 software-design-philosophy；不确定的技术选型先 research。
- 修改、架构设计、技术选型前后，在 `docs/ai/context/` 新建 plan/design/result 文档；文件名统一为 `YYYYMMDD-HHMMSS-*.md`。
- `docs/ai/context/` 只允许新增，不覆盖、重命名或删除历史文件。长期流水不要再直接堆入根 `AGENTS.md`。

## 当前架构与运行态

### 公网与服务职责

- Sub2API 是唯一公网 API 入口、唯一用户 API Key、计费和用量事实源。
- 当前主 OpenAI 链路为：`Cloudflare Tunnel -> sub2api-public-nginx-local:8080 -> 外层定制版 sub2api-dev:18080 -> 内层 latest sub2api-upstream-latest:18086 -> OpenAI`。
- 2026-07-28 只读核验：Nginx upstream 为 `host.docker.internal:18080`；`18080/health`、`18086/health`、`8080/health` 均返回 200，外层和内层应用均 healthy。
- 外层 `18080` 负责用户、套餐、流量卡、价格、usage fact 和最终计费；内层 `18086` 负责 OpenAI OAuth 账号池、调度和协议上游。
- `cliproxyapi-local-dev:8317` 当前容器可能仍在运行，但 2026-07-22 已退出主 OpenAI 调度链路；不能因容器 healthy 就推断其参与实际请求，必须核对外层账号 schedulable、请求日志和 usage 关联。
- `aaccx.pw/shop` 归 yui.web；`aaccx.pw/v1/*`、`/api/*`、Sub2API 控制台和 `api.aaccx.pw` 归 Sub2API。
- 正式模型 API 只支持 `/v1/*`；裸 `/responses`、`/models`、`/chat/completions`、`/embeddings`、`/images/*` 不做静默兼容。

### 运行态边界

- 旧 `18084` candidate、`18082`、`18085`、旧 `18080 preview` 和 CPA 链路都是历史环境；判断当前状态必须检查 Nginx upstream、容器、端口和 health，不能只读旧文档。
- Docker Compose project 未隔离曾误停/重建公网栈。任何候选、恢复、替换、清理前必须确认 project、容器名、端口、网络、volume 和 Nginx 指向。
- PostgreSQL、Redis、SMTP、支付 provider、套餐、订阅、余额和流量卡均以运行态数据库为准，不会随镜像替换自动同步。
- 修改运行态 DB、Redis、容器、Nginx 或公网链路前，必须写计划、备份、验证备份可读并明确回滚边界。应用回滚不等于数据库回滚，不能擅自恢复整库。
- 2026-07-26 已清理指定 PKB/Supabase 容器和无用镜像，但保留公网 Nginx、两套 Sub2API 数据容器和 volume；后续清理仍禁止删除 PostgreSQL/Redis 数据和构建缓存，除非用户明确授权并完成目标核验。
- `xiaobianfuai@gmail.com` 是管理员和本机 Codex Local Key 所属账号，不按普通用户删除。

## 最高优先级：OpenAI 计费状态机

### 2026-07-28 审计事实

- 外层 `sub2api-dev:18080` 为 `RUN_MODE=standard`，流量卡 reservation enabled 且 shadow=false；问题是代码状态机缺口，不是配置绕过。
- 当前有 74 条已过期 `dispatched` reservation 无 usage fact，冻结 5 个用户合计 `31.49608750 USD`。
- 最近冻结可由 `/v1/responses` 上游 HTTP/SSE/流式失败后未终结 reservation 复现。后台只回收未派发 `reserved`，不能自动释放已派发 `dispatched`。
- 2026-07-22～2026-07-25 有 2902 条历史 `usage_facts.billing_status='debt'`，合计 `531.91499889 USD`；全部是 OpenAI `/v1/responses`、无 reservation，说明旧路径在上游消费后才动态选择流量卡。
- 主 `/v1/responses` 成功路径已有请求前预授权，但 HTTP/SSE/流式失败终结仍不完整；`/v1/embeddings`、OpenAI 分组 `/v1/messages`、WebSocket ingress 每个 turn 仍未统一接入预授权。
- 套餐额度只有检查，没有并发 hold；结算阶段仍存在缺 authorization 时重新选流量卡的旧逻辑。
- 本轮审计只读，未修复代码，未批量释放 74 条冻结 reservation，未处理 2902 条 debt。

### 不变量与目标状态机

- 没有请求前不可变资金授权，绝不调用上游。
- 资金来源固定为：套餐原子 hold -> 流量卡 reservation -> 两者都不足则请求前 402。
- 账户余额只用于购买、充值、退款等资金业务，不参与 OpenAI 模型请求计费。
- 一次请求只能有一个 authorization 和一个资金来源；failover/retry 复用同一个 authorization，结算禁止重新选源。
- 套餐判断必须满足 `used_usd + held_usd + request_budget <= quota_limit`；流量卡继续使用 `remaining_usd - reserved_usd` 和行锁防并发超卖。
- 已派发请求必须终结为 `settled`、可证明未计费的 `released`、结果不确定的 `unknown` 或异常兜底 `debt`，不能长期停留在 `dispatched`。
- `response.failed` 可能带 billable usage，不能统一释放；transport/DNS/TCP/TLS 等无法确认是否到达上游时进入 `unknown`。
- `unknown/dispatched` 必须有 reconciliation：关联外层 request ID、内层 usage log 和上游 request ID；无法确认时转平台 suspense/manual review，不能无限冻结用户额度。
- 客户端成功响应可以不等待实时扣款，但必须等待 durable usage fact 落库；usage fact 必须引用 authorization。

### 2 USD 单请求预算边界

- 所有 OpenAI 请求统一单请求预算硬上限 `2 USD`；这是请求前拒绝线，不是固定冻结额。
- 统一预算：`B_total = 文本输入 + 文本输出 + 附件输入 + 图片输出`。`B_total <= 2` 才尝试套餐/流量卡 hold，超过则请求前 402。
- GPT-5.5 按 2 倍、GPT-5.6 按 2.5 倍、GPT Image 2 按 2 倍；倍率只能应用一次，结算后的 `usage_logs.total_cost` 不再重复乘倍率。
- 文字输入只统计最终送入模型的文本、工具和 schema，不把 base64、文件字节数或图片 URL 当文本 Token。
- 显式输出上限按用户值估算；未显式指定时默认最多 8192 Token，并按剩余 2 USD 预算压缩；最低保留 256 Token，剩余预算不足则 402。显式上限超预算时不静默修改。
- 独立生图模型按 `gpt-image-2` 解析；按模型、尺寸、质量、数量和图片输入成本精确预算。同一价格模式不能同时重复累加按张费用和 image-token 费用。
- 文字与生图混合请求预算为外层文字模型成本加 `gpt-image-2` 图片成本，仍只创建一个 authorization。
- 允许多图，但必须能在请求前或工具执行前约束同一 authorization 的剩余图片预算；不能生成后再换资金来源。
- 图生图按输入图片处理成本加输出图片成本；PDF 按可提取文本和必要的视觉页处理；视频只有在能确定模型采样规则时计费，否则请求前拒绝。
- 不能用 1.5/2 USD 金额阈值判断生图。审计中 261 条图片计费请求仅 2 条超过 1.5 USD；大额请求多数是超大文本上下文。
- 待最终确认：是否完全取消 10% Token 安全系数并改为最终变换后的精确 Tokenizer 统计；多图超出剩余预算时是部分生成还是整体拒绝。

### 历史计费基础

- 已有 migration `164/165`、durable usage fact/outbox、流量卡 reservation/debt gate 和逐张流量卡耗尽事件基础，但 2026-07-28 审计证明入口覆盖和失败终结仍不完整。
- OpenAI 自动透传分支已于 2026-07-24 接入 `authorizeOpenAIForward`，但统一 finalizer 尚未完成。
- 流量卡按 `expires_at, credited_at, id` 顺序逐张预留和扣费；单卡耗尽事件按 `credit_id` 幂等。
- 生图实际结算应使用上游可确认的文本输入、缓存、文本输出、图片输入、图片输出 usage；失败/取消但存在 usage 时也要落 durable fact。

关键文档：

- `docs/ai/context/20260728-130248-traffic-credit-billing-state-machine-audit-and-design-result_CN.md`
- `docs/ai/context/20260728-145406-usage-high-cost-image-analysis_CN.md`
- `docs/ai/context/20260728-150025-gpt-image-2-budget-cap-design_CN.md`
- `docs/ai/context/20260728-150440-openai-billing-budget-formula-and-confirmations_CN.md`
- `docs/ai/context/20260728-150850-openai-attachment-billing-clarification_CN.md`

## 价格与模型

- 运行态最新口径：GPT-5.5 为 2x，`gpt-5.6-sol/terra/luna` 为 2.5x，GPT Image 2 为 2x。
- GPT-5.6 三模型价格在外层 `deploy/data/model_pricing.json` 单独调整；`deploy/data/config.yaml` 已关闭默认价格哈希同步，避免远程默认价格覆盖本地运行价。
- GPT-5.6 只展示和调用完整模型名，不提供裸 `gpt-5.6` alias；缺价不得回退 GPT-5.4。
- Responses 中记录的 GPT-5.5/GPT-5.6 可能只是外层文字模型；图片工具默认仍按 `gpt-image-2` 定价。
- 价格、service tier、长上下文倍率和分组倍率必须形成请求前不可变价格快照，预算与结算不能读取不同价格版本。

## 内层 OpenAI OAuth 账号池

- 内层 latest Sub2API 位于 `127.0.0.1:18086`，账号统一绑定 `groups.id=2 / internal-openai-upstream`。
- 截至 2026-07-28 08:18，内层 OpenAI OAuth 账号共 399 个，`active/schedulable` 320 个，其他状态 79 个；最新新增 `id=395..399 / k12/728-1..5`，未跑模型测试。
- 2026-07-22～2026-07-28 已连续导入 agent identity、free、plus、k12 等批次。单批 ID、来源文件、测试结果和备份路径保留在对应 `upstream-latest-*-import-*-result_CN.md`。
- `active/schedulable=true` 只表示可进入调度，不等于已验证支持 `gpt-5.4`。free/k12/plus/agent identity 都出现过：成功、400 模型不支持、401 `token_invalidated`、402 `deactivated_workspace`、403 agent runtime deleted。
- 账号被真实请求命中后，内层可能自动改为 `status=error/schedulable=false`；账号总量和可调度数必须实时查 DB，不能沿用旧批次统计。
- 导入前必须备份并验证可读，按名称、邮箱、`outlook_email`、`chatgpt_account_id` 和目标批次名去重；同一来源文件内部也可能共享 `chatgpt_account_id`。
- 通过正式管理接口导入和更新，确认 group 绑定、status、schedulable、error_message；是否逐个跑真实模型测试由用户明确决定。
- 不在文档或日志记录完整 OAuth access/refresh token。
- 内层管理员 `xiaobianfuai@gmail.com/users.id=1` 于 2026-07-26 增加 1000 万余额，审计记录已落库；初始化密码与现有管理员凭据存在漂移且未配置管理 API Key，后续需单独轮换并验证，不能顺手重置。

最新账号池结果：`docs/ai/context/20260728-081808-upstream-latest-k12-728-import-result_CN.md`。

## 订阅、额度与退款

- 订阅到期不能联动停用 API Key，因为有效流量卡必须继续使用。
- 用户无有效订阅时可买任意套餐；已有有效订阅时只允许购买相同 `group_id` 续费。不同 `group_id` 必须提示先退款，不创建订单、不扣余额、不自动切换。
- `29 元订阅池` 对应 `subscription_plans.id=1 -> group_id=2 -> codex-pool-19-usd`；不要误绑 `codex-pool-29-usd`。
- `79 元订阅池` 对应 `codex-pool-69-usd`，每日 69 USD。
- 公共 Codex 订阅目标为 28 天有效、按订阅锚点每 7 天滚动刷新额度；代码、权益周期、退款 quote、Dashboard/API 已接入，但 2026-07-22 cutover dry-run 仍被 51 个历史对象阻塞，不能假定运行态已完成 cutover。
- 日额度超额顺延逻辑已实现：跨日 carryover 为 `max(旧日用量 - 日额度 × 跨天数, 0)`；管理端手动重置仍清零。依赖具体 migration 和运行态部署时要重新核对。
- 管理端撤销订阅已改为物理删除订阅及其权益周期、额度债务调整；订单和用量历史保留，关联 usage 由外键置空，撤销后失效 L1/Redis/PubSub 缓存。
- Dashboard 套餐额度以 `subscription_entitlement_periods` 和 durable usage facts 为事实；历史无法证明周期边界时显式降级，不伪造精确额度。
- 支付宝 + 余额组合支付、退款状态机、迟到付款补偿、`MANUAL_REVIEW` 和 1% 手续费规则沿用上一版压缩记忆，不临时绕过状态机。
- 2026-07-22 曾给当时 119 个 active 用户各发 10 USD GPT/OpenAI 流量卡，到期 2027-07-22；这是一次性运行态操作，使用前仍应实时核对用户、卡和 ledger。

## API、错误契约与客户端

- CLIProxyAPI 是聚合上游，不是静态 OpenAI Key；若重新参与链路，Sub2API 上游账号必须使用 `credentials.pool_mode=true`，401/403/429 在同账号内重试。
- 模型 API 上游失败契约已实现：全账号冷却为 `S2A-5004 / UPSTREAM_RATE_LIMITED / HTTP 429` 并保留 `Retry-After`；凭据不可用 503、超时 504、连接/无效响应 502。普通 REST 端点尚未全部迁移统一错误契约。
- 普通用户 Key 默认自动分组，后端根据 active OpenAI 套餐或流量卡解析 request-scoped effective group；管理员固定分组能力保留。
- `/v1/responses/responses` 是客户端重复路径错误，不做服务端兼容。
- `/v1/responses` 收到 `messages` 且无 `input` 时返回明确 400，提示使用 `input` 或 `/v1/chat/completions`。
- CCSwitch/Codex endpoint 必须恰好包含一个 `/v1`；用户 401 优先检查 Key 是否已删除、被掩码、复制不完整或 Authorization 头错误。
- `thinking_signature_invalid` 常与旧会话 encrypted reasoning 和多 OAuth 轮询有关；`Selected model is at capacity` 是上游容量不足，不能误判为本地套餐错误。

## 前端、基础设施与代码治理

- Material Relay 全前端重设计已于 2026-07-20 完成，覆盖公开页、认证页、用户端、管理端和通用组件；typecheck、lint、test、build 通过，移动端和桌面端无横向溢出。
- 代码冗余治理第二阶段已完成账号弹窗唯一化、设置 mapper 统一、旧直接计费链和失效 Makefile 目标清理；长期重构仍围绕可靠计费、支付状态机、OpenAI failover、用量统计和一次性工具边界。
- 本地曾实现 Sub2API/CLIProxyAPI 独立 Compose + 专用 bridge + 内部 CA/叶子证书。当前主链路已切换到双 Sub2API，但相关网络和证书经验仍适用于后续隔离设计。
- Docker Desktop 前端仍在不代表后端/WSL2 正常；2026-07-26 的 18080/18086 故障根因是 Docker backend 与 `docker-desktop` WSL2 停止，恢复后应用和数据容器自动健康。
- original 上游与本地定制版迁移和 durable billing 分叉严重，禁止直接 merge；应按能力移植协议、调度、Agent Identity 和 usage 解析补丁，并逐步灰度。

## 安全与高风险操作

- 不在文档、提交、终端摘要或日志中记录完整 API Key、OAuth token、内部 token、HMAC secret、SMTP 密码、支付密钥。
- `deploy/backups/` 的 dump 只记录路径、大小、权限和可读性，不复制内容。
- 删除或重建容器前先确认依赖；默认保留 PostgreSQL/Redis 容器、volume、Nginx 和当前公网链路。
- 对运行态用户、订阅、余额、流量卡、账号池做写操作时，优先正式管理接口；必须有变更前备份、结果复核和 health 验证。
- 真实模型测试会消费上游资源并可能触发账号自动禁用，只有用户明确要求时执行。

## 当前未完成事项

1. 统一 OpenAI `FinalizeBillingAuthorization`，覆盖 HTTP/SSE/流式失败和 partial usage。
2. 为 Embeddings、OpenAI Messages、WebSocket 每 turn 接入强制预授权。
3. 为套餐 entitlement 增加原子 hold，删除结算阶段动态选源和余额 overdraft。
4. 增加 unknown/dispatched reconciliation、状态转换 RowsAffected 检查和计费一致性指标。
5. 后续单独核对 74 条冻结 reservation 和 2902 条历史 debt；当前禁止仅按过期时间批量释放。
6. 确认 2 USD 预算中的 Token 精确统计和多图超预算策略，再进入 migration、部署和真实 Key 验证。
7. 单独轮换并验证内层管理员凭据，不与账号导入或计费改动混做。

## 追溯入口

- 上一版完整压缩记忆：`docs/ai/context/20260717-093308-agents-memory-condensed_CN.md`
- 更早压缩记忆：`docs/ai/context/20260624-195608-agents-memory-compressed_CN.md`
- 2026-07-26 工作树整理、迁移 178/179 和双容器重建：`docs/ai/context/20260726-120900-worktree-consolidation-and-container-rebuild-result_CN.md`
- 订阅硬撤销：`docs/ai/context/20260726-083450-admin-subscription-hard-revoke-result_CN.md`
- 双 Sub2API 桥接：`docs/ai/context/20260722-193400-dual-sub2api-local-bridge-result_CN.md`
- 公网双 Sub2API 计费 smoke：`docs/ai/context/20260722-194350-public-local-dual-sub2api-billing-smoke-result_CN.md`
- 28 天/周额度补齐：`docs/ai/context/20260722-035013-weekly-rolling-subscription-quota-28day-gap-fix-result_CN.md`
- 错误契约：`docs/ERROR_CONTRACT.md`、`docs/ai/context/20260719-213825-error-contract-unification-result_CN.md`
- Material Relay：`docs/ai/context/20260720-151230-sub2api-material-relay-frontend-redesign-result_CN.md`

## 维护与 Git

- 新增长期上下文只在 `docs/ai/context/` 创建新文件；若根 `AGENTS.md` 再次膨胀，先生成下一版压缩记忆，再精简入口。
- 合并、提交或收尾前运行 `git ls-files --others --exclude-standard docs/ai/context`，确认上下文文档没有遗漏和敏感信息。
- 不把 `docs/ai/context/` 加回 `.gitignore`；未跟踪表示尚未提交，不表示被忽略。
- 当前只读核验的 Git 远端为 `origin=https://github.com/cnYui/sub2api.git`，即用户个人 fork；当前没有单独配置 Wei-Shaw 上游远端。需要同步上游时先核对/临时添加远端，禁止凭旧记忆直接推送或 merge。
