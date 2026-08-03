-- 邀请返利改为自动进入邀请人余额：余额可参与模型扣费，返利台账继续记录冻结截止时间。
-- 迁移只归集旧版尚未转入余额的返利，使用 settings 标记保证重复执行不会重复入账。
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM settings WHERE key = 'affiliate_auto_balance_migrated_v1'
    ) THEN
        UPDATE users u
        SET balance = u.balance + legacy.amount,
            total_recharged = COALESCE(u.total_recharged, 0) + legacy.amount,
            updated_at = NOW()
        FROM (
            SELECT user_id,
                   SUM(COALESCE(aff_quota, 0) + COALESCE(aff_frozen_quota, 0)) AS amount
            FROM user_affiliates
            GROUP BY user_id
        ) legacy
        WHERE u.id = legacy.user_id
          AND legacy.amount > 0;

        -- aff_frozen_quota 仍保留为冻结状态快照；aff_quota 已经随着迁移自动入账。
        UPDATE user_affiliates
        SET aff_quota = 0,
            updated_at = NOW()
        WHERE aff_quota <> 0;

        INSERT INTO settings (key, value, updated_at)
        VALUES ('affiliate_auto_balance_migrated_v1', 'true', NOW());
    END IF;
END $$;

-- 当前实例按管理员要求开启邀请返利，并采用 8% / 24 小时策略。
INSERT INTO settings (key, value, updated_at)
VALUES
    ('affiliate_enabled', 'true', NOW()),
    ('affiliate_rebate_rate', '8', NOW()),
    ('affiliate_rebate_freeze_hours', '24', NOW())
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value,
    updated_at = EXCLUDED.updated_at;
