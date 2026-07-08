# 当前 Nginx 生效配置清单与不规范点核查

## 范围

- 主配置路径：`/opt/homebrew/etc/nginx/nginx.conf`
- 验证命令：`nginx -T`
- 结果：语法通过，配置测试成功。
- 生效 include：`include servers/*;`

## 当前目录文件

`/opt/homebrew/etc/nginx` 下文件：

- `nginx.conf`
- `nginx.conf.default`
- `mime.types`
- `mime.types.default`
- `fastcgi.conf`
- `fastcgi.conf.default`
- `fastcgi_params`
- `fastcgi_params.default`
- `scgi_params`
- `scgi_params.default`
- `uwsgi_params`
- `uwsgi_params.default`
- `koi-utf`
- `koi-win`
- `win-utf`
- `aaccx-root.conf.20260618-112506.bak`
- `servers/aaccx-root.conf`
- `servers/cliproxy.conf`
- `backups/aaccx-root.conf.20260624-200948.bak`
- `backups/cliproxy.conf.20260624-200948.bak`

## 实际生效配置

`nginx -T` 显示实际生效文件只有：

- `/opt/homebrew/etc/nginx/nginx.conf`
- `/opt/homebrew/etc/nginx/mime.types`
- `/opt/homebrew/etc/nginx/servers/aaccx-root.conf`
- `/opt/homebrew/etc/nginx/servers/cliproxy.conf`

未生效：

- `/opt/homebrew/etc/nginx/backups/*`
- `/opt/homebrew/etc/nginx/*.bak`
- `*.default`
- `fastcgi_*`、`scgi_params`、`uwsgi_params` 当前没有被任何 active server 引用。

## 主配置摘要

- `worker_processes 1`
- `worker_connections 1024`
- `include mime.types`
- `default_type application/octet-stream`
- `sendfile on`
- `keepalive_timeout 65`
- `gzip` 未开启
- 仍保留 Homebrew 默认 `server_name localhost`，监听 `8080`，返回默认 html。
- `include servers/*`

## `api.aaccx.pw` server 摘要

文件：`/opt/homebrew/etc/nginx/servers/cliproxy.conf`

- `listen 8080 default_server`
- `server_name api.aaccx.pw`
- `client_max_body_size 256m`
- 所有路径最终代理到 `http://127.0.0.1:18084`
- `/assets/pkg-`、`/assets/app-index-`、`/assets/` 代理到 Sub2API，并通过 `sub_filter` 把 `vendor-` 改成 `pkg-`、`index-` 改成 `app-index-`
- `/`、`/index.html` 代理到 Sub2API，附带 `Clear-Site-Data: "cache"`
- 通用 `location /` 也代理到 Sub2API，支持控制台 SPA 与 `/v1/*` 等 API。

## `aaccx.pw` server 摘要

文件：`/opt/homebrew/etc/nginx/servers/aaccx-root.conf`

- `listen 8080`
- `server_name aaccx.pw www.aaccx.pw`
- `client_max_body_size 256m`
- `root /Users/wujianxiang/CodeSpace/yui.web`
- `/v1/`、`/api/` 代理到 `http://127.0.0.1:18084`
- 裸 OpenAI/Codex 兼容路径代理到 `18084`：
  - `/responses`
  - `/responses/*`
  - `/backend-api/codex/responses`
  - `/backend-api/codex/responses/*`
  - `/chat/completions`
  - `/embeddings`
  - `/images/generations`
  - `/images/edits`
- Sub2API 控制台页面路径代理到 `18084`
- `/shop/(login|register|...)` 旧路由返回 `410`
- 其它扩展名页面做 `.html` 去后缀 301
- 其它 yui.web 页面代理到 `127.0.0.1:4173`，并通过 `@html_fallback` 做 `.html` fallback
- `/SKILL.md` 公开暴露为纯文本。

## 不规范或建议收敛点

1. 主配置仍保留 Homebrew 默认 `localhost` server。
   - 现象：`Host: localhost` 命中默认 HTML，而不是 Sub2API health。
   - 风险：本地排查或自动 health check 如果用 `localhost:8080` 会看到错误目标。
   - 建议：生产配置中删除默认示例 server，或让 `localhost` 明确返回 `444/404` 或代理到当前默认服务。

2. 多处无条件设置 `proxy_set_header Connection "upgrade"`。
   - 现象：普通 HTTP 请求也会带 `Connection: upgrade` 给上游。
   - 风险：一般不致命，但不是标准写法；更推荐用 `map $http_upgrade $connection_upgrade` 按需设置。
   - 涉及：`aaccx-root.conf` 裸兼容路径、控制台路径、yui.web fallback；`cliproxy.conf` 根路径和通用路径。

3. `proxy_read_timeout 86400` 覆盖面过宽。
   - 原因：对 Codex/SSE/长请求合理。
   - 问题：静态资源、普通页面、yui.web 页面也套用 24 小时 timeout，不够精细。
   - 建议：只在 `/v1/`、裸 `/responses`、图片生成/编辑、WebSocket/SSE 路径保留超长 timeout。

4. `proxy_request_buffering off` 覆盖面过宽。
   - 原因：大请求和流式请求需要。
   - 问题：静态资源/普通页面没必要，会降低 Nginx 作为缓冲层的价值。
   - 建议：只在模型请求、上传接口、SSE/WebSocket 路径关闭 buffering。

5. `sub_filter + Accept-Encoding "" + no-store` 是历史兼容 hack。
   - 原因：绕过旧 `vendor-*` 静态资源和 `index-*` chunk 缓存问题。
   - 问题：禁用上游压缩、动态改 JS/CSS/HTML、静态资源不缓存，性能和可维护性都差。
   - 建议：如果当前前端构建产物已稳定，应逐步移除这些 rewrite/filter，并恢复正常静态资源缓存策略。

6. `Clear-Site-Data: "cache"` 仍挂在 `api.aaccx.pw` 根页面和 `/index.html`。
   - 原因：曾用于清理旧白屏缓存。
   - 问题：每次访问都清缓存，体验和性能不好。
   - 建议：作为一次性修复保留窗口即可，后续删除或改成临时 query 参数触发。

7. `aaccx.pw /assets/*` 全部代理到 Sub2API。
   - 原因：保护 Sub2API 控制台资源路径。
   - 问题：如果 yui.web 将来也使用根级 `/assets/*`，会被 Sub2API 抢走。
   - 建议：长期可用更明确的前缀或域名隔离静态资源。

8. `/SKILL.md` 公开暴露。
   - 当前看不像敏感配置，但属于非业务文件暴露。
   - 建议：确认这是有意公开；若无必要，删除该 location。

9. 裸 `/responses` 兼容路径不属于标准 OpenAI URL。
   - 当前不是 HTTP redirect，而是内部 proxy/后端 alias，兼容性比 301/302 更稳。
   - 建议：保留兼容但不宣传；对外继续推荐 `https://api.aaccx.pw/v1`。

## 验证

- `nginx -T`：通过。
- `Host: api.aaccx.pw http://127.0.0.1:8080/health`：`200 application/json`
- `Host: localhost http://127.0.0.1:8080/`：`200 text/html`，说明默认示例 server 仍可命中。
- `Host: 127.0.0.1 http://127.0.0.1:8080/`：`200 text/html; charset=utf-8`，命中 `api.aaccx.pw` default_server 下的 Sub2API 前端。
