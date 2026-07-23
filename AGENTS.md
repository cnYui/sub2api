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

- 2026-07-23 已将附件 `sub2api-selected-accounts-2026-07-22134906.json` 中 10 个 OpenAI agent identity 账号追加到内层 latest Sub2API，新增账号 `id=23..32`，均 active/schedulable 并绑定 `internal-openai-upstream`（`groups.id=2`）；但管理测试接口显式使用 `gpt-5.4` 对新增 10 个账号均返回上游 `402 deactivated_workspace`，这批账号已入池但不可按可用账号看待。本次 SQL 核对显示内层 OpenAI OAuth 总数 32，当前 active/schedulable 为 10。结果见 `docs/ai/context/20260723-083722-upstream-latest-selected-accounts-import-result_CN.md`。

- 2026-07-22 已将附件 `20260722-210541-sub2-API.json` 中 3 个 OpenAI agent identity 账号追加到内层 latest Sub2API，新增账号 `id=19..21`，均 active/schedulable 并绑定 `internal-openai-upstream`（`groups.id=2`）；内层当前 OpenAI OAuth 账号共 21 个，管理测试接口显式使用 `gpt-5.4` 对新增 3 个账号均返回 200 且无错误信号。结果见 `docs/ai/context/20260722-220808-upstream-latest-three-agent-identity-import-result_CN.md`。

- 2026-07-22 已将附件 `20260722-204504-sub2-API.json` 与 `20260722-205243-sub2-API.json` 中 8 个 OpenAI agent identity 账号追加到内层 latest Sub2API，新增账号 `id=11..18`，均 active/schedulable 并绑定 `internal-openai-upstream`（`groups.id=2`）；内层当前 OpenAI OAuth 账号共 18 个，管理测试接口显式使用 `gpt-5.4` 对新增 8 个账号均返回 200 且无错误信号。结果见 `docs/ai/context/20260722-220259-upstream-latest-batch-agent-identity-import-result_CN.md`。

- 2026-07-22 已将附件 `sub2api-agentIdentity-alive (1).json` 中 5 个 OpenAI agent identity 账号追加到内层 latest Sub2API，新增账号 `id=6..10`，均 active/schedulable 并绑定 `internal-openai-upstream`（`groups.id=2`）；内层当前 OpenAI OAuth 账号共 10 个，管理测试接口对新增 5 个账号均返回 200。结果见 `docs/ai/context/20260722-200430-upstream-latest-agent-identity-import-result_CN.md`。

- 2026-07-22 已完成 CPA 临时去除的双方案与 GitHub original 同步只读准备：当前本地 `origin` 实际指向 `cnYui/sub2api`，已临时抓取 `Wei-Shaw/sub2api` 到 `original/main@60013c5f1`；original 已到 `0.1.163`，本地为 `0.1.138`，共同祖先 `5f022663a`，original ahead 1152、本地 ahead 294，迁移号与本地 durable billing/订阅权益周期严重冲突，禁止直接 merge。推荐先维持 CPA 公网链路，按能力移植 original 的 OpenAI/Codex 协议、调度、Agent Identity、usage 解析补丁，再灰度 Sub2API 单体真实凭证链路。计划见 `docs/ai/context/20260722-185918-cpa-removal-dual-path-and-original-sync-plan_CN.md`。

- 2026-07-22 已在本地完成“外层定制版 Sub2API + 内层 latest Sub2API”桥接并暂停 CPA：latest 克隆在 `D:\CodeWorkSpace\sub2api-upstream-latest`，容器 `sub2api-upstream-latest` 通过 `127.0.0.1:18086` 提供本机控制台；用户提供的 GPT agent identity 凭证已导入内层，内层 `/v1/models` 与 `/v1/responses` 已 200。外层本地 `sub2api-dev:18080` 新增 `sub2api-latest-openai-upstream` 指向 `http://host.docker.internal:18086/v1`，CPA 账号 `cliproxy-local-openai` 已设为不可调度，`cliproxyapi-local-dev` 已停止；外层 `api_key_id=173` 验证 `/v1/responses` 200，`usage_facts` settled，计费仍在外层。结果见 `docs/ai/context/20260722-193400-dual-sub2api-local-bridge-result_CN.md`。

- 2026-07-22 已将内层 latest Sub2API 本地管理账号同步为 `xiaobianfuai@gmail.com`，登录接口 `POST http://127.0.0.1:18086/api/v1/auth/login` 已验证 200；控制台 `http://127.0.0.1:18086`，账号管理 `http://127.0.0.1:18086/admin/accounts`。结果见 `docs/ai/context/20260722-193505-upstream-latest-admin-login-result_CN.md`。

- 2026-07-22 已将附件 4 个 OpenAI agent identity 账号追加到内层 latest Sub2API，内层当前 5 个 OpenAI OAuth 账号均 active/schedulable 且绑定 `internal-openai-upstream`；已用本地 Key 从公网 `https://api.aaccx.pw/v1/responses` 验证链路为公网入口 -> 外层 `sub2api-dev:18080` 计费 -> 内层 `sub2api-upstream-latest:18086` 请求上游，外层 `usage_facts.id=14923` settled、`usage_logs.id=172539 actual_cost=0.0007372500`，内层 `usage_logs.id=32 account_id=3`，CPA 容器停止且未参与。结果见 `docs/ai/context/20260722-194350-public-local-dual-sub2api-billing-smoke-result_CN.md`。

- 2026-07-22 已在当前公网事实源给所有 active 用户批量发放一张 10 USD GPT/OpenAI 流量卡：批次 `grant-20260722-10usd-current-users`，覆盖 `users.deleted_at IS NULL AND status='active'` 的 119 个账号（117 user + 2 admin），写入 `payment_orders/user_traffic_credits/traffic_credit_ledger/payment_audit_logs` 各 119 条，订单金额合计 0，流量卡合计 1190 USD，到期 `2027-07-22 15:24:05 +08`；备份在 `backups/20260722-162105-current-users-10usd-traffic-pack-prechange.sql`。结果见 `docs/ai/context/20260722-162735-current-users-10usd-traffic-pack-grant-result_CN.md`。

- 2026-07-22 已在当前公网事实源手动为 `377293029@qq.com` 补发 29 元套餐：用户 `id=117`、Key `id=173`，补发 `user_subscriptions.id=127`、`subscription_entitlement_periods.id=211`，来源 `manual_zpay/12344239`，有效期 `2026-07-22 15:15:05 +08` 至 `2026-08-19 15:15:05 +08`；公网 `/v1/models`、`/v1/responses`、`/v1/chat/completions` 已用该用户真实 Key 验证 200，usage fact 已 settled。结果见 `docs/ai/context/20260722-162950-user-377293029-manual-subscription-grant-result_CN.md`。

- 2026-07-22 已在本地代码层完成公共 Codex 订阅“28 天有效 + 按订阅锚点每 7 天滚动刷新额度”补齐：schema/Ent、周窗口计算器、订单不可变快照、权益段周额度/周期总额度、usage_facts 归属、退款 quote/二次计算、Dashboard/Key/订阅/API 字段和前端整数 USD 文案均已接入；`go test ./...`、前端 typecheck/lint/test/build 通过。未执行 cutover `--apply`，本地 dry-run 仍阻塞 51 个历史对象；未碰公网、未提交、未推送。结果见 `docs/ai/context/20260722-035013-weekly-rolling-subscription-quota-28day-gap-fix-result_CN.md`。

- 2026-07-21 已在本地开发态完成订阅日额度超额顺延：跨自然日时新日窗口写入 `max(旧日用量 - 日额度 × 已跨天数, 0)`，请求前限额判断、成功结算落库、每日校准和 Dashboard quota 均使用该 carryover；管理员手动重置仍清零，无限额订阅不产生 carryover。已备份并重建本地 `sub2api-dev:8080`，172/173 本地回填迁移已应用，当前 64 个有限额 active 订阅里有 2 个超额，合计约 424.15 USD 将逐日抵扣；未触碰公网。结果见 `docs/ai/context/20260721-212052-subscription-daily-overage-carryover-result_CN.md`。

- 2026-07-21 已在本地开发态完成额度削减、用户用量/错误请求可见性与图片扣费修复：Codex 订阅日额度统一为 15/25/39/53/66/100/133 USD，`/api/v1/payment/plans` 返回 group 限额字段，默认启用 `allow_user_view_error_requests=true`，图片 usage 解析兼容 `image_tokens.*` 并在图片端点缺少拆分时把输出 token 归入图片输出 token；本地 `sub2api-dev:8080` 已重建健康，未触碰公网。结果见 `docs/ai/context/20260721-181826-billing-quota-usage-image-result_CN.md`。

- 2026-07-21 已创建个人 Codex 排查 skill `diagnose-sub2api-cpa`：位置 `C:\Users\yui\.codex\skills\diagnose-sub2api-cpa`，用于 Sub2API/CPA 部署后公网 `/v1/models`、`/v1/responses`、`/v1/chat/completions`、图片生成、`usage_facts`、用户面板扣费、TLS/x509、CPA 凭证和 `auth_unavailable`/429/502/503/504 分段排查；脚本只从环境变量读取 Key，不记录完整密钥。结果见 `docs/ai/context/20260721-173923-diagnose-sub2api-cpa-skill-result_CN.md`。

- 2026-07-20 已批准全前端 Material Relay 重设计：范围覆盖公开页、认证页、用户端、管理端和通用组件；目标是信息效率、品牌辨识度、交互手感同等重要。视觉基线为通透但克制的浮动材质，半透明只用于浮动层，内容面使用实色高可读表面；动效遵循 `Press / Tap feedback`、`Origin-aware animation`、`Continuity transition`、`Stagger` 词汇与统一 easing/时长契约。设计文档见 `docs/ai/context/20260720-102228-sub2api-material-relay-frontend-redesign-design_CN.md`，尚未改业务代码、未部署。

- 2026-07-19 已完成本地 Sub2API/CLIProxyAPI 共享 Docker bridge 实施：`sub2api-dev` 保留 PostgreSQL/Redis 数据网络并额外加入 `sub2api-cliproxy-local`，`cliproxyapi-local-dev` 只加入该共享网络；账号 `cliproxy-local-openai` 已通过正式管理 API 切换为 `https://cliproxyapi:8317/v1`，数据库与 Redis 快照一致。新增内部 CA/叶子证书、可选运行时 CA 注入、两仓库本地 Compose 与回归测试；两个应用分别重建后 DNS、TLS、业务和 usage 回调仍有效，数据容器未替换。CLI 本地 `auths/` 为空，成功响应/成功 usage event 尚未验证；失败事件回调 200 且不产生计费事实。未改公网、未提交、未推送，结果见 `docs/ai/context/20260719-204112-sub2api-cliproxyapi-shared-network-implementation-result_CN.md`。
- 2026-07-19 已完成全项目错误契约只读调查：429 偶发显示为 502 的根因是错误语义在账号池调度和 failover 聚合时丢失，CLIProxyAPI 可把账号级 429 折叠为 `auth_unavailable`/503，Sub2API 又把上游 500/502/503/504 统一映射为 502；项目同时存在至少六套错误响应结构，前端还有约 121 处手写解析且未覆盖 OpenAI `error.message`。建议建立 Sub2API/CLIProxyAPI 跨服务结构化错误契约、协议 renderer 和统一前端 normalizer，并采用 `S2A-四位数字 + 英文符号码` 双码；本轮未改业务代码和运行态，结果见 `docs/ai/context/20260719-202238-project-error-contract-investigation-result_CN.md`。
- 2026-07-19 已完成计费来源顺序与生图预算只读调查：历史 100+ USD 现象是请求前预算把 JSON/base64 字节误当 Token 后产生的错误预算和 402，不是成功后的实际扣款；修复提交 `e16a67a5` 已于 2026-07-18 部署。当前仍有四个根问题：套餐预算不通过会跳过余额直接尝试流量卡、无授权快照入口仍会响应后重新选来源、套餐没有并发 reservation、图片编辑输入预算按每张 `23719` Token 粗估。另有未启用的 CLIProxy usage event 路径硬编码余额并使用独立 `cliproxy:` 请求 ID，存在错来源和双计费风险。建议统一为单一预授权决策器，按“套餐 -> 余额 -> 流量卡”选择完整请求唯一来源，结算层禁止改源；结果见 `docs/ai/context/20260719-201010-billing-source-priority-and-image-budget-investigation_CN.md`。
- 2026-07-19 已完成 Sub2API 与 CLIProxyAPI Docker 网络调查：当前本地 `cliproxyapi-local-dev` 已是 Docker 容器，实际端口为 8317，不是 8137；现状通过宿主机发布端口和 `host.docker.internal` 双向通信。目标架构应保持两个 Compose project 独立，新增只连接 Sub2API 与 CLIProxyAPI 的环境专用外部 bridge 网络，通过稳定服务别名直连，不让 CLIProxyAPI 进入 PostgreSQL/Redis 网络；生产保留 TLS，但应从“自签名端点证书同时作为信任锚并打包进 Sub2API”改为内部 CA 与叶子证书分离，叶子 SAN 覆盖 Docker 服务名。未改运行态，结果见 `docs/ai/context/20260719-192431-sub2api-cliproxyapi-docker-network-investigation-result_CN.md`。
- 2026-07-19 已在 Windows/Docker Desktop 启动隔离的本地开发链路：附件 PostgreSQL 18.4 custom dump 恢复到 `sub2api-postgres-dev`，Redis 从空库启动并由应用重建缓存，`sub2api-dev` 绑定 `127.0.0.1:8080`，`cliproxyapi-local-dev` 绑定 HTTPS `127.0.0.1:8317`；CLIProxyAPI 使用仓库内空 `auths/`，明确不读取本机全局账号池，用户后续自行添加账号。最小模型请求已验证到达 CLIProxyAPI，并按预期因空账号池返回 502。过程中修复四份 Compose 的 Redis 多行启动命令失效和空密码健康检查 AUTH 噪声，结果见 `docs/ai/context/20260719-181758-sub2api-cliproxyapi-local-development-result_CN.md`。
- 2026-07-19 已完成模型 API 上游失败契约：CLIProxyAPI 全账号冷却明确返回 429，Sub2API 转换为 `S2A-5004 / UPSTREAM_RATE_LIMITED / HTTP 429` 并保留 `Retry-After`；凭据不可用为 503、超时为 504、连接/无效响应为 502。规范见 `docs/ERROR_CONTRACT.md`，结果见 `docs/ai/context/20260719-213825-error-contract-unification-result_CN.md`。目录覆盖全项目标准，但当前已迁移的是模型 API 上游失败路径，普通 REST 端点仍需按域逐步迁移；已本地合并、未推送、未部署。
- 2026-07-20 已完成 Sub2API 前端 Material Relay 重设计：公开页/认证页/用户端/管理端统一实色表面、系统字体和克制动效，修复历史图片计费显示、图表空值、旧分页状态与相关测试断言；`pnpm typecheck`、`pnpm lint:check`、`pnpm test:run`、`pnpm build` 通过，且在 390×844 与 1440×900 下验收测试用户和管理员仪表盘均无横向溢出；结果见 `docs/ai/context/20260720-151230-sub2api-material-relay-frontend-redesign-result_CN.md`。
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
- OpenAI 模型请求必须按套餐额度、流量卡额度的顺序确定唯一计费来源；账户余额只用于购买、退款等资金业务，不参与模型请求计费。流量卡路径必须请求前预授权，成功响应前必须持久化 usage fact。
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
