ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS image_input_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS image_input_cost DECIMAL(20, 10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS billing_incomplete BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS traffic_credit_exhaustion_events (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credit_id BIGINT NOT NULL REFERENCES user_traffic_credits(id) ON DELETE RESTRICT,
    request_id VARCHAR(255) NOT NULL,
    batch_key VARCHAR(255) NOT NULL,
    reason VARCHAR(32) NOT NULL DEFAULT 'depleted',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    acknowledged_at TIMESTAMPTZ NULL,
    CONSTRAINT traffic_credit_exhaustion_events_reason_check CHECK (reason = 'depleted'),
    CONSTRAINT traffic_credit_exhaustion_events_user_credit_unique UNIQUE (user_id, credit_id)
);

CREATE INDEX IF NOT EXISTS idx_traffic_credit_exhaustion_events_pending
    ON traffic_credit_exhaustion_events (user_id, created_at, id)
    WHERE acknowledged_at IS NULL;

ALTER TABLE groups
    DROP COLUMN IF EXISTS image_rate_independent,
    DROP COLUMN IF EXISTS image_rate_multiplier,
    DROP COLUMN IF EXISTS image_price_1k,
    DROP COLUMN IF EXISTS image_price_2k,
    DROP COLUMN IF EXISTS image_price_4k;
