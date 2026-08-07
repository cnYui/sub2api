# Claude Kiro 0.45 倍率同步

## 决策

- 按管理员要求，本地 Claude Kiro 的用户价从 `0.35x` 调整为 `0.45x`。
- 用户计费取分组倍率；该分组无用户专属倍率覆盖，因此不需要逐个 API Key 修改。
- `accounts.rate_multiplier=3.5` 是账号统计口径，不参与用户扣费，保持不变。

## 配置变更

- `groups.id=5`：名称由 `Claude - Kiro` 改为 `0.45X Claude - Kiro`，`rate_multiplier` 由 `0.35` 改为 `0.45`。
- `accounts.id=3`：名称由 `Claude Kiro反代官方0.35倍价格` 改为 `Claude Kiro反代官方0.45倍价格`。
- 未修改模型基础单价、模型映射、账号统计倍率、最终倍率、历史用量和余额。

## 计费与展示口径

- 模型广场按隐藏最终倍率前展示：基础价 × `0.45`。例如 `claude-fable-5` 显示输入 `$4.50`、输出 `$22.50` / 1M token。
- 实际用户扣费：基础成本 × 分组倍率 `0.45` × `BILLING_FINAL_MULTIPLIER=15`。例如 Claude Fable 5 的输入/输出等效最终扣费分别为 `$67.50/$337.50` / 1M token。

## 验证

- 该分组没有 `user_group_rate_multipliers.rate_multiplier` 覆盖；11 个活跃 API Key 直接使用分组默认倍率。
- 数据库分组更新触发鉴权缓存失效，`auth_cache_invalidation_outbox` 已消费完毕。
- 使用该分组的一把有效 API Key 调用本地只读 `/v1/sub2api/billing`，`group_rate_multiplier`、`resolved_rate_multiplier`、`effective_rate_multiplier` 均为 `0.45`。
- 本地与公网 `/api/v1/model-plaza` 均返回名称 `0.45X Claude - Kiro` 和倍率 `0.45`。

## 上游差异

- 本次修改前以账号 `id=3` 的凭证查询上游 `/v1/sub2api/billing`，上游仍返回 `group/resolved/effective_rate_multiplier=0.35`。
- 本次只调整本地用户定价与展示，未改写上游账号或上游分组；若上游应当同样为 `0.45`，需在上游站点另行完成其分组配置。
