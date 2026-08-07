# 全部分组用户计费倍率统一为 1

## 变更原因

用户于 2026-08-05 明确要求当前所有模型渠道的用户计费倍率统一为 `1x`，实际扣费只保留最终 `15x`：

```text
用户扣费 = 标准成本 × 1 × 15
```

## 生产变更

- 生产实例：`sub2api-official-18082`。
- PostgreSQL 中所有 `status='active'` 的 `groups.rate_multiplier` 已统一写为 `1.0000`，共 10 个分组（ID `3` 至 `12`）。
- `BILLING_FINAL_MULTIPLIER=15` 保持不变。
- `accounts.rate_multiplier` 未修改；它只用于账号统计/配额，不参与用户余额扣费。
- `image_rate_multiplier`、`video_rate_multiplier` 未修改。
- `user_group_rate_multipliers` 当前无非空用户专属覆盖，因此不存在绕过本次统一设置的用户倍率。
- 分组名称中的历史“0.6 倍/0.35 倍/0.5 折”等文字未改写；实际计费以数值字段为准。

## 缓存与运行状态

- 分组更新触发 `168` 条认证缓存失效事件。
- `auth_cache_invalidation_outbox` 已清空，最大重试次数无异常。
- 应用容器状态：`running/healthy`。
- 历史 `usage_logs`、用户余额和补偿流水未改写。

## 当前活跃分组核验

| 分组 ID | 分组 | 当前用户计费倍率 |
| ---: | --- | ---: |
| 3 | Grok | `1.0000x` |
| 4 | Claude Max | `1.0000x` |
| 5 | Claude Kiro | `1.0000x` |
| 6 | GLM | `1.0000x` |
| 7 | Kimi | `1.0000x` |
| 8 | DeepSeek | `1.0000x` |
| 9 | GPT 0.15 | `1.0000x` |
| 10 | GPT 0.35 | `1.0000x` |
| 11 | GPT 0.1 | `1.0000x` |
| 12 | GPT-Image-2 | `1.0000x` |
