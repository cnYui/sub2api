# milesyang987@gmail.com GPT-5.6 与额度为 0 只读审计结果

## 结论

`milesyang987@gmail.com` 今天早上没有使用 GPT-5.6，也没有任何成功 API usage；当前每日额度为 `0` 是正常结果，不是 GPT-5.6 计费漏扣。

本次只读查询运行态 `sub2api-candidate-postgres`，没有修改 Postgres、Redis、nginx、容器或用户余额。

## 用户与订阅

- 用户：`user_id=3`
- 邮箱：`milesyang987@gmail.com`
- 用户状态：`active`
- 用户余额：`0.00000000`
- API Key：`api_keys.id=2`，状态 `active`
- API Key 最后使用时间：`2026-07-09 12:10:50.522231+08`
- active 订阅：`subscription_id=2`
- 分组：`codex-pool-19-usd`
- 订阅日额度：`19.00000000 USD`
- `daily_window_start=2026-07-10 00:00:00+08`
- `daily_usage_usd=0.0000000000`

## 今天与早上使用情况

北京时间 `2026-07-10 00:00:00+08` 到 `2026-07-11 00:00:00+08`：

- usage rows：`0`
- total cost：`0`
- API Key `id=2` usage rows：`0`
- error logs：`0`
- system logs：`0`

北京时间 `2026-07-10 00:00:00+08` 到 `2026-07-10 12:00:00+08`：

- usage rows：`0`
- total cost：`0`
- GPT-5.6 rows：`0`
- GPT-5.6 cost：`0`

因此他当前每日使用额度为 `0` 的直接原因是：日窗口今天 0 点重置后，没有新请求。

## 全历史模型使用

该用户全历史没有 GPT-5.6：

- `gpt-5.6-*` all-time rows：`0`
- `gpt-5.6-*` all-time cost：`0`

该用户历史成功 usage 主要是：

- `gpt-5.4`：`3224` 条，`170.7046385000 USD`，最后一条在 `2026-07-09 12:11:26.07198+08`
- `codex-auto-review`：`15` 条，`0.5598920000 USD`
- `gpt-5.5`：`4` 条，`0.0141040000 USD`

最近 20 条 usage 均为 `gpt-5.4`，都发生在 `2026-07-09 12:04:23+08` 到 `12:11:26+08`，没有今天记录。

## 昨日计费闭环

北京时间 `2026-07-09 00:00:00+08` 到 `2026-07-10 00:00:00+08`：

- usage rows：`131`
- total cost：`4.8950790000 USD`
- `131/131` 条为 `billing_type=1`
- `131/131` 条绑定 `subscription_id=2`
- `131/131` 条有 `usage_billing_dedup`
- 缺 dedup：`0`

说明这个用户昨天请求的计费链路是正常的。

## 额度为 0 的解释

订阅窗口字段：

- `daily_window_start=2026-07-10 00:00:00+08`
- `daily_usage_usd=0.0000000000`
- 从 `daily_window_start` 起重新汇总 usage logs 的 `daily_sum=0`
- `daily_diff=0`

所以当前每日额度为 0，是窗口正常重置加今天没有请求，不是 GPT-5.6 计费问题。

## 对“其他用户是否都出问题”的判断

不能用这个用户推导“其他所有用户计费有问题”，因为这个用户本身今天没有 GPT-5.6，也没有任何 API usage。

前一轮全局审计已经确认：

- 今天早上 GPT-5.6 317 条全部 `billing_type=1`
- 全部有 `subscription_id`
- 全部有 `usage_billing_dedup`
- 涉及 6 个订阅的 `daily_usage_usd` 与今日窗口 usage 总和一致

本用户不在那 6 个 GPT-5.6 早上使用者中。
