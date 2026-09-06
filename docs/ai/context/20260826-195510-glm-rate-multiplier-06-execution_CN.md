# GLM 分组倍率调整为 0.6x

## 变更内容

- 执行时间：2026-08-26 18:54（Asia/Tokyo）。
- 用户请求“GLM 分组改为 6 折”；按中文价格语义将“6 折”解释为 `0.6x`，不是 `6x`。
- 目标生产分组：`groups.id=6`，原名称 `GLM0.5倍率`，原 `rate_multiplier=3.5000`。
- 在 PostgreSQL 单事务内将名称改为 `GLM0.6倍率`，`rate_multiplier` 改为 `0.6000`。
- 同事务写入 `scheduler_outbox` 的 `group_changed` 事件；既有分组缓存失效触发器为 3 个有效 API Key 写入认证缓存失效事件，后台已全部消费。
- 官方 GLM 基础价格、账号统计倍率、用户专属倍率、历史用量、余额、订单和最终计费倍率均未修改。

## 验证结果

- 数据库最终值：`id=6`、名称 `GLM0.6倍率`、`rate_multiplier=0.6000`。
- `user_group_rate_multipliers` 中 `group_id=6` 覆盖数仍为 `0`；`usage_logs` 中该分组历史记录仍为 `2` 条。
- `scheduler_outbox` 与 `auth_cache_invalidation_outbox` 均无待处理事件。
- 本地 `http://127.0.0.1:18082/api/v1/model-plaza` 和 `aaccx.pw`、`www.aaccx.pw`、`api.aaccx.pw` 三个公网端点均返回 HTTP `200`，且返回该分组倍率 `0.6` 和名称 `GLM0.6倍率`。
- `sub2api-official-18082`、PostgreSQL、Redis 容器均保持 `healthy`；未重建镜像、容器或数据卷。
