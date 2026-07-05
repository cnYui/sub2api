-- 自动 API Key 运行时解析 OpenAI effective group。
-- 旧 OpenAI Key 改为 group_id=NULL，避免用户升级/退款/重购后仍被固定旧套餐分组锁住。

DO $$
DECLARE
    traffic_group_id BIGINT;
BEGIN
    INSERT INTO groups (
        name,
        description,
        rate_multiplier,
        is_exclusive,
        status,
        platform,
        subscription_type,
        daily_limit_usd,
        weekly_limit_usd,
        monthly_limit_usd,
        default_validity_days,
        allow_image_generation,
        image_rate_independent,
        image_rate_multiplier,
        sort_order,
        created_at,
        updated_at
    )
    SELECT
        'traffic-pack-openai',
        'OpenAI/GPT 流量包自动 Key 内部入口分组',
        1.0,
        TRUE,
        'active',
        'openai',
        'standard',
        NULL,
        NULL,
        NULL,
        30,
        TRUE,
        FALSE,
        1.0,
        10000,
        NOW(),
        NOW()
    WHERE NOT EXISTS (
        SELECT 1
        FROM groups
        WHERE name = 'traffic-pack-openai'
          AND deleted_at IS NULL
    )
    RETURNING id INTO traffic_group_id;

    SELECT id INTO traffic_group_id
    FROM groups
    WHERE name = 'traffic-pack-openai'
      AND deleted_at IS NULL
    ORDER BY id
    LIMIT 1;

    UPDATE groups
    SET
        description = 'OpenAI/GPT 流量包自动 Key 内部入口分组',
        rate_multiplier = 1.0,
        is_exclusive = TRUE,
        status = 'active',
        platform = 'openai',
        subscription_type = 'standard',
        daily_limit_usd = NULL,
        weekly_limit_usd = NULL,
        monthly_limit_usd = NULL,
        default_validity_days = 30,
        allow_image_generation = TRUE,
        image_rate_independent = FALSE,
        image_rate_multiplier = 1.0,
        updated_at = NOW()
    WHERE id = traffic_group_id;
END $$;

INSERT INTO account_groups (account_id, group_id, priority, created_at)
SELECT a.id, g.id, a.priority, NOW()
FROM accounts a
JOIN groups g
  ON g.name = 'traffic-pack-openai'
 AND g.deleted_at IS NULL
WHERE a.platform = 'openai'
  AND a.deleted_at IS NULL
ON CONFLICT (account_id, group_id) DO NOTHING;

UPDATE api_keys ak
SET group_id = NULL,
    updated_at = NOW()
FROM groups g
WHERE ak.group_id = g.id
  AND g.platform = 'openai'
  AND ak.deleted_at IS NULL;
