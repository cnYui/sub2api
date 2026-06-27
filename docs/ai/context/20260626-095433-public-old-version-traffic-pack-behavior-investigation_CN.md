# 公网旧版本流量卡行为调查

## 调查目标

用户在批量赠送 10 USD GPT 流量卡前，要求确认当前公网上部署的旧版本是否真正支持展示与扣费，以及扣费优先级、耗尽状态和跨日保留规则。

## 运行态对象

- 公网链路当前落到 Docker 容器 `sub2api`。
- 容器镜像：`weishaw/sub2api:latest`。
- 容器创建时间：`2026-06-24T09:52:23Z`。
- 端口映射：`127.0.0.1:18080 -> 8080`。
- 数据库容器：`sub2api-postgres`。
- 生产库迁移 `151_gpt_traffic_packs.sql` 已应用，应用时间为 `2026-06-24 10:10:41.6774+08`。

## 证据

### 1. 旧公网接口已经返回流量卡配置

对 `127.0.0.1:18080` 登录后调用：

```text
GET /api/v1/payment/checkout-info
```

返回中包含：

- `traffic_packs` 共 3 个：
  - `gpt_traffic_5usd_2cny`
  - `gpt_traffic_10usd_3cny`
  - `gpt_traffic_20usd_5cny`
- `traffic_credit_summary`：
  - 当前为 `total_remaining_usd = 0`
  - 当前为 `next_expiring_usd = 0`

这证明旧公网后端已经对用户端暴露流量卡购买项和用户可用额度摘要。

### 2. 旧公网二进制包含真实扣费逻辑

运行中二进制 `/app/sub2api` 的字符串中包含：

- `github.com/Wei-Shaw/sub2api/internal/service.shouldBillWithTrafficPack`
- `UPDATE user_traffic_credits SET remaining_usd = remaining_usd - $1`
- `INSERT INTO traffic_credit_ledger`
- `WHERE user_id = $1 AND platform = $2 AND remaining_usd > 0 AND expires_at > NOW()`
- `deduct traffic pack failed`

这证明当前公网旧版本不是只展示流量卡，而是包含扣减 `remaining_usd` 和写入 `traffic_credit_ledger` 的运行逻辑。

### 3. 生产库表结构没有“卡状态”字段

`user_traffic_credits` 字段包括：

- `initial_usd`
- `remaining_usd`
- `credited_at`
- `expires_at`
- `updated_at`

没有 `status`、`used`、`consumed` 一类状态字段。

`traffic_credit_ledger` 记录流水，字段包括：

- `entry_type`
- `amount_usd`
- `balance_after_usd`
- `created_at`

因此流量卡状态由 `remaining_usd > 0` 且 `expires_at > NOW()` 推导，而不是显式状态机。

## 逐项结论

### 1. 流量卡里的美金真的会被消耗吗？

会。

旧公网版本会在 OpenAI 平台请求产生实际费用后，从 `user_traffic_credits.remaining_usd` 扣减本次 `ActualCost`，并写入 `traffic_credit_ledger`，`entry_type = 'deduction'`。

它扣的是美元额度，不是人民币余额，也不是 `users.balance`。

### 2. 用户每天套餐额度用完后，会自动开始消耗流量卡吗？

会，前提是请求平台是 OpenAI，且用户有未过期、剩余大于 0 的流量卡额度。

计费判断：

- 如果用户有订阅套餐，并且套餐的日/周/月限制都还能覆盖本次费用，则继续扣套餐。
- 如果有任一限制无法覆盖本次费用，例如日额度已用完，则只要用户有可用 OpenAI 流量卡，就改扣流量卡。
- 如果用户没有套餐，且有可用 OpenAI 流量卡，则扣流量卡。

注意：流量卡只对 `platform = openai` 生效，不覆盖 Anthropic、Gemini 等其它平台。

### 3. 流量卡消耗完后状态怎样？

没有“已使用”显式状态。

消耗完后：

- `user_traffic_credits.remaining_usd` 变成 0。
- `HasAvailableCredit` 不再把它视为可用，因为查询条件是 `remaining_usd > 0 AND expires_at > NOW()`。
- `traffic_credit_summary` 不再统计这张卡。
- 前端不会展示“已使用”标签；用户只会在 `/purchase` 页面看到可用额度减少，耗尽后显示当前可用 `0.00` 刀。
- 数据库中仍保留原额度记录和扣费流水，便于审计。

### 4. 用户在哪里看到发放的流量卡？

主要在用户端 `/purchase` 页面，导航名是“充值/订阅”。

页面上的位置：

- 在订阅套餐列表下方的 `GPT 流量包` 区块。
- 展示文案包含：`当前可用 X.XX 刀`。
- 如果有最近到期时间，会展示：`最近 YYYY/MM/DD 到期`。
- 同一区块还展示可购买的 5/10/20 USD 流量包卡片。

如果通过订单承载赠送，用户的 `/orders` 页面也会看到一条订单记录；但订单页只展示订单状态和支付信息，不展示流量卡剩余额度，也不显示“已使用/未使用”的卡状态。

### 5. 扣费优先级是不是套餐优先，然后流量卡？

对订阅套餐用户：是。

优先级为：

1. 套餐额度仍可覆盖本次费用：扣套餐。
2. 套餐任一周期额度不足，且存在可用 OpenAI 流量卡：扣流量卡。
3. 没有可用流量卡：保持原来的超限/余额不足错误。

对没有套餐的用户：

- 如果有可用 OpenAI 流量卡，扣流量卡。
- 如果没有可用流量卡，再按余额模式走原来的余额检查和扣费。

补充：在当前实现里，OpenAI 平台只要有可用流量卡，最终用量落库阶段会优先把费用写入 `TrafficPackCost`，不会同时扣 `users.balance` 或套餐。

### 6. 流量卡没用完，第二天是否保留？

会保留。

流量卡没有日窗口，也不会每天刷新。它只有：

- `remaining_usd`
- `expires_at`

每天请求只会递减 `remaining_usd`，剩余多少就是多少。第二天继续沿用剩余额度，直到：

- `remaining_usd <= 0`，或
- `expires_at <= NOW()`。

当前 10 USD 流量卡有效期是 365 天。

## 对批量发放方案的影响

- 可以继续使用直接写库方案。
- 发放后用户主要在 `/purchase` 页面看到 `当前可用 10.00 刀`。
- 不应期待前端出现“已拥有一张 10 USD 卡”或“已使用”标签；当前版本只展示汇总额度。
- 若希望用户订单页不出现奇怪的 0 元手工订单，需要后续单独改前端或订单过滤逻辑；但不影响扣费。
