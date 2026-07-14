# xunskyler@gmail.com 套餐到期后 API Key 请求能力只读核验

## 结论

- 当前用户仍然可以通过 `api_keys.id=137/name=5.6` 请求模型并获得 `HTTP 200` 响应。
- 这不是套餐仍有效：唯一订阅 `user_subscriptions.id=21` 已为 `expired`，账户余额为 `0`。
- 用户只剩一张尚未归零的 OpenAI 流量卡，余额仅 `0.0011115500 USD`。
- 当前准入逻辑只判断是否存在 `remaining_usd > 0` 的流量卡，没有判断本次请求的预计费用是否能被覆盖，因此这点残余额度仍会放行请求。
- 模型响应写回客户端后，`RecordUsage` 在异步任务中执行；实际费用超过剩余流量卡额度时，记账返回 `INSUFFICIENT_BALANCE`，但已经完成的 HTTP 200 响应不会被撤回。

## 运行态证据

- 北京时间 `2026-07-14 19:13:07+08`、`19:13:25+08`、`19:13:52+08` 等多次 `POST /v1/responses`：
  - 用户 `user_id=19`
  - Key `api_key_id=137`
  - 模型 `gpt-5.6-sol`
  - 网关最终返回 `HTTP 200`
  - 随后日志记录 `openai.record_usage_failed`，错误为 `INSUFFICIENT_BALANCE`
- 最近 1 小时日志快照中，该用户约有 `192` 次模型请求返回 HTTP 200，同时有 `193` 次异步 usage 记账失败。
- API Key 最后活动时间已推进到北京时间 `2026-07-14 19:14:22+08`，但 `usage_logs` 最后一条成功落库仍停在 `2026-07-14 14:23:09+08`，证明后续请求完成但没有成功记账。

## 根因

- `BillingCacheService.CheckBillingEligibility()` 在余额不足时调用 `canUseTrafficPackCredit()`。
- `trafficPackRepository.HasAvailableCredit()` 只要求存在任意 `remaining_usd > 0` 且未过期的流量卡。
- OpenAI 网关先完成转发和客户端响应，再通过 worker 异步调用 `RecordUsage()`；记账失败只写日志，不改变已经返回的 HTTP 响应。
- 这是 2026-07-13 全链路审计已识别的 P0 问题在具体用户上的直接复现：成功转发后余额不足会导致账务整体回滚，且缺少不可变 usage fact。

## 本次操作

- 只执行 PostgreSQL、容器日志和源码只读查询。
- 未发起额外模型请求，未修改数据库、Redis、容器、Nginx、用户、套餐、API Key 或额度。
- 未停用 Key；如需立即阻止继续使用，需要另行授权执行运行态变更。
