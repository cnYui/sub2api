CREATE TABLE IF NOT EXISTS subscription_entitlement_periods (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    subscription_id BIGINT NOT NULL,
    group_id BIGINT NOT NULL,
    source_type VARCHAR(40) NOT NULL,
    source_id VARCHAR(128) NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    period_days INTEGER NOT NULL,
    daily_limit_usd NUMERIC(20,10),
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    revoked_at TIMESTAMPTZ,
    revoked_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT subscription_entitlement_periods_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT,
    CONSTRAINT subscription_entitlement_periods_subscription_id_fkey
        FOREIGN KEY (subscription_id) REFERENCES user_subscriptions(id) ON DELETE RESTRICT,
    CONSTRAINT subscription_entitlement_periods_group_id_fkey
        FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE RESTRICT,
    CONSTRAINT subscription_entitlement_periods_days_check CHECK (period_days > 0),
    CONSTRAINT subscription_entitlement_periods_range_check CHECK (expires_at > starts_at),
    CONSTRAINT subscription_entitlement_periods_limit_check CHECK (daily_limit_usd IS NULL OR daily_limit_usd >= 0),
    CONSTRAINT subscription_entitlement_periods_status_check CHECK (status IN ('active', 'revoked'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_subscription_entitlement_periods_source
    ON subscription_entitlement_periods (source_type, source_id);

CREATE INDEX IF NOT EXISTS idx_subscription_entitlement_periods_active_user_expiry
    ON subscription_entitlement_periods (user_id, expires_at, starts_at DESC)
    WHERE status = 'active';
