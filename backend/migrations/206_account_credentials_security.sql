-- 凭证指纹只用于并发控制和同 Key 分组，表达式索引避免为随机密文查询全表扫描。
CREATE INDEX IF NOT EXISTS idx_accounts_active_credential_fingerprint
    ON accounts ((credentials ->> '_credential_fingerprint'))
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_accounts_active_api_key_fingerprint
    ON accounts ((credentials ->> '_api_key_fingerprint'))
    WHERE deleted_at IS NULL;
