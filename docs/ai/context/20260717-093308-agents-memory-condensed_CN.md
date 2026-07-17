# Sub2API AGENTS 压缩记忆

> 本文由 2026-07-17 根目录 `AGENTS.md` 压缩迁移而来。根 `AGENTS.md` 只保留短入口；长期上下文以本文和历史 `docs/ai/context/` 文档为准。

## 协作规则

- 默认使用中文；文档、说明、总结、计划、回复和代码注释都使用中文，除非用户明确要求英文。
- 代码注释写原因，不写过程。
- 表达简洁直接，不写多余总结。
- 函数式优先，组合优于继承；TS/JS 中避免 OOP。
- 新功能优先复用或重构现有代码，不堆砌；遵循 KISS、DRY 和 ai-coding-discipline。
- 发现小的设计不合理可直接重构；大的设计问题原地加 TODO 并说明原因。
- 从第一性原理解构问题，先明确必须解决什么，再决定怎么做；警惕 XY 问题，不做 workaround。
- 架构设计参考 ddia-principles 和 software-design-philosophy。
- 进行修改、架构设计、技术选型时，必须在 `docs/ai/context/` 新建 design/plan/result 上下文；只创建新文件，不覆盖、重命名或删除历史文件。

## 架构定论

- Sub2API 是唯一公网 API 入口，也是唯一用户 Key、计费和用量事实源。
- CLIProxyAPI 只作为内网账号池、OAuth、协议转换和轮询上游；它不是单个静态 OpenAI Key。
- yui.web / shop 只保留展示、说明和跳转。
- 不要让 Sub2API、yui.web、CLIProxyAPI 同时对同一个用户 Key 做状态判定或扣费。
- 当前主链路：`Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> sub2api-candidate 127.0.0.1:18084 -> CLIProxyAPI 127.0.0.1:8317`。
- `aaccx.pw/shop` 归 yui.web；`aaccx.pw/v1/*`、`/api/*` 和 Sub2API 控制台路由归 Sub2API；`api.aaccx.pw` 也是 Sub2API 入口。
- 正式模型 API 只保留 `/v1/*`；裸 `/responses`、`/models`、`/chat/completions`、`/embeddings`、`/images/*` 应由 Nginx / 后端返回明确 `400 INVALID_BASE_URL` 或规范提示。
- Nginx 公网入口请求体上限已与 Sub2API 对齐为 256MB；真实 100MB+ 公网请求仍可能受 Cloudflare 套餐上传上限影响。
- `xiaobianfuai@gmail.com` 是管理员和本机 Codex Local Key 所属账号，不要按普通用户删除。

## 当前运行态

- 当前公网由 18084 候选环境承接：应用容器 `sub2api-candidate`，数据库 `sub2api-candidate-postgres`，Redis `sub2api-candidate-redis`。
- 容器内访问上游聚合入口为 `host.docker.internal:8317`。
- 旧 `sub2api` 18080 曾多次作为 preview / 历史生产链路存在；判断运行态时以当前 Nginx 指向和 health 验证为准，不要混淆 18080、18084、18085。
- 18085 曾是 SMTP 测试栈，后续按用户要求停止；若需要重新启用，必须确认独立 DB/Redis，不能误动公网 18084。
- 历史候选预演曾因 Docker Compose project 未隔离误停/重建公网栈；任何候选/恢复操作前必须确认 Compose project、容器名、volume、端口和 Nginx 指向。
- 生产数据历史曾保留在 Docker named volume `deploy_postgres_data`，而 `deploy/postgres_data` bind 目录曾是旧库；恢复或克隆前必须先验证数据源，不要凭目录名判断。
- 运行态 SMTP 是数据库 `settings`，不会随镜像/源码替换自动进入 18084；密码只能记录为脱敏状态。
- `backend/resources/certs/tls.crt` 是 CLIProxyAPI TLS CA 相关文件；注意区分代码资产和运行态证书更新。

## 支付、商品与退款

- 订阅、余额充值和 GPT/OpenAI 流量包购买全部走 Sub2API 支付订单与 ZPay/EasyPay。
- 当前 ZPay 支付宝实例应使用 `popup/submit.php` 托管收银台；不要把 ZPay `mapi.php` 返回的 `qr.alipay.com` 原始码直接渲染给用户扫码。
- 自动履约必须以签名正确、订单号匹配、provider/merchant metadata 匹配、回调或查单金额等于本地 `pay_amount` 为准。
- ZPay 1% 手续费通过运行态 `RECHARGE_FEE_RATE=1` 处理；不要修改套餐/流量包基础价格。后端订单用 `pay_amount` 承载实付金额，前端展示含手续费实付价。
- `/purchase` 只负责订阅和 GPT 流量包购买，不展示当前订阅详情；当前订阅状态在“我的订阅”页查看。
- 用户无有效订阅时可购买任意套餐；已有有效订阅时，只允许购买相同 `group_id` 作为续费，并从当前 `expires_at` 累加有效期，不重置当前用量窗口。
- 购买不同 `group_id` 必须直接提示“当前套餐仍在有效期内，如需更换套餐，请先退款后再购买”，不创建订单、不扣余额、不自动退款或自动切换。
- 外部支付订单在创建后、付款回调前若用户获得不同 `group_id` active 订阅，首次发放前也必须拒绝并把订单标记为 `FAILED`；已发放过的订单仍按审计/note 幂等恢复。
- 支付宝 + 余额组合支付已在本地分支实现：套餐/流量包下单时在同一商品订单内冻结可用余额，只让支付宝收差额；`PAID/UNPAID/UNKNOWN` 三态决定捕获、释放或短暂确认。
- 组合支付 UNKNOWN 最多确认到 `expires_at + 5m`；余额释放后的迟到付款转入站内余额并记为 `COMPENSATED`，不重新扣已释放余额，不发原商品。
- 退款状态机定论：支付宝和余额订阅退款以不含手续费的 `payment_orders.amount` 为基数，按北京时间自然日计费；购买日算第 1 天；未知结果禁止自动重试，资金已退但撤权失败时重试只能撤权。
- 余额退款必须余额、订阅、订单同事务；网关成功必须先落库再撤权；共享或购买后权益变化应转 `MANUAL_REVIEW`。
- 运行态曾开启订阅到期提醒邮件，模板为自然中文并删除退订链接文字；SMTP 密码不得进入文档、提交或日志摘要。

## 套餐与商品映射

- 套餐分组名：
  - `codex-pool-19-usd`：每日 19 USD。
  - `codex-pool-29-usd`：每日 29 USD。
  - `codex-pool-49-usd`：每日 49 USD。
  - `codex-pool-69-usd`：每日 69 USD。
- 当前售卖套餐里 `29 元订阅池` 对应 `subscription_plans.id=1 -> group_id=2 -> codex-pool-19-usd`；不要误绑到 `codex-pool-29-usd`。
- `79 元订阅池` 基础价 79 元、30 天有效期，对应 `codex-pool-69-usd`，每日 69 USD；1% 手续费下展示和支付 79.79 元。
- 149/199 元订阅套餐已在本地 main 和公网 18084 相关发布历史中出现；判断当前是否上架必须看运行态 DB 与 migration 状态。
- `gpt-image-2` 图生图使用 `https://api.aaccx.pw/v1/images/edits`，分离填写工具路径填 `/images/edits`，支持 JSON `images[].image_url` 或 multipart `image=@...`。

## 计费与 API

- 订阅到期不能联动停用 API Key，因为有效流量卡必须继续可用。
- 根修复采用“请求前事务预授权 + 响应后不可丢失 usage fact / durable outbox”。
- 请求应先确定订阅、余额或流量卡中的唯一计费来源；订阅和余额不可用时，按最终出站请求、输出上限和同一价格快照计算流量卡保守预算。
- 流量卡预留使用 PostgreSQL `remaining_usd - reserved_usd` 与 `FOR UPDATE` 防止并发超卖；预算不足、定价不可用或存在 debt 时在上游前拒绝。
- 上游成功后必须先持久化 usage fact，再幂等结算并写 usage log；扣费不足保留 fact 为 debt，禁止财务任务被内存队列 drop/sample。
- 计费漏洞修复第一阶段已完成：migration `164`、`usage_facts` durable outbox、OpenAI/Anthropic/Images/Embeddings/WS 兼容入口在成功响应前同步持久化 fact。
- 计费漏洞修复第二阶段已完成：migration `165`、`traffic_credit_reservations/items`、`user_traffic_credits.reserved_usd`、请求前预授权、reservation 贯穿结算、debt gate。
- 生图实际 Token 计费设计/实现定论：OpenAI Responses/Chat/Images/图片编辑统一按上游可确认的主模型输入、缓存、文本输出、图片输入和图片输出 Token 结算。
- 生图主模型与图片工具模型分别定价，图片模型缺失时回退 `gpt-image-2`，缺失 Token 类别按 0 并记录 `billing_incomplete`；失败/取消/不完整响应有 Token 也先落 usage fact。
- 旧尺寸固定价和独立图片倍率字段已从运行时代码、API、Ent 和前端控件删除；多图不乘张数。
- 流量卡按 `expires_at, credited_at, id` 逐张预留和扣费；`billing.traffic_credit_minimum_reserve_usd=$0.01` 是唯一耗尽门槛。
- 单卡从 `>0.01` 降到 `<=0.01` 时按 `credit_id` 幂等写耗尽事件；`/auth/me` 投递 pending event ids；前端右上角仅首次弹“流量卡已用完”并批量 ack。
- Dashboard 套餐额度实时展示设计：新增不可变 `subscription_entitlement_periods` 记录每次发放周期和每日额度快照；Dashboard 分子按 `usage_facts` 与未对应 fact 的历史 `usage_logs` 去重聚合 `actual_cost`，可超过分母。
- Dashboard quota 初始 `/usage/dashboard/stats` 附带，轮询用轻量 `/usage/dashboard/quota`；历史仅回填可证明周期，不可证明边界显式降级为 rolling 30 天口径。
- Sub2API 模型入口并发已改为按 `api_key_id` 使用 Redis 槽，每把 Key 复用默认并发 5；用户创建多把 Key 可以扩大并发。
- Sub2API 唯一上游账号 `accounts.id=1/cliproxy-local-openai` 运行态并发曾从 10 调整到 100，与 CLIProxyAPI 普通 100/100 对齐；图片并发仍由 CLIProxyAPI endpoint override 限制为 10。
- CLIProxyAPI `inbound-limits per-api-key=5` 曾误伤，因为按 Sub2API 共用上游 Key 聚合计数会把全站卡成 5；不要用该方式实现用户 Key 并发。
- CLIProxyAPI 上游账号需启用 `credentials.pool_mode=true`，并让 401/403/429 在同账号内重试。
- GPT-5.4 Priority 为 2x，GPT-5.5 为 2.5x，GPT-5.6 sol/terra/luna 为 2x；Standard、Flex、长上下文、分组倍率和 reasoning token 口径另有既定规则。
- GPT-5.6 只展示和调用完整模型名 `gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna`，不展示裸 `gpt-5.6` alias；缺价不得回退到 GPT-5.4。
- Dashboard “可用模型”曾按用户要求仅展示 `gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna`、`gpt-5.5`、`gpt-5.4`、`gpt-image-2`。

## API Key、自动分组与客户端问题

- 普通用户前端不选择/提交 API Key 分组，新建 Key 默认 `group_id=NULL`。
- 后端按 request-scoped `effective_group` 解析：有 active OpenAI 套餐时选择当前套餐 group，无套餐但有 GPT/OpenAI 流量包时选择内部 `traffic-pack-openai` 入口 group；非 OpenAI 固定 Key 和管理员固定分组能力保留。
- 旧 OpenAI Key 已通过 `159_auto_api_key_effective_group.sql` 迁移为自动 Key；判断是否已上线看运行态 migration。
- 自动 Key 必须支持规范 `/v1/models` 等 OpenAI 端点；不支持的 `/v1beta` 与 `/antigravity/*` 应返回明确 `AUTO_KEY_UNSUPPORTED_ENDPOINT`。
- CCSwitch / Codex 导入 OpenAI/Codex endpoint 必须恰好带一个 `/v1`；根地址 `usageBaseUrl` 保持 `{{baseUrl}}/v1/usage`。
- 客户端 401 多数发生在 Key 查找阶段，早于套餐/额度判断；优先检查是否仍使用已删除 Key、掩码 Key、不完整 Key、错误 Authorization 头。
- `/v1/responses/responses` 是客户端路径重复配置错误；服务端不应为其做静默兼容。
- `/v1/responses` 如果收到 Chat Completions 的 `messages` 且无 `input`，应本地返回 400 并提示使用 `input` 或改走 `/v1/chat/completions`。
- `thinking_signature_invalid` 多与旧会话 encrypted reasoning 和多 OAuth 账号轮询有关，不应简单归责用户；用户可新建 task，服务端应正确分类并清理重试。
- `Selected model is at capacity. Please try a different model.` 是 OpenAI 官方/上游容量不足，不是 Sub2API 自造拦截。
- `/v1/chat/completions` 标准 body 在 OpenAI APIKey 账号自动模式下应优先 raw 直转上游，避免转换路径导致 `stream usage incomplete: missing terminal event`。

## 注册、邮箱与邀请

- 注册重复邮箱预检已实现：`POST /api/v1/auth/precheck-register` 只读校验注册开放、保留邮箱、邮箱后缀和邮箱唯一性，不生成验证码、不写验证码缓存、不入队邮件。
- Gmail SMTP 配置是运行态 DB settings；同步到 18084 时只更新 settings，不重建镜像、不重启容器、不改 DB/Redis/Nginx。
- 邀请返利码为 `aff_code`，区别于注册准入 `invitation_code`。
- 用户页 `/affiliate` 可复制 `/register?aff=...` 邀请链接；返利默认 8%、冻结 24 小时、有效期 365 天、单被邀请用户累计上限 100 元；历史曾从 20% 调整并回算。
- 用户侧返利展示使用人民币 `¥`；`users.balance` 统一视为人民币余额。
- 余额充值支付宝-only 且 1:1 人民币入账；余额支付购买套餐/流量包不产生返利，支付宝完成订单按 `amount` 返利。

## 订阅窗口与额度

- 东八区订阅日窗口规则很敏感。历史问题包括 `daily_window_start IS NULL` 时只补窗口、不清零旧 `daily_usage_usd`，以及管理端列表不会触发 API 计费入口刷新。
- 当前目标：完成时记账窗口自愈，按 `usage_logs.created_at` / `CompletedAt` 计入当天；active 订阅后台 daily window 校准 scheduler；展示读路径对 `NULL + usage > 0` 归一化。
- 订阅日/周/月窗口初始化或跨窗口刷新必须同步清零 usage，并有回归测试覆盖。
- 在运行态手动校准 active 订阅用量前必须备份 Postgres，并按今天 0 点后的 `usage_logs.total_cost` 聚合值校准；完成后清理 `billing:sub:*` 缓存。

## 部署、数据库与高风险操作

- 任何修改运行态 DB、Redis、容器、Nginx 或公网链路前，必须先明确目标环境、写计划、备份、验证备份可读，并说明回滚边界。
- 替换公网应用的推荐方式通常是只替换 `sub2api-candidate` 应用容器，保留 `sub2api-candidate-postgres`、`sub2api-candidate-redis`、Nginx 指向和 Cloudflare Tunnel。
- 发布前后至少验证：容器 health、`18084/health`、`8080/health`、`api.aaccx.pw/health`、关键页面、关键 API、migration 数、日志无 panic/DB/Redis/account-select 关键错误。
- 数据层回滚必须另行授权；不能因为应用发布失败就擅自恢复整库 dump。
- 候选 sanitize 不能把 `payment_enabled=false` 写入克隆库，否则 HTML 注入配置和前端路由守卫会隐藏 `/purchase`；只禁用具体 provider、可见支付方式、SMTP 和监控副作用。
- 旧预演事故说明：Docker Compose project 未隔离会误停/重建公网容器；这是高危红线。
- 不要在文档、提交或日志中记录完整 API Key、内部 token、HMAC secret、SMTP 密码、支付密钥。
- `deploy/backups/` 中的 dump 文件只记录路径、大小、权限和可读性，不复制内容。

## 运维与容量

- 300 API 日活评估：当前约 35 日活、约 1.1 万成功请求/天、成功请求峰值并发约 32；线性放大到 300 日活预计双向 15-22 TB/月、出站 8-13 TB/月、峰值 300-500 Mbps、在途请求约 320。
- 采购建议曾定为 `16 vCPU / 32 GB / 500 GB-1 TB NVMe / 1 Gbps / 双向 20-30 TB 或出站 15-20 TB 每月`；最低 8 vCPU/16 GB 只适合作为过渡。
- 单纯换大服务器不能解决 CPA Transport/FD、Nginx 单 worker/1024 connections、Sub2API 与 CPA 全局并发、取消传播、首响应期限和用量事实持久化问题。
- 当前 524 根因多为上游慢首包叠加缺少首响应硬期限和取消传播；`context.WithoutCancel` 会导致客户端失败后后台仍继续执行并落库。
- 全链路安全审计 P0：不可变 usage fact + durable outbox、禁止财务任务丢弃、凭据日志、匿名 WS、CPA 权限、Transport/FD、TLS/首响应期限、单机恢复和观测。

## 重要用户与历史处置索引

- `2262423876@qq.com/users.id=114`：持续重连只读诊断显示服务端请求均 200，证据指向客户端本地处理；见 `docs/ai/context/20260716-224059-2262423876-reconnect-diagnosis_CN.md`。
- `1772475120@qq.com/users.id=91`：失败主要来自 `/v1/responses/responses` 路径重复和模型名不支持；见 `docs/ai/context/20260715-190855-user91-request-failure-log-diagnosis_CN.md`。
- `xunskyler@gmail.com/users.id=19`：曾复现套餐到期、余额 0、流量卡极小余额仍 200 且记账失败，后按用户要求阻断全部 API Key；见 `docs/ai/context/20260714-201507-xunskyler-expired-subscription-api-key-billing-gap_CN.md`、`docs/ai/context/20260714-202700-xunskyler-user-disabled-request-block-result_CN.md`。
- `1510623550@qq.com/users.id=41`：支付宝补差流程未续接导致余额留存，已按授权核销 74 元；见 `docs/ai/context/20260714-212435-user41-balance-reconciliation-result_CN.md`。
- `xiaobianfuai@gmail.com/users.id=13`：唯一 LOCAL Key 已设置入口层不限并发；也是管理员和本机 Codex Local Key 所属账号。
- `2799523972@qq.com/users.id=31`：常用测试账号，曾用于余额购买/退款、流量卡、真实公网验收；操作前仍需确认当前状态。
- `xinlise@gmail.com`：历史有退款、套餐、CCSwitch 代理、thinking signature、真实公网 Key 测试等多次记录，不能把单次错误简单归因为账号/套餐不可用。
- `405045701@qq.com`：29 元购买后 401 排查显示多为客户端仍使用已删除/错误 Key；当前 Key/订阅需按运行态再核对。

## 追溯索引

近期高优先级：

- 生图实际 Token 计费与逐张流量卡耗尽提醒：`docs/ai/context/20260717-010321-image-token-billing-traffic-card-per-card-result_CN.md`
- Dashboard quota 设计：`docs/ai/context/20260716-153520-dashboard-subscription-quota-realtime-design_CN.md`
- Dashboard quota 实施计划：`docs/ai/context/20260716-155243-dashboard-subscription-quota-realtime-implementation-plan_CN.md`
- 生图服务计费调研：`docs/ai/context/20260716-142225-image-generation-pricing-research_CN.md`
- 流量卡 durable outbox / reservation：`docs/ai/context/20260716-113813-openai-usage-fact-durable-outbox-result_CN.md`、`docs/ai/context/20260716-133255-traffic-credit-reservation-debt-gate-result_CN.md`
- 同套餐续费与跨套餐拦截：`docs/ai/context/20260715-222742-subscription-same-plan-renewal-result_CN.md`、`docs/ai/context/20260715-224354-subscription-same-plan-renewal-review-fix-result_CN.md`
- 支付宝 + 余额组合支付：`docs/ai/context/20260715-231613-alipay-balance-hybrid-payment-result_CN.md`
- 退款业务逻辑修复：`docs/ai/context/20260714-232208-refund-business-logic-hardening-result_CN.md`
- 中转站安全白皮书：`docs/ai/context/20260713-134754-relay-architecture-security-hardening-whitepaper_CN.md`

运行态与部署：

- 当前 18084 公网链路：`docs/ai/context/20260627-102157-18084-public-candidate-chain-agents-update-result_CN.md`
- 候选预演事故：`docs/ai/context/20260626-205933-sub2api-candidate-rehearsal-incident-diagnosis_CN.md`
- 最新 DB volume 纠偏：`docs/ai/context/20260626-214600-latest-db-volume-candidate-rehearsal-result_CN.md`
- 自动 API Key 上线：`docs/ai/context/20260705-225115-auto-api-key-effective-group-result_CN.md`
- 公网 main redeploy：`docs/ai/context/20260706-081100-public-main-redeploy-result_CN.md`
- 正式 `/v1/*` API 约束：`docs/ai/context/20260708-164114-formal-v1-only-api-design_CN.md`、`docs/ai/context/20260708-204335-public-18084-formal-v1-api-redeploy-result_CN.md`

支付与余额：

- ZPay 支付配置：`docs/ai/context/20260626-135223-zpay-alipay-runtime-config-result_CN.md`
- ZPay 1% 手续费：`docs/ai/context/20260626-151923-zpay-1-percent-fee-runtime-result_CN.md`
- 人民币余额与邀请返利：`docs/ai/context/20260708-090140-rmb-balance-payment-affiliate-rebate-result_CN.md`
- 自动套餐退款：`docs/ai/context/20260709-101745-auto-subscription-refund-result_CN.md`

API 与模型：

- CCSwitch `/v1` endpoint 修复：`docs/ai/context/20260711-210311-ccswitch-v1-endpoint-fix-result_CN.md`
- GPT-5.6 模型与计费：`docs/ai/context/20260710-104141-gpt56-priority15-billing-implementation-result_CN.md`
- OpenAI Priority 价格修复：`docs/ai/context/20260711-213759-openai-official-priority-pricing-fix-result_CN.md`
- Chat Completions raw routing：`docs/ai/context/20260710-111508-961109198-chat-stream-502-root-fix-result_CN.md`

旧压缩记忆：

- `docs/ai/context/20260624-195608-agents-memory-compressed_CN.md`
- `docs/ai/context/20260619-151920-sub2api-compressed-memory_CN.md`

## 维护与 Git

- 需要新增长期上下文时，只在 `docs/ai/context/` 创建新文件。
- 进入实现前先写 design/plan 上下文；完成后写结果上下文。
- 每次合并功能分支、提交 main 或做收尾前，必须运行：

```bash
git ls-files --others --exclude-standard docs/ai/context
```

- 确认无敏感信息后，把上下文文档纳入同一次功能提交，或单独做 `docs: archive ai context` 提交。
- 不要把 `docs/ai/context/` 加回 `.gitignore`；未跟踪状态表示尚未提交，不表示被忽略。
- 如果某个上下文文档暂不提交，必须在回复里说明原因和后续处理方式。
- 若上下文再次过长，先沉淀到新的压缩文档，再继续精简根 `AGENTS.md`。
- Git 远端：`origin=https://github.com/Wei-Shaw/sub2api.git` 是上游；`personal=https://github.com/cnYui/sub2api.git` 是用户个人 fork。保存当前工作分支优先推送到 `personal`，不要误推到上游 `origin`。
