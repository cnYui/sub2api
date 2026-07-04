# 管理员 LOCAL API Key 延迟排查计划

## 背景

- 用户反馈：管理员使用包含 `LOCAL` 的 API Key，通过 Codex 发送消息后，需要等待很久才收到回答。
- 目标：判断延迟主要发生在 Sub2API、CLIProxyAPI、上游账号池，还是并发/排队/重试导致。
- 约束：不输出完整 API Key、内部 token、HMAC secret、SMTP 密码；只保留脱敏摘要。

## 排查原则

- 先确认当前公网入口实际指向，避免 18080/18084 数据源混淆。
- 以最近一次 Codex 请求为时间锚，关联 Sub2API `usage_logs`、应用日志、账号并发配置、CLIProxyAPI 日志。
- 不先改配置；只有定位到根因后再提出最小修复建议。

## 计划

1. 确认 nginx、Sub2API、Postgres、Redis、CLIProxyAPI 的当前运行态。
2. 在当前公网库中定位管理员 `xiaobianfuai@gmail.com` 和包含 `LOCAL` 的 active API Key，只记录 key id、名称、分组、并发等非敏感字段。
3. 查询最近请求的模型、耗时、计费、错误码、上游账号、分组和创建时间。
4. 对齐 Sub2API 日志和 CLIProxyAPI 日志，确认耗时发生在请求进入 Sub2API 前、Sub2API 排队、CLIProxyAPI 内部账号池，还是上游 OpenAI 返回慢。
5. 形成结论和后续动作建议。
