# AGENTS.md 压缩上下文

> 来源：项目根目录 `AGENTS.md`，压缩时间：2026-07-16 10:09 JST。
> 本文件是长期上下文入口；当前事实优先看本文件，具体证据和操作过程按文末索引追溯。
> 压缩时只读核对了容器、数据库迁移、Git 分支和远端；未修改运行态。

## 协作规则

- 默认使用中文；文档、说明、计划、总结、回复和代码注释均使用中文，除非用户明确要求英文。代码注释写原因，不写过程。
- 函数式优先，组合优于继承；KISS、DRY；优先复用或重构现有代码，不堆砌旁路方案。小设计问题直接修，大问题保留 TODO 并说明根因。
- 进入实现前先写 design / plan；完成后写 result。所有长期上下文只允许新建到 `docs/ai/context/`，命名为 `YYYYMMDD-HHMMSS-*.md`，不得覆盖、重命名或删除历史文件。
- 合并、提交或收尾前必须运行 `git ls-files --others --exclude-standard docs/ai/context`；未跟踪表示尚未提交，不表示被忽略。暂不提交的文档必须在回复中说明。
- 不记录完整 API Key、内部 token、HMAC secret、SMTP 密码、支付密钥、OAuth 凭据或敏感 dump 内容。
- 运行态写入、迁移、部署、支付和计费操作必须先确认真正目标、影响面、备份与回滚条件；不要用 workaround 掩盖架构不支持。

## 架构定论

- Sub2API 是唯一公网 API 入口，也是唯一用户 Key、计费和用量事实源。
- CLIProxyAPI（下文简称 CPA）只做内网账号池、OAuth、协议转换和上游轮询；yui.web/shop 只做展示、说明和跳转。三者不得同时对同一用户 Key 做状态判定或扣费。
- 当前真实链路：`Cloudflare Tunnel -> Nginx *:8080 -> sub2api-candidate 127.0.0.1:18084 -> CPA *:8317 TLS/H2 -> 多个 Codex OAuth 上游`。容器内访问 CPA 使用 `host.docker.internal:8317`。
- `aaccx.pw/shop` 归 yui.web；`aaccx.pw/v1/*`、`/api/*`、Sub2API 控制台路由和 `api.aaccx.pw` 归 Sub2API。
- 正式模型 API 只允许 `/v1/*`。裸 `/responses`、`/models`、`/chat/completions`、`/embeddings`、`/images/*` 和 `/backend-api/codex/responses*` 应在 Nginx 与后端返回 `400 INVALID_BASE_URL`，不做重定向。客户端 Base URL 使用 `https://api.aaccx.pw/v1`。
- CPA 是聚合上游，不是单个静态 OpenAI Key；Sub2API 上游账号需要池化凭据，并允许 401/403/429 在账号内部切换或重试。

## 当前运行态

- 2026-07-16 只读快照：运行容器为 `sub2api-candidate`、`sub2api-candidate-postgres`、`sub2api-candidate-redis`，均 healthy；应用镜像为 `sub2api-candidate:20260712-090731-e316ebf52`，监听 `127.0.0.1:18084->8080`。
- 当前数据库是 `sub2api-candidate-postgres/sub2api`，共有 197 条迁移，最新为 `161_seed_codex_149_199_subscription_plans.sql`。计费修复计划中的 migration 163/164 尚未进入当前运行库。
- 旧 18080、18085 和 preview 测试栈不属于当前公网事实源；做恢复、克隆或排障时不得把旧容器、旧 bind 目录或旧数据库当成生产数据。
- CPA 当前是宿主机进程，工作目录为 `/Users/wujianxiang/CodeSpace/CLIProxyAPI`，监听 8317，不是 Docker 容器。Sub2API 内置 CPA CA 位于 `backend/resources/certs/tls.crt`。
- Nginx 公网请求体上限为 256 MB；100 MB 以上请求仍可能受 Cloudflare 套餐限制。Nginx 正式模型路径已收敛到 `/v1/*`。
- 当前核心组件、PostgreSQL 和 Redis 都在同一台 8 GiB Mac 上，仍是单机单应用节点，不具备跨主机 HA。
- 2026-07-15 数据快照约有 107 个未删除 active 用户、约 35 API 日活、约 1.1 万成功请求/天；数据库中的 `usage_logs` 只代表成功落库下限，不能用于精确计算失败请求、网页 UV/PV 或网络 MB/GB。

## 容量与稳定性

- 当前双向总流量估算约 1.5-2.2 TB/月，可能计费出站约 0.8-1.3 TB/月，平均合计带宽约 6-7 Mbps；过去 30 天一分钟最大响应正文带宽约 19.73 Mbps。
- 按当前重度 Codex 行为线性放大到 300 API 日活，预计双向 15-22 TB/月、出站 8-13 TB/月、峰值 300-500 Mbps、在途请求约 320；系统设计目标至少 500 并发。
- 采购建议为 `16 vCPU / 32 GB / 500 GB-1 TB NVMe / 1 Gbps / 双向 20-30 TB 或出站 15-20 TB 每月`；8 vCPU/16 GB 只适合作为过渡。稳定规模应演进为双应用节点加独立 PostgreSQL。
- 扩容前必须先处理：CPA 每请求新建 Transport 导致 FD 接近上限、Nginx 单 worker/1024 connections、Sub2API 与 CPA 全局并发、请求取消传播、首响应硬期限、用量事实持久化、Redis 持久化和自动 PITR。单纯换大服务器不能解决这些问题。
- 服务器地区未定，采购前需分别测试候选机房到 Cloudflare、OpenAI 和中国运营商的真实路径。

## 计费与 API

- 普通用户 API Key 使用自动分组：`group_id=NULL`，请求时根据有效 OpenAI 套餐或 GPT/OpenAI 流量卡解析 request-scoped effective group；管理员仍可保留固定分组能力。
- 套餐到期不能联动停用 API Key，因为有效流量卡仍应可用。计费来源必须在订阅、余额、流量卡之间确定唯一来源，不能重复扣费或在不同层各自判定。
- 当前 P0 漏洞仍需根修：流量卡准入只检查余额大于 0，实际费用超过剩余额度时，上游可能已经向客户端返回 200，但异步 `RecordUsage` 因余额不足失败，费用、`usage_logs` 和不可变 usage fact 都不落库；内存任务还可能被 sample/drop。
- 已定修复顺序：第一阶段 migration 163 建立 `usage_facts`、durable outbox、fact-first settlement 和 HTTP/SSE 响应屏障；第二阶段 migration 164 增加 `reserved_usd`、reservation/items、请求前保守预算、原子结算和 debt gate。必须按第一阶段 -> 第二阶段 -> shadow -> canary -> 全量执行，不能只加固定 `0.01 USD` 门槛。
- 用户侧 524 主要来自首包超过 Cloudflare 约 120 秒；Nginx 同时常记 499，cloudflared 记 `context canceled`。Sub2API 流式路径使用 `context.WithoutCancel`，客户端失败后后台仍可能继续运行；必须增加首响应硬期限并正确传播取消。
- API Key 入口并发按 `api_key_id` 使用 Redis 槽，但上限仍复用可修改的 `users.concurrency`，不是代码硬编码 5。`concurrency<=0` 表示不限并发；管理员 `users.id=13` 当前为 0。唯一 Sub2API 上游账号并发已调为 100；CPA 普通请求限制为 100/100，图片为 10/10。
- GPT-5.4/5.5/5.6 Priority 正确目标倍率分别为 2x、2.5x、2x；reasoning tokens 已包含在 output tokens 内，不得重复计费。本地已有修复记录，但任何运行态判断都应以当前部署镜像和真实价格快照复核。
- `/v1/chat/completions` 标准 `messages` 应走 Chat Completions raw 路径；`/v1/responses` 应使用 `input`，混入 `messages` 时本地返回 400，不自动掩盖协议错误。

## 支付、订阅与退款

- `users.balance` 统一视为人民币站内余额。支付宝 1% 手续费由运行态费率处理：商品基础价写 `amount`，用户实付写 `pay_amount`；不要修改套餐或流量包基础价来吸收手续费。
- ZPay/EasyPay 当前使用支付宝托管收银台 `popup/submit.php`；不要直接渲染 `mapi.php` 返回的 `qr.alipay.com` 原始码。履约必须校验签名、订单号、provider/merchant metadata 和实付金额。
- 余额支付套餐/流量包必须在同一事务内完成条件扣款、订单完成和权益发放；余额不足不得透支。余额支付不产生邀请返利。
- 邀请返利现行目标：支付宝订单按不含手续费的 `amount` 计提，默认 8%，冻结 24 小时，有效期 365 天，单被邀请用户累计上限 100 元；“提现”仍是转入站内余额，不是外部打款。
- 用户无有效订阅时可购买任意套餐；已有有效订阅时只允许购买相同 `group_id` 续费，复用原订阅并从当前 `expires_at` 累加，不重置当前用量窗口。购买其他组应要求先退款，不创建订单、不扣余额、不自动发起退款。该方案目前是设计定论，实施前需核对是否已合入目标分支和运行态。
- 本地分支已完成退款状态机加固，但截至压缩时未部署公网：支付宝和余额订阅退款以不含手续费的商品本金为基数，按北京时间自然日计费，购买日算第 1 天；首次退款金额和稳定请求号持久化，`PENDING/UNKNOWN` 禁止自动重试，网关已退款但撤权未完成时只能续接权益收尾，权益变化或证据不足进入 `MANUAL_REVIEW`。
- 旧 SQLite 消费记录不应伪造成当前 `payment_orders`；只有存在可靠购买证据的当前订单可以准确回填 `subscription_id`，缺少网关交易号的历史订单只能人工核验退款。
- 当前所谓“余额不足后充值再买套餐”不是可靠组合支付：前端会清空待购上下文，充值成功只刷新余额，可能不再创建商品订单。真实组合支付尚属设计：单一商品订单、`payment_balance_holds` 冻结余额、支付宝只付精确差额；订单有效期 30 分钟，支付查询使用 `PAID/UNPAID/UNKNOWN` 三态，UNKNOWN 最多延长 5 分钟；释放余额后的迟到付款只转站内余额补偿，不重新扣余额或发原商品。
- 组合支付必须依赖退款状态机先集成；支付金额核对用 `gateway_amount`，支付宝退款用 `refund_gateway_amount`，Provider 创建需要 `CREATING`、数据库抢占、可恢复租约和稳定 `out_trade_no`。
- 套餐重要映射：29 元套餐实际对应 `group_id=2/codex-pool-19-usd`，不要误绑 `codex-pool-29-usd`；79 元套餐对应 `group_id=9/codex-pool-69-usd`。当前迁移还包含 149 元/每日 135 USD 与 199 元/每日 179 USD 套餐；新增套餐上线后必须确认唯一上游账号已绑定对应 group。

## 注册、邮箱与用户保护

- SMTP 是运行态数据库设置，不随镜像或源码自动迁移。当前 Gmail SMTP 已配置，密码只允许以 `[CONFIGURED]` 表示。
- 订阅到期提醒已启用；中英文模板当前都使用自然中文，主题为会员订阅剩余天数提醒，正文末尾只保留“这是一封自动提醒邮件”，不再包含退订链接文字或 `unsubscribe_url`。
- 后台任务每分钟扫描 active 订阅，在剩余完整 24 小时块为 7/3/1 天时发送并按订阅、收件人和档位去重。
- 注册重复邮箱预检接口只读校验注册开放、保留邮箱、后缀和唯一性，不生成验证码、不写缓存、不入队邮件。注册手填优惠码入口隐藏，`aff_code` 仍可通过邀请链接、OAuth 和 session 传递。
- `xiaobianfuai@gmail.com/users.id=13` 是管理员和本机 Codex Local Key 所属账号，不得按普通用户删除；其 LOCAL Key 当前在 Sub2API 入口层不限并发。
- `xunskyler@gmail.com/users.id=19` 已因计费漏洞导致大量未落库成功响应而被设置为 disabled；两把 API Key 保留 active 记录但鉴权返回 `USER_INACTIVE`。恢复前必须先确认计费漏洞处置方案。
- 给测试账号加余额、撤销套餐或执行真实请求前，必须让用户明确邮箱或 user ID，不得从相邻上下文推断目标。历史上误把正常用户当测试账号，虽已回滚，但该约束长期有效。

## 数据库、迁移与部署风险

- 任何运行态数据库写入、支付配置修改、用户权益调整或应用替换前，先创建 PostgreSQL dump，权限设为 600，并用 `pg_restore -l` 验证可读；涉及 Redis 会话、并发或缓存状态时同时备份或明确可丢失范围。
- Redis 当前曾以 `--save "" --appendonly no` 运行，启动/重启前不能假设有持久化；需要主动导出 RDB。不要把清缓存当作无风险步骤，鉴权和并发缓存要按精确 key 失效并广播 L1。
- Docker Compose project 必须隔离。2026-06-26 曾因候选 project 与公网 project 冲突误停并重建公网应用、Postgres、Redis，造成 refresh token 会话丢失；再次候选预演前必须先读事故文档。
- 当前运行数据目录曾位于旧 candidate rehearsal worktree，不能根据当前 `main/deploy/candidate` 猜挂载路径。操作前用 `docker inspect` 确认容器、volume、workdir、env 和真实数据源。
- `deploy/postgres_data` 历史 bind 目录不是最新生产库；恢复或克隆必须以当前运行容器 volume 或已验证 dump 为源。`deploy/backups/` 含敏感数据，不得提交。
- 公网发布优先只替换 `sub2api-candidate` 应用容器，保留 PostgreSQL、Redis、Nginx 和 Cloudflare Tunnel；发布前后核对迁移、关键设置、health、控制台、购买页和真实模型路径。数据层回滚需要单独授权，不能随应用回滚自动执行。
- 已应用 migration 不得修改内容或 checksum。历史上 156 seed 被改动曾触发 checksum mismatch；后续修正必须新建 migration。
- 不要在未确认备份和引用关系前执行 `docker volume prune` 或删除旧数据 volume。

## 维护与 Git

- 2026-07-16 快照分支为 `codex/fix-user-concurrency-zero-form`，HEAD `737e8ed27`；根目录 `AGENTS.md` 已有用户改动，本次压缩不修改它。
- Git 远端：`origin=https://github.com/Wei-Shaw/sub2api.git` 是上游，`personal=https://github.com/cnYui/sub2api.git` 是用户个人 fork。保存工作分支优先推送 `personal`，不要误推 `origin`，不要无理由 force push。
- 当前 `docs/ai/context/` 已存在多份 2026-07-14/15 未跟踪文档；提交功能或上下文时要明确纳入范围，避免遗漏，也不能擅自删除或覆盖。
- 本地已完成但未部署的分支结果不能写成公网已生效；部署前必须重新检查目标分支、镜像源码、迁移差异和运行态设置。

## 追溯索引

- 全链路安全审计与白皮书：`docs/ai/context/20260713-134754-relay-architecture-security-hardening-whitepaper_CN.md`
- 300 API 日活容量评估：`docs/ai/context/20260715-191239-300-dau-bandwidth-server-capacity-assessment_CN.md`
- 流量与用户峰值审计：`docs/ai/context/20260715-174343-database-traffic-users-peak-audit_CN.md`
- 当前 524 根因：`docs/ai/context/20260715-123122-current-524-log-diagnosis_CN.md`
- 用量事实与流量卡预授权设计：`docs/ai/context/20260715-090554-traffic-credit-preauthorization-and-durable-usage-design_CN.md`
- 计费两阶段计划：`docs/ai/context/20260715-092959-openai-usage-fact-durable-outbox-implementation-plan_CN.md`、`docs/ai/context/20260715-093000-traffic-credit-reservation-debt-gate-implementation-plan_CN.md`
- 退款状态机结果：`docs/ai/context/20260714-232208-refund-business-logic-hardening-result_CN.md`
- 同套餐续费设计：`docs/ai/context/20260715-093256-subscription-same-plan-renewal-design_CN.md`
- 组合支付设计、自审与超时修正：`docs/ai/context/20260715-093416-alipay-balance-hybrid-payment-design_CN.md`、`docs/ai/context/20260715-094616-alipay-balance-hybrid-payment-design-self-review_CN.md`、`docs/ai/context/20260715-092546-hybrid-payment-timeout-grace-design-correction_CN.md`
- 到期计费漏洞复现与用户阻断：`docs/ai/context/20260714-201507-xunskyler-expired-subscription-api-key-billing-gap_CN.md`、`docs/ai/context/20260714-202700-xunskyler-user-disabled-request-block-result_CN.md`
- 组合支付前端断链诊断：`docs/ai/context/20260714-201654-alipay-balance-resume-subscription-failure-diagnosis_CN.md`
- 并发设计与运行态调整：`docs/ai/context/20260711-211142-sub2api-cliproxyapi-api-key-concurrency-design-audit_CN.md`、`docs/ai/context/20260711-220953-sub2api-upstream-account-concurrency-100-result_CN.md`
- 正式 `/v1/*` 路由：`docs/ai/context/20260708-164114-formal-v1-only-api-design_CN.md`、`docs/ai/context/20260708-205014-nginx-formal-v1-api-audit-result_CN.md`
- 候选预演事故与数据源纠偏：`docs/ai/context/20260626-205933-sub2api-candidate-rehearsal-incident-diagnosis_CN.md`、`docs/ai/context/20260626-214600-latest-db-volume-candidate-rehearsal-result_CN.md`
- 上一版压缩记忆：`docs/ai/context/20260629-095000-agents-memory-condensed_CN.md`、`docs/ai/context/20260624-195608-agents-memory-compressed_CN.md`
