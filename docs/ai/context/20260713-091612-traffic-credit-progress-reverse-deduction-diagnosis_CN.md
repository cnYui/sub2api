# 流量卡进度条反向扣费诊断

## 结论

- 当前流量卡实际扣账方向正确：请求费用从 `user_traffic_credits.remaining_usd` 递减，并写入 `traffic_credit_ledger`。
- 用户看到 `0/10 -> 0/8 -> 0/7` 是前端展示契约错误，不是后端把总额度反向扣小。
- 根因是后端 `traffic_credit_summary` 只返回剩余额度，前端又把该剩余额度同时当作进度条分母，并把已用额度硬编码为 `0`。
- 该行为自 2026-06-26 的 `0df3031508 feat: 订阅页统一展示流量包用量` 引入；2026-07-11 的余额购买流量包事务修复没有改变汇总或扣费口径。

## 代码证据

### 后端汇总缺少初始总额与累计已用

`backend/internal/repository/traffic_pack_repo.go` 的 `GetSummary()` 仅汇总可用批次的 `SUM(remaining_usd)`。

`backend/internal/service/traffic_pack.go` 的 `TrafficCreditSummary` 仅包含：

- `total_remaining_usd`
- `next_expiring_usd`
- `next_expires_at`

接口没有提供进度条需要的 `total_initial_usd` 或 `total_used_usd`，前端无法从当前响应可靠恢复原始总额。

### 前端把剩余额度作为动态分母

`frontend/src/views/user/SubscriptionsView.vue` 当前逻辑为：

```ts
const remaining = trafficCreditSummary.value?.total_remaining_usd ?? 0
value: `$0.00 / $${remaining.toFixed(2)}`
progressWidth: getProgressWidth(0, remaining)
```

因此一张 10 USD 卡：

- 未使用时显示 `0.00 / 10.00`。
- 剩余 8 USD 时显示 `0.00 / 8.00`。
- 剩余 7 USD 时显示 `0.00 / 7.00`。
- 进度条永远是 `0%`。

`frontend/src/views/user/__tests__/SubscriptionsView.spec.ts` 只覆盖未消费的 10 USD 场景，并明确断言 `$0.00 / $10.00` 与 `width: 0%`，没有覆盖扣费后的场景，因此把缺陷固化成了测试预期。

## 运行态交叉验证

本次只读查询当前公网事实源 `sub2api-candidate-postgres`。

示例用户的一张 10 USD 可用流量卡当前数据为：

- 初始额度：`10.0000000000`
- 累计已用：`2.9675530000`
- 当前剩余：`7.0324470000`

数据库账面满足 `初始额度 - 累计已用 = 当前剩余`。现有后端接口只会返回约 `7.032447`，所以页面按当前代码显示为 `$0.00 / $7.03`，与用户反馈完全一致。

## 正确修复方向

不能只在前端用在售流量包面额、订单或流水猜测原始总额，因为用户可能有多张卡、赠送卡、不同面额、已耗尽但未过期的批次，以及先到期先扣的批次语义。

应从后端汇总层提供稳定的数据契约：

- `total_initial_usd`：统计未过期批次的 `SUM(initial_usd)`，包括 `remaining_usd = 0` 的批次。
- `total_remaining_usd`：统计未过期批次的 `SUM(remaining_usd)`。
- `total_used_usd = total_initial_usd - total_remaining_usd`。

前端展示应为 `已用 / 总额`，例如 `2.97 / 10.00`，进度宽度为 `2.97 / 10.00`。若产品希望展示“剩余”，应明确显示 `剩余 7.03 / 总额 10.00` 并让进度条方向与文案一致，不能继续使用 `0 / 剩余`。

还需明确耗尽后的产品语义：若希望进度走到 `10/10`，页面可见条件不能继续只判断 `total_remaining_usd > 0`，应按是否存在未过期批次判断；否则额度耗尽后卡片会直接消失。

## 本轮范围

- 未修改前后端代码或测试。
- 未修改 PostgreSQL、Redis、nginx、容器或运行态配置。
- 未发起真实模型请求，未产生费用。
