# OpenAI 透传分支流量卡预授权缺失修复计划

## 背景

- 排查 `luzhiyuan2026@163.com` 发现：用户套餐已被撤销，但 API Key 仍 active，且请求命中 `OpenAI 自动透传` 分支。
- 普通 OpenAI `/v1/responses` 分支会在请求上游前执行 `authorizeOpenAIForward`，按套餐额度或流量卡做请求前预授权。
- `forwardOpenAIPassthrough` 分支目前没有执行同一套预授权，成功响应后才退回旧的流量卡扣费判断，因此流量卡不足时会出现“先返回成功，再记 debt”的情况。

## 必须修复的问题

- OpenAI 自动透传分支必须在上游请求前进行计费预授权。
- 透传分支必须在请求真正发给上游前把 reservation 标记为 dispatched。
- 构建 `OpenAIForwardResult` 时必须携带 `BillingAuthorization`，让后续 usage fact 使用固定的 subscription entitlement 或 traffic credit reservation。
- 上游请求构造、取 token、发送前失败时必须释放未 dispatch 的 reservation；发送后异常必须标记 unknown，避免假释放已经可能产生成功成本的请求。

## 验证点

- 透传分支成功响应时，`OpenAIForwardResult.BillingAuthorization` 不为空。
- 流量卡不足或已有欠费时，透传分支在请求上游前返回 402，不再产生成功 usage。
- 保持普通分支原有行为不变。

## 非目标

- 不针对单个用户写特判。
- 不直接修改线上或本地运行态数据库。
- 不在文档、日志或测试中记录完整密钥。
