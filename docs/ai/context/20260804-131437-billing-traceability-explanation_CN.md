# 用户请求与上游费用溯源边界

## 结论

当前系统可以把一次本地请求关联到：用户、API Key、上游账号、请求 ID、请求模型、实际发送的上游模型、输入/输出/缓存 token、分组和本地费用快照。

当前系统不能把每一笔本地请求自动一一对应到上游网站 `/usage` 页面中的具体费用行，因为本地没有保存上游网站的逐笔账单 ID，也没有在每次请求后读取上游网站账单；本地只接收上游响应中的 usage/token，并按本地价格表重算费用。

## 上游倍率探测

`UpstreamBillingProbeService` 按周期使用上游账号的 API Key 请求 `base_url/v1/sub2api/billing`。该接口返回的是 Key 级别的 `resolved_rate_multiplier`、峰值倍率和有效倍率，不是某一次请求的美元金额。结果保存到 `accounts.extra.upstream_billing_probe`，当前 Kimi 账号快照为 `resolved_rate_multiplier=3.5`，GPT 账号快照为 `0.15`。

## 普通 token 请求计费

1. 用户请求经 API Key 认证并选定分组/上游账号。
2. 上游响应返回 usage；本地拆分普通输入、输出、缓存读取/写入 token。
3. 本地价格表按模型和 token 计算 `total_cost`，这是标准基础成本，不是上游网站已经扣好的最终金额。
4. 解析用户/分组倍率，并计算 `actual_cost = total_cost × rate_multiplier × BILLING_FINAL_MULTIPLIER`。
5. `actual_cost` 写入 `usage_logs`，并作为 `BalanceCost` 在数据库事务中扣用户余额。

账号的 `accounts.rate_multiplier` 是账号统计/配额口径，单独使用 `total_cost × account_rate_multiplier`，不参与普通用户余额的 `actual_cost`。

## 现场记录

- Kimi `usage_logs.id=287142`：`0.0536268 × 3.5 × 15 = 2.815407`。
- GPT `usage_logs.id=286781`：`0.001928 × 0.15 × 15 = 0.004338`。

这两笔记录均可追溯到本地请求链路，但不能仅凭本地记录证明上游网页中的逐笔金额；若要做到逐笔对账，需要保存上游响应/账单 ID，或让上游提供按请求 ID 查询账单的接口。
