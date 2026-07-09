# Cloudflare 502 origin_bad_gateway 诊断

> 2026-07-09 10:18 JST，只读排查；未改代码、DB、nginx、Cloudflare Tunnel 或容器。

## 用户现象

客户端报错：

- `API call failed after 3 retries`
- `HTTP 502`
- Cloudflare Problem JSON：`error_name=origin_bad_gateway`、`error_category=origin`
- Ray ID 示例：`a17f6164dfcb553a`、`a17f6443c81565fc`

该响应体是 Cloudflare/边缘层错误格式，不是 Sub2API 应用层 OpenAI 兼容错误格式。

## 当前链路

公网链路仍是：

`Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> sub2api-candidate 127.0.0.1:18084 -> CLIProxyAPI 127.0.0.1:8317`

当前健康检查：

- `sub2api-candidate`、Postgres、Redis 均 healthy
- `http://127.0.0.1:18084/health` 返回 200
- `https://api.aaccx.pw/health` 返回 200

## 关键证据

1. nginx error.log 在 `2026-07-09 08:17-08:36 JST` 多次记录：

   `connect() failed (61: Connection refused) while connecting to upstream`

   upstream 是 `http://127.0.0.1:18084/...`，说明当时 nginx 能收到 Cloudflare Tunnel 转来的请求，但无法连接 Sub2API 应用容器。

2. nginx access.log 同一时间段大量返回 `502`，包括：

   - `/api/v1/auth/me`
   - `/api/v1/subscriptions/active`
   - `/v1/models`
   - `/v1/chat/completions`

   其中 `/v1/chat/completions` 有 `OpenAI/Python 2.24.0` 请求在 `08:31:20`、`08:31:24`、`08:31:32` 等时间返回 502，和客户端“重试 3 次后失败”的形态吻合。

3. `docs/ai/context/20260709-084617-public-18084-new-subscription-plans-redeploy-result_CN.md` 记录今天上午发布前有 Docker/磁盘/Postgres 启动问题；这段时间 18084 应用容器不可用或未完全恢复，导致源站连接被拒。

4. `08:36:39 JST` 后 nginx 开始对控制台 API 返回 200，`08:36:44` `/health` 返回 200，说明源站后来恢复。

## 结论

这次用户贴出的 Cloudflare `origin_bad_gateway` 不是 API Key、订阅、额度或模型协议问题；根因是 Cloudflare Tunnel 后面的源站链路短时不可用，具体是 nginx 反代 `127.0.0.1:18084` 时连接 Sub2API 应用容器被拒绝。

最可能发生窗口是 `2026-07-09 08:13-08:36 JST` 左右，背景是公网 18084 应用容器发布/恢复过程。

## 容易混淆的另一类 502

源站恢复后，应用层仍出现过其它 502，但这些不是 Cloudflare `origin_bad_gateway`：

- `08:37-08:55 JST`：`group_id=5 / codex-pool-local-unlimited` 的请求出现 `no available accounts` 或 `invalid base_url: invalid url scheme: http`
- `10:02-10:03 JST`：`user_id=69 / api_key_id=99 / group_id=8` 的请求上游返回 `thinking_signature_invalid`，错误为 `The encrypted content ... could not be decrypted or parsed`
- 这类请求已经到达 Sub2API，属于 Sub2API 转上游或 CLIProxyAPI/上游模型返回问题，响应体应是应用层错误，不是 Cloudflare origin 错误

## 建议

- 对用户这次截图：解释为源站短时不可用，重试即可；当前健康检查已恢复。
- 后续发布要降低 18084 停机窗口：先启动新容器并健康检查，再切流量，或至少在停止旧容器前确保新镜像、数据目录和 Postgres 认证问题已全部验证。
- 对应用层 502 另开问题处理，不要和 Cloudflare `origin_bad_gateway` 合并判断。
