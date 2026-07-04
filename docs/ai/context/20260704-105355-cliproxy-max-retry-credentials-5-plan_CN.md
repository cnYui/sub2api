# CLIProxyAPI max-retry-credentials 调整计划

## 背景

2026-07-04 管理员使用包含 LOCAL 标识的本机 Codex API Key 发送消息后等待较久。前置排查结论是：Sub2API 侧并发不是主要瓶颈，慢点集中在 `Sub2API -> CLIProxyAPI -> OpenAI/Codex 内部账号池`，其中部分凭据返回 429 或不可用，部分凭据可成功但流式响应很慢。

用户要求将 CLIProxyAPI `max-retry-credentials` 从 `3` 提到 `5-8`。

## 可选方案

1. 调整为 `5`：保守增加凭据 failover 次数，降低连续命中坏凭据的概率，同时控制单次请求最坏等待时间。
2. 调整为 `8`：最大化凭据尝试范围，但在多个凭据都慢或失败时，单次请求可能被拖得更久。
3. 先不改 CLIProxyAPI，只继续扩 Sub2API 并发：不匹配当前诊断，因为慢点不在 Sub2API 并发队列。

## 执行选择

采用方案 1，将 `/Users/wujianxiang/CodeSpace/CLIProxyAPI/config.yaml` 的 `max-retry-credentials` 从 `3` 调整为 `5`。

## 边界

- 不修改 Sub2API 源码、数据库、nginx 或 Docker 容器。
- 不修改 CLIProxyAPI 账号池、凭据内容、模型路由或 `request-retry`。
- 不记录完整 API Key、refresh token、账号凭据或其他敏感值。
- 优先依赖 CLIProxyAPI 配置热加载；只有热加载无法生效时才考虑重启。

## 验证

- 确认配置文件已变为 `max-retry-credentials: 5`。
- 检查 CLIProxyAPI 本地入口 `127.0.0.1:8317` 仍可访问。
- 查看 CLIProxyAPI 运行日志中是否出现配置变化或 retry config 更新迹象。
