# 原生 Gemini 与 Claude 渠道接入

## 目标

在生产 `18082` 新增两个用户可选的公开标准 API Key 分组，并把模型目录可达性接入用户侧 `/monitor`。

## 配置结果

| 分组 | 分组 ID | 平台 | 分组倍率 | 账号 ID | 上游地址 | 用户可选模型数 | 监控 ID |
| --- | ---: | --- | ---: | ---: | --- | ---: | ---: |
| Gemini1倍率 | 70 | `gemini` | `1.0000` | 1165 | `https://api.ai-genesis.app` | 12 | 13 |
| Claude0.78倍率 | 71 | `anthropic` | `0.7800` | 1166 | `https://huoshenai.net` | 17 | 14 |

- 两个分组均为 `active`、`standard`、非专属；用户 API Key 创建页会按既有逻辑自动将其列为可选分组，不需要新增前端静态配置。
- 两个账号均为 `apikey`、`active`、`schedulable=true`，并且只绑定各自的新分组，账号并发为 `50`。
- Gemini 按原生 Gemini 协议转发，Claude 按原生 Anthropic Messages 协议转发；没有把它们错误地当成 OpenAI 渠道。
- 账号 API Key 通过 `CredentialCodec` 写入，数据库只保留加密值和指纹；监控 API Key 通过既有 AES-GCM 加密器写入。本文不保存任何明文凭证。
- Gemini 仅公开当前本地定价文件已覆盖的 12 个模型。上游存在但本地暂无定价的目录项没有开放给用户，避免无价格计费。

## 监控

- 新增 `Gemini1倍率监控`（ID 13）与 `Claude0.78倍率监控`（ID 14）。
- 两条监控都是 `api_mode=models`、`enabled=true`、`interval_seconds=1800`，每轮只发起一次带鉴权的 `GET /v1/models`，不产生推理用量。
- 初次手动探测和应用重启后的首轮调度均成功：Gemini 12/12、Claude 17/17 模型均为 `operational`。
- 为使已运行的调度器装载新增记录，仅重启 `sub2api-official-18082` 应用容器；启动日志确认已加载 14 条监控。

## 验证

- 两个上游分别使用 Gemini `x-goog-api-key` 与 Anthropic `x-api-key` 请求模型目录，均返回 HTTP 200。
- 数据库回读确认分组、账号关联、倍率、状态与加密状态正确。
- `127.0.0.1:18082`、本地 Nginx `127.0.0.1:8080`、`aaccx.pw`、`www.aaccx.pw`、`api.aaccx.pw` 的 `/health` 均返回 HTTP 200。

## 未变更项

- 未修改现有分组、用户 API Key、模型价格、历史用量或余额。
- 未重建 PostgreSQL、Redis、Nginx 或 Cloudflare Tunnel。
