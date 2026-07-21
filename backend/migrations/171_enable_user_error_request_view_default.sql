-- 默认允许用户在用量页查看自己的错误请求；后端仍按 user_id 过滤并脱敏详情。
INSERT INTO settings (key, value, updated_at)
VALUES ('allow_user_view_error_requests', 'true', NOW())
ON CONFLICT (key) DO UPDATE
SET
    value = EXCLUDED.value,
    updated_at = NOW();
