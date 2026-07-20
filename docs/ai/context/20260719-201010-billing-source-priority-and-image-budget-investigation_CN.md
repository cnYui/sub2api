# 计费来源顺序与生图预算调查结果

## 范围

- 只读检查当前 `main`、历史提交、恢复到本地开发库的生产数据和应用日志。
- 未修改计费代码、数据库、Redis、容器、Nginx 或公网链路。
- 本文记录根因和推荐方向，不作为已批准的实施设计。

## 结论

当前问题不是单一的“生图价格公式错”，而是“预算估算、来源选择、最终结算”三层职责没有完全统一。

历史 100 多美元现象发生在请求前预算阶段，不是成功后的实际扣款。旧代码把完整 JSON 请求体字节数当作输入 Token，24,263,139 字节按 `gpt-5.6-terra` 长上下文价格会得到约 121.32 USD 的错误预算。该错误使套餐预算检查失败，随后代码转入流量卡预授权并返回 402。

提交 `e16a67a5` 已改为只估算 JSON 文本并跳过图片/base64/file 载荷，且已通过镜像 `sub2api-candidate:20260718-124408-e16a67a58cd3` 于 2026-07-18 部署。当前仍存在来源顺序、并发预留和其它入口绕过的问题。

## 数据库与日志证据

- 用户 `id=41` 在事故时有 active 套餐 `user_subscriptions.id=110`，分组 `codex-pool-69-usd`，每日额度 69 USD。
- 2026-07-18 20:23:40 至 21:14:50 共查到 24 条失败日志，错误均为 `traffic credit is insufficient for request budget`，发生在上游调用前。
- 用户两张流量卡只剩 `0.00346455 USD`，最后一次 ledger 扣款发生在 2026-07-05；本次事故没有继续扣流量卡。
- 2026-07-18 起该用户 2159 条 `usage_facts` 均为 `is_subscription=true`、`is_traffic_credit=false`，说明成功请求按套餐结算。
- 数据库全部生图相关成功记录最大实际费用为 `1.8149825 USD`，没有任何一条达到 10 USD，更没有 100 USD 级别实际扣款。
- 历史生图记录中有 255 条旧固定价 `billing_mode=image`，另有 17 条新 Token 计费记录。用户 `id=41` 的图片编辑新记录约为 `0.03379` 至 `0.044115 USD`。
- 当前数据库没有 `request_id LIKE 'cliproxy:%'` 的 usage fact，说明新的 CLIProxy usage event 计费路径尚未在该数据快照中实际写入。

## 根因

### 1. 请求体字节与模型 Token 混用

旧预算使用 `inputTokens := len(body)`。JSON 结构、转义和 base64 图片是传输载荷，不是文本 Token，不能直接进入模型输入价格公式。

当前提交已经跳过常见图片字段，但仍是保守字符估算，不是真实 tokenizer。JSON 解析失败时仍回退到 `len(body)`，异常格式依然可能产生极端预算。

### 2. 来源顺序不是完整的“套餐 -> 余额 -> 流量卡”

`OpenAIBillingAuthorizationService.Authorize()` 当前行为为：

1. 有套餐时，先用预算调用 `CheckAllLimits()`。
2. 套餐预算通过则选择套餐。
3. 套餐预算不通过时，不再检查 `BalanceEligible`，直接进入流量卡预授权。
4. 只有没有套餐时，才会选择余额。

因此实际顺序是：

```text
有套餐：套餐能覆盖 -> 套餐；否则 -> 流量卡
无套餐：余额可用 -> 余额；否则 -> 流量卡
```

这会让保守预算误差直接表现成“流量卡优先”，也会让有套餐但套餐不足的用户跳过余额。

### 3. 来源决策有两条路径

正常 OpenAI 入口会携带 `BillingAuthorization`，结算按授权来源执行。但如果某个入口没有授权快照，`recordUsage()` 会调用 `shouldBillWithTrafficPack()`，根据响应后的实际成本再次选择来源。

同一业务规则存在请求前和响应后两套实现，端点漏接授权时会产生不同顺序。来源应该只决定一次，结算层只能执行，不能改源。

### 4. 套餐没有 reservation

套餐检查只读取 `DailyUsageUSD/WeeklyUsageUSD/MonthlyUsageUSD` 快照，没有像流量卡一样预留预算。多个并发请求可能同时通过检查，最终一起写入后超过套餐额度。

如果要求严格保证顺序和额度上限，套餐也必须具备 reservation；仅调整 `if` 顺序无法解决并发超卖。

### 5. 生图预授权仍然过度保守

最终结算已经按上游实际 Token 拆分主模型文本、缓存、图片输入和图片输出，并从主模型 Token 中减去图片 Token，未发现重复计费。

但图片编辑的请求前输入预算仍按以下公式估算：

```text
图片输入 Token 上界 = 输入图片数量 * 23719
```

输出尺寸或质量未知时，输出预算也使用每张 23719 Token。它不再依赖 3MB 文件大小，但仍可能把正常图片编辑请求估得过高，再次触发错误来源切换。

### 6. CLIProxy usage event 不能独立决定计费

新增的 `InternalUsageEventService` 当前：

- 硬编码 `BillingTypeBalance` 和 `BalanceCost`，不识别 active 套餐或流量卡 reservation。
- 使用 `cliproxy:{request_id}` 新建独立 fact，不能与 Sub2API 已有 fact 共用去重键，存在同一请求双计费风险。
- 回调协议没有图片输入/输出 Token，无法准确补充生图账单。

该路径当前未观察到实际写入，但启用前必须修正。CLIProxy 回调只能补全同一个 Sub2API usage fact，不能自行重新选择付款来源。

## 修改方案比较

### 方案 A：统一预授权与 reservation，推荐

- 建立唯一 `BillingSourceDecision`，顺序固定为“套餐 -> 余额 -> 流量卡”。
- 每次请求只能选择一个完整计费来源，不跨来源拆分。
- 套餐、余额、流量卡分别尝试原子 reservation；前一来源不能完整覆盖预算时才尝试后一来源。
- 授权结果携带来源、reservation、价格快照和请求指纹，并写入 `usage_facts`。
- 最终结算只使用上游实际 Token 结算已选来源，多退少补或记录 debt，禁止重新选源。
- CLIProxy 回调使用统一 correlation id 更新原 fact，不另建独立账单。

优点：顺序、并发和幂等一次解决。缺点：需要新增套餐 reservation，并统一现有入口。

### 方案 B：只改来源判断

- 有 active 套餐且仍有剩余额度时始终使用套餐。
- 套餐耗尽后再检查余额和流量卡。
- 保留现有无套餐 reservation 的结构。

优点：改动小。缺点：并发请求可超额；单个大请求可能让套餐超过日限额；旧后置判断和内部回调仍可能绕过规则。只能作为临时行为修正，不是根治。

### 方案 C：单请求跨来源拆分

- 一个请求先消耗套餐剩余，再扣余额或流量卡。

不建议。它破坏“一次请求一个唯一计费来源”，使退款、重试、幂等、usage fact 和流量卡 debt 都显著复杂化。

## 推荐设计方向

采用方案 A，按以下顺序推进：

1. 用决策表和失败测试固化“套餐 -> 余额 -> 流量卡”以及“一次请求唯一来源”。
2. 把所有 OpenAI Responses、Chat、Images、Embeddings、WebSocket 入口统一接入同一个 authorizer。
3. 删除结算层 `shouldBillWithTrafficPack()` 的来源重判，只保留对历史无授权 fact 的显式兼容处理。
4. 为套餐增加按日/周/月窗口的 reservation，使用事务锁或原子 SQL 防止并发超卖。
5. 将图片预算改为基于解析后的语义字段和图片元数据估算；传输字节永不参与价格计算。
6. CLIProxy usage event 改为更新相同 correlation id 的 pending fact；无法关联时只记录审计事件，不直接扣费。
7. 最终用数据库断言验证：来源唯一、套餐优先、无重复 fact、图片实际费用来自上游 Token。

## 待确认的业务边界

当套餐仍有少量余额，但无法完整覆盖单次请求预算时，推荐整次请求按下一来源计费，顺序为余额后流量卡；不拆分该请求。若业务要求套餐只要还有余额就必须先扣到零，则需要跨来源拆分，与现有唯一来源红线冲突，不建议采用。

## 验证

以下现有单测已通过：

- `TestOpenAITrafficCreditBudget_DoesNotPriceBase64ImageBytesAsTextTokens`
- `TestOpenAITrafficCreditBudget_ImageUsesImageTokenBoundsWithoutOutputClamp`
- `TestOpenAIBillingAuthorization_UsesSubscriptionWhenBudgetFits`
- `TestOpenAIBillingAuthorization_ReservesTrafficCreditWhenSubscriptionExceeded`

这些测试同时证明：base64 字节问题已修复，但“套餐预算不通过后转流量卡”仍是当前被单测固化的行为。
