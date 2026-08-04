# 18082 分组倍率重新应用记录

## 背景

恢复昨晚版本的数据后，7 个可用模型分组的 `groups.rate_multiplier` 被还原为 `1.0000`，导致用户 API Key 页面和可用渠道页面全部显示 `1x`。历史配置要求按模型渠道使用不同的分组倍率。

## 变更

在 `sub2api-official-18082-postgres` 中以事务更新分组 ID 3-9：

| 分组 | ID | 分组展示/基础计费倍率 |
| --- | ---: | ---: |
| Grok | 3 | 0.6x |
| Claude Max | 4 | 1.5x |
| Claude Kiro | 5 | 0.35x |
| GLM | 6 | 1.4x |
| Kimi | 7 | 3.5x |
| DeepSeek | 8 | 3.0x |
| GPT/Codex | 9 | 0.15x |

按后续历史记录保留 GLM、Kimi、DeepSeek 的现有分组名称文字，不修改名称字段。

## 15x 口径

`BILLING_FINAL_MULTIPLIER=15` 仍由 `deploy/docker-compose.18082.yml` 注入，仅用于服务端实际模型扣费。它不叠加到前端分组倍率、模型价格或套餐展示中；实际扣费按分组倍率与最终倍率分别进入服务端计费链路。

## 验证

- 7 条分组记录回读为 `0.6000`、`1.5000`、`0.3500`、`1.4000`、`3.5000`、`3.0000`、`0.1500`。
- 仅重启 `sub2api-official-18082` 应用容器，`/health` 返回 `200`，容器状态为 `healthy`。
- 容器运行环境回读 `BILLING_FINAL_MULTIPLIER=15`。
