# 18084 候选环境承接公网链路记忆更新计划

## 目标

将当前公网运行态写入 `AGENTS.md`，避免后续协作继续按旧 `18080` 正式容器判断链路。

## 已知状态

- 当前链路：`Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> sub2api-candidate 127.0.0.1:18084 -> CLIProxyAPI 127.0.0.1:8317`。
- Docker 内访问 CLIProxyAPI 使用 `host.docker.internal:8317`。
- 公网应用容器为 `sub2api-candidate`，状态 healthy。
- 候选数据库为 `sub2api-candidate-postgres`，候选最新库统计为 `47 users / 40 keys / 191 migrations`。
- 旧 `sub2api` 应用容器 `127.0.0.1:18080` 已退出，`weishaw/sub2api:latest` 镜像暂留，`sub2api-postgres` 和 `sub2api-redis` 未停。
- Nginx 反代全部指向 `127.0.0.1:18084`。
- `/purchase`、`/health`、`/v1/responses`、`/v1/chat/completions` 均已返回 200。
- LLM 真实回复已证明 `sub2api -> CLIProxyAPI -> 上游 OpenAI` 全栈打通。

## 修改范围

- 更新 `AGENTS.md` 的最高优先级主链路。
- 在 `AGENTS.md` 当前运行态提醒中新增 2026-06-27 记录。
- 完成后新增结果上下文文档。

## 不做

- 不修改运行中容器、镜像、Nginx、数据库、Redis 或 Cloudflare Tunnel。
- 不记录任何完整 API Key、内部 token、HMAC secret、SMTP 密码。
