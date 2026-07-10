# CLIProxyAPI per-api-key 并发误伤修正记录

## 背景

用户反馈使用 API Key 两个并行请求时容易出现 429，怀疑 Sub2API 并发设置过小。

排查后确认：

- Sub2API 上游账号 `accounts.id=1 / cliproxy-local-openai` 当前 `concurrency=10`，不是 2。
- 近期 429 日志显示 `upstream_status=429`、`account_id=1`，说明 429 是 CLIProxyAPI 作为上游返回给 Sub2API，不是 Sub2API 本地用户槽位直接拒绝。
- 429 同一时间涉及多个 Sub2API 用户和多个 `api_key_id`，不是某个用户自己的 Key 单独超过 5。
- 当前 Sub2API 用户表 `users.concurrency` 默认是 5，这一层已经能实现“每个用户最多 5 并发”。

根因是 CLIProxyAPI 的 `per-api-key-concurrency: 5` 统计的是 CLIProxyAPI 看到的入站认证 Key。当前 Sub2API 转发到 CLIProxyAPI 使用同一个上游 Key，因此该值实际变成了“整个 Sub2API -> CLIProxyAPI 链路最多 5 并发”，而不是“每个 Sub2API 用户 Key 5 并发”。

同理，图片接口 `per-api-key-concurrency: 1` 在当前架构下也会变成“整个服务同时只能跑 1 个图片请求”，不是每个用户 1 个图片请求。

## 修改内容

已修改运行态配置：

`/Users/wujianxiang/CodeSpace/CLIProxyAPI/config.yaml`

修正后配置为：

```yaml
inbound-limits:
  enabled: true
  global-concurrency: 100
  per-api-key-concurrency: 100
  reject-status-code: 429
  reject-message: "too many concurrent requests"
  endpoint-overrides:
    - path-prefix: "/v1/images/generations"
      global-concurrency: 10
      per-api-key-concurrency: 10
    - path-prefix: "/v1/images/edits"
      global-concurrency: 10
      per-api-key-concurrency: 10
```

## 新语义

- 普通请求：CLIProxyAPI 全局最多 100 并发。
- 普通请求：CLIProxyAPI 入站单 Key 限制也设为 100，避免把 Sub2API 共用上游 Key 当成单个终端用户限流。
- 图片生成与图片编辑：全局最多 10 并发。
- 图片接口的单 Key 限制也设为 10，避免误伤整个 Sub2API 图片链路。
- 用户级普通请求并发仍由 Sub2API `users.concurrency=5` 控制。

## 验证

已执行：

```bash
ruby -e 'require "yaml"; YAML.load_file("config.yaml"); puts "yaml ok"'
git diff --check -- config.yaml
```

结果：

- YAML 解析通过。
- 空白检查通过。
- 当前 8317 进程仍是从 `/Users/wujianxiang/CodeSpace/CLIProxyAPI` 启动，启动参数为 `--config config.yaml`。

## 后续建议

- 如果要实现“每个 Sub2API API Key 最多 5 并发”，正确位置应在 Sub2API 增加 API Key 级并发槽位，而不是在 CLIProxyAPI 用 `per-api-key-concurrency` 控制。
- 另一种方案是 Sub2API 向 CLIProxyAPI 透传可信的 Sub2API `api_key_id`，并让 CLIProxyAPI 按该 header 作为 principal 计数；这需要代码改造，不是当前配置能完成的。
- 当前更合理的边界是：Sub2API 管用户/Key 级限制，CLIProxyAPI 只管整池全局保护和图片接口全局保护。
