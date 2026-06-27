# 413 Payload Too Large 修复结果

## 结论

已修复公网 Nginx 提前以 100MB 拦截 Sub2API 大请求的问题。

## 修改内容

- 已修改 `/opt/homebrew/etc/nginx/servers/aaccx-root.conf`：
  - `client_max_body_size 100m` 改为 `client_max_body_size 256m`。
  - 新增裸 OpenAI/Codex 兼容路径代理到 `127.0.0.1:18080`：
    - `/responses`
    - `/responses/*`
    - `/backend-api/codex/responses`
    - `/backend-api/codex/responses/*`
    - `/chat/completions`
    - `/embeddings`
    - `/images/generations`
    - `/images/edits`
- 已修改 `/opt/homebrew/etc/nginx/servers/cliproxy.conf`：
  - `client_max_body_size 100m` 改为 `client_max_body_size 256m`。
- 已平滑 reload Nginx。
- 已新增仓库回归测试 `backend/internal/server/http_test.go`，确认 `ProvideHTTPServer` 的全局请求体限制会挂到最终 `http.Server.Handler`。

## 备份

Nginx 修改前备份已移到未被 `include servers/*` 加载的目录：

- `/opt/homebrew/etc/nginx/backups/aaccx-root.conf.20260624-200948.bak`
- `/opt/homebrew/etc/nginx/backups/cliproxy.conf.20260624-200948.bak`

注意：不要把备份放在 `/opt/homebrew/etc/nginx/servers/` 下，因为当前主配置 `include servers/*` 会把 `.bak` 文件也当作 server block 加载。

## 验证

- `/opt/homebrew/bin/nginx -t -c /opt/homebrew/etc/nginx/nginx.conf`：通过。
- `/opt/homebrew/bin/nginx -s reload -c /opt/homebrew/etc/nginx/nginx.conf`：成功。
- 本机 Nginx 配置检查：
  - `aaccx-root.conf` 与 `cliproxy.conf` 均为 `client_max_body_size 256m`。
- 本机入口验证：
  - `POST http://127.0.0.1:8080/responses` + `Host: aaccx.pw` + invalid key 返回 Sub2API `401 INVALID_API_KEY`，不再落到 yui.web。
  - `POST http://127.0.0.1:8080/responses` + `Host: api.aaccx.pw` + invalid key 返回 Sub2API `401 INVALID_API_KEY`。
  - 声明 `Content-Length: 104857601` 的请求进入 Sub2API 后返回 `401 INVALID_API_KEY`，不再被 Nginx 100MB 限制拦截。
  - 声明 `Content-Length: 268435457` 的请求仍返回 `413 Request Entity Too Large`，说明 256MB 边界仍生效。
- 公网 HTTPS 小请求验证：
  - `POST https://aaccx.pw/responses` + invalid key 返回 Sub2API `401 INVALID_API_KEY`。
  - `POST https://api.aaccx.pw/v1/responses` + invalid key 返回 Sub2API `401 INVALID_API_KEY`。
- Go 回归测试：
  - `go test ./internal/server -run TestProvideHTTPServerAppliesGlobalRequestBodyLimit -count=1`：通过。

## 影响范围

- 未重启 Sub2API、CLIProxyAPI、Postgres、Redis 或 Cloudflare Tunnel。
- 请求体上限现在与 Sub2API 默认 `server.max_request_body_size` / `gateway.max_body_size` 的 256MB 对齐。
- 超过 256MB 的请求仍会被拒绝，不会无限放大入口内存风险。
- 剩余外部边界：公网仍经过 Cloudflare。Cloudflare 官方文档说明请求体上限依赖账号套餐，Free/Pro 为 100MB，Business 为 200MB，Enterprise 默认 500MB；如果真实上传 100MB 以上完整请求仍在 Cloudflare 层返回 413，需要调整 Cloudflare zone/套餐或改为 DNS-only/分片请求。参考：
  - `https://developers.cloudflare.com/workers/platform/limits/#request-limits`
  - `https://developers.cloudflare.com/support/troubleshooting/http-status-codes/4xx-client-error/error-413/`
