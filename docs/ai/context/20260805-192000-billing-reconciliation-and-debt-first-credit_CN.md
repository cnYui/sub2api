# 计费失败留档与欠费优先抵扣

## 当前结论

2026-08-05 停止公网入口、`sub2api-official-18082` 应用容器和 Cloudflare Tunnel，保留 PostgreSQL/Redis。数据库确认 `ops_system_logs` 中有 3,933 条 `record_usage_failed` 且错误为 `INSUFFICIENT_BALANCE`，涉及当前 16 个 `quota_exhausted` API Key、15 个用户和 5 个上游账号。

这些失败记录只有用户、API Key、账号、模型、时间和错误，没有 `request_id`、client request ID、输入/输出 token 或实际费用。上游 `/v1/usage` 仅提供账号聚合，不能把费用逐笔映射到本地用户和 Key。因此本次不按失败次数、时延、请求体大小或平均价硬扣余额。

## 代码变更

- 余额套餐首期到账和每周刷新都先用额度偿还 `users.balance` 的负值。
- 周额度不足时，余额继续为负，套餐 `remaining_usd` 为 0；额度超过欠费时，剩余部分才写入套餐窗口。
- 新增 `balance_debt_ledger`，记录欠费还款金额、余额前后值、来源和时间；退款、续费、窗口刷新不会删除历史流水。
- 新增 `billing_reconciliation_cases`，逐条复制历史计费失败日志，状态为 `pending_external_usage`，`amount_usd` 保持空值，等待后续上游逐笔明细导入。

## 恢复条件

迁移和新镜像发布后，将当前 16 个 `quota_exhausted` Key 在事务内改回 `active`，保留 `billing_reconciliation_cases` 和既有停用审计；认证缓存由数据库触发器写入失效 outbox，应用启动后核验 outbox 已消费。历史金额仍不视为已结算，后续拿到外部逐笔 token/费用后再按请求逐条入账。

