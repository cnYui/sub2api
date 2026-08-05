-- 记录余额欠费和后续周额度还款，避免退款、续费或窗口刷新覆盖欠费事实。
CREATE TABLE IF NOT EXISTS balance_debt_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entry_type VARCHAR(16) NOT NULL,
    amount_usd NUMERIC(20,10) NOT NULL,
    balance_before_usd NUMERIC(20,10) NOT NULL,
    balance_after_usd NUMERIC(20,10) NOT NULL,
    source_type VARCHAR(64) NOT NULL,
    source_ref VARCHAR(160) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT balance_debt_ledger_entry_type_check CHECK (entry_type IN ('debt', 'repayment')),
    CONSTRAINT balance_debt_ledger_amount_check CHECK (amount_usd > 0)
);

CREATE INDEX IF NOT EXISTS idx_balance_debt_ledger_user_created
    ON balance_debt_ledger(user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_balance_debt_ledger_source
    ON balance_debt_ledger(source_type, source_ref);
