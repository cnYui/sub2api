-- 固化 2026-07 削减后的 Codex 订阅日额度，避免前端展示与计费事实源分叉。
WITH plan_limits(group_name, daily_usd, plan_label, price) AS (
    VALUES
        ('codex-pool-19-usd'::TEXT, 15::NUMERIC, '29 元订阅池'::TEXT, 29.00::NUMERIC),
        ('codex-pool-29-usd'::TEXT, 25::NUMERIC, '39 元订阅池'::TEXT, 39.00::NUMERIC),
        ('codex-pool-49-usd'::TEXT, 39::NUMERIC, '59 元订阅池'::TEXT, 59.00::NUMERIC),
        ('codex-pool-69-usd'::TEXT, 53::NUMERIC, '79 元订阅池'::TEXT, 79.00::NUMERIC),
        ('codex-pool-89-usd'::TEXT, 66::NUMERIC, '99 元订阅池'::TEXT, 99.00::NUMERIC),
        ('codex-pool-135-usd'::TEXT, 100::NUMERIC, '149 元订阅池'::TEXT, 149.00::NUMERIC),
        ('codex-pool-179-usd'::TEXT, 133::NUMERIC, '199 元订阅池'::TEXT, 199.00::NUMERIC)
)
UPDATE groups g
SET
    daily_limit_usd = plan.daily_usd,
    description = plan.plan_label || '，每日 ' || trim_scale(plan.daily_usd) || ' USD，30 天有效期',
    updated_at = NOW()
FROM plan_limits plan
WHERE g.name = plan.group_name
  AND g.subscription_type = 'subscription';

WITH plan_limits(group_name, daily_usd, plan_label, price) AS (
    VALUES
        ('codex-pool-19-usd'::TEXT, 15::NUMERIC, '29 元订阅池'::TEXT, 29.00::NUMERIC),
        ('codex-pool-29-usd'::TEXT, 25::NUMERIC, '39 元订阅池'::TEXT, 39.00::NUMERIC),
        ('codex-pool-49-usd'::TEXT, 39::NUMERIC, '59 元订阅池'::TEXT, 59.00::NUMERIC),
        ('codex-pool-69-usd'::TEXT, 53::NUMERIC, '79 元订阅池'::TEXT, 79.00::NUMERIC),
        ('codex-pool-89-usd'::TEXT, 66::NUMERIC, '99 元订阅池'::TEXT, 99.00::NUMERIC),
        ('codex-pool-135-usd'::TEXT, 100::NUMERIC, '149 元订阅池'::TEXT, 149.00::NUMERIC),
        ('codex-pool-179-usd'::TEXT, 133::NUMERIC, '199 元订阅池'::TEXT, 199.00::NUMERIC)
)
UPDATE subscription_plans sp
SET
    description = '月度订阅-时间 30天，日限额 ' || trim_scale(plan.daily_usd) || '刀，24点刷新',
    updated_at = NOW()
FROM plan_limits plan
JOIN groups g ON g.name = plan.group_name
WHERE sp.group_id = g.id
  AND sp.name = plan.plan_label
  AND sp.price = plan.price;
