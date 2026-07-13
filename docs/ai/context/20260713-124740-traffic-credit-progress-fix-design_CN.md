# 流量卡进度条修复设计

## 背景

用户侧 `/subscriptions` 当前把 `traffic_credit_summary.total_remaining_usd` 同时当作流量卡进度条的总额度，并把已用额度硬编码为 `0`。因此一张 10 USD 流量卡从 10 USD 扣到 8 USD、7 USD 后，页面依次显示 `0/10`、`0/8`、`0/7`，进度条始终为 0%。

运行态账面与后端扣费方向正常：`user_traffic_credits.remaining_usd` 会随真实费用递减，`traffic_credit_ledger` 会记录 deduction。问题仅在汇总接口缺少固定总额，前端无法构造 `已用/总额`。

## 已确认的历史语义

依据以下历史文档：

- `docs/ai/context/20260626-095433-public-old-version-traffic-pack-behavior-investigation_CN.md`
- `docs/ai/context/20260626-095839-traffic-pack-bulk-grant-result_CN.md`
- `docs/ai/context/20260703-112900-traffic-card-subscriptions-page-diagnosis-result_CN.md`

本次必须保持：

- 只有 `remaining_usd > 0` 且未过期的流量卡批次参与可用额度汇总。
- 某个批次用满后自动从汇总移除，不展示“已使用完”状态。
- 最后一张可用流量卡用满后，`/subscriptions` 的流量卡卡片自动消失。
- 不修改订阅优先、订阅不可用或额度耗尽后使用流量卡的计费顺序。
- 不修改扣费、流水、有效期、购买履约或多批次先到期先扣规则。

## 方案比较

### 方案一：后端扩展当前可用批次汇总，前端计算已用额度

在现有 `TrafficCreditSummary` 增加 `total_initial_usd`。仓储使用与 `total_remaining_usd` 相同的可用批次过滤条件，同时汇总 `SUM(initial_usd)` 和 `SUM(remaining_usd)`。前端使用 `total_initial_usd - total_remaining_usd` 得到已用额度。

优点：

- 数据源与扣费事实一致。
- 保持用满批次自动消失的历史语义。
- 兼容赠送卡、多面额、多批次和部分消费。
- 改动范围小，不新增接口或明细模型。

缺点：

- 当多张卡中的一张用满并从汇总移除时，汇总进度会自然切换到剩余可用批次；这是现有“用满即消失”语义的直接结果。

### 方案二：通过 `traffic_credit_ledger` 重算购买与扣费总额

从流水汇总 purchase 和 deduction，再构造进度。

不采用。流水需要额外处理过期、耗尽、历史赠送和批次边界，查询成本和错误空间都更大，且没有比 `user_traffic_credits` 提供更可靠的当前状态。

### 方案三：前端从在售商品或订单推断原始额度

不采用。商品可能下架或改价，赠送卡和历史订单不一定与当前在售商品一致，多批次也无法可靠对应。

## 最终设计

采用方案一。

### 后端契约

`TrafficCreditSummary` 新增：

```json
{
  "total_initial_usd": 10,
  "total_remaining_usd": 7,
  "next_expiring_usd": 7,
  "next_expires_at": "2027-06-26T08:57:24+08:00"
}
```

`total_initial_usd` 与 `total_remaining_usd` 使用相同过滤条件：

```sql
user_id = $1
AND platform = 'openai'
AND remaining_usd > 0
AND expires_at > now
```

因此已耗尽和已过期批次都不会进入分子或分母。

### 前端展示

`SubscriptionsView` 计算：

```ts
const total = summary.total_initial_usd
const remaining = summary.total_remaining_usd
const used = Math.max(total - remaining, 0)
```

展示值和进度：

- 未使用：`$0.00 / $10.00`，0%。
- 剩余 7 USD：`$3.00 / $10.00`，30%。
- 所有可用卡耗尽：`total_remaining_usd = 0`，沿用现有条件隐藏整张流量卡卡片。

前端不从商品、订单或流水推断额度，也不改变到期文案和卡片结构。

### 兼容性

- 新字段是 JSON 响应的向后兼容扩展，不删除或改名现有字段。
- 当前仓储始终返回 `total_initial_usd`；前端类型将其设为必填字段，不为旧后端响应增加猜测性 fallback，避免再次用剩余额度冒充总额。
- 本地前后端必须作为同一版本发布。

## 测试设计

### 后端仓储

补充或扩展 `traffic_pack_repo_test.go`：

- 部分消费的可用批次同时返回初始总额和剩余额度。
- 已耗尽批次不进入 `total_initial_usd` 与 `total_remaining_usd`。
- 已过期批次不进入汇总。
- 多个可用批次按总和返回。

### 前端

扩展 `SubscriptionsView.spec.ts`：

- `total_initial_usd=10`、`total_remaining_usd=10` 时显示 `$0.00 / $10.00` 和 0%。
- `total_initial_usd=10`、`total_remaining_usd=7` 时显示 `$3.00 / $10.00` 和 30%。
- `total_remaining_usd=0` 时不显示流量卡卡片。

## 范围

本次只修改：

- 流量卡汇总结构和仓储查询。
- 前端支付类型定义。
- 订阅页流量卡进度展示。
- 对应后端和前端测试。

不修改数据库 schema、支付流程、真实扣费逻辑、流量卡流水、订阅计费、运行态数据或部署配置。
