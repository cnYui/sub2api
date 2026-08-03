# 18082 分组计费倍率更新

## 变更

已在 18082 PostgreSQL 中按分组 ID 更新 `groups.rate_multiplier`：

| 分组 | ID | 新倍率 |
| --- | ---: | ---: |
| Grok | 3 | 0.6 |
| Claude Max | 4 | 1.5 |
| Claude Kiro | 5 | 0.35 |
| GLM | 6 | 1.4 |
| Kimi | 7 | 3.5 |
| DeepSeek | 8 | 3.0 |
| GPT/Codex | 9 | 0.15 |

同时修正了 GLM、Kimi、DeepSeek 分组名称中遗留的旧倍率文字，避免界面名称与实际配置不一致。

## 验证

- 7 个分组更新事务全部提交成功。
- 仅重启 `sub2api-official-18082` 应用容器，PostgreSQL 和 Redis 未重启。
- `/health` 返回 `200`，容器状态为 `running/healthy`。
- 使用用户 2 的 API Key 调用免计费的 `/v1/sub2api/billing`，返回 `group_rate_multiplier=0.15`、`resolved_rate_multiplier=0.15`、`effective_rate_multiplier=0.15`。
- 未发送模型请求，未产生额外费用。
