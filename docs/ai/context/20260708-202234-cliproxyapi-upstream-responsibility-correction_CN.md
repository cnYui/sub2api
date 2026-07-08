# CLIProxyAPI 上游问题职责边界修正

## 修正结论

- 用户指出正确：Sub2API 当前只负责公网 Key、订阅、计费、用量记录和转发入口。
- 模型选择、OAuth/API Key 上游账号池、账号轮询、quota cooldown、模型 provider 映射都属于 CLIProxyAPI。
- Sub2API 表里的 `accounts.id=1/cliproxy-local-openai` 不是一个真实 OpenAI/Codex 上游账号池成员，而是 Sub2API 到 CLIProxyAPI 的单个内网上游入口。
- 因此此前“给 `group_id=4/8` 在 Sub2API 增加更多上游账号”的建议不准确；正确方向是在 CLIProxyAPI 项目里查和修账号选择、模型映射、冷却策略和请求重试。

## 已确认链路

- Sub2API 公网链路：`Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> sub2api-candidate 127.0.0.1:18084`。
- Sub2API 上游入口：`host.docker.internal:8317`。
- CLIProxyAPI 当前本机进程：`/Users/wujianxiang/CodeSpace/CLIProxyAPI/cli-proxy-api`，监听 `*:8317`。
- CLIProxyAPI 日志目录：`/Users/wujianxiang/CodeSpace/CLIProxyAPI/logs/`。

## 新证据

- CLIProxyAPI 错误日志中存在 `usage_limit_reached`：
  - 文件：`logs/error-v1-responses-2026-07-08T175650-17a3c6d3.log`。
  - HTTP 状态：`429`。
  - 错误：`usage_limit_reached`。
  - `plan_type=k12`。
- CLIProxyAPI 错误日志中存在 `unknown provider for model gpt-4o-mini`：
  - 文件：`logs/error-v1-responses-2026-07-08T193452-e712bd12.log`。
  - HTTP 状态：`502`。
  - 错误：`unknown provider for model gpt-4o-mini`。
- CLIProxyAPI 配置当前关键项：
  - `request-retry: 1`
  - `max-retry-credentials: 5`
  - `quota-exceeded.switch-project: true`
  - `quota-exceeded.switch-preview-model: false`
  - `disable-cooling: false`
  - `streaming.bootstrap-retries: 1`

## 正确处理方向

1. `gpt-4o-mini` 失败应在 CLIProxyAPI 修模型映射或路由：
   - 现状是 CLIProxyAPI 对 `gpt-4o-mini` 返回 `unknown provider`。
   - 如果对外要支持该模型，需要在 CLIProxyAPI 配置/registry/路由中给它明确 provider。
   - 如果不支持，应从 Sub2API 对外模型列表或用户配置中移除，避免用户请求一个 CLIProxyAPI 不认识的模型。

2. `gpt-5.5` 的 `503/no available accounts` 要在 CLIProxyAPI 查账号选择状态：
   - Sub2API 只看到 CLIProxyAPI 返回 503/429/502。
   - CLIProxyAPI 内部虽然有多个账号，但仍可能因为模型支持范围、quota cooldown、账号状态、sticky session、请求重试上限、项目切换策略等原因，没有选到可用账号。
   - `usage_limit_reached plan_type=k12` 说明至少有一个被选中的账号确实触发了上游使用限制，并可能进入 cooldown。

3. `stream usage incomplete: missing terminal event` 仍是 Sub2API 侧流式转发/计费完整性问题：
   - 这类问题表现为上游流式连接已开始，但终止时没有完整 usage 事件。
   - 需要在 Sub2API 的 `/v1/chat/completions` 流式处理里单独设计：不应把它误判为订阅/Key/账号池问题。

## 建议下一步

- 在 CLIProxyAPI 项目里按 request_id/client_request_id 对齐 Sub2API 失败样本，查看 CLIProxyAPI 实际选中了哪个 provider/account、为什么没有切到其它账号。
- 优先修 `gpt-4o-mini` 模型映射，因为这是明确配置/路由问题。
- 再查 `gpt-5.5` 账号冷却和选择逻辑，重点看 `usage_limit_reached` 后是否把可用账号整体冷却、是否模型约束导致候选集为空、`max-retry-credentials=5` 是否不足以扫到可用账号。
- 不建议在 Sub2API 里增加更多 `accounts` 记录来解决这个问题，除非未来架构改成 Sub2API 直接管理多个 CLIProxyAPI 实例。

## 本轮影响

- 本轮只读查看 CLIProxyAPI 进程、配置和错误日志。
- 未修改 CLIProxyAPI 代码或配置。
- 未重启 CLIProxyAPI。
- 未修改 Sub2API 运行态 DB、容器、nginx 或 Redis。
