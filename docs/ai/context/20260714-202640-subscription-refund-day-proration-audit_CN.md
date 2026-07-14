# 套餐退款按天比例口径核对

时间：2026-07-14 20:26 JST

## 结论

当前退款金额包含两个独立口径：

1. 支付宝套餐退款不退手续费，基数使用订单 `amount`，不是包含 1% 手续费的 `pay_amount`。
2. 剩余天数按 `floor((subscription.expires_at - now) / 24h)` 计算，属于“剩余完整 24 小时块”，不是按北京时间自然日计算。

因此，用户提出的公式只满足第一部分；“使用到第六天，即使当天未用完也算第六天已用完，剩余 24 天”当前并不总能满足。

## 当前代码

`backend/internal/service/payment_amounts.go`：

```go
base := orderAmount
if includeFee {
    base = payAmount
}
remainingDays := int(expiresAt.Sub(now).Hours() / 24)
refund := roundToOneDecimal(base * remainingDays / subscriptionDays)
```

- 支付宝路径传入 `includeFee=false`，所以 29 元套餐实付 29.29 元时，退款基数是 29 元。
- 余额支付路径传入 `includeFee=true`，退款基数使用 `pay_amount`。
- 最终退款金额保留 1 位小数并四舍五入。

## 示例

### 当前实现能符合的情况

若剩余时长为 `24 天 4.8 小时`：

- `remainingDays = floor(24.2) = 24`
- 29 元套餐退款：`round(29 * 24 / 30, 1) = 23.2 元`

### 当前实现不符合用户口径的情况

假设北京时间 7 月 1 日 23:00 开通 30 天套餐，7 月 6 日 01:00 申请退款：

- 从自然日看已经使用到第六天，应按已用 6 天、剩余 24 天。
- 当前只经过 4 天 2 小时，距离到期仍约 25 天 22 小时。
- 当前代码会得到 `remainingDays=25`，多退 1 天。

即使退款时刻正好是开通后 5 个 24 小时，当前公式也会得到剩余 25 天；而“第六天一开始就算第六天已用完”的口径要求剩余 24 天。

## 正确业务表达

若按用户描述，应明确采用北京时间自然日：

```text
used_days = 北京时间日期差 + 1
remaining_days = max(0, subscription_days - used_days)
refund_amount = round_to_0.1(order_amount * remaining_days / subscription_days)
```

例如购买日为第 1 天，进入第 6 个北京时间自然日后，无论当天使用了多久，都按已使用 6 天、剩余 24 天计算。

实现前还需补充边界测试：购买当天立即退款、跨午夜、到期当天、夏令时无关性，以及支付宝不退手续费和余额支付是否继续退手续费。

## 验证

以下目标测试通过：

```text
go test -count=1 -tags=unit ./internal/service -run 'TestCalculateSubscriptionRefundAmountFloorsRemainingDaysAndRoundsToOneDecimal|TestRequestRefundAutomaticallyRefunds(AlipaySubscriptionWithoutFeeAndRevokesSubscription|BalanceSubscriptionIncludingFee)'
```

本轮未修改退款代码、订单、订阅、支付配置或运行态数据。
