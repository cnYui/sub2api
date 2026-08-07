# 全渠道加价倍率修正验证

## 生产配置

2026-08-05 10:54（本地 +09）复核生产 PostgreSQL：

| 分组 ID | 分组 | 当前用户扣费倍率 |
| ---: | --- | ---: |
| 4 | Claude Max | `1.0000` |
| 6 | GLM | `1.0000` |
| 7 | Kimi | `1.0000` |
| 8 | DeepSeek | `1.0000` |

折扣分组保持：Grok `0.6`、Claude Kiro `0.35`、GPT `0.15/0.35/0.1`，GPT-Image-2 为 `1.0`。容器仍使用 `BILLING_FINAL_MULTIPLIER=15`。

## 缓存与新请求核验

- 三个本轮更新分组共写入 6 个认证缓存失效事件。
- 双阶段失效完成后，`auth_cache_invalidation_outbox` 待处理数为 `0`，最大重试次数为 `0`。
- 分组 4、6、7、8 的修正时间之后没有新的用量记录，也没有新记录携带旧倍率快照。
- 应用容器健康状态为 `healthy`。

## 测试

以下现有单元测试通过：

- `TestFinalBillingMultiplierOnlyChangesActualCost`
- `TestCalculateCost_RateMultiplier`

本次没有发起收费模型请求，没有改写历史 `usage_logs`，也没有执行历史余额补偿。
