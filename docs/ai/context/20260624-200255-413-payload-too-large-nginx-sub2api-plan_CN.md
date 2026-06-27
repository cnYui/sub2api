# 413 Payload Too Large 排查与修复计划

## 目标

让公网入口的请求体上限不再低于 Sub2API 网关上限，避免 100MB 到 256MB 之间的合法大上下文请求被 Nginx 提前拦截；同时修正仓库内全局请求体限制挂载顺序问题。

## 当前证据

- Cloudflare Tunnel 三个域名 `aaccx.pw`、`www.aaccx.pw`、`api.aaccx.pw` 都进入 `http://127.0.0.1:8080`。
- `/opt/homebrew/etc/nginx/servers/aaccx-root.conf` 和 `/opt/homebrew/etc/nginx/servers/cliproxy.conf` 均配置 `client_max_body_size 100m`。
- Sub2API 配置默认值是：
  - `server.max_request_body_size = 268435456`
  - `gateway.max_body_size = 268435456`
- Sub2API `/v1`、裸 `/responses`、`/chat/completions`、`/embeddings` 等网关路由都挂了 `RequestBodyLimit(cfg.Gateway.MaxBodySize)`。
- Nginx access log 中最近仍有大量 `POST /responses HTTP/1.1` 返回 `413 56`，User-Agent 为 Windows Codex Desktop。
- 仓库内 `ProvideHTTPServer` 当前先把 `httpHandler` 填进 `http.Server.Handler`，再包装 `http.MaxBytesHandler`，导致全局限制可能只打印日志但未实际生效；这不影响 `/v1` 网关路由级限制，但会让全局保护语义不可靠。

## 根因判断

直接根因是公网 Nginx 的 `client_max_body_size 100m` 小于 Sub2API 的 256MB 网关限制，100MB 到 256MB 的请求在到达 Sub2API 前就被 Nginx 拦截，因此不会进入 `usage_logs` 或 `ops_error_logs`。

并行存在一个路径兼容问题：`api.aaccx.pw` 的 `/responses` 会因通用 `location /` 转给 Sub2API；`aaccx.pw` 的裸 `/responses` 仍会落到 yui.web 分流。大请求在分流前先被 100MB 限制拦掉，所以日志里会表现为 `/responses` 413。若希望 `aaccx.pw` 也能作为无 `/v1` Base URL，需要另加裸 OpenAI 兼容路径代理规则。

## 修复计划

1. 仓库内小修：给 `ProvideHTTPServer` 增加测试，先证明 `server.max_request_body_size` 应实际挂到最终 `http.Server.Handler`。
2. 修复 `backend/internal/server/http.go`：先完成 `http.MaxBytesHandler` 包装，再创建或回写 `server.Handler`。
3. 运行 Go 单测，至少覆盖新增的 `backend/internal/server` 测试。
4. 运行态 Nginx 调整：
   - 将 `aaccx-root.conf` 与 `cliproxy.conf` 的 `client_max_body_size 100m` 改为 `256m`。
   - 若要兼容 `https://aaccx.pw` 作为 Codex Base URL，同步给 `aaccx-root.conf` 添加裸路径代理：`/responses`、`/responses/`、`/backend-api/codex/`、`/chat/completions`、`/embeddings`、`/images/generations`、`/images/edits`。
5. 对 Nginx 执行 `nginx -t -c /opt/homebrew/etc/nginx/nginx.conf`，通过后 `nginx -s reload -c /opt/homebrew/etc/nginx/nginx.conf`。
6. 验证：
   - 配置文件中两个 server 都是 `client_max_body_size 256m`。
   - `api.aaccx.pw` 与 `aaccx.pw` 的相关网关路径不再被 100MB 配置提前拦截。
   - 小请求仍能进入 Sub2API 并返回认证错误或正常业务响应，不落到 yui.web。

## 风险与取舍

- `256m` 与 Sub2API 上限保持一致，避免扩大公网入口暴露面；超过 256MB 的请求仍应被拒绝。
- 修改 Nginx 属于运行态变更，不需要重启 Sub2API、Postgres、Redis、CLIProxyAPI 或 Cloudflare Tunnel。
- 仅提高 Nginx 上限不能解决 `aaccx.pw/responses` 小请求路由到 yui.web 的问题；要兼容无 `/v1` Base URL，必须补裸路径代理。
