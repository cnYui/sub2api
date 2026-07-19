# AI 协作入口

> 最新压缩记忆见 `docs/ai/context/20260717-093308-agents-memory-condensed_CN.md`。
> 上一版压缩记忆见 `docs/ai/context/20260624-195608-agents-memory-compressed_CN.md`。
> 早期压缩记忆见 `docs/ai/context/20260619-151920-sub2api-compressed-memory_CN.md`。
> 后续长期上下文统一新建到 `docs/ai/context/YYYYMMDD-HHMMSS-*.md`，不要覆写、重命名或删除历史文档。

## 协作规则

- 默认使用中文；文档、说明、总结、计划、回复和代码注释都使用中文，除非用户明确要求英文。
- 代码注释写原因，不写过程。
- 表达简洁直接，不要多余总结。
- 函数式优先，组合优于继承；TS/JS 中避免 OOP。
- 新功能优先复用或重构现有代码，不堆砌；遵循 KISS、DRY。
- 解决根本问题，不做 workaround；发现大设计问题先原地加 TODO 并说明原因。
- 修改、架构设计、技术选型前后要在 `docs/ai/context/` 新建 design/plan/result 上下文。

## 最高优先级定论

- 2026-07-18 新增新人部署 Runbook：`docs/SUB2API_CLIPROXYAPI_DEPLOYMENT_RUNBOOK_CN.md`，并在 `deploy/README.md` 建入口。当前 CLIProxyAPI 8317 是 HTTPS/TLS，HTTP `Empty reply` 只是协议错；重新部署前空间不足要先按 Runbook 清理已停止旧容器和无用镜像，禁止删 DB/Redis volume；历史 `auth_unavailable`/502 根因是 Sub2API account 1 被临时失败状态/Redis 调度快照排除（日志 `excluded_account_count=1`），不是 CLIProxyAPI 调度器坏。
- 2026-07-18 本地修复 OpenAI 预授权预算单位错误：旧逻辑把 JSON `len(body)` 当作 `input_tokens`，导致 24MB 请求体按 `gpt-5.6-terra` 长上下文倍率误估到约 121 USD，并把 active 套餐用户导向流量卡 402；新逻辑只估算 JSON 文本输入、跳过图片/base64 传输载荷，套餐仍按修正后的预算优先计费，预算真实超过套餐剩余时保留流量卡兜底。仅本地代码与文档，未部署、未改运行态。
- 2026-07-17 独立 worktree `.worktrees/codex-code-redundancy-cleanup-phase2`、分支 `codex/code-redundancy-cleanup-phase2` 已完成代码冗余治理第二阶段：账号弹窗唯一化、设置响应 mapper 统一、旧直接计费链删除、失效 Makefile 目标清理；计划见 `docs/ai/context/20260717-153936-code-redundancy-cleanup-phase2-plan_CN.md`，结果见 `docs/ai/context/20260717-163551-code-redundancy-cleanup-phase2-result_CN.md`。未改运行态、未部署、未推送。
- 2026-07-17 正在 `codex/code-redundancy-refactor` 分阶段治理可靠计费、支付重复状态机、失效充值倍率、OpenAI failover、用量统计和一次性补录工具边界；计划见 `docs/ai/context/20260717-110156-code-redundancy-refactor-plan_CN.md`。
- 当前根 `AGENTS.md` 已压缩，完整迁移记忆见 `docs/ai/context/20260717-093308-agents-memory-condensed_CN.md`；不要再把流水账直接堆回本文件。
- 2026-07-17 本地分支 `codex/dashboard-subscription-quota-realtime` 已完成并合并 Dashboard 套餐额度实时展示：新增 `subscription_entitlement_periods` 权益周期事实、来源幂等发放/撤销、事实优先 `UserDashboardQuota` 读模型和 `GET /api/v1/usage/dashboard/quota`；精确周期为 `entitlement_period`，历史 active 订阅无不可变 `daily_limit_usd` 快照降级 `rolling_30d_legacy`，无套餐为 `none`；前端消费卡改为套餐额度，页面可见时 15 秒轻量轮询 quota。功能分支提交 `bd30ae9eb`，main merge 提交 `b2be93978`；未部署、未改运行态。结果见 `docs/ai/context/20260717-093136-dashboard-subscription-quota-task4-6-result_CN.md`。
- Sub2API 是唯一公网 API 入口、唯一用户 Key、计费和用量事实源；CLIProxyAPI 只作为内网账号池、OAuth、协议转换和轮询上游；yui.web/shop 只保留展示、说明和跳转。
- 当前主链路：`Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> sub2api-candidate 127.0.0.1:18084 -> CLIProxyAPI 127.0.0.1:8317`。
- `aaccx.pw/shop` 归 yui.web；`aaccx.pw/v1/*`、`/api/*` 和 Sub2API 控制台路由归 Sub2API；`api.aaccx.pw` 也是 Sub2API 入口。
- 正式模型 API 只保留 `/v1/*`；裸 `/responses`、`/models`、`/chat/completions`、`/embeddings`、`/images/*` 不应继续做静默兼容。
- 不要在文档、提交或日志中记录完整 API Key、内部 token、HMAC secret、SMTP 密码、支付密钥。
- `xiaobianfuai@gmail.com` 是管理员和本机 Codex Local Key 所属账号，不要按普通用户删除。

## 当前运行态提醒

- 当前公网由 18084 候选环境承接：应用容器 `sub2api-candidate`，数据库 `sub2api-candidate-postgres`，Redis `sub2api-candidate-redis`。
- 容器内访问上游聚合入口为 `host.docker.internal:8317`。
- 18080、18082、18085 都曾作为 preview/test 历史环境出现；判断运行态必须以当前 Nginx 指向、容器状态和 health 验证为准。
- Docker Compose project 未隔离曾造成误停/重建公网栈；任何候选、预演、恢复、替换容器前必须先确认 project、容器名、volume、端口和 Nginx 指向。
- 运行态 SMTP、支付 provider、套餐上架、订阅状态、余额和流量卡都以数据库为准，不会随镜像替换自动同步。
- 任何修改运行态 DB、Redis、容器、Nginx 或公网链路前，必须先写计划、备份、验证备份可读，并明确回滚边界。

## 业务红线

- 订阅到期不能联动停用 API Key，因为有效流量卡必须继续可用。
- 请求计费必须先确定订阅、余额或流量卡中的唯一计费来源；流量卡路径必须请求前预授权，成功响应前必须持久化 usage fact。
- `usage_facts` durable outbox、流量卡 reservation/debt gate、生图实际 Token 计费与逐张流量卡耗尽事件的最新定论见最新压缩记忆和对应结果文档。
- 已有有效订阅时，只允许购买相同 `group_id` 续费；购买不同 `group_id` 必须提示“当前套餐仍在有效期内，如需更换套餐，请先退款后再购买”，不创建订单、不扣余额、不自动切换。
- 支付宝 + 余额组合支付、退款状态机、迟到付款补偿和 `MANUAL_REVIEW` 规则见最新压缩记忆；不要临时绕过状态机。
- `29 元订阅池` 对应 `subscription_plans.id=1 -> group_id=2 -> codex-pool-19-usd`；不要误绑到 `codex-pool-29-usd`。
- `79 元订阅池` 对应 `codex-pool-69-usd`，每日 69 USD。
- CLIProxyAPI 是聚合上游，不是单个静态 OpenAI Key；Sub2API 上游账号需启用 `credentials.pool_mode=true` 并让 401/403/429 在同账号内重试。

## 维护规则

- 需要新增长期上下文时，只在 `docs/ai/context/` 创建新文件。
- 进入实现前先写 design/plan 上下文；完成后写 result 上下文。
- 每次合并功能分支、提交 main 或做收尾前，必须运行 `git ls-files --others --exclude-standard docs/ai/context` 检查未跟踪上下文文档；确认无敏感信息后纳入同一次功能提交，或单独做 `docs: archive ai context` 提交。
- 不要把 `docs/ai/context/` 加回 `.gitignore`；未跟踪状态表示尚未提交，不表示被忽略。
- 如果某个上下文文档暂不提交，必须在回复里说明原因和后续处理方式。
- 若上下文再次过长，先沉淀到新的压缩文档，再继续精简本文件。
- Git 远端：`origin=https://github.com/Wei-Shaw/sub2api.git` 是上游；`personal=https://github.com/cnYui/sub2api.git` 是用户个人 fork。后续保存当前工作分支优先推送到 `personal`，不要误推到上游 `origin`。
