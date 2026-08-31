-- 余额套餐实际消费账本：只记录真正从余额套餐扣除的金额。
CREATE TABLE IF NOT EXISTS balance_package_usage_ledger (
    id BIGSERIAL PRIMARY KEY,
    balance_package_id BIGINT NOT NULL REFERENCES user_balance_packages(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    api_key_id BIGINT NOT NULL DEFAULT 0,
    request_id VARCHAR(128) NOT NULL,
    amount_usd DECIMAL(20,10) NOT NULL CHECK (amount_usd >= 0),
    source_type VARCHAR(32) NOT NULL DEFAULT 'request',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT balance_package_usage_ledger_request_uniq
        UNIQUE (balance_package_id, request_id, api_key_id)
);

CREATE INDEX IF NOT EXISTS idx_balance_package_usage_ledger_package_time
    ON balance_package_usage_ledger (balance_package_id, created_at);

-- 旧套餐没有资金来源归因，写入零金额标记并在退款时转人工审核，禁止误判为零用量。
INSERT INTO balance_package_usage_ledger
    (balance_package_id, user_id, api_key_id, request_id, amount_usd, source_type, created_at)
SELECT id, user_id, 0, 'legacy_unattributed:' || id, 0, 'legacy_unattributed', NOW()
FROM user_balance_packages
WHERE NOT EXISTS (
    SELECT 1 FROM balance_package_usage_ledger l
    WHERE l.balance_package_id = user_balance_packages.id
      AND l.source_type = 'legacy_unattributed'
);
