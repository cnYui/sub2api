-- 保留无法从本地响应恢复逐笔 token 的计费失败请求，供后续外部账单核对。
CREATE TABLE IF NOT EXISTS billing_reconciliation_cases (
    id BIGSERIAL PRIMARY KEY,
    source_ops_log_id BIGINT NOT NULL UNIQUE,
    user_id BIGINT,
    api_key_id BIGINT,
    account_id BIGINT,
    model VARCHAR(128) NOT NULL DEFAULT '',
    failed_at TIMESTAMPTZ NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending_external_usage',
    amount_usd NUMERIC(20,10),
    error_code VARCHAR(64) NOT NULL DEFAULT 'INSUFFICIENT_BALANCE',
    metadata JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT billing_reconciliation_cases_status_check CHECK (status IN ('pending_external_usage', 'reconciled', 'waived'))
);

CREATE INDEX IF NOT EXISTS idx_billing_reconciliation_cases_user_failed
    ON billing_reconciliation_cases(user_id, failed_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_billing_reconciliation_cases_status
    ON billing_reconciliation_cases(status, failed_at DESC);
