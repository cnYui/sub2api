# CLIProxyAPI 入站并发限制配置记录

## 背景

用户确认当前 CLIProxyAPI 后面约有 20 个真实账号，希望在 CLIProxyAPI 入口增加一层应用级并发闸门：

- 单个 API Key 最多同时跑 5 个请求。
- 图片生成与图片编辑接口使用相同限制：全局最多 10 个并发，单个 API Key 最多 1 个并发。
- 整个 CLIProxyAPI 服务最多同时处理 100 个请求。

这层限制用于保护 CLIProxyAPI 后面的真实账号池，避免 Sub2API 放行后所有请求直接压到官方账号。

## 修改内容

已修改运行态配置文件：

`/Users/wujianxiang/CodeSpace/CLIProxyAPI/config.yaml`

新增配置：

```yaml
inbound-limits:
  enabled: true
  global-concurrency: 100
  per-api-key-concurrency: 5
  reject-status-code: 429
  reject-message: "too many concurrent requests"
  endpoint-overrides:
    - path-prefix: "/v1/images/generations"
      global-concurrency: 10
      per-api-key-concurrency: 1
    - path-prefix: "/v1/images/edits"
      global-concurrency: 10
      per-api-key-concurrency: 1
```

## 参数含义

- `enabled: true`：启用 CLIProxyAPI 入站并发限制。
- `global-concurrency: 100`：整个 CLIProxyAPI 服务最多同时处理 100 个已认证客户端 API 请求。
- `per-api-key-concurrency: 5`：同一个客户端 API Key 最多同时跑 5 个请求。
- `reject-status-code: 429`：超过限制时返回 HTTP 429。
- `reject-message: "too many concurrent requests"`：超过限制时返回的错误信息。
- `/v1/images/generations`：文字生图接口全局最多 10 个并发，单 Key 最多 1 个并发。
- `/v1/images/edits`：图生图接口全局最多 10 个并发，单 Key 最多 1 个并发。

## 生效情况

已确认当前 8317 监听进程：

- PID：`24104`
- cwd：`/Users/wujianxiang/CodeSpace/CLIProxyAPI`
- 启动参数：`--config config.yaml`

也就是说，本次修改的是当前进程正在使用的配置文件。

CLIProxyAPI 代码中存在 config watcher，配置热更新时会调用 `inboundLimiter.SetConfig(cfg.InboundLimits)`，因此该配置理论上保存后可热加载，不必须重启。若担心 watcher 未触发，可在低峰期重启 CLIProxyAPI；重启会中断正在进行的请求。

## 验证

已执行：

```bash
ruby -e 'require "yaml"; YAML.load_file("config.yaml"); puts "yaml ok"'
git diff --check -- config.yaml
```

结果：

- YAML 解析通过。
- 空白检查通过。
- `config.yaml` 看起来是运行态配置文件，未被 Git 跟踪，所以不会出现在 `git diff` 中。

## 注意事项

- `inbound-limits` 限制的是并发，不是 RPM。`global-concurrency: 100` 表示同时最多 100 个请求，不表示每分钟最多 100 个请求。
- 如果 Sub2API 里指向 CLIProxyAPI 的上游账号并发仍是 10，那么整个链路进入 CLIProxyAPI 的并发仍可能先被 Sub2API 卡在 10；要实际达到 CLIProxyAPI 的 100 全局并发，需要 Sub2API 对应上游账号/分组的并发入口也匹配放开。
- 单 Key 并发为 5，因此即使全局还有空余，同一个 API Key 第 6 个并发请求也会被 CLIProxyAPI 返回 429。
- 图片接口命中 endpoint override 后，同一个 API Key 同时只能跑 1 个图片请求。
