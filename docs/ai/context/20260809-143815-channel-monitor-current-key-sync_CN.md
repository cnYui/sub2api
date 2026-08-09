# 监控上游凭证同步与目录探测核验

## 背景

`/monitor` 已统一使用低成本的 `GET /v1/models` 目录探测。旧监控 `1` 至 `9` 仍保存独立的加密 API Key 快照，而上游账号密钥已经轮换，导致最新记录均为上游 `401 INVALID_API_KEY`，用户侧显示红色。

## 执行

- 使用隔离的一次性维护程序。
- 维护命令通过 `AccountRepository` 解密当前账号凭证，只在内存中把 `credentials.api_key` 交给 `ChannelMonitorService.Update`。
- `ChannelMonitorService` 使用既有 `SecretEncryptor` 写入新的监控密文；没有直接复制数据库中的密文、没有输出明文 Key。
- 保留原有端点、模型、启用状态、`models` 模式和 `1800` 秒间隔。
- 同步后对每条监控立即调用一次 `RunCheck`，实际请求仅为认证 `GET /v1/models`，没有执行 `chat_completions`、`responses` 或其他推理请求。

## 固定映射与结果

| 监控 ID | 账号 ID | 最新目录探测状态 | 延迟（ms） |
| --- | --- | --- | ---: |
| 1 | 1 | operational | 872 |
| 2 | 2 | operational | 307 |
| 3 | 3 | operational | 295 |
| 4 | 4 | operational | 332 |
| 5 | 5 | operational | 313 |
| 6 | 6 | operational | 305 |
| 7 | 1128 | operational | 266 |
| 8 | 1129 | operational | 272 |
| 9 | 1130 | operational | 321 |

生产数据库回读确认上述 9 条监控均为 `enabled=true`、`api_mode=models`、`interval_seconds=1800`，监控 API Key 字段保存为加密文本。

## 生产核验

- `sub2api-official-18082` 保持 healthy，未重建应用、PostgreSQL、Redis、Nginx 或 Cloudflare Tunnel。
- `127.0.0.1:18082/health`、`127.0.0.1:8080/health`、`https://aaccx.pw/health` 均返回 HTTP 200。
