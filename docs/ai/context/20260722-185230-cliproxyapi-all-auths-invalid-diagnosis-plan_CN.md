# CLIProxyAPI 账号全失效只读诊断计划

时间：2026-07-22 18:52 +09

## 背景

用户反馈当前 CLIProxyAPI 账号突然全部失效。当前只做只读诊断，不修改 DB、Redis、容器、Nginx、Cloudflare、CPA auths。

## 目标

- 确认当前实际公网/本地链路与容器事实。
- 判断失败发生在 Sub2API 调度、Sub2API 到 CPA 网络/TLS、CPA 账号池、还是 OpenAI 上游凭证。
- 找到“突然全部失效”的最小证据链和下一步修复边界。

## 检查项

- Docker 容器、端口、健康状态。
- Nginx/Cloudflare 本地配置与公网 health。
- Sub2API 账号 1 配置、调度状态和最近模型请求错误。
- CLIProxyAPI 健康、日志、auth 文件/管理状态。
- 最近失败日志中的错误类别、HTTP 状态、冷却或凭证失效原因。

## 安全边界

- 不打印完整 API Key、内部 token、HMAC、证书私钥。
- 不删除、刷新、导入或改写 CPA auth。
- 不清 Redis 调度缓存，不重启生产或本地容器。
