-- 周滚动订阅额度的基础 schema。历史数据和公共套餐配置由显式 cutover 工具处理，
-- 避免应用启动时在未完成盘点、备份和门禁的情况下改写权益事实。

ALTER TABLE user_subscriptions
    ADD COLUMN IF NOT EXISTS weekly_anchor_at TIMESTAMPTZ;

ALTER TABLE payment_orders
    ADD COLUMN IF NOT EXISTS subscription_snapshot JSONB,
    ADD COLUMN IF NOT EXISTS refund_basis JSONB;

ALTER TABLE subscription_entitlement_periods
    ADD COLUMN IF NOT EXISTS weekly_limit_usd NUMERIC(20,10),
    ADD COLUMN IF NOT EXISTS period_total_quota_usd NUMERIC(20,10),
    ADD COLUMN IF NOT EXISTS quota_window_unit VARCHAR(20) NOT NULL DEFAULT 'day',
    ADD COLUMN IF NOT EXISTS quota_window_days INTEGER NOT NULL DEFAULT 1;

ALTER TABLE subscription_entitlement_periods
    DROP CONSTRAINT IF EXISTS subscription_entitlement_periods_quota_window_unit_check;
ALTER TABLE subscription_entitlement_periods
    ADD CONSTRAINT subscription_entitlement_periods_quota_window_unit_check
    CHECK (quota_window_unit IN ('day', 'week', 'month', 'none'));
ALTER TABLE subscription_entitlement_periods
    DROP CONSTRAINT IF EXISTS subscription_entitlement_periods_quota_window_days_check;
ALTER TABLE subscription_entitlement_periods
    ADD CONSTRAINT subscription_entitlement_periods_quota_window_days_check
    CHECK (quota_window_days > 0);
ALTER TABLE subscription_entitlement_periods
    DROP CONSTRAINT IF EXISTS subscription_entitlement_periods_weekly_limit_check;
ALTER TABLE subscription_entitlement_periods
    ADD CONSTRAINT subscription_entitlement_periods_weekly_limit_check
    CHECK (weekly_limit_usd IS NULL OR weekly_limit_usd >= 0);
ALTER TABLE subscription_entitlement_periods
    DROP CONSTRAINT IF EXISTS subscription_entitlement_periods_total_quota_check;
ALTER TABLE subscription_entitlement_periods
    ADD CONSTRAINT subscription_entitlement_periods_total_quota_check
    CHECK (period_total_quota_usd IS NULL OR period_total_quota_usd >= 0);

ALTER TABLE usage_facts
    ADD COLUMN IF NOT EXISTS entitlement_period_id BIGINT
    REFERENCES subscription_entitlement_periods(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_usage_facts_entitlement_period_id
    ON usage_facts (entitlement_period_id, completed_at)
    WHERE entitlement_period_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS subscription_quota_debt_adjustments (
    id BIGSERIAL PRIMARY KEY,
    subscription_id BIGINT NOT NULL REFERENCES user_subscriptions(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE RESTRICT,
    source_key VARCHAR(160) NOT NULL,
    overage_usd NUMERIC(20,10) NOT NULL,
    weekly_limit_usd NUMERIC(20,10) NOT NULL,
    daily_equivalent_usd NUMERIC(20,10) NOT NULL,
    raw_deduction_days NUMERIC(20,10) NOT NULL,
    deducted_days INTEGER NOT NULL,
    original_expires_at TIMESTAMPTZ NOT NULL,
    new_expires_at TIMESTAMPTZ NOT NULL,
    application_status VARCHAR(32) NOT NULL,
    applied_at TIMESTAMPTZ,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT subscription_quota_debt_adjustments_source_key_unique UNIQUE (source_key),
    CONSTRAINT subscription_quota_debt_adjustments_status_check
        CHECK (application_status IN ('pending', 'applied', 'already_applied', 'manual_review')),
    CONSTRAINT subscription_quota_debt_adjustments_days_check CHECK (deducted_days >= 0)
);

CREATE INDEX IF NOT EXISTS idx_subscription_quota_debt_adjustments_subscription
    ON subscription_quota_debt_adjustments (subscription_id, created_at DESC);
