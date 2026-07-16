CREATE TABLE IF NOT EXISTS usage_facts (
    id BIGSERIAL PRIMARY KEY,
    request_id VARCHAR(255) NOT NULL,
    api_key_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    request_fingerprint VARCHAR(64) NOT NULL,
    payload_version INT NOT NULL DEFAULT 1,
    payload JSONB NOT NULL,
    billing_status VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempt_count INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error TEXT NOT NULL DEFAULT '',
    completed_at TIMESTAMPTZ NOT NULL,
    settled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT usage_facts_billing_status_check
        CHECK (billing_status IN ('pending', 'settling', 'settled', 'debt', 'failed')),
    CONSTRAINT usage_facts_attempt_count_check CHECK (attempt_count >= 0),
    CONSTRAINT usage_facts_payload_version_check CHECK (payload_version > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_usage_facts_request_api_key
    ON usage_facts (request_id, api_key_id);

CREATE INDEX IF NOT EXISTS idx_usage_facts_pending_claim
    ON usage_facts (next_attempt_at, id)
    WHERE billing_status IN ('pending', 'settling');
