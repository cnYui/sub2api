-- 199_normalize_balance_package_lifecycle.sql
-- 迁移历史订阅时曾按结束时间差向上取整，28 天套餐被错误写成 5 次或更长周期。
-- 用户套餐的标准周期固定为 28 天、每 7 天到账一次、共 4 次；多条有效记录只保留最新未退款记录。

UPDATE user_balance_packages
SET refresh_count = 4,
    refresh_interval_days = 7,
    expires_at = starts_at + INTERVAL '28 days',
    credited_count = LEAST(GREATEST(credited_count, 0), 4),
    updated_at = NOW()
WHERE refresh_count <> 4
   OR refresh_interval_days <> 7
   OR expires_at <> starts_at + INTERVAL '28 days'
   OR credited_count < 0
   OR credited_count > 4;

WITH ranked_packages AS (
    SELECT
        id,
        ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at DESC, id DESC) AS row_number
    FROM user_balance_packages
    WHERE status <> 'refunded'
)
UPDATE user_balance_packages AS package
SET status = 'expired',
    next_credit_at = NULL,
    updated_at = NOW()
FROM ranked_packages AS ranked
WHERE package.id = ranked.id
  AND ranked.row_number > 1
  AND package.status <> 'expired';

UPDATE user_balance_packages
SET status = CASE
        WHEN status = 'refunded' THEN 'refunded'
        WHEN status = 'expired' OR expires_at <= NOW() THEN 'expired'
        WHEN credited_count >= 4 THEN 'completed'
        ELSE 'active'
    END,
    next_credit_at = CASE
        WHEN status IN ('refunded', 'expired')
          OR expires_at <= NOW()
          OR credited_count >= 4
          THEN NULL
        ELSE starts_at + make_interval(days => credited_count * 7)
    END,
    updated_at = NOW();
