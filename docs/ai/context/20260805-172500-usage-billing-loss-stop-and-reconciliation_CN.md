# 用量后扣失败止损与历史对账

## 现象

生产 `18082` 的 OpenAI 兼容请求在上游已成功返回后异步执行 `RecordUsage`。当普通余额不足且 OpenAI 流量卡不能完整覆盖该笔真实费用时，原实现回滚结算事务并只写 `record_usage_failed` 运维日志。客户端已收到 `200`，但不会产生 `usage_logs` 或余额扣减。

截至 `2026-08-05 17:21 +08`，`ops_system_logs` 中可确认 `3,887` 次 `INSUFFICIENT_BALANCE` 后扣失败，涉及 `14` 个用户和 `16` 个 API Key。每条失败记录可关联成功 HTTP 响应的本地请求 ID，但历史日志没有保存上游返回的 token usage 或精确费用。

## 已执行止损

- 未停止公网 API、Nginx、Cloudflare Tunnel 或上游账户。
- 将上述 `16` 个 API Key 设为 `quota_exhausted`，通过现有数据库触发器写入认证缓存失效 outbox；其它用户和 API Key 不受影响。
- 另写入一条 `audit_logs` 系统审计 `billing_safety_suspend_api_keys`，记录原因和受影响 Key ID。

## 结算修复

OpenAI 流量卡不足以覆盖已完成请求时，统一计费事务不再返回 `INSUFFICIENT_BALANCE` 并丢弃该笔使用，而是按现有普通余额透支规则写入余额欠款。这样请求的真实 token、`usage_logs` 与 `billing_usage_entries` 都会保留；余额缓存随欠款失效，下一次请求会在鉴权阶段拒绝。

这不替代请求前预授权：文本和流式请求的最终输出 token 在转发完成前不可精确得知。当前修复保证这种边界请求不会再无账单；零余额且无有效流量卡的请求仍由鉴权和 handler 双重预检在上游转发前拒绝。

## 历史补账边界

不能依据失败次数、响应时长或模型平均价对 `3,887` 笔历史请求估价扣款。上游 `GET /v1/usage` 只能返回账号维度聚合，无法映射到本地用户/API Key；本地遗漏 token 的历史响应也没有其它持久副本。必须取得上游 `/usage` 的逐笔导出或等价的 token 明细后，按本地请求时间、账号、模型和请求 ID 一一匹配，再写入可审计的用量和余额补账。
