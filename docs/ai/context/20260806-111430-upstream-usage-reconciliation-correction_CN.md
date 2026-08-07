# 上游 Usage 用户归属对账勘误

## 勘误结论

`20260806-104500-upstream-usage-unrecovered-charge-reconciliation_CN.md` 中以“相同模型且时间差在正负 5 秒内”贪心一对一分配上游账单的方法不具备用户级归属能力。

其中列出的 `xunskyler@gmail.com` `$384.62055975` 与总计 `$652.17178575` 均为时间近邻推测，不是已确认欠费，不得用于扣减余额、生成 usage 记录或标记对账完成。

## 证据

- `xunskyler@gmail.com` 的失败事件共有 `1,738` 条，其中 `1,671` 条被旧算法分配了金额；该算法仅比较模型和时间，候选时间覆盖 `2026-08-04 12:03` 至 `2026-08-05 16:50`。
- 失败事件只包含用户、API Key、上游账号、模型、失败时间和 `INSUFFICIENT_BALANCE`。`request_id`、客户端请求 ID、输入/输出/缓存 Token 和实际费用均为空或不存在。
- 生产 `gateway_handler_responses.go` 在上游成功响应后异步调用 `RecordUsage`；失败日志仅记录账号 ID 和错误，未把该次上游响应的唯一标识、Token 或费用写入可查询的对账记录。
- 上游 Excel 是共享上游账号的 Usage 明细，不能仅凭同模型和秒级时间戳确定其属于哪一个本地用户或 API Key。高并发时该弱关联会把其他用户的账单错误归属给目标用户。

## 当前处理边界

- 所有相关 `billing_reconciliation_cases` 保持 `pending_external_usage`，`amount_usd` 仍为空。
- 未执行余额扣减、usage_logs 写入、API Key 状态修改或任何补扣。
- 要恢复逐笔对账，必须取得可双端关联的唯一键：上游完整请求 ID 与本地请求 ID 的透传映射，或失败时持久化的上游响应 Token/费用快照。仅靠现有 Excel 和失败日志不能安全做到用户级补扣。
