# 18084 候选环境承接公网链路记忆更新结果

## 结论

已将 2026-06-27 当前公网运行态写入 `AGENTS.md`。

## 记录内容

- 最高优先级主链路已更新为：

```text
Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> sub2api-candidate 127.0.0.1:18084 -> CLIProxyAPI 127.0.0.1:8317
```

- Docker 容器内访问 CLIProxyAPI 的入口为 `host.docker.internal:8317`。
- 公网应用容器为 `sub2api-candidate`，状态 healthy。
- 候选数据库为 `sub2api-candidate-postgres`，候选最新库统计为 `47 users / 40 keys / 191 migrations`。
- 旧 `sub2api` 18080 已 `Exited (0)`。
- `weishaw/sub2api:latest` 镜像暂留，`sub2api-postgres` 和 `sub2api-redis` 未停。
- Nginx 反代全部指向 `127.0.0.1:18084`。
- `/purchase`、`/health`、`/v1/responses`、`/v1/chat/completions` 均为 200。
- LLM 真实回复已证明 `sub2api -> CLIProxyAPI -> 上游 OpenAI` 全栈打通。

## 本次未做

- 未修改容器、镜像、Nginx、数据库、Redis 或 Cloudflare Tunnel。
- 未记录完整 API Key、内部 token、HMAC secret、SMTP 密码。
