WITH candidates AS (
    SELECT ubp.id
    FROM user_balance_packages AS ubp
    JOIN users AS u ON u.id = ubp.user_id
    WHERE ubp.status = 'active'
      AND u.deleted_at IS NULL
      AND u.balance < 0
      AND ubp.credited_count < ubp.refresh_count
      AND ubp.expires_at > NOW()
), paused AS (
    UPDATE user_balance_packages AS ubp
    SET status = 'debt_paused', updated_at = NOW()
    FROM candidates AS c
    WHERE ubp.id = c.id
    RETURNING ubp.id, ubp.payment_order_id, ubp.user_id, ubp.credited_count,
              ubp.weekly_credit_usd, ubp.next_credit_at
)
INSERT INTO payment_audit_logs (order_id, action, detail, operator, created_at)
SELECT paused.payment_order_id::text,
       'BALANCE_PACKAGE_DEBT_PAUSED_MIGRATION_' || paused.id::text,
       jsonb_build_object(
           'package_id', paused.id,
           'user_id', paused.user_id,
           'credited_count', paused.credited_count,
           'weekly_credit_usd', paused.weekly_credit_usd,
           'planned_next_credit_at', paused.next_credit_at,
           'reason', 'negative_balance_at_deployment'
       )::text,
       'migration:204',
       NOW()
FROM paused
ON CONFLICT (order_id, action) DO NOTHING;
