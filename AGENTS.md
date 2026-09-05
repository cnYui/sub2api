# 项目协作约定

- 2026-09-05：按管理员要求调整生产分组倍率与上下架。`Claude0.4倍率(日常1)` 改名 `Claude0.5倍率(日常1)` 并将 `rate_multiplier` 由 `0.4` 改为 `0.5`；`【国产】DeepSeek（5折）` 改名 `【国产】DeepSeek（7折）` 并由 `3.5x` 改为 `4.9x`（涨价 1.4 倍，与 5 折→7 折口径一致）；`Grok0.9倍率(优质)` 与 `Gpt0.1倍率(优惠)` 状态改为**停用**下架，倍率保持原值。名称与真实扣费倍率同步修改，不留名称与实际不一致的情况；隐藏最终倍率 `18x`、上游账号凭证、账号分组绑定、用户余额、订单和历史用量均未改动。下架后已绑定这两个分组的存量 API Key 将不可用，需要时可把状态改回「正常」回滚。另发现分组编辑弹窗提交后按钮长期停在「更新中...」但 `PUT` 已返回 200、数据已落库，勿重复提交。详见 `docs/ai/context/20260905-120500-group-rate-and-shelf-adjustments_CN.md`。

- 2026-09-05：新增 GPT 分组 `GPT0.28倍率`（分组 ID `74`、OpenAI、`0.28x`、公开非专属）与上游账号 `#1168`（API Key 类型，Base URL `${OPS_UPSTREAM_GPT028_HOST}`，并发数对齐为 `100`，创建时自动探测上游声明倍率为 `0.28x`）。模型白名单按上游 `/v1/models` 实际返回的 23 个模型配置为 **20 个文本模型**，排除 `gpt-image-1/1.5/2`——`Image-2生图` 分组为 `1x`，放行图像模型会让生图以约 1/3.5 价格绕过独立生图渠道。创建表单的默认预置列表与该上游不匹配，已移除上游不存在的 `gpt-5.6-luna`、`gpt-5.4-mini`，补入缺失的 `gpt-5.3-codex`、`gpt-5.2-openai-compact`、`gpt-5.3-codex-openai-compact`、`gpt-5.4-openai-compact`、`gpt-5.5-openai-compact`。**注意：分组「复制」会把源分组已绑定的账号一并绑到副本**，本次账号 `#1164` 被连带绑定后已解绑，用复制建分组时务必检查。实测 `/v1/models` 返回 20 个模型、`gpt-5.6-terra` 与 `gpt-5.6-sol` 均 HTTP `200`，计费 `0.003382 × 0.28 × 18 = 0.017043` 与既有公式一致；`gpt-5.3-codex` 的 `502` 经直连上游复现为 `429` 限流，属上游问题。账号「同步最新支持模型」按钮不发请求、疑似失效，待排查。详见 `docs/ai/context/20260905-114500-gpt028-group-and-account-creation_CN.md`。

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
- 生产数据变更必须写 `payment_audit_logs` 审计，并处理认证/余额缓存失效。
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
  | GLM | 6 | 0.6 | 名称已同步为 `GLM0.6倍率` |
  | Kimi | 7 | 4.9 | **名称仍是 `Kimi0.7倍率`，与数值不一致** |
  | DeepSeek | 8 | 4.9 | 名称已同步为 `【国产】DeepSeek（7折）` |
  | GPT 0.15 | 9 | 0.15 | |
  | GPT 0.35 / GPT-Image-2 | — | 0.35 / 1.0 | 生图渠道单独隔离，见坑 6 |
  | GPT 0.28 | 74 | 0.28 | 上游为独立中转站，非 GPT 其余分组的上游 |
  | Gemini | 70 | 1.0 | 原生 |
  | Claude | 71 | 0.78 | 原生 |
  | Grok 0.9 / GPT 0.1 | — | 0.9 / 0.1 | **已停用下架**；存量绑定该分组的 API Key 会不可用，改回「正常」即可回滚 |

  **分组名称不能作为倍率依据**，只查数据库。

- 汇率展示：国外模型按 `1 USD = 1 CNY`，国产模型按 `1 USD = 7 CNY`，模型广场标题下有固定说明。`BALANCE_RECHARGE_MULTIPLIER` 是"每支付 1 CNY 获得多少 USD"，**不要把汇率写进模型扣费倍率**。
- 模型基础价登记在 `billing_service.go` 的 `fallbackPrices`。`gpt-6-astra` 已登记（$10/$1/$12.50/$50 每百万 token，>272K 输入转 2x 输入与缓存、1.5x 输出，复用 `openAIGPT54LongContext*` 常量）。见坑 10。
  **但 2026-09-05 实测两个上游对 `gpt-6` 均返回 404，该模型尚未真正可用**——这份定价是预防性登记，不是在修复正在发生的漏计费。上游真正上线后需重新确认实际模型 ID。

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
10. **OpenAI 族计价是白名单**：`getFallbackPricing` 只认已登记型号，未登记的返回 `nil`——**用户能调用但平台不扣款**。上游每上一个新 OpenAI 模型，必须同时登记 `fallbackPrices` 与 `normalizeKnownOpenAICodexModel`，否则静默漏计费。这是刻意设计（避免未知型号误计价），不是 bug；但要配套「上新模型即登记」的动作。GPT-6 时踩过一次。
11. **`Dockerfile` 的 `GOPROXY`/`GOSUMDB` 默认是国内镜像**（`goproxy.cn`/`sum.golang.google.cn`）。在 GitHub Actions 等海外 runner 上构建必须显式覆盖为官方源。
12. **`internal/service` 的 `unit` 标签测试套件当前无法编译**（多个文件的未定义符号，非近期引入）。该包暂时跑不了全量单测，改动只能跑定向用例。
13. **上游模型白名单存在 `accounts.credentials.model_mapping`**（恒等映射），不是单独的表或字段。改白名单走 `PUT /api/v1/admin/accounts/{id}`：按 `EditAccountModal.vue` 的既有约定，**请求不携带 `api_key` 字段即保留原加密凭证**。不要试图在 UI 上逐个删模型 chip——14×14 的删除按钮被 `.modal-footer` 覆盖（`elementFromPoint` 命中 footer），误点会关掉弹窗丢改动。
14. **分组「复制」会把源分组已绑定的账号一并绑到副本**。用复制建新分组后必须检查并解绑，否则新分组的请求会调度到旧账号并按新分组倍率计费。

## 六、负面教训（结论已撤回，不要重复）

- **不能用"同模型 + 时间窗口正负 N 秒"把共享上游账号的账单归属到本地用户**。据此得出的 `$652.17178575` 未追回扣费和单用户 `$384.62055975` 均已撤回，**不得用于补扣**。相关失败事件缺请求 ID、Token 和费用快照，`billing_reconciliation_cases` 一律保持待核对且金额留空——**不用次数或平均价伪造扣费**。
- **上游 Usage 页面的"费用"就是标准费用**，本地按 `标准成本 × 分组倍率 × 最终倍率` 扣费。曾因把它误读为"已含上游加价的用户价"，在一天内把 Kimi 倍率来回改了四次。
- **三种验证假阳性**（都真实误导过）：`nc -z` 对开放端口报不可达；`wget` 收到 `401` 被当成网络不通；命令管道到 `head` 使退出码恒为 0，脚本从头到尾没检查任何东西却报"✓ 通"。

## 七、未完成

- **`api.ai-genesis.app` 上游 8 个白名单模型里只有 `gpt-5.5`、`gpt-5.6-sol`、`gpt-5.6-terra` 实际可用**，`gpt-5.4`、`gpt-5.4-2026-03-05`、`gpt-5.4-mini`、`gpt-5.6`、`gpt-5.6-luna` 稳定 502/503。是否进一步收紧账号 `#1128/#1129` 的白名单未定。
- **`build.yml` 从未实际触发过**——需手动跑一次确认能出镜像且前端不 OOM。
- **外部拨测未配置**——机器宕机时无人知晓。
- **单点故障无冗余**——全部服务跑在一台 VPS 上。
- `api_base_url` 仍为空（`/keys` 页面已硬编码 `https://api.aaccx.pw/v1` 兜底，不依赖该设置）。
- Kimi、DeepSeek 分组**名称与实际倍率不一致**，待统一。
- `billing_reconciliation_cases` 3,933 条余额不足案件待外部逐笔账单核对。
- 12 条零 token / 零成本占位记录（请求类型 2）业务语义待确认。
- 续费订单与套餐的多对一审计映射缺失：当前套餐 `payment_order_id` 只绑定最新续费订单，历史续费订单无法各自独立退款。
