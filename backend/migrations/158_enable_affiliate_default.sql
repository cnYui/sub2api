-- 默认打开邀请返利功能，并让已存在环境同步启用该开关。
INSERT INTO settings (key, value, updated_at)
VALUES ('affiliate_enabled', 'true', NOW())
ON CONFLICT (key) DO UPDATE
SET value = 'true',
    updated_at = NOW()
WHERE settings.value IS DISTINCT FROM 'true';
