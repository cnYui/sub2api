# 18082 真实 Chat Completions 计费验证

## 请求

- 地址：`POST http://127.0.0.1:18082/v1/chat/completions`
- 用户：`2799523972@qq.com`（用户 ID 2）
- 模型：`gpt-5.6-terra`
- 请求内容：最小非流式请求，要求上游仅返回 `OK`，最大输出 token 为 8。
- API Key 未写入本文档、日志或验证结果。

## 结果

- HTTP 状态：`200`。
- 上游响应：`OK`。
- 响应 token：prompt 4391（其中 cached 3840）、completion 5、total 4396。
- 服务端用量记录：`usage_logs.id=285710`，API Key ID 210，分组 ID 9，账户 ID 1128。
- 入站端点：`/v1/chat/completions`；上游端点：`/v1/responses`。
- 计费记录：`total_cost=0.0019300000`，`actual_cost=0.0019300000`，`billing_mode=token`。
- 用户余额从 `157.920000` 变为 `157.918070`，差额与计费金额一致。

## 结论

18082 的真实转发、上游响应、用量落库和余额扣费均正常。该次请求未在 `billing_usage_entries` 生成独立行，当前计费事实以 `usage_logs` 与用户余额变动为准。
