-- 余额套餐只保留当前 7 天窗口的未用额度；刷新时替换，不得累加到下一周。
ALTER TABLE user_balance_packages
    ADD COLUMN IF NOT EXISTS remaining_usd DECIMAL(20,8) NOT NULL DEFAULT 0;

-- 旧版本的刷新直接把一整周额度追加到 users.balance。对每条旧审计记录，
-- 只回收刷新前窗口遗留的套餐额度，不触碰普通充值、返利或迁移后的其他余额。
WITH legacy_refreshes AS (
    SELECT
        p.id AS package_id,
        p.user_id,
        p.weekly_credit_usd,
        p.starts_at,
        (pal.detail::JSONB ->> 'credited_count')::INTEGER AS refresh_number
    FROM user_balance_packages AS p
    JOIN payment_audit_logs AS pal
      ON pal.order_id = p.payment_order_id::TEXT
     AND pal.action = 'BALANCE_PACKAGE_WEEKLY_CREDIT'
    WHERE (pal.detail::JSONB ->> 'credited_count') ~ '^[0-9]+$'
), legacy_carryovers AS (
    SELECT
        refresh.user_id,
        SUM(GREATEST(
            refresh.weekly_credit_usd - COALESCE((
                SELECT SUM(usage.actual_cost)
                FROM usage_logs AS usage
                WHERE usage.user_id = refresh.user_id
                  AND usage.created_at >= refresh.starts_at
                      + make_interval(days => (refresh.refresh_number - 2) * 7)
                  AND usage.created_at < refresh.starts_at
                      + make_interval(days => (refresh.refresh_number - 1) * 7)
            ), 0),
            0
        )) AS amount
    FROM legacy_refreshes AS refresh
    WHERE refresh.refresh_number >= 2
    GROUP BY refresh.user_id
)
UPDATE users AS account
SET balance = account.balance - carryover.amount,
    updated_at = NOW()
FROM legacy_carryovers AS carryover
WHERE account.id = carryover.user_id
  AND carryover.amount > 0;

-- 迁移时 users.balance 已包含旧系统当前周的剩余额度。根据当前 7 天窗口的
-- 实际用量重建套餐剩额，再以修正后的总余额作上限，避免把普通余额标记为套餐额度。
WITH package_windows AS (
    SELECT
        p.id,
        p.user_id,
        p.weekly_credit_usd,
        p.starts_at,
        p.refresh_interval_days,
        p.expires_at,
        p.status,
        CASE
            WHEN p.status IN ('active', 'completed')
             AND p.expires_at > NOW()
             AND p.starts_at <= NOW()
             AND p.refresh_interval_days > 0
                THEN p.starts_at + make_interval(
                    days => FLOOR(EXTRACT(EPOCH FROM (NOW() - p.starts_at))
                        / (p.refresh_interval_days * 86400.0))::INTEGER * p.refresh_interval_days
                )
            ELSE NULL
        END AS current_window_start
    FROM user_balance_packages AS p
), calculated_remaining AS (
    SELECT
    package_window.id,
    CASE
            WHEN package_window.current_window_start IS NULL THEN 0::DECIMAL(20,8)
            ELSE LEAST(
                package_window.weekly_credit_usd,
                GREATEST(
                    package_window.weekly_credit_usd - COALESCE((
                        SELECT SUM(usage.actual_cost)
                        FROM usage_logs AS usage
                        WHERE usage.user_id = package_window.user_id
                          AND usage.created_at >= package_window.current_window_start
                          AND usage.created_at < LEAST(
                              package_window.expires_at,
                              package_window.current_window_start
                                + make_interval(days => package_window.refresh_interval_days)
                          )
                    ), 0),
                    0
                )
            )
        END AS usage_derived_remaining
    FROM package_windows AS package_window
)
UPDATE user_balance_packages AS package
SET remaining_usd = LEAST(
        calculated.usage_derived_remaining,
        GREATEST(account.balance, 0)
    ),
    updated_at = NOW()
FROM calculated_remaining AS calculated,
     users AS account
WHERE package.id = calculated.id
  AND account.id = package.user_id;

ALTER TABLE user_balance_packages
    DROP CONSTRAINT IF EXISTS user_balance_packages_remaining_usd_check;
ALTER TABLE user_balance_packages
    ADD CONSTRAINT user_balance_packages_remaining_usd_check
    CHECK (remaining_usd >= 0 AND remaining_usd <= weekly_credit_usd);
