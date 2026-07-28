-- 仅允许在外层候选数据库克隆中执行，确保外层 OpenAI 调度唯一指向候选内层。
UPDATE accounts
SET schedulable = false,
    updated_at = NOW()
WHERE deleted_at IS NULL
  AND platform = 'openai'
  AND COALESCE(credentials->>'base_url', '') NOT LIKE '%18086%';

UPDATE accounts
SET credentials = jsonb_set(credentials, '{base_url}', '"http://billing-inner:8080/v1"'::jsonb, true),
    status = 'active',
    schedulable = true,
    updated_at = NOW()
WHERE deleted_at IS NULL
  AND platform = 'openai'
  AND credentials->>'base_url' LIKE '%18086%';
