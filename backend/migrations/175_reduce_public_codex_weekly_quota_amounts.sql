-- 将公共 Codex 周额度下调到 2026-07-22 确认的新金额。
-- 只更新可继续消费的额度事实，不改已支付订单金额和不可变订单快照。

WITH plan_limits(group_name, weekly_usd, plan_label) AS (
  VALUES
    ('codex-pool-19-usd'::text, 58::numeric, '29 元订阅池'::text),
    ('codex-pool-29-usd', 78::numeric, '39 元订阅池'),
    ('codex-pool-49-usd', 118::numeric, '59 元订阅池'),
    ('codex-pool-69-usd', 158::numeric, '79 元订阅池'),
    ('codex-pool-89-usd', 198::numeric, '99 元订阅池'),
    ('codex-pool-135-usd', 299::numeric, '149 元订阅池'),
    ('codex-pool-179-usd', 400::numeric, '199 元订阅池')
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
    ('codex-pool-19-usd'::text, 58::numeric, '29 元订阅池'::text),
    ('codex-pool-29-usd', 78::numeric, '39 元订阅池'),
    ('codex-pool-49-usd', 118::numeric, '59 元订阅池'),
    ('codex-pool-69-usd', 158::numeric, '79 元订阅池'),
    ('codex-pool-89-usd', 198::numeric, '99 元订阅池'),
    ('codex-pool-135-usd', 299::numeric, '149 元订阅池'),
    ('codex-pool-179-usd', 400::numeric, '199 元订阅池')
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
    ('codex-pool-19-usd'::text, 58::numeric),
    ('codex-pool-29-usd', 78::numeric),
    ('codex-pool-49-usd', 118::numeric),
    ('codex-pool-69-usd', 158::numeric),
    ('codex-pool-89-usd', 198::numeric),
    ('codex-pool-135-usd', 299::numeric),
    ('codex-pool-179-usd', 400::numeric)
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
