# CLIProxyAPI max-retry-credentials 调整结果

## 调整内容

2026-07-04 已按用户要求将 CLIProxyAPI 运行态配置：

- 文件：`/Users/wujianxiang/CodeSpace/CLIProxyAPI/config.yaml`
- 字段：`max-retry-credentials`
- 调整前：`3`
- 调整后：`5`

选择 `5` 是保守值：增加 CLIProxyAPI 在内部账号池中遇到 429、不可用或临时失败凭据时的跨凭据重试机会，同时避免 `8` 在多个凭据都慢时过度拉长单次请求等待。

## 执行边界

- 未修改 Sub2API 源码。
- 未修改 Sub2API 数据库、Redis、nginx 或 Docker 容器。
- 未修改 CLIProxyAPI 账号池、凭据内容、模型路由、`request-retry` 或 `max-retry-interval`。
- 未记录完整 API Key、refresh token、账号凭据或其他敏感值。
- `CLIProxyAPI/config.yaml` 是本地 ignored 运行态配置，`git status --ignored config.yaml` 显示为 `!! config.yaml`，因此不会出现在 CLIProxyAPI git diff 中。

## 验证

配置文件片段已确认：

```yaml
request-retry: 1
max-retry-credentials: 5
max-retry-interval: 0
```

CLIProxyAPI 进程未重启：

```text
PID 96237
STARTED Mon Jun 22 14:36:14 2026
COMMAND /Users/wujianxiang/CodeSpace/CLIProxyAPI/cli-proxy-api
```

本地入口验证：

```text
http://127.0.0.1:8317/ -> 200, 0.001739s
http://127.0.0.1:8080/health -> 200, 0.004920s
```

代码层确认 CLIProxyAPI 对该字段有热加载路径：

- `internal/watcher/config_reload.go` 会把 `MaxRetryCredentials` 纳入 `retryConfigChanged`。
- `internal/api/server.go` reload 后调用 `AuthManager.SetRetryConfig(...)`。
- `sdk/cliproxy/auth/conductor.go` 中 `SetRetryConfig` 会写入 `maxRetryCredentials` 原子值。

未发现最近 10 分钟新生成的 CLIProxyAPI 日志文件；当前进程 stdout/stderr 未落到 `logs/cli-proxy-api.local.log`，因此本次无法从日志文件直接截取 reload 行。

## 后续观察点

如果管理员 LOCAL Key 仍然等待很久，下一步应看 CLIProxyAPI 请求级日志中是否仍连续命中 429 或慢账号；若 5 仍不足，再把 `max-retry-credentials` 升到 `8`，但需要同步观察单次请求最坏耗时是否变长。
