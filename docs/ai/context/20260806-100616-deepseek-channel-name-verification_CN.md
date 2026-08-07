# DeepSeek 渠道名称核验

时间：2026-08-06 10:06:16（Asia/Tokyo）

## 目标

将当前项目的 DeepSeek 渠道展示名称统一为“DeepSeek模型官方0.7折价格”。

## 核验结果

- 生产实例 `sub2api-official-18082` 的 `groups.id=8` 已为目标名称，平台为 `openai`，状态为 `active`。
- 条件更新未命中任何记录，说明无需额外写入数据库。
- `channel_monitors`、`content_moderation_logs`、`prompt_audit_events` 和 `prompt_audit_jobs` 中不存在 DeepSeek 名称快照。
- Redis 中不存在分组名称缓存键。

## 影响范围

未修改 `rate_multiplier`、模型映射、用户权限、历史用量或审计记录。历史文档中的旧名称用于描述当时状态，保留不改。
