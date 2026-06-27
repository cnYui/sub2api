# CLIProxyAPI 账号 502 排查计划

## 背景

用户截图显示 `codex-xiaobianfuai@gmail.com-plus.json` 的 Codex 额度仍有剩余：5 小时限制约 94%，周限额约 99%，但当前经 Sub2API/CLIProxyAPI 反代请求返回 502。当前线上链路预期为：

`Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> Sub2API 127.0.0.1:18080 -> CLIProxyAPI 127.0.0.1:8317`

## 必须先确认的问题

- 502 是 Sub2API 返回给客户端的上游错误，还是 CLIProxyAPI 自身返回的错误。
- 失败是否只发生在截图中的 Codex 账号，还是 CLIProxyAPI 的整个 Codex 池都不可用。
- 截图中的“额度充足”是否等价于当前访问路径可用；OAuth token、账号状态、模型/协议、地区风控、WebSocket/Responses 链路都可能独立失败。
- Sub2API 传给 CLIProxyAPI 的模型、base_url、认证方式和 CLIProxyAPI 真实可用模型是否一致。

## 排查步骤

1. 只读确认 Sub2API、nginx、CLIProxyAPI 的监听端口和运行进程。
2. 查看 Sub2API 上游账号配置，确认指向 `127.0.0.1:8317` 且启用账号池模式。
3. 查看 CLIProxyAPI 配置、账号文件和最近日志，定位和该账号、502、401、403、429、WebSocket、Codex 相关的错误。
4. 用最小本地请求分别命中：
   - CLIProxyAPI `/v1/models`
   - CLIProxyAPI `/v1/chat/completions` 或 `/v1/responses`
   - Sub2API 本地 `/v1/...`
5. 对比成功账号和失败账号的加载状态、provider/type、plan_type、token 过期/刷新状态。
6. 只在根因明确后再决定是否需要刷新额度、重置 OAuth、禁用坏账号或改配置。

## 不做的事

- 不直接重启公网服务。
- 不泄露完整 API Key、OAuth token、HMAC secret。
- 不修改数据库、账号文件或配置，除非用户确认并且已经定位根因。
