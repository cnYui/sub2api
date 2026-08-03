-- 余额套餐已独立于模型分组。此前一次错误的初始化为 7 个标准渠道组
-- 创建了重复的 paid-subscription 分组和订阅计划；它们未产生业务依赖，
-- 但会让后台数据与当前购买语义不一致。仅在完整的已知映射仍成立时清理。

DO $$
DECLARE
    legacy_group_ids BIGINT[] := ARRAY[10, 11, 12, 13, 14, 15, 16];
    legacy_plan_ids BIGINT[];
BEGIN
    IF (
        SELECT COUNT(*)
        FROM groups
        WHERE id = ANY(legacy_group_ids)
          AND subscription_type = 'subscription'
          AND name LIKE 'paid-subscription-%'
    ) = 0 THEN
        RETURN;
    END IF;

    IF (
        SELECT COUNT(*)
        FROM groups
        WHERE id = ANY(legacy_group_ids)
          AND subscription_type = 'subscription'
          AND name LIKE 'paid-subscription-%'
    ) <> 7 THEN
        RAISE EXCEPTION '拒绝清理不完整的遗留订阅分组';
    END IF;

    IF EXISTS (
        WITH mapping(legacy_group_id, standard_group_id) AS (
            VALUES
                (10::BIGINT, 3::BIGINT),
                (11::BIGINT, 4::BIGINT),
                (12::BIGINT, 5::BIGINT),
                (13::BIGINT, 6::BIGINT),
                (14::BIGINT, 7::BIGINT),
                (15::BIGINT, 8::BIGINT),
                (16::BIGINT, 9::BIGINT)
        )
        SELECT 1
        FROM mapping
        JOIN groups legacy_group ON legacy_group.id = mapping.legacy_group_id
        JOIN groups standard_group ON standard_group.id = mapping.standard_group_id
        WHERE legacy_group.name <> 'paid-subscription-' || standard_group.name
           OR legacy_group.platform <> standard_group.platform
           OR EXISTS (
                SELECT account_id, priority
                FROM account_groups
                WHERE group_id = mapping.legacy_group_id
                EXCEPT
                SELECT account_id, priority
                FROM account_groups
                WHERE group_id = mapping.standard_group_id
            )
           OR EXISTS (
                SELECT account_id, priority
                FROM account_groups
                WHERE group_id = mapping.standard_group_id
                EXCEPT
                SELECT account_id, priority
                FROM account_groups
                WHERE group_id = mapping.legacy_group_id
            )
    ) THEN
        RAISE EXCEPTION '拒绝清理：遗留分组与标准渠道分组不一致';
    END IF;

    SELECT array_agg(id ORDER BY id)
    INTO legacy_plan_ids
    FROM subscription_plans
    WHERE group_id = ANY(legacy_group_ids);

    IF EXISTS (
        SELECT 1
        FROM api_keys
        WHERE group_id = ANY(legacy_group_ids)
        UNION ALL
        SELECT 1
        FROM payment_orders
        WHERE subscription_group_id = ANY(legacy_group_ids)
           OR plan_id = ANY(COALESCE(legacy_plan_ids, ARRAY[]::BIGINT[]))
        UNION ALL
        SELECT 1
        FROM user_subscriptions
        WHERE group_id = ANY(legacy_group_ids)
        UNION ALL
        SELECT 1
        FROM user_allowed_groups
        WHERE group_id = ANY(legacy_group_ids)
        UNION ALL
        SELECT 1
        FROM usage_logs
        WHERE group_id = ANY(legacy_group_ids)
    ) THEN
        RAISE EXCEPTION '拒绝清理：遗留订阅分组已产生业务依赖';
    END IF;

    DELETE FROM subscription_plans
    WHERE group_id = ANY(legacy_group_ids);

    DELETE FROM groups
    WHERE id = ANY(legacy_group_ids);
END;
$$;
