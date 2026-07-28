-- 仅允许在隔离的候选数据库克隆中执行，禁止用于公网数据库。
DO $$
BEGIN
  IF to_regclass('public.settings') IS NOT NULL THEN
    INSERT INTO settings (key, value, updated_at)
    VALUES
      ('payment_enabled', 'false', NOW()),
      ('payment_visible_method_alipay_enabled', 'false', NOW()),
      ('payment_visible_method_wxpay_enabled', 'false', NOW()),
      ('ENABLED_PAYMENT_TYPES', '', NOW()),
      ('smtp_host', '', NOW()),
      ('smtp_username', '', NOW()),
      ('smtp_password', '', NOW()),
      ('ops_monitoring_enabled', 'false', NOW()),
      ('ops_realtime_monitoring_enabled', 'false', NOW()),
      ('channel_monitor_enabled', 'false', NOW()),
      ('available_channels_enabled', 'false', NOW()),
      ('subscription_expiry_notify_enabled', 'false', NOW()),
      ('balance_low_notify_enabled', 'false', NOW()),
      ('account_quota_notify_enabled', 'false', NOW())
    ON CONFLICT (key) DO UPDATE
      SET value = EXCLUDED.value,
          updated_at = NOW();
  END IF;

  IF to_regclass('public.payment_provider_instances') IS NOT NULL THEN
    EXECUTE 'UPDATE payment_provider_instances SET enabled = false, refund_enabled = false, allow_user_refund = false, updated_at = NOW()';
  END IF;

  IF to_regclass('public.channel_monitors') IS NOT NULL THEN
    EXECUTE 'UPDATE channel_monitors SET enabled = false';
  END IF;

  IF to_regclass('public.ops_alert_rules') IS NOT NULL THEN
    EXECUTE 'UPDATE ops_alert_rules SET enabled = false';
  END IF;
END $$;
