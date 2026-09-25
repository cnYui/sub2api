# 欠费、逐笔扣费与漏收核查（2026-09-25）

管理员问：当前用户欠费情况，每一笔是否真实扣费，有没有漏收，还有没有 0 元就能用的 bug。

- 方式：诊断密钥 SSH 进生产 Mac，psql 会话级强制 `READ ONLY` + 语句超时，全程只读；另派两个代理读代码审计准入与计费路径。
- 生产镜像 `sha-2a1f90bb`（= main HEAD，含 #57「余额恰为 0 按耗尽处理」），`BILLING_FINAL_MULTIPLIER=19`。
- 时间均为 +08。

## 隐藏倍率分段（核对历史扣费必须按此分段）

| 区间（UTC） | 倍率 |
| --- | --- |
| 2026-09-17 06:42:20 之前 | 18 |
| 2026-09-17 06:42:20 ～ 2026-09-25 02:25:24 | 17 |
| 2026-09-25 02:25:24 之后（容器重建时刻） | 19 |

按这三段算，近 30 天 70,484 条 `total_cost > 0` 的记录**全部**满足 `actual_cost = total_cost × usage_logs.rate_multiplier × 倍率`（numeric 精确到 8 位），0 条偏差，说明切换时刻取得准确。

## 一、逐笔扣费是真实的

1. **成功返回的请求都进了扣费**：
   - 注意：`usage_logs` **不**在扣费事务里。扣费事务（dedup、余额、套餐、流量卡、欠费、Key 与账号配额）提交后才写 `usage_logs`（`writeUsageLogBestEffort`）。扣费事务报错时整笔回滚，两边都不留痕，只有 `*.record_usage_failed` 日志，没有重试。所以「dedup 与用量记录 1:1」只能说明写用量记录从没失败，证明不了没有漏扣。
   - 08-04 以来 151,736 条用量记录都有 dedup 行，08-05 以来 dedup 行也都有用量记录，确实 1:1。
   - 真正的证据是下面两条：
     - 近 7 天 HTTP 200 且打到上游的推理请求 9,917 次，9,871 次有用量记录，其余 46 次见三-D。
     - `record_usage_failed` 自 08-25（日志保留起点）起全部是舍入 bug，09-17 修复后为 0。
2. **余额真的被扣了（锚点对账）**：套餐到账时 `balance_debt_ledger` 和提前到账审计会记下「到账前 / 到账后」余额。相邻两次到账之间，走余额路径的扣费合计应当正好等于余额的下降。
   - 共 113 段，其中 57 段中间没有其它动余额的事件（兑换码、返利、其它套餐审计），可精确比对，合计覆盖 13,210 条请求、$6,933.63。
   - 其中 56 段一分不差（误差 < $0.000001）。
   - 1 段（user 516，08-27 → 09-03）余额比扣费多降 $17.00：09-03 旧套餐 25 到期回收剩余额度（到期回收没有审计记录）。方向是多扣，不是漏收。
   - 判定「走余额路径」= 该请求在 `traffic_credit_ledger`（deduction）和 `traffic_credit_debt_ledger`（debt）里都没有记录。
3. **流量卡路径**：122 笔流量卡欠费记录，每笔「卡扣 + 欠费」都精确等于该请求 `actual_cost`。
4. **单价**：近 14 天 23 个模型的实际单价逐条一致（每个模型 min = max），与 AGENTS.md 登记价相同。
5. **没有带 token 的 0 元记录**：近 30 天 572 条 0 元记录全部是 0 token 占位（与 AGENTS.md 第七节结论一致）。
6. **09-17 舍入 bug 修复后 `record_usage_failed` 为 0**；此前的失败已在 `billing_reconciliation_cases` 补扣。
7. **长连接 WebSocket 没有漏收**：近 7 天 971 次 `GET /v1/responses` 升级全部没选到账号（access 日志无 account_id），60 天内 `openai_ws_mode=true` 的用量为 0，从未打到上游。

## 二、当前欠费（2026-09-25 11:00 快照）

- 余额为负：35 人，合计 **-$883.69**。
  - user 529 一人 -$747.39，是 09-17 舍入 bug 的补扣（当时没扣到的钱），不是新漏洞。
  - 其余 34 人合计 -$136.30。
- 流量卡欠费（`traffic_credit_debt_ledger` 净额）：34 人，合计 **$107.09** 未还。
- 余额恰为 0：169 人，其中 112 人没有可用流量卡（#57 之后这些人会被 403 拦下）。

负余额怎么来的（看最后一笔走余额的请求，`当前余额 + 该笔费用` 反推扣前余额）：

| 类型 | 人数 | 金额 | 说明 |
| --- | --- | --- | --- |
| 0 余额白用一次（#57 修复前） | 8 | -$37.92 | 505 $31.48、674 $3.95、656、534、654、504、596、494；最晚一笔 09-23 22:59，修复后没有再出现 |
| 正余额最后一笔透支（设计口径） | 8 | -$7.51 | 480、643、644、478、547、522、500、645 |
| 其后还有补扣 / 取消 / 到期等事件 | 17 | 其余 | 包括 529 的补扣、605 的补扣 $25.86、506 的 08-07 前旧欠费等 |
| 08-09 旧代码时期 | 1 | -$0.49 | 497 |
| 最后一笔是 0 元占位 | 1 | -$0.17 | 557，流量卡用户 |

从来没付过任何钱却用过服务的只有 2 人：656（$1.03，09-19）、654（$0.42，09-08），都是 #57 修复前的 0 余额漏洞。

## 三、漏收

### A. 并发请求在余额转负后落进「流量卡欠费」，而套餐到账不抵它（结构性，仍在发生）

- 机制：准入按请求开始时的余额放行；请求结束扣费时如果余额已被同一用户的其它并发请求扣成负数，`applyUsageBillingEffects` 走流量卡分支（`balanceBefore < 0` 时不论流量卡有没有钱都算流量卡），扣不到的部分记进 `traffic_credit_debt_ledger`。
- 这笔欠费只在**再买流量卡**时抵扣；套餐到账（首期 / 周刷新 / 提前刷新 / 续费）只抵负余额。只续套餐的用户这笔会一直挂着，且余额转正后照常使用，不受影响。
- 实例：user 500 09-24 14:16:51 一笔 $3.34（14:12:02 开始时余额为正，14:16:46 被别的请求透支到 -0.2289）；14:34 续费到账只抵了 -0.2289，$3.34 仍挂在流量卡欠费上。
- 规模：122 笔共 $190.44，其中 36 笔（$44.34）结束时一张卡的钱都没扣到；已随买卡还上 $83.35，未还 $107.09。未还部分里 22 人名下有有效套餐（$94.69），17 人余额为正正常在用。
- #57 已对「余额恰为 0 且没有任何流量卡」改为记余额透支（理由正是「记成流量卡欠费反而收不回来」），但「余额已为负且没有流量卡」这一支没改。

### B. 正余额最后一笔不设上限整笔透支，且每人并发 20（设计口径，敞口大）

- 余额只要 > 0，任意大小的请求都放行；所有用户 `concurrency=20`，余额 $0.01 时 20 个并发大请求都能进来，第一笔透支后其余落入 A。
- 近 30 天单笔最大 $206.19（kimi-k3，77.9 万 token 上下文），p99 $4.16，p99.9 $11.85。
- 历史上透支后被套餐到账还上的 $995.33（137 次）；未还的就是上面 34 人的 -$136.30。

### C. 0 余额白用一次（已由 #57 修复）

- #57 镜像 09-24 12:14（+09）构建并部署。修复前 8 人共 $37.92；修复后没有新的「0 → 负」记录。
- 修复后所有流量卡欠费（500、557、478）都能用 A 的并发时序解释：请求开始时余额为正或流量卡有钱。

### D. 流式中途断开不计费（小额）

- 近 7 天 HTTP 200 且打到上游的推理请求 9,917 次，46 次没有用量记录；全部是流中途失败：`stream usage incomplete: missing terminal event`、上游 `overloaded / temporarily unavailable / rate limit`、HTTP/2 `INTERNAL_ERROR`。
- 其中不少耗时恰好约 128 秒，集中在火神账号 1168（分组 74），像是上游约 2 分钟截断。用户拿到了部分输出，我方不扣费，上游可能照收。
- 09-11～09-16 每天 60～143 次是舍入 bug 回滚（已补扣），09-17 后降到每天 0～24 次。

### E. CF 约 125 秒超时（已知，见 AGENTS.md 第七节）

- 14 天 669 次 499，`ops_error_logs` 显示全部没收到首字，用户没拿到内容；我方不扣费，上游可能照收。

## 四、代码审计（两个只读代理，结论已逐条复核代码与数据）

路径相对 `backend/internal/`。

### 准入：余额模式主链路没有绕过

- 中间件用认证快照余额（L1 15 秒、Redis 300 秒）做粗筛，每个转发 handler 在选账号、转发之前再调 `CheckBillingEligibility`，它通过 `getFreshBillingBalance` 实时读库（`service/billing_cache_service.go:797-810`）。
- 覆盖 Messages / Responses / Chat Completions / Gemini / Images / Embeddings / count_tokens / WS / Live 全部转发入口。
- 流量卡净额 = 未过期卡的 `remaining_usd` − 流量卡欠费，严格大于 0 才放行，直连库没有缓存（`repository/traffic_pack_repo.go:52-77`）。
- `reserved_usd` 是旧迁移脚本留下的列，现行代码不读。
- 两个不影响生产的缺口：
  - 订阅型分组分支不读余额，只靠中间件快照拦。生产 27 个分组全是 `standard`，不可达。
  - Gemini / Codex 的模型清单 GET 会打上游，但不推理、不计费。

### 用户可以主动触发的漏收

1. **删 Key 让整笔扣费回滚（最严重，0 元无限用）**
   - Key 设了 `quota` 或 `rate_limit_*`（用户自己就能设，`handler/api_key_handler.go:38-59`）时，扣费事务会更新 Key 用量。
   - 请求进行中删掉这把 Key（软删），更新语句带 `deleted_at IS NULL`，0 行返回 `ErrAPIKeyNotFound`（`repository/usage_billing_repo.go:919/941`）。整笔事务回滚，已扣的余额也退回，用户完整拿到输出。
   - 余额不变，所以准入一直放行：建 Key → 并发 20 个请求 → 删 Key → 再建，可以无限重复。只需要余额为正（或流量卡有钱）。
   - 数据：08-25 以来 `record_usage_failed` 全部是舍入 bug，没有 `API_KEY_NOT_FOUND`，**还没人用过**。现有 19 把有效 Key 设了 quota，1 把设了限速。
   - **已修复（同日，管理员下令，PR #60）**：`applyUsageBillingEffects` 遇到 `ErrAPIKeyNotFound` 只跳过 Key 的额度 / 限速统计，余额照扣，并打日志 `[UsageBilling] api key deleted before settlement`（以后搜这句就能看到有没有人试）。其它数据库错误仍然整笔回滚。
   - 测试：sqlmock 单测 3 个（额度 Key 被删、限速 Key 被删、其它错误照常回滚）+ 真实 Postgres 集成测试 1 个（走真实软删路径）。修复前的代码上这 3 个删 Key 用例都失败，报 `API_KEY_NOT_FOUND`。
   - 同类但没改：账号被管理员删除时 `incrementUsageBillingAccountQuota` 返回 `ErrAccountNotFound`，同样会整笔回滚。只有管理员能触发，没列入这次修复。
2. **Responses 结果里带图就只按张收费，文本 token 不计**
   - `service/openai_gateway_usage.go:436-440`：`ImageCount > 0` 且渠道不是 token 定价时，只算图片费。
   - 在文本分组里带上 `image_generation` 工具、要一张图，整段上下文就免费。
   - 近 30 天发生 4 次，例：09-05 user 563 在分组 74 用 gpt-5.6-terra，17.5 万输入、1.5 万输出 token，只按 7 张图收了 $7.09。
3. **Claude 分组经 `/v1/chat/completions`、`/v1/responses` 转换时，客户端一断就只收输入费**
   - `service/gateway_forward_as_chat_completions.go:413-415,469-470`、`gateway_forward_as_responses.go:533-537,590-591`：写客户端失败就停止读上游，输出 token 在最后的 `message_delta` 里，收不到。原生 `/v1/messages` 会读完上游，不受影响。
   - 近 30 天只在已停用的分组 71 出现 17 次；在售的 4、84 目前全走 `/v1/messages`。

### 上游或运维触发的漏收（小额）

- **OpenAI 流中途失败不计费**（`service/openai_gateway_forward.go:933-936`）：即三-D 的 46 次。Anthropic `/v1/messages` 有补记，OpenAI 路径没有。
- **Claude `/v1/messages` 流里出现 `event: error` 不计费**：日志 `[Forward] SSE error event in stream` 共 2 次（08-27、09-11）。
- **上游不给 usage 就按 0 token 记，不估算、不打日志**：近 30 天 572 条 0 元记录里，不少有首字、耗时几十秒甚至几十分钟，是真有输出。AGENTS.md 第七节记录 09-17 与 ai-genesis 逐条核对过，上游同样记 0，对我们不是损失；但那次只核了 1128、1129，账号 1164、1132、1166、1173 的 14 条没核对过。
- **发版停机只等 5 秒**（`cmd/server/main.go:182`）：超过 5 秒的在途请求被切断，不计费也不留日志。近 7 天平均同时在途 0.44 个请求、单笔均价 $0.58，每次发版约漏 $0.25。另外 Docker 默认 10 秒后强杀，只改 Go 超时不够，还要设 compose `stop_grace_period`。

### 潜伏（当前不触发）

- 缺价按 0 计费（坑 12）：08-25 以来 `pricing_missing_record_zero_cost` 为 0 次。分组 76/81/82 的 glm-5.3、glm-5.3-flash、deepseek-v4.1-flash 只靠渠道定价；渠道定价缓存加载失败时有约 5 秒空窗（`service/channel_service.go:148,304-307`）。
- 账号开 `openai_passthrough` 或白名单为空时放行任意模型：在售分组的 12 个账号都没开 passthrough，白名单都非空。
- `service_tier=flex` ×0.5 仍在，零使用。Live 仍被硬拒，batch_image 仍关闭。

## 查询与方法

- 只读执行脚本：`SET SESSION CHARACTERISTICS AS TRANSACTION READ ONLY` + `statement_timeout`，SQL 从 stdin 传给 `docker exec -i sub2api-postgres psql`。只读会话里不能建临时表，要用 CTE。
- 请求与用量对应：`usage_logs.request_id = 'client:' || ops_system_logs.client_request_id`（http.access 日志）。
- 锚点对账的 SQL 思路：锚点取 `balance_debt_ledger(balance_before_usd, balance_after_usd)` ∪ 审计 detail 带 `balance_before_usd/after_usd` 的提前到账；相邻锚点之间 `前一锚点 after − 后一锚点 before = Σ 走余额路径的 actual_cost`，段内有兑换码 / 返利 / 其它套餐审计的段跳过。
