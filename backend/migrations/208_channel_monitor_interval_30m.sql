-- Migration: 208_channel_monitor_interval_30m
-- 渠道监控统一按 30 分钟执行一次目录探测。

UPDATE channel_monitors
SET interval_seconds = 1800
WHERE interval_seconds IS DISTINCT FROM 1800;

