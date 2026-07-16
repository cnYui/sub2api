# 流量卡请求预授权与不可丢失用量事实设计

## 状态

- 设计已根据当前问题和用户确认的业务语义收敛。
- 本文只定义目标架构、数据流、失败策略和验收标准，不修改业务代码或运行态。
- 后续实施必须另写 implementation plan，并按 TDD 分阶段完成。

## 背景

当前产品语义是：用户的订阅套餐到期后，套餐权益消失，但未过期流量卡仍可继续使用。因此不能把 API Key 到期时间绑定到订阅到期时间，也不能在订阅到期时直接停用用户或 API Key。

已确认的真实漏洞是：

1. 订阅到期后，请求会正确进入余额/流量卡兜底路径。
2. 流量卡准入只检查是否存在 `remaining_usd > 0` 的未过期批次。
3. `0.00111155 USD` 这样的残余额度也能放行一次成本明显更高的请求。
4. 模型响应已经返回后才异步执行 `RecordUsage()`。
5. 流量卡不足覆盖完整费用时，扣费事务返回 `INSUFFICIENT_BALANCE`，随后 `usage_logs` 也不会写入。

问题不是“订阅到期后仍然检查流量卡”，而是系统把“存在流量卡余额”错误等同于“本次请求具备支付能力”，并且没有为已经交付的模型响应保留不可丢失的用量事实。

## 目标

本设计建立以下硬约束：

1. 有效订阅优先使用订阅；订阅不存在、到期或额度不可用时，仍允许切换到流量卡。
2. 流量卡请求只有在请求前成功预留预算后，才允许进入上游。
3. 同一用户的并发请求不能重复占用同一份流量卡额度。
4. 上游一旦产生可计费用量，必须先持久化 usage fact；扣费失败不能删除或回滚该事实。
5. 结算出现未覆盖金额时记录 debt，并阻止该用户继续发起新的流量卡请求，直到债务被处理。
6. 财务任务不能被内存队列的 `drop/sample` 策略静默丢弃。
7. API Key 保持 active，不与订阅到期时间绑定；只有现有计费来源均不可用时才拒绝请求。

## 非目标

- 不在本轮设计中修改套餐退款、购买、续费或流量卡销售规则。
- 不补扣历史上缺失 usage fact 的请求；现有日志不足以支撑可靠补扣。
- 不通过停用用户、停用 API Key 或手工清零流量卡处理系统性问题。
- 不把 Redis 作为财务事实来源；Redis 只能承担缓存和加速职责。
- 不要求首个实现同时覆盖所有非 OpenAI 平台。先覆盖已经支持 GPT 流量卡的 OpenAI 链路，再复用边界扩展。

## 方案比较

### 方案一：API Key 跟随订阅到期

订阅到期时同步设置 API Key `expires_at` 或 `status=expired`。

优点是鉴权阶段即可阻断，改动小。缺点是会错误阻断仍有有效流量卡的用户，续费和多订阅场景还需要复杂同步。该方案违背当前业务语义，不采用。

### 方案二：增加最低流量卡余额阈值

新增 `minimum_traffic_credit_reserve`，把 `remaining_usd > 0` 改成总可用额度不低于固定阈值，例如 `0.01 USD`。

该方案能立即阻止 `0.00111155 USD` 的残余额度继续调用，但不能解决：

- 单次请求实际成本高于固定阈值；
- 多个并发请求同时通过检查；
- 响应成功后 usage fact 因扣费失败而消失；
- 内存队列丢弃财务任务。

因此固定阈值只能作为预授权预算的最低门槛，不能单独作为根修复。

### 方案三：事务预授权 + durable usage fact/outbox

请求前根据最终出站请求和实际计费规则计算保守预算，在 PostgreSQL 中事务预留流量卡额度；响应完成后先写不可变 usage fact，再由 durable outbox 幂等结算，预留不足时记录 debt 并阻止后续请求。

该方案同时解决支付能力判断、并发超卖、usage 丢失和异步任务丢弃问题。改动范围较大，但职责边界清晰，能够建立可验证的财务不变量，因此采用该方案。

## 业务语义

每次请求只选择一个确定的计费来源，并把选择结果贯穿到响应后结算，不能在请求前和响应后分别重新猜测：

```text
有效订阅且当前请求可使用订阅
  -> billing_source=subscription
否则按现有规则允许使用账户余额
  -> billing_source=balance
否则 OpenAI 流量卡预授权成功
  -> billing_source=traffic_credit + reservation_id
否则
  -> 请求前拒绝
```

关键要求：

- 订阅到期不会使 API Key 失效。
- 有流量卡但预授权失败，不得进入上游。
- 请求前选择了 `traffic_credit`，响应后必须使用同一个 `reservation_id` 结算，不能再次调用 `HasAvailableCredit()` 改变计费来源。
- 余额和流量卡的先后顺序保持现有产品规则；本设计只替换流量卡的弱准入条件，不改变余额计费语义。

## 架构组件

### `TrafficCreditBudgetEstimator`

负责基于最终出站请求计算本次请求的保守费用上界，不读写数据库。

输入包括：最终计费模型和模型映射结果、最终出站请求体、endpoint、`service_tier`、分组倍率、渠道定价、其他影响 `ActualCost` 的倍率、客户端声明的最大输出 token，以及模型输出上限。

输出包括：

- `estimated_input_tokens_upper_bound`；
- `effective_max_output_tokens`；
- `reserve_usd`；
- 可重放的 `pricing_snapshot`；
- 是否需要向缺少输出上限的请求注入上限。

估算规则：

1. 按次和图片计费使用可确定的请求次数、尺寸和数量计算预算。
2. token 计费必须在模型映射、请求转换和渠道选择完成后计算，避免使用与实际结算不同的模型或价格。
3. 输入预算 V1 使用最终出站 JSON 的 UTF-8 字节数作为保守 token 上界；只对能够证明 byte-level tokenizer 满足该上界的 OpenAI 模型启用。无法证明时必须使用可靠 tokenizer，不能乐观估算。
4. 客户端显式提供 `max_output_tokens`、`max_completion_tokens` 或等价字段时，以该硬上限计算；预算不足时直接拒绝，不静默缩小客户端明确指定的上限。
5. 客户端未提供输出上限时，根据可用额度、输入预算和模型上限计算可支付的输出上限，并写入最终上游请求。若可支付输出低于配置的最低有效输出 token，则拒绝请求。
6. 定价不可用、倍率不完整或无法形成费用上界时 fail closed，返回计费预授权不可用，不能回退到 `remaining_usd > 0`。
7. `reserve_usd` 不得低于全局最低预留金额，用于过滤无实际支付能力的极小残余额度。

预算估算和最终结算必须复用同一套定价解析器，并将解析结果冻结为 `pricing_snapshot`。动态价格在请求进行期间发生变化时，结算仍使用预授权时的快照，避免预留和扣费口径漂移。

### `TrafficCreditReservationService`

负责流量卡额度的事务预留、派发标记、释放和结算。

预留事务执行：

1. 按 `expires_at, credited_at, id` 锁定用户未过期流量卡批次。
2. 计算 `available_usd = remaining_usd - reserved_usd`。
3. 使用现有 FEFO 顺序生成预留分配。
4. 可用总额不足 `reserve_usd` 时回滚并返回 `TRAFFIC_CREDIT_INSUFFICIENT`。
5. 可用总额足够时增加各批次 `reserved_usd`，创建 reservation 和 allocation，提交后才允许请求进入上游。

`FOR UPDATE` 和数据库约束是并发正确性的最终边界。Redis 可以缓存可用余额，但不能决定是否成功预留。

### `UsageFactService`

负责把一次已经产生的上游用量写成不可丢失事实。

usage fact 至少包含：

- `request_id`、`api_key_id`、`user_id`、`account_id`；
- 请求指纹；
- 请求模型、最终计费模型、上游模型；
- token、图片和缓存用量；
- 费用分项和 `ActualCost`；
- `billing_source`、`subscription_id` 或 `reservation_id`；
- `pricing_snapshot`；
- endpoint、stream、完成时间；
- `billing_status`、重试次数和最后错误。

事实字段写入后不可修改；只有结算状态、错误和重试元数据可以更新。

对于非流式响应，应在向客户端写出成功响应前持久化 fact。对于流式响应，应在发送终止事件或关闭成功流之前持久化 fact。这样数据库写入成为“模型输出完整交付”的响应屏障。

### `UsageSettlementWorker`

worker 不再接收包含业务闭包的普通内存任务，而是从 PostgreSQL durable outbox 拉取 `pending` usage fact ID。

结算必须幂等：

1. 锁定 usage fact、reservation 和相关流量卡批次。
2. 校验 `(request_id, api_key_id, request_fingerprint)`，复用或替代现有 `usage_billing_dedup`。
3. `ActualCost <= reserved_usd` 时扣除实际费用，释放多余预留。
4. `ActualCost > reserved_usd` 时尝试从尚未预留的可用额度补扣。
5. 仍有未覆盖金额时保留 fact，写入 `billing_status=debt` 和未覆盖金额；不得回滚或删除 fact。
6. 从 fact 幂等写入现有 `usage_logs`，供用户和后台统计读取。
7. 结算成功后标记 `settled`；重复消费返回同一结果，不重复扣费。

## 数据模型

### 修改 `user_traffic_credits`

新增：

```sql
reserved_usd DECIMAL(20,10) NOT NULL DEFAULT 0
```

约束：

```text
reserved_usd >= 0
remaining_usd >= 0
reserved_usd <= remaining_usd
```

对外可用额度统一定义为 `remaining_usd - reserved_usd`。现有进度条仍展示真实剩余额度 `remaining_usd`，预留金额属于短暂内部状态，不改变用户已消费金额。

### 新增 `traffic_credit_reservations`

主要字段：`id`、`request_id`、`api_key_id`、`user_id`、`platform`、`model`、`reserved_usd`、`settled_usd`、`debt_usd`、`request_fingerprint`、`pricing_snapshot JSONB`、`expires_at`、`created_at`、`updated_at`。

状态为 `reserved/dispatched/settled/released/debt`，唯一约束为 `(request_id, api_key_id)`。

### 新增 `traffic_credit_reservation_items`

记录一次预留分布在哪些流量卡批次，字段包括 `reservation_id`、`credit_id`、`reserved_usd`、`settled_usd`，唯一约束为 `(reservation_id, credit_id)`。

### 新增 `usage_facts`

使用窄索引列加版本化 JSONB payload：

- 索引列保存 request、用户、Key、状态、完成时间、reservation ID；
- `payload_version` 和 `payload JSONB` 保存完整不可变用量与价格快照；
- `billing_status` 为 `pending/settling/settled/debt/failed`；
- `attempt_count`、`next_attempt_at`、`last_error`；
- 唯一约束 `(request_id, api_key_id)`。

该表同时承担 usage fact 和 durable outbox，避免再创建一张内容重复的 outbox 表。

## 请求数据流

### 请求前

1. API Key 和用户状态正常鉴权。
2. 原有轻量资格检查继续提前过滤明显无权益请求，但不再把 `HasAvailableCredit()` 当作最终流量卡准入结果。
3. 完成分组、渠道、模型映射、上游账号选择和最终请求转换。
4. 确定订阅是否可以承担本次请求。
5. 需要流量卡时，计算预算并事务预留。
6. 预留提交后记录 `billing_source=traffic_credit` 和 `reservation_id`。
7. 在真正发起上游网络请求前，把 reservation 标记为 `dispatched`。

### 上游失败

- 明确未产生计费用量的连接、鉴权或请求校验失败，释放全部预留。
- 上游已经开始生成但最终 usage 不确定时，不得直接释放；记录待对账状态，由 reconciliation worker 处理。
- 客户端断开不能自动取消记账；上游若继续产生用量，仍需生成 fact 并结算。

### 上游成功

1. 从最终响应提取真实 usage。
2. 使用预授权时冻结的价格快照计算实际费用。
3. 同步写入 usage fact。
4. fact 写入成功后，才向客户端发送非流式成功响应或流式终止事件。
5. durable worker 异步执行实际结算和 `usage_logs` 投影。

## 债务和后续阻断

只要用户存在 OpenAI 平台的未结清 debt，新的流量卡请求必须在预授权前拒绝，返回 `BILLING_DEBT_OUTSTANDING`。

正常情况下，保守费用上界应保证实际费用不超过预留。debt 是针对上游违反输出上限、价格快照缺陷、未知用量或程序错误的最后保护，不是正常计费路径。

用户补充流量卡后是否自动偿还 debt，应在后续实施计划中沿用现有资金规则明确；第一阶段可以由 settlement worker 优先补扣并解除阻断，不允许直接忽略 debt。

## 配置建议

```yaml
billing:
  traffic_credit_reservation_enabled: false
  traffic_credit_minimum_reserve_usd: 0.01
  traffic_credit_minimum_output_tokens: 256
  traffic_credit_default_max_output_tokens: 8192
  traffic_credit_reservation_timeout_seconds: 900
  usage_fact_settlement_batch_size: 100
  usage_fact_settlement_max_attempts: 0
```

- 首次发布保持 `traffic_credit_reservation_enabled=false`，先完成 shadow 估算和差异观测，再开启强制预留。
- `minimum_reserve_usd=0.01` 只是最低门槛，最终预留仍以模型和请求预算为准。
- `max_attempts=0` 表示财务事实不因达到固定次数而丢弃，持续指数退避并报警。
- 默认输出 token 只用于客户端未声明硬上限的请求；最终值还要受模型上限和可支付额度约束。

## 错误响应

- `402 TRAFFIC_CREDIT_INSUFFICIENT`：可用流量卡不足以完成预留。
- `402 TRAFFIC_CREDIT_OUTPUT_BUDGET_TOO_LOW`：剩余额度无法覆盖最低有效输出预算。
- `402 BILLING_DEBT_OUTSTANDING`：存在未结清债务。
- `503 BILLING_PREAUTH_UNAVAILABLE`：数据库、定价或预算估算不可用，无法形成可靠预授权。

这些错误必须在请求进入上游前返回。客户端不应再看到“HTTP 200 后服务端异步报余额不足”的组合。

## 失败策略

- PostgreSQL 不可用：流量卡请求 fail closed，不进入上游。
- 定价不可用：fail closed，不使用固定猜测价格放行。
- durable worker 停止：fact 继续留在 PostgreSQL，恢复后重放。
- 内存 worker 队列满：只影响唤醒速度，不影响事实持久性；禁止 drop 财务事实。
- usage fact 写入失败：不得发送成功终止事件；记录高优先级报警。
- settlement 失败：保留 fact 和 reservation，指数退避重试。
- reservation 超时：只有确认请求未派发或明确失败时才能自动释放；`dispatched` 且 usage 未知的预留不得仅凭 TTL 清除。

## 并发与幂等

- 流量卡预留必须在 PostgreSQL 事务中对批次 `FOR UPDATE`。
- 所有金额使用 `DECIMAL(20,10)`，业务层继续使用现有金额舍入规则时必须在仓储边界统一转换，避免 float 累积误差扩大。
- request ID 和 API Key 组成财务幂等键，请求指纹不一致时返回冲突，不能复用旧结算结果。
- reservation、usage fact、ledger 和 usage log 都必须支持重复执行不重复扣费。
- 原有 `usage_billing_dedup` 在迁移期继续保留；新事实链路稳定后再决定归档或合并，不能直接删除。

## 可观测性

至少增加以下指标：

- 流量卡预授权请求数、成功数和拒绝数；
- 估算费用与实际费用差值分布；
- reservation 状态和超时数量；
- pending/debt/failed usage fact 数量及最老年龄；
- durable settlement 重试次数和延迟；
- 因最低余额、输出预算、定价不可用拒绝的请求数；
- `ActualCost > reserved_usd` 次数，正常目标为 0。

日志必须包含 `request_id`、`user_id`、`api_key_id`、`reservation_id` 和 `usage_fact_id`，但不得记录完整 API Key 或敏感请求内容。

## 测试策略

### 单元测试

- 订阅有效时不创建流量卡预留。
- 订阅到期但流量卡预算足够时成功预留并放行。
- `0.00111155 USD` 低于最低预留时请求前拒绝。
- 客户端显式输出上限超过可支付预算时拒绝。
- 客户端未提供输出上限时正确生成可支付上限。
- 定价缺失或无法证明 token 上界时 fail closed。
- 实际费用低于预留时释放差额。
- 实际费用高于预留时记录 debt，usage fact 仍存在。
- 重复 request ID 同指纹不重复扣费，不同指纹返回冲突。

### PostgreSQL 集成测试

- 多并发事务不能预留超过 `remaining_usd` 的总金额。
- 多流量卡批次按最早过期顺序预留和结算。
- 预留、结算、释放和 ledger 在事务失败时保持一致。
- usage fact 已提交后，即使 settlement 返回余额不足，fact 仍可查询且状态为 debt。
- worker 重启后可以继续结算 pending fact。

### Handler/网关测试

- 套餐到期、余额 0、流量卡足够时仍可请求。
- 套餐到期、余额 0、流量卡仅剩极小残额时不调用上游。
- 非流式响应在 fact 持久化失败时不返回成功响应。
- 流式终止事件在 fact 持久化成功前不会发出。
- usage worker 队列满时财务事实不会丢失。
- 客户端断开但上游已产生 usage 时仍可结算。

## 发布顺序

1. 发布向后兼容的 schema：`reserved_usd`、reservation 表和 `usage_facts`。
2. 先接入 usage fact/durable settlement，保留旧账务结果做双写比对，确保成功响应不再丢事实。
3. 接入预算估算 shadow mode，只记录估算值、实际值和差异，不阻断请求。
4. 修正估算偏差，确认 `ActualCost <= reserve_usd` 在目标模型和入口成立。
5. 小范围开启流量卡事务预留，观察拒绝率、预留释放和 settlement 延迟。
6. 全量开启，移除 `HasAvailableCredit()` 作为最终准入条件。
7. 稳定后再清理旧异步闭包任务和重复账务路径。

每一步都必须可以通过关闭 `traffic_credit_reservation_enabled` 回退到上一阶段，但已经写入的 usage fact 和 reservation 不得删除。回退只停止新预留，已有事实仍要完成结算。

## 验收标准

- 套餐到期但流量卡足够的用户仍能正常请求。
- 套餐到期且流量卡无法完成预授权的用户在进入上游前收到明确错误。
- 同一份流量卡余额不能被并发请求重复占用。
- 上游成功后，即使扣费失败，也能按 request ID 查询到唯一 usage fact。
- usage fact 可以重放结算且不会重复扣费。
- `RecordUsage INSUFFICIENT_BALANCE` 不再导致 usage 明细永久缺失。
- 普通 OpenAI 请求不再受 usage worker `drop/sample` 策略影响。
- API Key 无需跟随套餐过期，流量卡耗尽或存在 debt 后新的请求会被阻断。

## 实施拆分建议

该设计应拆成两个连续实施计划，避免一次改动同时重构所有网关路径：

1. `usage fact + durable outbox + 幂等结算`：先保证事实不丢、财务任务可重放。
2. `流量卡预算估算 + 事务预留 + debt gate`：再把最终准入从余额存在升级为可支付能力。

第二阶段依赖第一阶段；不能只上线预留而继续允许结算失败时丢 usage。

## 相关上下文

- `docs/ai/context/20260714-204422-expired-user-request-billing-gap-code-cause_CN.md`
- `docs/ai/context/20260714-201507-xunskyler-expired-subscription-api-key-billing-gap_CN.md`
- `docs/ai/context/20260713-134754-relay-architecture-security-hardening-whitepaper_CN.md`
