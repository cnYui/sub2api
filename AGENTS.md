# 项目协作约定

> 本文件每个会话完整载入上下文。只写**当前生效的事实、规则和坑**。
>
> 一次性执行流水（发放、刷新、取消、重启、镜像替换、单用户核查）不进本文件，只写
> `docs/ai/context/`。压缩前的 392 行全文与逐条取舍判据归档在
> `docs/ai/context/20260905-112834-agents-md-compression_CN.md`。
>
> 新增条目前先回答一句：**下一个会话读不到这条，会做错什么？** 答不上来就只写 context 文档。

## 一、协作约定

- **本仓库是公开仓库**（`cnYui/sub2api`，`Wei-Shaw/sub2api` 的 fork）。任何 IP、桶名、
  Tunnel ID、账号地址一律写 `${变量名}`，真实值只进 `deploy/ops.env`（已 gitignore）。
  历史上多处文档误称「私有」，据此做的判断都要重查。
- 默认使用中文；代码注释只说明原因，不复述代码。
- 支付订单的金额、退款金额和订单状态**以服务端为准**，前端金额只用于展示。
- 退款必须绑定创建订单时的支付服务商实例，并保留可审计的订单状态变化。
- 设计与实现上下文写入 `docs/ai/context/`，**历史文档只新增不覆盖**。
- **`docs/ai/context/` 不是永久存储：本机 Windows 计划任务 `Prune AI Context Docs` 每天 06:00 自动删除超过 15 天的文档**（跑 `scripts/prune-ai-context.ps1`）。它**只删本地文件，不 commit 也不 push**，删完留在工作区。
  所以**长期生效的结论必须写进本文件**，只写 context 文档等于 15 天后丢失。
  两条护栏：被 AGENTS.md 或 `docs/` 下其它 Markdown **引用到的文档不会被删**（上面那些手册链接因此安全）；正文带 `<!-- prune:keep -->` 的也不会被删。预演用 `-DryRun`，日志在 `logs/prune-ai-context.log`。
- 生产数据变更必须写 `payment_audit_logs` 审计，并处理认证/余额缓存失效。
- **SSH 私钥不叫 `id_*`**，部署密钥按机器命名（文件名见 `deploy/ops.env` 的 `OPS_SSH_KEY_FILE`）。用 `ls ~/.ssh/*.pem ~/.ssh/id_*` 过滤会漏掉它、误判为「无 SSH 访问」——**要 `ls ~/.ssh/` 全量看**。默认 `id_ed25519` 确实被服务器拒绝，容易据此得出确定的错误结论。
- **生产 VPS 远程操作手册**：`docs/ai/context/20260905-173123-vps-ssh-db-operations-runbook_CN.md`。连接方式、psql 用法、写操作的事务模板、已知表结构坑、缓存失效、核验清单。**动生产数据库前先读它。**
- **部署生产镜像**：`docs/ai/context/20260905-200812-first-ghcr-image-deploy_CN.md`。生产已切到 GHCR 镜像，换版本只需改 `${OPS_DEPLOY_DIR}/.env` 的 `IMAGE_TAG` 再 `docker compose -f docker-compose.yml -f docker-compose.vps.yml up -d sub2api`。**重启前先 `docker compose config` 渲染检查** image / 端口绑定 / `BILLING_FINAL_MULTIPLIER` / secrets 四项。只 `prune -f` 不要 `prune -a`，否则丢回滚镜像。
- 改公网 Nginx 必须先 `nginx -t` 通过再 `reload`；**不要重建 Cloudflare Tunnel**。
- 数据库迁移已应用后内容不可改（有 checksum 保护），只能新增迁移号。当前最大迁移号 `212`。

## 二、当前部署拓扑（2026-09-05）

> **本仓库是公开仓库。** 运维敏感值一律用 `${变量名}` 占位，实际值只写在
> `deploy/ops.env`（已 gitignore，模板见 `deploy/ops.env.example`）。
> 不要把 IP、桶名、Tunnel ID 写回任何被跟踪的文件。

```
aaccx.pw / www.aaccx.pw / api.aaccx.pw
  → Cloudflare Tunnel ${OPS_TUNNEL_ID}
  → DediOne 洛杉矶 VPS ${OPS_VPS_HOST}  (${OPS_DEPLOY_DIR})
  → 应用容器（仅 127.0.0.1:8080）
```

- **compose 必须带 `-f docker-compose.vps.yml`**，见下方坑 1。
- postgres / redis 只 `EXPOSE`，不发布端口；应用 `BIND_HOST=127.0.0.1`。
- 防火墙：`docker-user-firewall.service`（`PartOf=docker.service`，幂等，已实测清空后可完整恢复），IPv4/IPv6 同步。
- 看门狗：每 2 分钟，故障注入实测 22.5 秒恢复。
- 异地备份：Cloudflare R2 桶 `${OPS_R2_BUCKET}`，令牌限 IP `${OPS_VPS_HOST}`。
- 已开 `totp_enabled` 与 `step_up_enabled`；默认管理员 `admin@sub2api.local` 已硬删除。
- `DATABASE_MAX_OPEN_CONNS=25`。
- 容量实测：**瓶颈是 CPU 不是带宽**（真实流量仅占 200Mbps 的 0.3%，iowait 0.0%），每请求约 121ms CPU，上限约 990 请求/分钟或 425 条并发流。
- 笔记本 `sub2api-official-18082` 及其 Cloudflared、watchdog 已全部停止，**数据卷原样保留可回滚**。历史上的笔记本链路（`host.docker.internal:18082` + `sub2api-public-nginx-local`）已不再生效。

详见 `docs/ai/context/20260905-100322-vps-cutover-hardening-and-capacity-audit_CN.md`、`20260905-110342-docker-user-firewall-hardening_CN.md`。

## 三、计费口径

**唯一公式**：`actual_cost = total_cost(标准成本) × 分组倍率 × BILLING_FINAL_MULTIPLIER`

- 当前 `BILLING_FINAL_MULTIPLIER=18`，**隐藏倍率**，以运行态容器环境变量为准，改动只走 compose + 替换应用容器。
- 模型广场**刻意不展示、不叠加**最终倍率，只显示 `基础单价 × 生效倍率`。
- 账号统计倍率（`accounts` 上的）**不参与用户扣费**，只做渠道统计。
- 分组倍率以数据库 `groups.rate_multiplier` 为准。截至 2026-09-05：

  | 分组 | id | 倍率 | 备注 |
  | --- | --- | --- | --- |
  | Grok | 3 | 0.6 | |
  | Claude Max | 4 | 1.5 | |
  | Claude Kiro | 5 | 0.45 | |
  | Claude 日常1 | — | 0.5 | 名称 `Claude0.5倍率(日常1)`，2026-09-05 由 `0.4` 调至 `0.5` |
  | GLM | 6 | 0.6 | 名称已同步为 `GLM0.6倍率` |
  | Kimi | 7 | 4.9 | **名称仍是 `Kimi0.7倍率`，与数值不一致** |
  | DeepSeek | 8 | 4.9 | 名称已同步为 `【国产】DeepSeek（7折）` |
  | GPT 0.15 | 9 | 0.15 | |
  | GPT 0.35 / GPT-Image-2 | 10 / 12 | 0.35 / 1.0 | 生图渠道单独隔离，见坑 6 |
  | GPT 0.28 | 74 | 0.28 | 上游为独立中转站，非 GPT 其余分组的上游 |
  | Gemini | 70 | 1.0 | 原生 |
  | Claude | 71 | 0.78 | 原生 |
  | Grok 0.9 / GPT 0.1 | — | 0.9 / 0.1 | **已停用下架**；存量绑定该分组的 API Key 会不可用，改回「正常」即可回滚 |

  **分组名称不能作为倍率依据**，只查数据库。

- 汇率展示：国外模型按 `1 USD = 1 CNY`，国产模型按 `1 USD = 7 CNY`，模型广场标题下有固定说明。`BALANCE_RECHARGE_MULTIPLIER` 是"每支付 1 CNY 获得多少 USD"，**不要把汇率写进模型扣费倍率**。
- **登记一个新 OpenAI 模型的价格，必须同时改两处**（见坑 10、11）：`pricing_service.go` 的 `matchOpenAIModel`（加显式前缀分支 + 静态价，**这是生产实际走的路径**）和 `billing_service.go` 的 `fallbackPrices`（目录失效时的兜底）。只改后者等于没改。
- `gpt-6-astra` 两处均已登记：$10/$1/$12.50/$50 每百万 token，>272K 输入转 2x 输入与缓存、1.5x 输出（复用 `openAIGPT54LongContext*` 常量）。别名 `gpt-6`、`gpt6`、带 effort/日期后缀的写法都解析到同一份价格，回归测试见 `billing_service_gpt6_test.go`。
- **`gpt-6-astra` 已于 2026-09-05 开放并实测计费正确**：三个 上游B 上游账号（`#1132`/`#1164`/`#1168`）的 `model_mapping` 已加入该键。实测 `usage_logs` id `404425`——`input 864×$10/M + cache_read 7488×$1/M + output 22×$50/M = total_cost 0.017228`，`actual_cost = 0.017228 × 0.16 × 18 = 0.04961664`，余额差额一致。**上游只认精确串 `gpt-6-astra`，别名一律 404**，所以坑 11 的少收风险在这条链路上触发不了。详见 `docs/ai/context/20260905-134951-gpt6-astra-enablement-and-billing-verification_CN.md`。
- **2026-09-05 又给 `Gpt0.35倍率(优质)`（分组 `10`、账号 `#1129`、上游 `api.ai-genesis.app`）加了该模型**，上游实测 `200`。当前 4 个分组可用：`GPT0.16(日常2)`、`GPT0.16(日常3)`、`GPT0.28`、`Gpt0.35`。**`#1128`（0.15）和 `#1130`（0.1，已下架）没加，也不要照搬着加**——见坑 24。详见 `docs/ai/context/20260905-203005-gpt6-astra-enable-on-035-group_CN.md`。

## 四、业务规则

### 负余额与流量卡

- 余额 `>= 0`：不切流量卡。
- 余额 `< 0`：下一次请求**不再扣普通余额**，必须使用用户级全渠道流量卡。
- 流量卡净额度必须**严格 `> 0`** 才允许本次调用；用尽或形成流量卡欠费后，下一次请求拒绝。
- 历史的 `BILLING_MINIMUM_BALANCE_RESERVE=0.01` 保底阈值**已删除**，不要再引用。
- 负余额准入以 PostgreSQL `users.balance` 为最终事实，订阅 / 流量卡 / SimpleMode / Gemini / Live / WebSocket 均不能绕过。

### 余额套餐

- 标准周期 28 天，每 7 天到账一次，共 4 期。每用户**最多一个有效套餐**；购买其他档位需先退款，服务端在用户行锁内二次校验。
- `remaining_usd` 只表示**本周**未用额度，刷新是按窗口替换而非累加。
- **续费 = 周期重置**：立即发放新周期第 1 期、`credited_count` 回到 `1`、`next_credit_at` 重新计时、`status` 恢复 `active`、有效期在原到期基础上 `+28` 天、旧周期未发放期数顺延进新周期总期数、`renewal_count + 1`。
- 首期与每次周刷新**先抵扣负余额**，不足则继续为负，只有偿还后的剩余部分进入 `remaining_usd`。

### 欠费连续结算

- 余额套餐**不因欠费暂停**，下一周额度优先抵消余额欠费。历史 `debt_paused` 数据只做兼容展示。
- 普通余额已欠费后，所有渠道统一扣**用户级全渠道流量卡**，不足部分记入独立欠费账本（`balance_debt_ledger`）。
- 流量卡充值优先抵消流量卡欠费。余额和流量卡均无净可用额度时拒绝请求。

### 退款

- **准入**：仅**真实支付的余额套餐订单**可退，需同时具备实付金额、支付完成时间、支付平台交易号。管理员发放、兑换码、零金额、流量卡、旧普通余额订单一律不可退，前端四处入口同步隐藏。
- **公式**：`Max(已用时间比例, 周期用量比例)`，实现在 `backend/internal/service/payment_balance_package_refund.go`。
- 用量账本按 `created_at >= starts_at` 限定**当前周期**，且只汇总**余额套餐实际承担**的扣款——流量卡、普通余额和流量卡欠费不参与退款计算。
- 历史套餐因缺少资金来源归因，由迁移 `211` 标记为 `legacy_unattributed` 转人工审核，**不伪造历史归因**。
- ZPay 易支付订单优先用 `out_trade_no` 发起退款；成功统一写 `REFUNDED`，只撤销对应套餐后续到账，不跨订单扣减既有余额；网关失败进 `REFUND_FAILED` 可重新报价重试。
- `payment_audit_logs(order_id, action)` 有唯一索引，**重复失败会导致 `REFUND_FAILED` 审计写入冲突**。

### 其他

- 邀请返利默认开启、比例 8%，直接增加 `users.balance`，`frozen_until` 记 24 小时冻结；**冻结不限制模型使用**。
- 充值手续费 `RECHARGE_FEE_RATE=1%`，只增加订单 `pay_amount`，**不改变套餐到账额度或流量卡额度**；服务端始终用商品服务端价格重算。
- `/monitor` 全部渠道统一为**每次一个带鉴权的 `GET /v1/models`** 目录探测，间隔 1800 秒，不发真实推理请求，不做额外 HEAD。生图渠道禁止周期性生图探测。
- 购买页余额套餐与流量卡**必须复用** `frontend/src/components/payment/PurchaseProductCard.vue`，禁止新增平行卡片样式。
- 商品当前只有余额套餐和流量卡；普通余额 / 旧订阅后端不再兼容，历史字段仅保留只读查询。

## 五、坑

1. **`BIND_HOST`**：基础 compose 写的是 `"${BIND_HOST:-0.0.0.0}:..."`。漏掉 `-f docker-compose.vps.yml` 就会真的绕过 UFW 把端口暴露到公网。现有三层防护：`.env` 改为 `127.0.0.1`、vps override、`DOCKER-USER` 兜底。
2. **`DOCKER-USER` 第一条规则绝不能删**：`-i eth0 ESTABLISHED,RELATED -j RETURN` 必须在 `-i eth0 -j DROP` 之前。容器访问上游的返回包也从 eth0 进入，只写 DROP 会掐断**全部上游调用**，症状是"所有上游超时"，极易把排查方向带偏。
3. **基础 compose 没有 `env_file:`**：`.env` 里的变量不会自动进容器。VPS 切换时 `BILLING_FINAL_MULTIPLIER=18` 就因此没生效，差点按应收的 1/18 收费。
4. **JWT secret 实际生效值来自 `/app/data/config.yaml`，不是 `.env`**。两端不一致会让全部用户网页会话失效。
5. **`6379/tcp` 与 `0.0.0.0:6379->6379/tcp` 是两回事**：前者只是 `EXPOSE`，后者才会插 DNAT 绕过 UFW。这是判断"有没有真的暴露"的唯一依据。
6. **GPT 低价渠道必须排除图像模型**：上游虽列出图像模型，但若映射进 `0.1x` 渠道，生图请求会按 `0.1x` 结算并绕过独立生图渠道。
7. **Vite 公共依赖分包不能叫 `vendor-*`**：Cloudflare 对该路径静态资源返回 403，导致 `/login` 白屏。现用 `lib-*`。
8. **迁移 checksum 保护**：已应用的迁移改内容会导致启动失败，只能新增迁移号（207 曾踩，用新增 208 解决）。
9. **Redis `sched:acc:*` 保存调度快照**：轮换上游凭证必须同时同步数据库和缓存，否则旧凭证继续被调度。凭证在 `accounts.credentials` 以 AES-256-GCM 服务端加密存储。
10. **取价有三级，`fallbackPrices` 是最后一级，登记它往往不解决问题**。顺序是 **① 渠道/分组定价（`model_pricing_resolver.go`）→ ② `PricingService` 远端价格目录 → ③ 硬编码 `fallbackPrices`**。远端目录由 `pricing.remote_url` 拉取，收录了大量模型，所以第 ② 级几乎总会命中，第 ③ 级只在目录失效时才走到。**只改 `fallbackPrices` 而没改 `PricingService`，等于什么都没改。**
11. **`matchOpenAIModel` 末尾有个 `DefaultTestModel`(= `gpt-5.4`) 兜底，会静默少收费**。任何以 `gpt-` 开头、又没被前面分支拦住的模型都掉进去，按 `$2.5/$15` 计价。新 OpenAI 模型必须在该函数里加显式前缀分支（照抄 `gpt-5.6-sol/terra/luna` 的写法），**否则别名写法（裸族名、缺连字符、effort/日期后缀）会按 gpt-5.4 价结算**。GPT-6 Astra 实际 `$10/$50`，掉兜底就是输入少收 4 倍、输出少收 3.33 倍。
12. **缺定价不会拒绝请求，会记零成本放行**。`openai_gateway_usage.go` 在取不到价时打 `pricing_missing_record_zero_cost` 日志后按 0 计费；通用网关 `gateway_usage_billing.go` 同样吞错返回 `ActualCost: 0`。计费发生在响应转发**之后**，认证层不看模型名——**所以「开放模型」必须在「定价确认生效」之后**，顺序反了中间窗口的少收无法追回（`usage_logs.actual_cost` 记下的就是错值）。
13. **远端目录的长上下文字段解析不到**。目录用 `*_above_272k_tokens` 表达，而解析器只认 `long_context_*`，命中数为 0。`>272K` 档位只能靠静态 fallback 价或 `applyModelSpecificPricingPolicy` 补，且后者还要求账号 `extra.openai_long_context_billing_enabled=true`（默认 false）。
14. **`newTestBillingService()` 传的 `pricingService` 是 `nil`**，只覆盖第 ③ 级。用它写的定价测试**测不到生产实际走的路径**，会给出假阳性。测生产行为要用 `&PricingService{pricingData: ...}` 构造非 nil 的，参见 `billing_service_gpt6_test.go`。
15. **改 `accounts.credentials` 必须先 GET 再整体 PUT**。`MergePreservingSensitiveCreds` 以 incoming 为基底，**非敏感键没传就是删除**——只传 `model_mapping` 会把 `base_url` 一起删掉、直接废掉账号。敏感键（`api_key` 等 14 个）在 GET 时被**整个移除**而非返回掩码，所以 PUT 回去不带它们会自动保留原加密值。`Name`/`Status` 有判空、`GroupIDs`/`Concurrency`/`Extra` 判 nil，只传 `credentials` 不影响其它字段。
16. **测模型可用性要用精确 ID，别用族名**。2026-09-05 用裸 `gpt-6` 测出 404，据此误判"上游没有 GPT-6"；实际上游只认 `gpt-6-astra`，一直是通的。**测错字符串比没测更误导**——它给出一个看起来有依据的错误结论。
17. **判断生产是否加载了远端价格目录，不需要 SSH**：模型广场 `/api/v1/model-plaza` 是公开端点且暴露价格。挑一个「只在远端目录（231 键）、不在内嵌目录（198 键）、且源码无硬编码兜底」的模型（如 `claude-fable-5`、`claude-sonnet-5`、`gemini-3.5-flash-lite`），生产能报出精确价格就说明目录已同步。
18. **`Dockerfile` 的 `GOPROXY`/`GOSUMDB` 默认是国内镜像**（`goproxy.cn`/`sum.golang.google.cn`）。在 GitHub Actions 等海外 runner 上构建必须显式覆盖为官方源。
19. **`internal/service` 的 `unit` 标签测试套件当前无法编译**（多个文件的未定义符号，非近期引入）。该包暂时跑不了全量单测，改动只能跑定向用例。
20. **上游模型白名单存在 `accounts.credentials.model_mapping`**（恒等映射），不是单独的表或字段。改白名单走 `PUT /api/v1/admin/accounts/{id}`：按 `EditAccountModal.vue` 的既有约定，**请求不携带 `api_key` 字段即保留原加密凭证**。不要试图在 UI 上逐个删模型 chip——14×14 的删除按钮被 `.modal-footer` 覆盖（`elementFromPoint` 命中 footer），误点会关掉弹窗丢改动。**但「加」模型 UI 是安全的**：编辑弹窗底部有 `自定义模型名称` 输入框 + `填入` 按钮，不碰 chip。同一弹窗里 `同步最新支持模型` / `同步上游支持的模型` 会用上游清单**整体替换**白名单（对 ai-genesis 账号 = 放进图像模型并删掉四个在售条目），`清除所有模型` 字面意思，三个都别碰。提交后按钮会卡在「更新中...」但 toast 已报成功、数据已落库，**不要重复提交**；**分组编辑弹窗是同款症状**——`PUT` 已返回 200、数据已落库，按钮却长期停在「更新中...」。
21. **分组「复制」会把源分组已绑定的账号一并绑到副本**。用复制建新分组后必须检查并解绑，否则新分组的请求会调度到旧账号并按新分组倍率计费。
22. **「提前刷新周额度」没有 API 也没有 UI，只能直连数据库**。管理侧 `balance-packages` 只有 `list`/`grant`/`resume-debt-paused`；`grant` 是新建套餐、`POST /admin/users/:id/balance` 只改余额数字。**用后者变通会导致 `next_credit_at` 不推进，定时任务到原日期重复发放**，且不写审计。正确做法是单个 SERIALIZABLE 事务：锁「用户→套餐→订单」、校验幂等（`payment_audit_logs` 对 `(order_id, action)` 唯一）、按锁内实时值算 `creditDueBalance`、`next_credit_at` 取「原值 + interval」保持节奏、`starts_at` 不动、`balance_debt_ledger` 有 `amount_usd > 0` 约束故无欠费时不可写入。模板见 `docs/ai/context/20260905-172724-user565-early-weekly-credit-execution_CN.md`。

23. **`schema_migrations` 行数多于迁移文件数是正常的**，不能据此判断代码来源。生产已应用 284 条而仓库只有 258 个文件，多出的 26 条是数据库从旧实例 pg_restore 带来的历史痕迹（旧实例跑过更新的上游构建）。迁移运行器只执行「文件存在但未应用」的，多余的行不影响启动。**据此误判过两次**：先认为「生产跑的不是本仓库代码」，再认为「部署本仓库是降级、须先合并落后 1095 提交的上游」，甚至已 merge 出 47 个冲突才发现搞错。判断代码来源要去看 `${OPS_DEPLOY_DIR}/src` 的实际文件，不看迁移行数。

24. **同一个 `base_url`，不同 API Key 的可用模型清单不一样**。2026-09-05 实测 `api.ai-genesis.app`：`#1129` 的凭证返回 8 个模型**含 `gpt-6-astra`**，`#1128` 的凭证只返回 6 个**不含**它。所以**「同上游 ⇒ 同模型」是错的**，按主机推广结论会把一个调不通的模型上架给用户（白名单只做准入，不保证上游真的有）。**逐账号问，别猜**——有两个零成本只读通道：
    - `POST /api/v1/admin/accounts/{id}/models/sync-upstream`：用该账号凭证打上游 `/v1/models` 并返回，**不写库**。（前端「同步最新支持模型」按钮点了没反应是前端的问题，**接口本身是好的**。）
    - `POST /api/v1/admin/accounts/{id}/test`（UI：账号行「更多 → 测试连接」，弹窗里可**指定模型**）：用账号凭证直连上游发一次真实请求，**不经用户网关、不写 `usage_logs`、不扣任何余额**。
      验模型可用性用它就够了。历史上为此建过 4 个临时 API Key 再删（`388`~`391`），也真实扣过 `$0.0496`——都是不必要的代价。**只有验「扣费金额对不对」才需要走真实网关。**
      副作用：测试成功会清空 `extra.model_rate_limits` 的冷却记录。

25. **核对历史扣费必须用 `usage_logs.rate_multiplier`，绝不能 JOIN `groups.rate_multiplier`**。每条 `usage_logs` 都存了**扣费当时生效的分组倍率**快照；`groups.rate_multiplier` 是**当前值**。分组倍率一改，用当前值做除数去反推最终倍率就会凭空造出「计费异常」。2026-09-05 核验时就据此报出「DeepSeek 分组比值恒为 12.857143」——实际是 `18 × 3.5 ÷ 4.9`（该分组当天刚从 `3.5x` 改成 `4.9x`，那 208 条是改之前扣的），按行内快照倍率重算全部精确等于 18。**这类假异常看起来极有依据**，与坑 16「测错字符串」同类。正确写法：`actual_cost = total_cost * usage_logs.rate_multiplier * 18`，用 numeric 精确等值比较而不是浮点容差。另注意 `total_cost = 0` 的行比值无定义会被聚合静默排除，要单独确认它们 token 数也为 0（是占位记录）而不是坑 12 的少收。

26. **`schedulerSnapshot` 有两个同名不同物的东西，别把它们当成一个**。`repository.schedulerCache`（Redis `sched:acc:`/`sched:meta:`）确实把 `Extra` 过了 `filterSchedulerExtra()` **白名单**、把 `Credentials` 过了 `buildSchedulerCredentialMetadata()`（**连 `api_key` 和 `base_url` 都不留**）。但网关持有的 `schedulerSnapshot` 是 **`service.SchedulerSnapshotService`** 这层包装，它的 `GetAccount()` 读完缓存**直接丢弃**、恒走 `accountRepo.GetByID` 回源（`scheduler_snapshot_service.go:266-289`，注释：「生产路径必须回源，确保请求只从受控仓储取得解密后的凭证」；`accountRepo == nil` 才返回 cached，那只是测试接线，`wire.go:341` 注入的是真 repo）。
    **所以请求期拿到的 `account.Extra` 是完整的**，`filterSchedulerExtra` 白名单只影响**候选集筛选**，不影响计费和任何按 `Extra` 分支的请求行为。2026-09-05 差点据此得出「改 `openai_long_context_billing_enabled` 不会生效、必须改代码加白名单」的错误结论。
    **反向自检**：快照连 `api_key`/`base_url` 都没有——如果请求真用快照 account，所有 apikey 账号会直接认证失败、自定义上游会被打到 `api.openai.com`。**生产明明是通的，就说明用的不是快照。** 这条比读调用链更快证伪。
    另：改账号 `extra` 单个键有个更干净的接口 **`POST /api/v1/admin/accounts/bulk-update`**（`{account_ids, extra}`），走 JSONB **key 级合并**（`account_repo.go:2971+`）；而 `PUT /accounts/{id}` 是**整体替换**（见坑 15 的同款陷阱），必须 GET 完整 `extra` 再整体回传。

## 五点五、待处理的计费偏差（已确认，未修复）

> **口径（管理员 2026-09-05 明确）：只有「少收」是缺陷，「多收」不是。**
> 「即使上游没多收，我这里多收了也是我的收入。」
> 因此本节的排查与修复**一律只针对平台少收 / 零成本放行**；
> 「我们收得比上游成本高」不算问题，不要作为缺陷上报，也不要主动往下调价。
> 判断一条偏差要不要修，先问方向：**钱是漏出去了还是留下来了。**

- **`gpt-5.6-sol` / `gpt-5.6` 在向用户超收**：按 OpenAI 2026-08-24 降价前的旧价计费——输入 `$5` vs 官方 `$4`、缓存读 `$0.50` vs `$0.40`、缓存写 `$6.25` vs `$5`、输出 `$30` vs `$20`（**输入 +25%、输出 +50%**）。从 `usage_logs` 反推确认，非推断。27 小时敞口 `$104.40`、约 `$93/天`，6 个在售 GPT 分组全量受影响。**三层取价同错**：远端目录该键就是旧价、硬编码兜底是同样的错值；目录里裸 `gpt-5.6` 键的值反而是对的，但别名归一化在查价前就把它改写成 sol，导致正确记录永不可达。对照组 `gpt-5.6-terra` 正确，说明只是这一条没跟上降价。**管理员已决定：先自行核实官方价再改，历史超收不退、只修当下。修复时注意 $4/$20 是促销价，官方只承诺至少持续到 2026-11-21。** 详见 `docs/ai/context/20260905-154436-gpt56-sol-overcharge-finding_CN.md`。
  **补充（2026-09-05）：这个旧价是从上游继承的，不是我们算错。** 上游 `api.ai-genesis.app` 的模型广场对 `gpt-5.6-sol` 同样报 `$5/$30`。所以改价会让我们和上游口径不一致——**上游按 $5 结算给我们、我们按 $4 收用户，差价由平台吃**。
  **⚠️ 本条按上面的新口径已不构成缺陷**：方向是多收、且广场展示的就是同一组数字（用户看到什么就付什么），上游也按同价结算给我们。**在管理员另行指示前不要"修复"它**——照官方 `$4/$20` 改反而会主动放弃收入并造成对上游倒挂。上面那句「先自行核实官方价再改」是新口径之前的决定，**已过期，别照做**。
- ~~**长上下文计费一道都没打开**~~ **2026-09-05 已对 6 个 GPT 账号开启**（`#1128`/`#1129`/`#1130`/`#1132`/`#1164`/`#1168`，`extra.openai_long_context_billing_enabled = true`）。**GLM(#4) / Kimi(#5) / DeepSeek(#6) / 生图(#1131) 仍为 false，按管理员要求先不动。** 开启前 34 小时的敞口实测：153 条 >272K 请求全部按短价收，少收约 `$189`（约 `$134/天`），最大单条上下文 1,067,149 token。详见 `docs/ai/context/20260905-213000-gpt-long-context-billing-enablement_CN.md`。
  背景机制：**OpenAI 网关是唯一默认关闭该计费的路径**（Claude/Gemini/Grok 不传该指针、`applyLongCtx` 默认 true）。扣费侧 `billing_service.go:1062` 是 `applyLongCtx := len(resolved.Intervals) == 0 && *LongContextBillingEnabled`，`resolved.Intervals` 只认**渠道/分组里真正配置过**的区间定价（GPT 全系为空），所以账号开关是唯一闸门。
  **广场展示的 `>272K` 档位曾是「看得见、收不到」的**：广场那两个区间是 `channel_plaza.go:269` 的 `pricingIntervalsFromBilling()` **纯展示合成**的（拿 `ModelPricing` 的 `LongContext*` 倍率现算），**不是区间定价行**，不进 `resolved.Intervals`。所以**广场有档位 ≠ 会按档位收**，别拿广场当计费依据。
  **开关只对登记了长上下文字段的模型有效**：`gpt-6-astra`/`gpt-6`、`gpt-5.6`/`-sol`/`-terra`/`-luna`、`gpt-5.4`、`gpt-5.5`/`-pro` 有效（`applyModelSpecificPricingPolicy` 会回填阈值 272000 + 2x/1.5x，不依赖远端目录）；`gpt-5.3-codex`、`gpt-5.2`、`gpt-5.4-mini/nano` 等**开了也是空操作**（`LongContextInputThreshold` 为 0）。影子账号按**母账号**的开关判定。
- **客户端可以自己在请求体里写 `"service_tier":"flex"` 拿五折**（`priority` 则是 2 倍）。`serviceTierCostMultiplier`（`billing_service.go:135`）对 `flex` 硬编码 `0.5`，且乘在 `TotalCost` 求和**之前**——所以 `usage_logs.total_cost` 一起被砍半，**坑 25 那条对账恒等式照样成立，事后 SQL 查不出来**，只能看 `usage_logs.service_tier` 列。tier 取自客户端 body（`openai_gateway_request_body.go:849`），**从不与上游返回的 tier 核对，也没有模型门槛**——GLM/Kimi/DeepSeek/Grok 在任何价目表里都没有 flex 档，照样五折。
  **2026-09-05 实测敞口为零**：48 小时 6000 条 `usage_logs` 的 `service_tier` 全是 `null`，无人使用。属**潜伏**漏洞，不是正在流血。修法是把 `flex` 分支改回 `1.0`（要改代码出镜像）。**在修掉之前别对外宣传这个字段。**
- **Claude 没有长上下文加价档，这是正确状态，不是漏收（2026-09-06 查官网确认，上一轮误报已撤回）。** Anthropic 官方定价页「Long context pricing」明确写：**Claude 4.6 及以后模型，完整 1M 上下文窗口全程按标准单价收**（原文「A 900k-token request is billed at the same per-token rate as a 9k-token request」）。我们白名单里的 Claude 全是 4.5~5 代，官方均无 >200K 涨价档。因此：① 账号上的 `openai_long_context_billing_enabled` 对 Claude **本就不生效**——`gateway_usage_billing.go:907` 的 Anthropic 计费路径根本不传该指针（那是 OpenAI 网关专用）；② Claude 模型没登记 `LongContext*` 字段是**对的**，不是缺陷；③ 我们的 Claude 基准价与官方标准价**逐个精确一致**（sonnet-5 `$2/$10`、opus-4-8/opus-5 `$5/$25`、sonnet-4-6 `$3/$15`、haiku-4-5 `$1/$5`、fable-5 `$10/$50`），再叠 `分组倍率 × 18`，全程平价——已按官方口径正确计费。
  - **想让 Claude 多赚**：干净的做法是抬 Claude 分组 `rate_multiplier`（配置即可、全上下文一律涨）；**不要**去代码里硬造一个 >200K 档——那是脱离上游的假分档，广场会露出比真实 Anthropic 更贵的价，且要出镜像。
  - 附：官方注明 **4.7 及以后模型换了新分词器、同文本约多产 30% token**（opus-4-7/4-8/5 受影响），我们按 token 收，这部分收入天然已在账上。
  - GPT 那次要开长上下文是因为 GPT/GLM **真有**涨价档（GPT >272K、GLM >32K）；Claude 没有，别照搬。
- **`batch_image.enabled` 默认 `false`，保持关闭**。批量生图结算不经 `applyFinalBillingMultiplier`（`batch_image_settlement.go:138`），开了会**少收 17/18 ≈ 94.4%**，且冻结额同样少 18 倍所以不会报错、静默通过。要开必须先补 `×18`。
- **超阈倍率是 2x 输入、2x 缓存读、2x 缓存写、1.5x 输出**——缓存同样翻倍，漏掉缓存会把长会话的主要成本项低估一半。边界是**严格大于** 272,000；我们广场 `tier_label` 写的「`<272K` / `>=272K`」措辞是错的（上游同款软件已修为「`≤272K` / `>272K`」，我们的构建更旧）。判定口径为 `输入 + 缓存写 + 缓存读` 三者之和。
- **`allow_live` 开启前必须先落地时长计费，否则平台单向净亏且不可追回。**
  `service/openai_live.go` 的 `finalizeLiveCall` 写死 `TotalCost=0 / ActualCost=0 /
  RateMultiplier=1`，完全绕过 `applyUsageBilling`（全仓非测试调用点只有
  `gateway_usage_billing.go` 与 `openai_gateway_usage.go` 两处，Live 不在其中）。
  用户余额一分不扣，因此**欠费闸永远不触发**，余额 `$0.01` 的用户可无限重复
  最长 `liveMaxSessionDuration`（默认 1 小时）的会话；落库的 `actual_cost` 就是 0，
  事后无法重建。**2026-09-05 已在 `handler/openai_live.go` 的 `liveEnabledForAPIKey`
  加了 `liveBillingImplemented = false` 硬拒绝**，一处早退覆盖全部 Live 路由；
  接通计费时连同早退分支与 `openai_live_test.go` 的对应断言一起改回。
  开启前置：① 先有按时长的定价并确认命中三级取价（只加 `fallbackPrices` 无效，见坑 10）——
  **仓库当前完全没有音频/时长价格**，接了计费管道而无价只会按 0 放行（坑 12）；
  ② 每分钟单价是业务决策，需管理员先拍板；③ 同步确认
  `openai_live.go` 里 `WithOpenAIProfitControlSuppressed` 的豁免是否仍成立
  （该行原注释谎称「Live 按通话时长计费」，已于 2026-09-05 更正）；
  ④ `TestFinalizeLiveCallIsIdempotentAndWritesZeroUsage` 用 `require.Zero` 钉死了零值，
  是有意设的绊线不是待修 bug，同一个 PR 里必须一并改断言。
  另注意分组「复制」会连 `allow_live` 一起复制（与坑 21 同源）。
  **上游 `Wei-Shaw/sub2api` 至 2026-09-05 仍是同款零计费，同步上游修不了它。**
- **不要把「Linux 上打开 `allow_live` 只会 503」当成 Live 安全。**
  `internal/platform/liveattestation` 是 **darwin-only**（`//go:build !darwin` 返回
  `ErrUnsupportedPlatform`），而 `prepareLiveAttestation` 在 `CreateLiveCall` 里是
  **选账号之前的硬闸**，所以生产 Alpine/Linux 镜像下 Live 结构性起不来。
  但这是平台的偶然属性、不是计费保护——**任何一次改成 macOS 部署或给非 darwin 补 provider，
  这道闸就没了**。判断 Live 敞口要看计费有没有接上，不能拿平台限制当理由。


## 六、负面教训（结论已撤回，不要重复）

- **不能用"同模型 + 时间窗口正负 N 秒"把共享上游账号的账单归属到本地用户**。据此得出的 `$652.17178575` 未追回扣费和单用户 `$384.62055975` 均已撤回，**不得用于补扣**。相关失败事件缺请求 ID、Token 和费用快照，`billing_reconciliation_cases` 一律保持待核对且金额留空——**不用次数或平均价伪造扣费**。
- **上游 Usage 页面的"费用"就是标准费用**，本地按 `标准成本 × 分组倍率 × 最终倍率` 扣费。曾因把它误读为"已含上游加价的用户价"，在一天内把 Kimi 倍率来回改了四次。
- **三种验证假阳性**（都真实误导过）：`nc -z` 对开放端口报不可达；`wget` 收到 `401` 被当成网络不通；命令管道到 `head` 使退出码恒为 0，脚本从头到尾没检查任何东西却报"✓ 通"。

## 七、未完成

- **`api.ai-genesis.app` 的白名单里有一半模型上游根本没有**。以 `sync-upstream` 实测（坑 24）：`#1129` 上游只有 `gpt-5.5`、`gpt-5.6-luna`、`gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-6-astra`（+ `codex-auto-review` 和两个图像模型），`#1128` 更少且无 `gpt-6-astra`；两个账号白名单里的 `gpt-5.4`、`gpt-5.4-2026-03-05`、`gpt-5.4-mini`、`gpt-5.6` 上游都没有，用户看得见调不通（502/503）。是否收紧 `#1128/#1129` 白名单**未定**——上游可能恢复，收紧后需再加回。
- **`build.yml` 从未实际触发过**——需手动跑一次确认能出镜像且前端不 OOM。
- **外部拨测未配置**——机器宕机时无人知晓。
- **单点故障无冗余**——全部服务跑在一台 VPS 上。
- `api_base_url` 仍为空（`/keys` 页面已硬编码 `https://api.aaccx.pw/v1` 兜底，不依赖该设置）。
- Kimi、DeepSeek 分组**名称与实际倍率不一致**，待统一。
- `billing_reconciliation_cases` 3,933 条余额不足案件待外部逐笔账单核对。
- 12 条零 token / 零成本占位记录（请求类型 2）业务语义待确认。
- 续费订单与套餐的多对一审计映射缺失：当前套餐 `payment_order_id` 只绑定最新续费订单，历史续费订单无法各自独立退款。
