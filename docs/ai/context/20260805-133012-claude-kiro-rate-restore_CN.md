# Claude Kiro 渠道倍率恢复

## 生产变更

- 生产分组：`groups.id=5`，Claude Kiro。
- 用户计费倍率：`1.0000x` → `0.3500x`。
- 最终计费倍率 `BILLING_FINAL_MULTIPLIER=15` 保持不变。
- 其它模型分组倍率未修改。

## 生效核验

- 分组更新触发 `5` 条认证缓存失效事件。
- `auth_cache_invalidation_outbox` 已清空，最大重试次数无异常。
- 历史 `usage_logs`、用户余额和补偿流水未修改。
- 后续 Claude Kiro 用户扣费公式为：`标准成本 × 0.35 × 15`。
