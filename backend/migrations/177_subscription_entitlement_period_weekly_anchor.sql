-- 每个权益段独立保存周窗口锚点。历史迁移批次沿用已经校准的订阅行锚点，
-- 新订单和未来续费则始终从权益段自身起点开始，避免跨权益段继承旧窗口。

ALTER TABLE subscription_entitlement_periods
    ADD COLUMN IF NOT EXISTS quota_window_anchor_at TIMESTAMPTZ;

UPDATE subscription_entitlement_periods AS period
SET quota_window_anchor_at = CASE
    WHEN subscription.weekly_anchor_at IS NOT NULL
        AND subscription.weekly_window_start = subscription.weekly_anchor_at
        AND subscription.weekly_anchor_at > period.starts_at
        THEN subscription.weekly_anchor_at
    ELSE period.starts_at
END,
updated_at = NOW()
FROM user_subscriptions AS subscription
WHERE period.subscription_id = subscription.id
  AND period.quota_window_unit = 'week'
  AND period.quota_window_anchor_at IS NULL;
