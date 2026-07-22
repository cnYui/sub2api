-- 将公共 Codex 周额度提升约 30%，价格和历史用量事实保持不变。
-- 本迁移只前向更新可变配置、有效权益上限和未完成发放订单快照。

WITH plan_limits(group_name, weekly_usd, plan_label) AS (
  VALUES
    ('codex-pool-19-usd'::text, 76::numeric, '29 元订阅池'::text),
    ('codex-pool-29-usd', 102::numeric, '39 元订阅池'),
    ('codex-pool-49-usd', 154::numeric, '59 元订阅池'),
    ('codex-pool-69-usd', 206::numeric, '79 元订阅池'),
    ('codex-pool-89-usd', 258::numeric, '99 元订阅池'),
    ('codex-pool-135-usd', 389::numeric, '149 元订阅池'),
    ('codex-pool-179-usd', 520::numeric, '199 元订阅池')
)
UPDATE groups g
SET
  daily_limit_usd = NULL,
  monthly_limit_usd = NULL,
  weekly_limit_usd = p.weekly_usd,
  default_validity_days = 28,
  description = p.plan_label || '，每 7 天 ' || trim_scale(p.weekly_usd) || ' USD，28 天有效期',
  updated_at = NOW()
FROM plan_limits p
WHERE g.name = p.group_name
  AND g.subscription_type = 'subscription'
  AND g.deleted_at IS NULL;

WITH plan_limits(group_name, weekly_usd, plan_label) AS (
  VALUES
    ('codex-pool-19-usd'::text, 76::numeric, '29 元订阅池'::text),
    ('codex-pool-29-usd', 102::numeric, '39 元订阅池'),
    ('codex-pool-49-usd', 154::numeric, '59 元订阅池'),
    ('codex-pool-69-usd', 206::numeric, '79 元订阅池'),
    ('codex-pool-89-usd', 258::numeric, '99 元订阅池'),
    ('codex-pool-135-usd', 389::numeric, '149 元订阅池'),
    ('codex-pool-179-usd', 520::numeric, '199 元订阅池')
)
UPDATE subscription_plans sp
SET
  name = p.plan_label,
  product_name = p.plan_label,
  validity_days = 28,
  validity_unit = 'day',
  description = '28 天订阅，每 7 天刷新 ' || trim_scale(p.weekly_usd) || ' USD 周额度，购买时间起滚动计算',
  features = '周额度 ' || trim_scale(p.weekly_usd) || ' USD' || E'\n28 天有效期' || E'\n购买时间起每 7 天刷新',
  updated_at = NOW()
FROM groups g
JOIN plan_limits p ON p.group_name = g.name
WHERE sp.group_id = g.id
  AND g.subscription_type = 'subscription'
  AND g.deleted_at IS NULL;

WITH plan_limits(group_name, weekly_usd) AS (
  VALUES
    ('codex-pool-19-usd'::text, 76::numeric),
    ('codex-pool-29-usd', 102::numeric),
    ('codex-pool-49-usd', 154::numeric),
    ('codex-pool-69-usd', 206::numeric),
    ('codex-pool-89-usd', 258::numeric),
    ('codex-pool-135-usd', 389::numeric),
    ('codex-pool-179-usd', 520::numeric)
)
UPDATE subscription_entitlement_periods sep
SET
  weekly_limit_usd = p.weekly_usd,
  period_total_quota_usd = p.weekly_usd * 4,
  quota_window_unit = 'week',
  quota_window_days = 7,
  updated_at = NOW()
FROM groups g
JOIN plan_limits p ON p.group_name = g.name
WHERE sep.group_id = g.id
  AND sep.status = 'active'
  AND sep.expires_at > NOW()
  AND g.subscription_type = 'subscription'
  AND g.deleted_at IS NULL;

WITH plan_limits(group_name, weekly_usd, plan_label) AS (
  VALUES
    ('codex-pool-19-usd'::text, 76::numeric, '29 元订阅池'::text),
    ('codex-pool-29-usd', 102::numeric, '39 元订阅池'),
    ('codex-pool-49-usd', 154::numeric, '59 元订阅池'),
    ('codex-pool-69-usd', 206::numeric, '79 元订阅池'),
    ('codex-pool-89-usd', 258::numeric, '99 元订阅池'),
    ('codex-pool-135-usd', 389::numeric, '149 元订阅池'),
    ('codex-pool-179-usd', 520::numeric, '199 元订阅池')
)
UPDATE payment_orders po
SET
  subscription_days = 28,
  subscription_snapshot = COALESCE(po.subscription_snapshot, '{}'::jsonb) || jsonb_build_object(
    'version', 1,
    'plan_id', po.plan_id,
    'plan_name', p.plan_label,
    'group_id', g.id,
    'group_name', g.name,
    'validity_days', 28,
    'weekly_limit_usd', p.weekly_usd,
    'period_total_quota_usd', p.weekly_usd * 4,
    'quota_window_unit', 'week',
    'quota_window_days', 7
  ),
  updated_at = NOW()
FROM groups g
JOIN plan_limits p ON p.group_name = g.name
WHERE po.subscription_group_id = g.id
  AND po.order_type = 'subscription'
  AND po.plan_id IS NOT NULL
  AND po.subscription_id IS NULL
  AND (
    po.status IN ('PAID', 'RECHARGING')
    OR (po.status = 'FAILED' AND po.paid_at IS NOT NULL)
    OR (po.status = 'PENDING' AND po.expires_at > NOW())
  )
  AND g.subscription_type = 'subscription'
  AND g.deleted_at IS NULL;
