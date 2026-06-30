# AGENTS.md 精简版上下文

> 来源：根目录 `AGENTS.md`，压缩时间：2026-06-29 09:50 JST。
> 本文件用于替代长流水账阅读入口；细节以被引用的历史上下文文档为准。

## 协作规则

- 默认使用中文；文档、说明、计划、总结、回复和代码注释都用中文，除非用户明确要求英文。
- 每次新对话先读 `/Users/wujianxiang/.codex/skills/using-superpowers/SKILL.md`，进入实现前先完成 design / plan，并把相关上下文保存到 `docs/ai/context/`。
- `docs/ai/context/` 下只新增文件，不覆写、重命名或删除历史文档；命名格式为 `YYYYMMDD-HHMMSS-文件名.md`。
- 完成实现或运行态操作后写结果上下文；合并、提交或收尾前运行 `git ls-files --others --exclude-standard docs/ai/context`，检查未跟踪上下文是否需要提交。
- 不记录完整 API Key、内部 token、HMAC secret、SMTP 密码、支付密钥或敏感 dump 内容。
- 代码偏好：函数式优先，组合优于继承；KISS、DRY；优先复用和小步重构；代码注释写原因，不写过程。
- 发现小设计问题可直接修；大问题原地加 TODO 并说明原因。不要 workaround 根因问题。

## 架构定论

- Sub2API 是唯一公网 API 入口，也是唯一用户 Key、计费和用量事实源。
- CLIProxyAPI 只做内网账号池、OAuth、协议转换和轮询上游；yui.web/shop 只保留展示、说明和跳转。
- 不要让 Sub2API、yui.web、CLIProxyAPI 同时对同一个用户 Key 做状态判定或扣费。
- 当前公网主链路：`Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> sub2api-candidate 127.0.0.1:18084 -> CLIProxyAPI 127.0.0.1:8317`。
- 容器内访问上游聚合入口用 `host.docker.internal:8317`。
- `aaccx.pw/shop` 归 yui.web；`aaccx.pw/v1/*`、`/api/*`、Sub2API 控制台路由和 `api.aaccx.pw` 归 Sub2API。

## 当前运行态

- 公网运行栈是 `sub2api-candidate`：应用端口 `127.0.0.1:18084->8080`，DB 为 `sub2api-candidate-postgres`，Redis 为 `sub2api-candidate-redis`。
- 18084 公网应用仍是已部署的 `traffic-card-fix` 版本；候选库仍为 191 migrations。替换公网应用前必须先备份候选库并验证迁移数。
- 本地 main-preview 运行在 `127.0.0.1:18080->8080`，用于蓝绿测试当前 main；最近一次镜像来自 `ddd4fb9a9`，preview 库为 194 migrations，最新迁移是 `158_enable_affiliate_default.sql`。
- 18085 SMTP 测试栈、旧 `sub2api` 18080、旧预览栈均已停止；不要把停止栈当作当前公网入口。
- Nginx 公网入口请求体上限已与 Sub2API 对齐为 256MB，并补了裸 `/responses`、`/chat/completions`、`/embeddings`、`/images/*` 代理；真实 100MB+ 公网请求仍可能受 Cloudflare 套餐限制。

## 支付与商品

- `/purchase` 只保留订阅套餐和 GPT 流量包购买；不再展示余额充值 UI，也不展示当前订阅详情。当前订阅状态在“我的订阅”页查看。
- 订阅、余额历史订单和 GPT 流量包都走 Sub2API 支付订单与 ZPay/EasyPay；自动履约必须校验签名、订单号、provider/merchant metadata，以及回调或查单金额等于本地 `pay_amount`。
- 当前 ZPay/EasyPay 使用支付宝-only，`payment_mode=popup`，走 `submit.php` 托管收银台；不要把 `mapi.php` 返回的 `qr.alipay.com` 原始码直接渲染给用户扫码。
- ZPay 1% 手续费通过运行态 `RECHARGE_FEE_RATE=1` 处理；不要修改套餐或流量包基础价，前端展示和后端支付用 `pay_amount` 承载实付价。
- 套餐分组：`codex-pool-19-usd`、`codex-pool-29-usd`、`codex-pool-49-usd`、`codex-pool-69-usd` 分别对应每日 19/29/49/69 USD。
- 当前售卖 `29 元订阅池` 是 `subscription_plans.id=1 -> group_id=2 -> codex-pool-19-usd`，不要误绑到 `codex-pool-29-usd`。
- `79 元订阅池` 基础价 79 元，30 天，对应 `codex-pool-69-usd`，每日 69 USD；1% 手续费下用户支付 79.79 元。

## 计费与 API

- CLIProxyAPI 是聚合上游，不是单个静态 OpenAI Key；Sub2API 上游账号需启用 `credentials.pool_mode=true`，并让 401/403/429 在同账号内重试。
- 运行态 `cliproxy-local-openai` 的 Sub2API 账号并发已从 3 调整为 10；未改用户并发、分组绑定或 CLIProxyAPI 配置。
- 订阅超限、余额为 0、订阅取消但有 GPT/OpenAI 流量卡时，计费资格应由 `BillingCacheService.CheckBillingEligibility()` 统一判定；中间件不能提前硬拒绝可由流量卡兜底的请求。
- 已公网实测：无 active subscription 但有 OpenAI 流量卡、以及订阅日限额耗尽但有流量卡，均可走流量卡扣费。
- `/usage-guide` 生图方法以真实图生图为主：公网完整 URL `https://api.aaccx.pw/v1/images/edits`，分离填写工具路径填 `/images/edits`，模型 `gpt-image-2`，支持 JSON `images[].image_url` 或 multipart `image=@...`。

## 注册、邮箱与邀请

- 公网 18084 的 SMTP 运行态配置已从 18085 同步，`smtp_from_name=天才程序员小站`，`smtp_password` 只允许以 `[CONFIGURED]` 形式确认。
- 邮箱验证码接口公网已验证能发送；SMTP 是运行态 DB settings，不随镜像或源码部署自动迁移。
- 注册重复邮箱预检已实现：`POST /api/v1/auth/precheck-register` 只读校验注册开放、保留邮箱、邮箱后缀和邮箱唯一性，不生成验证码、不写缓存、不入队邮件。
- 注册页手填优惠码入口固定隐藏；`aff_code` 仍通过链接、OAuth、session 传递；邀请返利默认开启。

## 数据库、迁移与部署风险

- 做候选预演或重部署时必须隔离 Docker Compose project，避免误停或重建公网容器。历史上曾因 project 未隔离误停公网栈。
- 当前公网事实源是 18084 候选栈及其数据库；恢复、克隆或迁移前先明确源库和目标库，避免把旧 bind 目录当成最新库。
- 历史警告仍有效：`deploy/postgres_data` bind 目录是旧库，不能当作最新生产 DB 克隆源；敏感 dump 位于 `deploy/backups/`，不要提交。
- 公网应用替换优先只换应用容器，保留 DB、Redis、nginx 指向；替换前备份 DB，替换后验证 `schema_migrations`、`/health`、公网入口和关键业务路径。
- 已应用迁移不要随意改内容或 checksum。`156_seed_codex_79_subscription_plan.sql` 保持原始 seed 价，`157` 修正基础价到 79.00，`158` 开启 affiliate 默认值。
- `xiaobianfuai@gmail.com` 是管理员和本机 Codex Local Key 所属账号，不要按普通用户删除。

## 维护与 Git

- 需要新增长期上下文时，只在 `docs/ai/context/` 创建新文件。
- 如果上下文过长，先沉淀到新的压缩文档，再继续精简 `AGENTS.md`。
- 不要把 `docs/ai/context/` 加回 `.gitignore`；未跟踪状态表示尚未提交，不表示被忽略。
- 若上下文文档暂不提交，回复里说明原因和后续处理方式。
- Git 远端：`origin=https://github.com/Wei-Shaw/sub2api.git` 是上游；`personal=https://github.com/cnYui/sub2api.git` 是用户个人 fork。保存当前工作分支优先推送到 `personal`，不要误推到 `origin`。

## 追溯索引

- 长期压缩记忆：`docs/ai/context/20260624-195608-agents-memory-compressed_CN.md`、`docs/ai/context/20260619-151920-sub2api-compressed-memory_CN.md`
- 本地分支收敛：`docs/ai/context/20260629-091206-merge-local-work-and-branches-result_CN.md`
- main-preview 最新重启：`docs/ai/context/20260629-093900-main-preview-restart-merged-main-result_CN.md`
- 公网 18084 应用替换：`docs/ai/context/20260627-215859-18084-app-image-replace-result_CN.md`
- 流量卡兜底修复：`docs/ai/context/20260627-222742-traffic-card-without-subscription-fix-result_CN.md`、`docs/ai/context/20260627-223338-cnfoxian-traffic-card-quota-exhausted-public-test-result_CN.md`
- 公网 SMTP 同步：`docs/ai/context/20260628-153845-sync-18085-smtp-config-to-18084-result_CN.md`
- ZPay 支付链路：`docs/ai/context/20260626-135223-zpay-alipay-runtime-config-result_CN.md`、`docs/ai/context/20260626-151923-zpay-1-percent-fee-runtime-result_CN.md`
- 候选预演事故与纠偏：`docs/ai/context/20260626-205933-sub2api-candidate-rehearsal-incident-diagnosis_CN.md`、`docs/ai/context/20260626-214600-latest-db-volume-candidate-rehearsal-result_CN.md`
- Nginx 413 与裸路由：`docs/ai/context/20260624-201352-413-payload-too-large-nginx-fix-result_CN.md`、`docs/ai/context/20260624-202759-usage-guide-image-url-413-result_CN.md`
