-- 201_normalize_balance_package_plan_snapshots.sql
-- 订单快照必须与套餐计划保持一致，避免重试履约时使用过期的周期配置。

UPDATE payment_orders AS order_row
SET balance_package_refresh_count = plan.refresh_count,
    balance_package_refresh_interval_days = plan.refresh_interval_days,
    balance_package_validity_days = plan.validity_days,
    updated_at = NOW()
FROM balance_package_plans AS plan
WHERE order_row.order_type = 'balance_subscription'
  AND order_row.balance_package_plan_id = plan.id
  AND (
      order_row.balance_package_refresh_count IS DISTINCT FROM plan.refresh_count
      OR order_row.balance_package_refresh_interval_days IS DISTINCT FROM plan.refresh_interval_days
      OR order_row.balance_package_validity_days IS DISTINCT FROM plan.validity_days
  );

UPDATE user_balance_packages AS package
SET refresh_count = plan.refresh_count,
    refresh_interval_days = plan.refresh_interval_days,
    expires_at = package.starts_at + make_interval(days => plan.validity_days),
    credited_count = LEAST(GREATEST(package.credited_count, 0), plan.refresh_count),
    updated_at = NOW()
FROM balance_package_plans AS plan
WHERE package.plan_id = plan.id
  AND (
      package.refresh_count IS DISTINCT FROM plan.refresh_count
      OR package.refresh_interval_days IS DISTINCT FROM plan.refresh_interval_days
      OR package.expires_at IS DISTINCT FROM package.starts_at + make_interval(days => plan.validity_days)
      OR package.credited_count < 0
      OR package.credited_count > plan.refresh_count
  );
