-- 渠道监控支持「按分组自动同步」：记录监控来自哪个分组的哪个账号。
-- 同步任务按 (source_group_id, source_account_id) 找到对应监控并更新；
-- 手工创建的监控两列都为 NULL，不受同步影响。
-- 不建外键：分组/账号删除后由同步任务停用监控，保留历史可用率。
-- 列名/可空性必须与 backend/ent/schema/channel_monitor.go 保持一致（ent 不做自动迁移）。
ALTER TABLE channel_monitors
    ADD COLUMN IF NOT EXISTS source_group_id BIGINT NULL,
    ADD COLUMN IF NOT EXISTS source_account_id BIGINT NULL;

CREATE INDEX IF NOT EXISTS channelmonitor_source_group_id_source_account_id
    ON channel_monitors (source_group_id, source_account_id);

COMMENT ON COLUMN channel_monitors.source_group_id IS '自动同步来源分组 ID（手工创建为 NULL）';
COMMENT ON COLUMN channel_monitors.source_account_id IS '自动同步来源账号 ID（手工创建为 NULL）';
