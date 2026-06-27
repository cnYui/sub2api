# 公网 502 Bad Gateway 排查记录

## 背景

2026-06-24 22:30 左右，用户反馈别人通过公网使用时出现 Cloudflare 502 页面：

- 域名：`aaccx.pw`
- 本地客户端报错入口：`http://localhost:64483/v1/responses`
- Cloudflare Ray：`a10c142eaab8f904-SIN`
- 错误页标题：`aaccx.pw | 502: Bad gateway`

## 当前链路

公网链路仍是：

`Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> Sub2API 127.0.0.1:18080 -> CLIProxyAPI 127.0.0.1:8317`

排查时本机监听状态：

- `nginx` 正在监听 `*:8080`
- `sub2api` 容器健康，监听 `127.0.0.1:18080`
- `CLIProxyAPI` 正在监听 `*:8317`
- `cloudflared` 正在运行，`aaccx.pw`、`www.aaccx.pw`、`api.aaccx.pw` 都转发到 `http://127.0.0.1:8080`

## 复现与边界

2026-06-24 22:33~22:35 JST 手工验证：

- `https://aaccx.pw/` 返回 `200`
- `https://aaccx.pw/v1/models` 未带真实 Key 返回 `401`
- `https://aaccx.pw/v1/responses` 未带真实 Key 返回 `401`
- `https://aaccx.pw/responses` 未带真实 Key 返回 `401`
- `https://api.aaccx.pw/v1/responses` 未带真实 Key 返回 `401`
- `http://127.0.0.1:18080/v1/models` 未带真实 Key 返回 `401`
- `http://127.0.0.1:8317/v1/models` 未带真实 Key 返回 `401`

结论：排查当下不是 Cloudflare Tunnel、nginx、Sub2API 或 CLIProxyAPI 全局不可用，也不是 `/v1/responses` 路由整体失效。

## 关键日志

在 2026-06-24 21:30~21:31 +0800，即 22:30~22:31 JST，Sub2API 日志中有连续 `/v1/chat/completions` 流式请求返回 502：

- `path="/v1/chat/completions"`
- `model="gpt-5.5"`
- `stream=true`
- `status_code=502`
- `client_ip="54.254.171.157"`
- `account_id=1`
- 错误：`stream usage incomplete: missing terminal event`

同一时间窗口内，`/v1/responses` 与裸 `/responses` 也有成功记录：

- `/v1/responses` 在 21:30:35、21:31:32 +0800 返回 `200`
- `/responses` 在 21:27:33、21:31:52 +0800 返回 `200`

nginx access log 同一时段也有多条 `/v1/chat/completions` `502`，UA 为 `Go-http-client/2.0`；22:33 后 Codex Desktop / VS Code 的 `/v1/responses`、`/responses` 有 200 成功记录。

## 判断

这次 502 的直接原因不是用户本地 `localhost:64483` 服务，也不是 Cloudflare、nginx 或 Sub2API 主链路整体挂掉。

更准确的归因是：公网请求已经进入 Sub2API，鉴权和分组选择已通过，但 Sub2API 转发到上游账号池后，流式响应没有正常结束，缺少终止事件，Sub2API 将其判定为 `stream usage incomplete: missing terminal event` 并返回 502。

这类错误属于上游流式协议/连接提前结束问题，位于 `Sub2API -> CLIProxyAPI/上游账号` 之后；表现到客户端时可能被 Cloudflare 包成 502 Bad Gateway 页面。

## 备注

- 该时间段的 `/v1/responses` 并非全部失败，存在大量 200。
- `/v1/chat/completions` 的连续 502 与大 body、流式 `gpt-5.5` 请求更相关。
- 若要继续修复，优先方向不是改 nginx 路由，而是检查 Sub2API 对流式上游缺失终止事件时的容错、重试、账号切换和错误体映射。
