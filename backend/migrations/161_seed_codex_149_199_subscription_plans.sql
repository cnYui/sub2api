-- 新增 149 元与 199 元订阅套餐：30 天有效期，每日 135/179 USD。
-- 只 seed 分组和商品计划，不复制或绑定上游账号。

DO $$
DECLARE
    plan RECORD;
    target_group_id BIGINT;
BEGIN
    FOR plan IN
        SELECT *
        FROM (
            VALUES
                ('codex-pool-135-usd'::TEXT, '149 元订阅池'::TEXT, 149.00::NUMERIC, 135::NUMERIC, 149::INTEGER),
                ('codex-pool-179-usd'::TEXT, '199 元订阅池'::TEXT, 199.00::NUMERIC, 179::NUMERIC, 199::INTEGER)
        ) AS plans(group_name, plan_name, price, daily_usd, sort_order)
    LOOP
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
            image_price_1k,
            image_price_2k,
            image_price_4k,
            allow_messages_dispatch,
            require_oauth_only,
            require_privacy_set,
            default_mapped_model,
            messages_dispatch_model_config,
            models_list_config,
            supported_model_scopes,
            model_routing,
            model_routing_enabled,
            sort_order,
            created_at,
            updated_at
        )
        SELECT
            plan.group_name,
            plan.plan_name || '，每日 ' || plan.daily_usd::TEXT || ' USD，30 天有效期',
            1.0,
            FALSE,
            'active',
            'openai',
            'subscription',
            plan.daily_usd,
            NULL,
            NULL,
            30,
            TRUE,
            COALESCE((SELECT image_rate_independent FROM groups WHERE name = 'codex-pool-49-usd' AND deleted_at IS NULL ORDER BY id LIMIT 1), FALSE),
            COALESCE((SELECT image_rate_multiplier FROM groups WHERE name = 'codex-pool-49-usd' AND deleted_at IS NULL ORDER BY id LIMIT 1), 1.0),
            0.10,
            0.20,
            0.40,
            COALESCE((SELECT allow_messages_dispatch FROM groups WHERE name = 'codex-pool-49-usd' AND deleted_at IS NULL ORDER BY id LIMIT 1), FALSE),
            COALESCE((SELECT require_oauth_only FROM groups WHERE name = 'codex-pool-49-usd' AND deleted_at IS NULL ORDER BY id LIMIT 1), FALSE),
            COALESCE((SELECT require_privacy_set FROM groups WHERE name = 'codex-pool-49-usd' AND deleted_at IS NULL ORDER BY id LIMIT 1), FALSE),
            COALESCE((SELECT default_mapped_model FROM groups WHERE name = 'codex-pool-49-usd' AND deleted_at IS NULL ORDER BY id LIMIT 1), ''),
            COALESCE((SELECT messages_dispatch_model_config FROM groups WHERE name = 'codex-pool-49-usd' AND deleted_at IS NULL ORDER BY id LIMIT 1), '{}'::jsonb),
            COALESCE((SELECT models_list_config FROM groups WHERE name = 'codex-pool-49-usd' AND deleted_at IS NULL ORDER BY id LIMIT 1), '{}'::jsonb),
            COALESCE((SELECT supported_model_scopes FROM groups WHERE name = 'codex-pool-49-usd' AND deleted_at IS NULL ORDER BY id LIMIT 1), '[]'::jsonb),
            COALESCE((SELECT model_routing FROM groups WHERE name = 'codex-pool-49-usd' AND deleted_at IS NULL ORDER BY id LIMIT 1), '{}'::jsonb),
            COALESCE((SELECT model_routing_enabled FROM groups WHERE name = 'codex-pool-49-usd' AND deleted_at IS NULL ORDER BY id LIMIT 1), FALSE),
            plan.sort_order,
            NOW(),
            NOW()
        WHERE NOT EXISTS (
            SELECT 1 FROM groups WHERE name = plan.group_name AND deleted_at IS NULL
        )
        RETURNING id INTO target_group_id;

        SELECT id INTO target_group_id
        FROM groups
        WHERE name = plan.group_name AND deleted_at IS NULL
        ORDER BY id
        LIMIT 1;

        UPDATE groups
        SET
            description = plan.plan_name || '，每日 ' || plan.daily_usd::TEXT || ' USD，30 天有效期',
            rate_multiplier = 1.0,
            is_exclusive = FALSE,
            status = 'active',
            platform = 'openai',
            subscription_type = 'subscription',
            daily_limit_usd = plan.daily_usd,
            weekly_limit_usd = NULL,
            monthly_limit_usd = NULL,
            default_validity_days = 30,
            allow_image_generation = TRUE,
            image_rate_independent = COALESCE((SELECT image_rate_independent FROM groups WHERE name = 'codex-pool-49-usd' AND deleted_at IS NULL ORDER BY id LIMIT 1), FALSE),
            image_rate_multiplier = COALESCE((SELECT image_rate_multiplier FROM groups WHERE name = 'codex-pool-49-usd' AND deleted_at IS NULL ORDER BY id LIMIT 1), 1.0),
            image_price_1k = 0.10,
            image_price_2k = 0.20,
            image_price_4k = 0.40,
            allow_messages_dispatch = COALESCE((SELECT allow_messages_dispatch FROM groups WHERE name = 'codex-pool-49-usd' AND deleted_at IS NULL ORDER BY id LIMIT 1), FALSE),
            require_oauth_only = COALESCE((SELECT require_oauth_only FROM groups WHERE name = 'codex-pool-49-usd' AND deleted_at IS NULL ORDER BY id LIMIT 1), FALSE),
            require_privacy_set = COALESCE((SELECT require_privacy_set FROM groups WHERE name = 'codex-pool-49-usd' AND deleted_at IS NULL ORDER BY id LIMIT 1), FALSE),
            default_mapped_model = COALESCE((SELECT default_mapped_model FROM groups WHERE name = 'codex-pool-49-usd' AND deleted_at IS NULL ORDER BY id LIMIT 1), ''),
            messages_dispatch_model_config = COALESCE((SELECT messages_dispatch_model_config FROM groups WHERE name = 'codex-pool-49-usd' AND deleted_at IS NULL ORDER BY id LIMIT 1), '{}'::jsonb),
            models_list_config = COALESCE((SELECT models_list_config FROM groups WHERE name = 'codex-pool-49-usd' AND deleted_at IS NULL ORDER BY id LIMIT 1), '{}'::jsonb),
            supported_model_scopes = COALESCE((SELECT supported_model_scopes FROM groups WHERE name = 'codex-pool-49-usd' AND deleted_at IS NULL ORDER BY id LIMIT 1), '[]'::jsonb),
            model_routing = COALESCE((SELECT model_routing FROM groups WHERE name = 'codex-pool-49-usd' AND deleted_at IS NULL ORDER BY id LIMIT 1), '{}'::jsonb),
            model_routing_enabled = COALESCE((SELECT model_routing_enabled FROM groups WHERE name = 'codex-pool-49-usd' AND deleted_at IS NULL ORDER BY id LIMIT 1), FALSE),
            sort_order = plan.sort_order,
            updated_at = NOW()
        WHERE id = target_group_id;

        UPDATE subscription_plans
        SET
            group_id = target_group_id,
            name = plan.plan_name,
            description = '月度订阅-时间 30天，日限额 ' || plan.daily_usd::TEXT || '刀，24点刷新',
            price = plan.price,
            original_price = NULL,
            validity_days = 30,
            validity_unit = 'day',
            features = '',
            product_name = plan.plan_name,
            for_sale = TRUE,
            sort_order = plan.sort_order,
            updated_at = NOW()
        WHERE group_id = target_group_id
           OR name = plan.plan_name
           OR product_name = plan.plan_name;

        INSERT INTO subscription_plans (
            group_id,
            name,
            description,
            price,
            original_price,
            validity_days,
            validity_unit,
            features,
            product_name,
            for_sale,
            sort_order,
            created_at,
            updated_at
        )
        SELECT
            target_group_id,
            plan.plan_name,
            '月度订阅-时间 30天，日限额 ' || plan.daily_usd::TEXT || '刀，24点刷新',
            plan.price,
            NULL,
            30,
            'day',
            '',
            plan.plan_name,
            TRUE,
            plan.sort_order,
            NOW(),
            NOW()
        WHERE NOT EXISTS (
            SELECT 1
            FROM subscription_plans
            WHERE group_id = target_group_id
               OR name = plan.plan_name
               OR product_name = plan.plan_name
        );
    END LOOP;
END $$;
