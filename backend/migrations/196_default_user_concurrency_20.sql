-- 将遗留的新用户默认并发从 5 升级到 20。
-- 仅替换旧默认值，保留运营方已手工设置的其他并发策略。

ALTER TABLE users
    ALTER COLUMN concurrency SET DEFAULT 20;

INSERT INTO settings (key, value)
VALUES
    ('default_concurrency', '20'),
    ('auth_source_default_email_concurrency', '20'),
    ('auth_source_default_linuxdo_concurrency', '20'),
    ('auth_source_default_oidc_concurrency', '20'),
    ('auth_source_default_wechat_concurrency', '20'),
    ('auth_source_default_github_concurrency', '20'),
    ('auth_source_default_google_concurrency', '20'),
    ('auth_source_default_dingtalk_concurrency', '20')
ON CONFLICT (key) DO UPDATE
SET value = CASE WHEN settings.value = '5' THEN EXCLUDED.value ELSE settings.value END,
    updated_at = CASE WHEN settings.value = '5' THEN NOW() ELSE settings.updated_at END;
