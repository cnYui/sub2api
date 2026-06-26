-- 固化 Codex 售卖套餐基线，避免新环境只 seed 99 元而缺少老三档。
-- 只补齐分组和订阅计划，不复制或绑定上游账号。

DO $$
DECLARE
    plan RECORD;
    target_group_id BIGINT;
BEGIN
    FOR plan IN
        SELECT *
        FROM (
            VALUES
                ('codex-pool-19-usd'::TEXT, '29 元订阅池'::TEXT, 29.00::NUMERIC, 19::NUMERIC, 29::INTEGER),
                ('codex-pool-29-usd'::TEXT, '39 元订阅池'::TEXT, 39.00::NUMERIC, 29::NUMERIC, 39::INTEGER),
                ('codex-pool-49-usd'::TEXT, '59 元订阅池'::TEXT, 59.00::NUMERIC, 49::NUMERIC, 59::INTEGER),
                ('codex-pool-89-usd'::TEXT, '99 元订阅池'::TEXT, 99.00::NUMERIC, 89::NUMERIC, 99::INTEGER)
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
            FALSE,
            1.0,
            0.10,
            0.20,
            0.40,
            FALSE,
            FALSE,
            FALSE,
            '',
            '{}'::jsonb,
            '{}'::jsonb,
            '[]'::jsonb,
            '{}'::jsonb,
            FALSE,
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
            image_rate_independent = FALSE,
            image_rate_multiplier = 1.0,
            image_price_1k = 0.10,
            image_price_2k = 0.20,
            image_price_4k = 0.40,
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
