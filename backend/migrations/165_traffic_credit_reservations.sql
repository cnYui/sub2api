ALTER TABLE user_traffic_credits
    ADD COLUMN IF NOT EXISTS reserved_usd DECIMAL(20,10) NOT NULL DEFAULT 0;

DO $$
BEGIN
    ALTER TABLE user_traffic_credits
        ADD CONSTRAINT user_traffic_credits_reserved_nonnegative CHECK (reserved_usd >= 0);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE user_traffic_credits
        ADD CONSTRAINT user_traffic_credits_reserved_within_remaining CHECK (reserved_usd <= remaining_usd);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS traffic_credit_reservations (
    id BIGSERIAL PRIMARY KEY,
    request_id VARCHAR(255) NOT NULL,
    api_key_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    platform VARCHAR(30) NOT NULL,
    model VARCHAR(255) NOT NULL,
    request_fingerprint VARCHAR(64) NOT NULL,
    pricing_snapshot JSONB NOT NULL,
    reserved_usd DECIMAL(20,10) NOT NULL,
    settled_usd DECIMAL(20,10) NOT NULL DEFAULT 0,
    debt_usd DECIMAL(20,10) NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'reserved',
    last_error TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT traffic_credit_reservations_status_check
        CHECK (status IN ('reserved', 'dispatched', 'unknown', 'settled', 'released', 'debt')),
    CONSTRAINT traffic_credit_reservations_amount_check
        CHECK (reserved_usd > 0 AND settled_usd >= 0 AND debt_usd >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_traffic_credit_reservations_request_api_key
    ON traffic_credit_reservations (request_id, api_key_id);

CREATE INDEX IF NOT EXISTS idx_traffic_credit_reservations_user_debt
    ON traffic_credit_reservations (user_id, platform, id)
    WHERE status = 'debt' AND debt_usd > 0;

CREATE TABLE IF NOT EXISTS traffic_credit_reservation_items (
    reservation_id BIGINT NOT NULL REFERENCES traffic_credit_reservations(id) ON DELETE RESTRICT,
    credit_id BIGINT NOT NULL REFERENCES user_traffic_credits(id) ON DELETE RESTRICT,
    reserved_usd DECIMAL(20,10) NOT NULL,
    settled_usd DECIMAL(20,10) NOT NULL DEFAULT 0,
    PRIMARY KEY (reservation_id, credit_id),
    CONSTRAINT traffic_credit_reservation_items_amount_check
        CHECK (reserved_usd > 0 AND settled_usd >= 0 AND settled_usd <= reserved_usd)
);

ALTER TABLE usage_facts
    ADD COLUMN IF NOT EXISTS reservation_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_usage_facts_reservation_id
    ON usage_facts (reservation_id)
    WHERE reservation_id IS NOT NULL;
