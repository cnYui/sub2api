# API Key 所有者与扣费归属审计

时间：2026-08-06 11:25（Asia/Tokyo）

## 结论

未发现 API Key 因显示名称相同而将费用扣到其他用户账户的情况。API Key 显示名称允许重复，但认证和扣费均不使用该字段。

生产库截至审计时的结果：

- `api_keys.key` 的唯一索引 `api_keys_key_key` 存在，真实凭证重复数为 `0`。
- 有 `22` 个显示名称被不同用户重复使用，涉及 `78` 把 Key；这只是 `api_keys.name` 的展示值，不参与认证、缓存键或账务归属。
- 东京时区当日（2026-08-06）`usage_logs` 共 `1293` 条，费用合计 `375.3203278950 USD`；`usage_logs.user_id != api_keys.user_id` 为 `0`，缺失 Key/用户外键引用为 `0`。
- 全历史 `269061` 条 `usage_logs`，上述所有者不一致和缺失引用均为 `0`。
- 对全部同时保存 `user_id` 与 `api_key_id` 的业务表进行了关联检查：`batch_image_jobs`、`billing_reconciliation_cases`、`billing_usage_entries`、`content_moderation_logs`、`deleted_api_key_audits`、`ops_error_logs`、`ops_ingress_reject_aggregates`、`ops_system_logs`、`prompt_audit_events`、`prompt_audit_jobs` 均为 `0` 条所有者不一致。

## 代码链路

1. `api_keys.key` 是全局唯一真实凭证；`name` 只是可重复显示名称。随机 Key 使用 32 字节安全随机数，自定义 Key 先查询存在性，数据库唯一索引仍作为并发最终约束。
2. 认证按真实 Key 精确查询，并加载该 Key 绑定的用户。认证缓存键是该真实 Key 的 SHA-256，不使用显示名称；缓存快照同时保留 API Key ID 与用户 ID。
3. 中间件把 `apiKey.User` 放入请求上下文。所有已检查的普通网关、OpenAI、Gemini、图片、Embeddings、Responses 和批量图片入口均把同一个已认证 `apiKey` 与其用户传入用量/扣费链路。
4. 用量日志、扣费幂等键、API Key 配额和余额扣款均使用 API Key 内部 ID 与用户内部 ID，不以名称匹配。`usage_logs` 对 API Key 和用户各有外键约束。

## 边界与建议

当前数据和正常入口不存在错扣证据，且“同名”本身不会造成冲突。

`usage_billingRepository.Apply` 接收的账务命令同时携带 `UserID` 和 `APIKeyID`，事务内尚未再次验证该 Key 的当前 `user_id`。现有所有生产调用方都传入 `apiKey.User`，所以并未触发问题；但这是可进一步收紧的防御性不变量。后续若重构账务入口或新增异步来源，应在同一事务中锁定 API Key 并验证 `api_keys.user_id = cmd.UserID`，不一致即拒绝，同时为该分支补回归测试。

本次为只读审计，未修改生产数据、配置或业务代码。
