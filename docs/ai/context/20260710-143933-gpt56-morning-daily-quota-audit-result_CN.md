# GPT-5.6 今日早上用量是否计入每日额度审计结果

## 结论

已计入。

本次只读审计运行态 `sub2api-candidate-postgres`，没有修改 Postgres、Redis、nginx、容器或用户余额。

今天早上北京时间 `2026-07-10 00:00:00+08` 到 `2026-07-10 12:00:00+08` 的 GPT-5.6 用量：

- 共 `317` 条。
- `gpt-5.6-sol`：`304` 条，`48.0182650000 USD`。
- `gpt-5.6-terra`：`13` 条，`0.7793635000 USD`。
- `gpt-5.6-luna`：`0` 条。
- 合计 `48.7976285000 USD`。
- 涉及 `6` 个用户、`6` 个订阅。
- `317/317` 条 `total_cost > 0`。
- `317/317` 条 `billing_type = 1`。
- `317/317` 条有 `subscription_id`。
- `317/317` 条有 `usage_billing_dedup`。
- `0` 条缺 dedup。
- `0` 条零成本。

## 每日额度核对

按涉及订阅从 `user_subscriptions.daily_window_start` 起汇总 `usage_logs.total_cost`，与 `user_subscriptions.daily_usage_usd` 对比：

- `6/6` 个相关订阅完全一致。
- 最大差值 `0.0000000000`。

说明 GPT-5.6 费用不是只写进 `usage_logs.total_cost`，而是已经进入订阅日用量字段。

早上 GPT-5.6 对订阅的贡献：

| subscription_id | 用户 | 日用量字段 | 今日窗口 usage 总和 | GPT-5.6 条数 | GPT-5.6 费用 |
| ---: | --- | ---: | ---: | ---: | ---: |
| 89 | `1***@qq.com` | `46.0693690000` | `46.0693690000` | 255 | `40.9627970000` |
| 79 | `3***@qq.com` | `4.2406336000` | `4.2406336000` | 20 | `4.2385720000` |
| 83 | `b***@163.com` | `51.8764020000` | `51.8764020000` | 29 | `2.8721710000` |
| 72 | `8***@qq.com` | `0.5490110000` | `0.5490110000` | 10 | `0.4147160000` |
| 86 | `9***@qq.com` | `2.9770810000` | `2.9770810000` | 2 | `0.3030775000` |
| 45 | `x***@gmail.com` | `92.9803105500` | `92.9803105500` | 1 | `0.0062950000` |

## 按当前规则复算

使用当前 GPT-5.6 规则复算早上 317 条：

- `gpt-5.6-sol`：input `0.000005`、cache read `0.0000005`、cache write `0.00000625`、output `0.000030`。
- `gpt-5.6-terra`：input `0.0000025`、cache read `0.00000025`、cache write `0.000003125`、output `0.000015`。
- `gpt-5.6-luna`：input `0.000001`、cache read `0.0000001`、cache write `0.00000125`、output `0.000006`。
- 超过 `272000` 输入上下文时：输入/cache/cache write `2x`，输出 `1.5x`。
- `service_tier=priority` 才 `1.5x`；本批早上记录 `service_tier` 全为空，`rate_multiplier` 全为 `1.0000`。

复算结果：

- 早上 `317` 条中 `1` 条触发长上下文倍率。
- stored total：`48.7976285000`。
- expected total：`48.7976285000000`。
- 最大单条差值：`0.000000000000`。
- mismatch rows：`0`。

因此早上 GPT-5.6 不仅进入了每日额度，价格也与当前规则一致。

## 截至 13:39 的追加快照

北京时间 `2026-07-10 13:39:20+08` 重新抓取今天全天 GPT-5.6 快照：

- `gpt-5.6-sol`：`996` 条，`127.7829300000 USD`。
- `gpt-5.6-terra`：`16` 条，`0.9321740000 USD`。
- 合计：`1012` 条，`128.7151040000 USD`。
- 涉及 `11` 个订阅。
- `1012/1012` 条 `total_cost > 0`。
- `1012/1012` 条 `billing_type = 1`。
- `1012/1012` 条有 `subscription_id`。
- `1012/1012` 条有 `usage_billing_dedup`。
- `11/11` 个相关订阅的 `daily_usage_usd` 与今日窗口 usage 总和一致，最大差值 `0.0000000000`。

说明新版本发布后新增的 GPT-5.6 请求也在持续实时进入订阅每日额度。

## 关于 `billing_usage_entries`

只读查询发现本批 GPT-5.6 在 `billing_usage_entries` 中没有对应行。

这不是“未扣费”的证据。当前生产热路径 `usageBillingRepository.Apply` 的真实动作是：

1. 写 `usage_billing_dedup` 占用幂等键。
2. 直接累加 `user_subscriptions.daily_usage_usd/weekly_usage_usd/monthly_usage_usd`。

代码没有在该路径写 `billing_usage_entries`，所以本次判断以 `usage_billing_dedup` 和 `user_subscriptions.daily_usage_usd` 为准。

## 风险与后续

- 今天早上这批无需补扣，重复补扣会造成重复扣费。
- 若未来发现旧 GPT-5.6 记录缺 `usage_billing_dedup`、缺 `subscription_id`、`total_cost=0` 或订阅日用量不匹配，才需要按 dry-run 清单处理。
- 因请求仍在持续产生，后续再次审计时数字会变化，应以新的快照时间为准。
