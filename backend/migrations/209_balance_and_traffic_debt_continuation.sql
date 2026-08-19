-- 余额欠费不再暂停后续周额度；流量卡透支单独记账，充值时优先抵消。
CREATE TABLE IF NOT EXISTS traffic_credit_debt_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entry_type VARCHAR(16) NOT NULL,
    amount_usd NUMERIC(20,10) NOT NULL,
    balance_after_usd NUMERIC(20,10) NOT NULL,
    source_type VARCHAR(64) NOT NULL,
    source_ref VARCHAR(160) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT traffic_credit_debt_entry_type_check CHECK (entry_type IN ('debt', 'repayment')),
    CONSTRAINT traffic_credit_debt_amount_check CHECK (amount_usd > 0)
);

CREATE INDEX IF NOT EXISTS idx_traffic_credit_debt_user_created
    ON traffic_credit_debt_ledger(user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_traffic_credit_debt_source
    ON traffic_credit_debt_ledger(source_type, source_ref);

-- 流量卡是用户级通用额度池，保留 platform 字段仅兼容历史订单快照，扣费不再按渠道过滤。
UPDATE traffic_packs
SET
    name = REGEXP_REPLACE(name, '^GPT[[:space:]]+', ''),
    description = '购买后获得全渠道可用的流量卡额度；普通余额不足时自动切换扣费。',
    platform = 'all',
    updated_at = NOW()
WHERE platform <> 'all';

UPDATE user_traffic_credits
SET platform = 'all', updated_at = NOW()
WHERE platform <> 'all';

DROP INDEX IF EXISTS idx_user_traffic_credits_available;
CREATE INDEX IF NOT EXISTS idx_user_traffic_credits_available
    ON user_traffic_credits (user_id, expires_at, credited_at, id)
    WHERE remaining_usd > 0;

-- 旧版本已经暂停的未过期套餐恢复到正常刷新状态；欠费由余额账本继续记录并在刷新时抵消。
UPDATE user_balance_packages
SET status = 'active', updated_at = NOW()
WHERE status = 'debt_paused'
  AND credited_count < refresh_count
  AND expires_at > NOW()
  AND next_credit_at IS NOT NULL;
