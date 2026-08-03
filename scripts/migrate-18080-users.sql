-- 18080 -> 18082 用户数据迁移。
-- 通过临时外部表读取源库，所有写入位于同一事务中；commit=false 时只演练并回滚。

\set ON_ERROR_STOP on
\if :{?commit}
\else
\set commit false
\endif
\if :{?source_password}
\else
\echo source_password is required
\quit 2
\endif
\if :{?sample_percent}
\else
\set sample_percent 100
\endif

-- 抽样只用于验证字段和约束；正式迁移必须使用 100。
SELECT set_config('migration.sample_percent', :'sample_percent', false);

DO $$
BEGIN
  IF current_setting('migration.sample_percent')::INTEGER NOT BETWEEN 1 AND 100 THEN
    RAISE EXCEPTION 'sample_percent must be between 1 and 100';
  END IF;
END;
$$;

SET lock_timeout = '30s';
SET statement_timeout = '30min';
SET client_min_messages = warning;

CREATE EXTENSION IF NOT EXISTS postgres_fdw;
DROP SERVER IF EXISTS migration_18080 CASCADE;
DROP SCHEMA IF EXISTS migration_src CASCADE;
CREATE SCHEMA migration_src;
CREATE SERVER migration_18080
  FOREIGN DATA WRAPPER postgres_fdw
  OPTIONS (host 'sub2api-postgres-dev', port '5432', dbname 'sub2api');
CREATE USER MAPPING FOR CURRENT_USER
  SERVER migration_18080
  OPTIONS (user 'sub2api', password :'source_password');

IMPORT FOREIGN SCHEMA public
  LIMIT TO (
    users,
    auth_identities,
    api_keys,
    usage_logs,
    payment_orders,
    payment_audit_logs,
    subscription_plans,
    user_subscriptions,
    groups,
    user_platform_quotas,
    user_avatars,
    user_affiliates,
    user_affiliate_ledger,
    redeem_codes,
    usage_dashboard_daily_users,
    usage_dashboard_hourly_users
  )
  FROM SERVER migration_18080 INTO migration_src;

BEGIN;

-- 源库用户邮箱存在重复值时，只保留最早的可登录账号为活动账号，其余行仍保留并软删除。
CREATE TEMP TABLE migration_user_map (
  source_user_id BIGINT PRIMARY KEY,
  target_user_id BIGINT NOT NULL,
  source_email TEXT NOT NULL
) ON COMMIT DROP;

DO $$
DECLARE
  source_user RECORD;
  target_id BIGINT;
  target_role TEXT;
  duplicate_email BOOLEAN;
  inserted_deleted_at TIMESTAMPTZ;
BEGIN
  FOR source_user IN
    SELECT * FROM migration_src.users ORDER BY id
  LOOP
    SELECT u.id, u.role
      INTO target_id, target_role
      FROM users u
     WHERE lower(trim(u.email)) = lower(trim(source_user.email))
       AND u.deleted_at IS NULL
       AND NOT EXISTS (
         SELECT 1 FROM migration_user_map m WHERE m.target_user_id = u.id
       )
     ORDER BY u.id
     LIMIT 1;

    duplicate_email := EXISTS (
      SELECT 1 FROM users u
       WHERE lower(trim(u.email)) = lower(trim(source_user.email))
         AND u.deleted_at IS NULL
    );

    IF target_id IS NOT NULL THEN
      -- 目标已有同邮箱账号时源库为准；已有管理员身份需要保留，避免迁移中断管理入口。
      UPDATE users
         SET email = source_user.email,
             password_hash = source_user.password_hash,
             role = CASE WHEN target_role = 'admin' THEN 'admin' ELSE source_user.role END,
             balance = source_user.balance,
             frozen_balance = 0,
             concurrency = source_user.concurrency,
             status = source_user.status,
             username = source_user.username,
             notes = source_user.notes,
             totp_secret_encrypted = source_user.totp_secret_encrypted,
             totp_enabled = source_user.totp_enabled,
             totp_enabled_at = source_user.totp_enabled_at,
             signup_source = source_user.signup_source,
             last_login_at = source_user.last_login_at,
             last_active_at = source_user.last_active_at,
             balance_notify_enabled = source_user.balance_notify_enabled,
             balance_notify_threshold_type = source_user.balance_notify_threshold_type,
             balance_notify_threshold = source_user.balance_notify_threshold,
             balance_notify_extra_emails = source_user.balance_notify_extra_emails,
             total_recharged = source_user.total_recharged,
             rpm_limit = source_user.rpm_limit,
             created_at = source_user.created_at,
             updated_at = source_user.updated_at,
             deleted_at = source_user.deleted_at
       WHERE id = target_id;
    ELSE
      inserted_deleted_at := source_user.deleted_at;
      IF duplicate_email AND inserted_deleted_at IS NULL THEN
        inserted_deleted_at := COALESCE(source_user.updated_at, now());
      END IF;

      INSERT INTO users (
        email, password_hash, role, balance, frozen_balance, concurrency, status,
        username, notes, totp_secret_encrypted, totp_enabled, totp_enabled_at,
        signup_source, last_login_at, last_active_at, balance_notify_enabled,
        balance_notify_threshold_type, balance_notify_threshold, balance_notify_extra_emails,
        total_recharged, rpm_limit, created_at, updated_at, deleted_at
      ) VALUES (
        source_user.email, source_user.password_hash, source_user.role, source_user.balance, 0,
        source_user.concurrency, source_user.status, source_user.username, source_user.notes,
        source_user.totp_secret_encrypted, source_user.totp_enabled, source_user.totp_enabled_at,
        source_user.signup_source, source_user.last_login_at, source_user.last_active_at,
        source_user.balance_notify_enabled, source_user.balance_notify_threshold_type,
        source_user.balance_notify_threshold, source_user.balance_notify_extra_emails,
        source_user.total_recharged, source_user.rpm_limit, source_user.created_at,
        source_user.updated_at, inserted_deleted_at
      ) RETURNING id INTO target_id;
    END IF;

    INSERT INTO migration_user_map(source_user_id, target_user_id, source_email)
    VALUES (source_user.id, target_id, source_user.email);
  END LOOP;
END;
$$;

-- 迁移仅复制两端共有字段；指定字段在目标端需要新的语义映射。
CREATE OR REPLACE FUNCTION migration_copy_common(
  target_table TEXT,
  source_table TEXT,
  source_filter TEXT DEFAULT '',
  overrides JSONB DEFAULT '{}'::jsonb
) RETURNS BIGINT
LANGUAGE plpgsql AS $$
DECLARE
  target_columns TEXT;
  source_expressions TEXT;
  joins TEXT := '';
  sql TEXT;
  row_count BIGINT;
  column_record RECORD;
  source_has_user_id BOOLEAN;
  source_has_used_by BOOLEAN;
  source_has_inviter_id BOOLEAN;
  source_has_source_user_id BOOLEAN;
BEGIN
  SELECT EXISTS (
    SELECT 1 FROM information_schema.columns
     WHERE table_schema = 'migration_src' AND table_name = source_table AND column_name = 'user_id'
  ) INTO source_has_user_id;
  SELECT EXISTS (
    SELECT 1 FROM information_schema.columns
     WHERE table_schema = 'migration_src' AND table_name = source_table AND column_name = 'used_by'
  ) INTO source_has_used_by;
  SELECT EXISTS (
    SELECT 1 FROM information_schema.columns
     WHERE table_schema = 'migration_src' AND table_name = source_table AND column_name = 'inviter_id'
  ) INTO source_has_inviter_id;
  SELECT EXISTS (
    SELECT 1 FROM information_schema.columns
     WHERE table_schema = 'migration_src' AND table_name = source_table AND column_name = 'source_user_id'
  ) INTO source_has_source_user_id;

  IF source_has_user_id THEN
    joins := joins || ' LEFT JOIN migration_user_map m_user ON m_user.source_user_id = s.user_id';
  END IF;
  IF source_has_used_by THEN
    joins := joins || ' LEFT JOIN migration_user_map m_used_by ON m_used_by.source_user_id = s.used_by';
  END IF;
  IF source_has_inviter_id THEN
    joins := joins || ' LEFT JOIN migration_user_map m_inviter ON m_inviter.source_user_id = s.inviter_id';
  END IF;
  IF source_has_source_user_id THEN
    joins := joins || ' LEFT JOIN migration_user_map m_source_user ON m_source_user.source_user_id = s.source_user_id';
  END IF;

  target_columns := '';
  source_expressions := '';
  FOR column_record IN
    SELECT t.column_name,
           EXISTS (
             SELECT 1 FROM information_schema.columns sc
              WHERE sc.table_schema = 'migration_src'
                AND sc.table_name = source_table
                AND sc.column_name = t.column_name
           ) AS source_exists
      FROM information_schema.columns t
     WHERE t.table_schema = 'public'
       AND t.table_name = target_table
       AND (
         EXISTS (
           SELECT 1 FROM information_schema.columns sc
            WHERE sc.table_schema = 'migration_src'
              AND sc.table_name = source_table
              AND sc.column_name = t.column_name
         ) OR overrides ? t.column_name
       )
     ORDER BY t.ordinal_position
  LOOP
    IF target_columns <> '' THEN
      target_columns := target_columns || ', ';
      source_expressions := source_expressions || ', ';
    END IF;
    target_columns := target_columns || format('%I', column_record.column_name);

    IF overrides ? column_record.column_name THEN
      source_expressions := source_expressions || (overrides ->> column_record.column_name);
    ELSIF column_record.column_name = 'user_id' AND source_has_user_id THEN
      source_expressions := source_expressions || 'm_user.target_user_id';
    ELSIF column_record.column_name = 'used_by' AND source_has_used_by THEN
      source_expressions := source_expressions || 'm_used_by.target_user_id';
    ELSIF column_record.column_name = 'inviter_id' AND source_has_inviter_id THEN
      source_expressions := source_expressions || 'm_inviter.target_user_id';
    ELSIF column_record.column_name = 'source_user_id' AND source_has_source_user_id THEN
      source_expressions := source_expressions || 'm_source_user.target_user_id';
    ELSE
      source_expressions := source_expressions || format('s.%I', column_record.column_name);
    END IF;
  END LOOP;

  sql := format(
    'INSERT INTO public.%I (%s) SELECT %s FROM migration_src.%I s%s %s',
    target_table,
    target_columns,
    source_expressions,
    source_table,
    joins,
    CASE WHEN btrim(source_filter) = '' THEN '' ELSE ' WHERE ' || source_filter END
  );
  EXECUTE sql;
  GET DIAGNOSTICS row_count = ROW_COUNT;
  RETURN row_count;
END;
$$;

CREATE OR REPLACE FUNCTION migration_reset_sequence(target_table TEXT)
RETURNS VOID LANGUAGE plpgsql AS $$
DECLARE
  sequence_name TEXT;
  max_id BIGINT;
BEGIN
  sequence_name := pg_get_serial_sequence('public.' || target_table, 'id');
  IF sequence_name IS NULL THEN
    RETURN;
  END IF;
  EXECUTE format('SELECT max(id) FROM public.%I', target_table) INTO max_id;
  IF max_id IS NULL THEN
    PERFORM setval(sequence_name, 1, false);
  ELSE
    PERFORM setval(sequence_name, max_id, true);
  END IF;
END;
$$;

-- 目标是 18080 的用户数据，目标端现有测试用户关联记录先清除，备份可用于回滚。
TRUNCATE TABLE
  payment_audit_logs,
  user_balance_packages,
  payment_orders,
  usage_logs,
  api_keys,
  auth_identities,
  user_platform_quotas,
  user_avatars,
  user_affiliate_ledger,
  user_affiliates,
  redeem_codes,
  usage_dashboard_daily_users,
  usage_dashboard_hourly_users
RESTART IDENTITY CASCADE;

-- 订单只复制普通余额订单和旧 28 天订阅订单；traffic_pack 及其流量卡数据完全排除。
SELECT migration_copy_common(
  'payment_orders',
  'payment_orders',
  $$s.order_type <> 'traffic_pack' AND (s.id % 100) < current_setting('migration.sample_percent')::BIGINT$$,
  jsonb_build_object(
    'order_type', $$CASE WHEN s.order_type = 'subscription' THEN 'balance_subscription' ELSE s.order_type END$$,
    'plan_id', 'NULL',
    'subscription_group_id', 'NULL',
    'balance_package_plan_id', $$CASE WHEN s.order_type = 'subscription' THEN (SELECT b.id FROM balance_package_plans b WHERE b.price_cny = s.amount ORDER BY b.id LIMIT 1) ELSE NULL END$$,
    'balance_package_weekly_credit_usd', $$CASE WHEN s.order_type = 'subscription' THEN (SELECT b.weekly_credit_usd FROM balance_package_plans b WHERE b.price_cny = s.amount ORDER BY b.id LIMIT 1) ELSE NULL END$$,
    'balance_package_refresh_count', $$CASE WHEN s.order_type = 'subscription' THEN 4 ELSE NULL END$$,
    'balance_package_refresh_interval_days', $$CASE WHEN s.order_type = 'subscription' THEN 7 ELSE NULL END$$,
    'balance_package_validity_days', $$CASE WHEN s.order_type = 'subscription' THEN 28 ELSE NULL END$$
  )
);
SELECT migration_reset_sequence('payment_orders');

-- 购买订单必须有对应套餐快照；任一旧订阅价格无法映射时整体回滚。
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM payment_orders
     WHERE order_type = 'balance_subscription'
       AND (balance_package_plan_id IS NULL OR balance_package_weekly_credit_usd IS NULL)
  ) THEN
    RAISE EXCEPTION '存在无法映射到 18082 余额套餐的旧订阅订单';
  END IF;
END;
$$;

-- 保留非流量订单的完整支付审计链。
SELECT migration_copy_common(
  'payment_audit_logs',
  'payment_audit_logs',
  $$EXISTS (
       SELECT 1
         FROM migration_src.payment_orders po
        WHERE po.id::text = s.order_id
          AND po.order_type <> 'traffic_pack'
          AND (po.id % 100) < current_setting('migration.sample_percent')::BIGINT
     )$$
);
SELECT migration_reset_sequence('payment_audit_logs');

-- API Key 的分组属于 18080 的模型池，18082 已重建渠道分组，无法按 ID 复用，因此只保留 Key 本身并解除旧分组绑定。
SELECT migration_copy_common('api_keys', 'api_keys', '', jsonb_build_object('group_id', 'NULL'));
SELECT migration_reset_sequence('api_keys');

-- 使用记录保留原始计费字段；旧分组和旧订阅关联解除，账户 ID 仅在两端都存在时复用。
SELECT migration_copy_common(
  'usage_logs',
  'usage_logs',
  $$((s.id % 100) < current_setting('migration.sample_percent')::BIGINT)$$,
  jsonb_build_object(
    'group_id', 'NULL',
    'subscription_id', 'NULL',
    'account_id', $$CASE WHEN EXISTS (SELECT 1 FROM accounts a WHERE a.id = s.account_id) THEN s.account_id ELSE 1 END$$
  )
);
SELECT migration_reset_sequence('usage_logs');

SELECT migration_copy_common('auth_identities', 'auth_identities');
SELECT migration_reset_sequence('auth_identities');
SELECT migration_copy_common('user_platform_quotas', 'user_platform_quotas');
SELECT migration_reset_sequence('user_platform_quotas');
SELECT migration_copy_common('user_avatars', 'user_avatars');
SELECT migration_reset_sequence('user_avatars');
SELECT migration_copy_common('user_affiliates', 'user_affiliates');
SELECT migration_copy_common(
  'user_affiliate_ledger',
  'user_affiliate_ledger',
  '',
  jsonb_build_object(
    'source_order_id', $$CASE WHEN EXISTS (
      SELECT 1
        FROM migration_src.payment_orders po
       WHERE po.id = s.source_order_id
         AND po.order_type <> 'traffic_pack'
         AND (po.id % 100) < current_setting('migration.sample_percent')::BIGINT
    ) THEN s.source_order_id ELSE NULL END$$
  )
);
SELECT migration_reset_sequence('user_affiliate_ledger');
SELECT migration_copy_common('redeem_codes', 'redeem_codes', '', jsonb_build_object('group_id', 'NULL'));
SELECT migration_reset_sequence('redeem_codes');
SELECT migration_copy_common('usage_dashboard_daily_users', 'usage_dashboard_daily_users');
SELECT migration_copy_common('usage_dashboard_hourly_users', 'usage_dashboard_hourly_users');

-- 为每个付费订阅订单构建一段独立的 7 天刷新周期；续费订单从下一笔支付时间开始，避免重复发放额度。
CREATE TEMP TABLE migration_package_source (
  source_order_id BIGINT,
  target_order_id BIGINT,
  source_subscription_id BIGINT,
  target_user_id BIGINT NOT NULL,
  source_user_id BIGINT NOT NULL,
  weekly_credit_usd NUMERIC(20,8) NOT NULL,
  starts_at TIMESTAMPTZ NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  refresh_count INTEGER NOT NULL,
  credited_count INTEGER NOT NULL,
  status VARCHAR(20) NOT NULL,
  next_credit_at TIMESTAMPTZ,
  issued_credit_usd NUMERIC(20,8) NOT NULL DEFAULT 0,
  used_credit_usd NUMERIC(20,8) NOT NULL DEFAULT 0,
  remaining_credit_usd NUMERIC(20,8) NOT NULL DEFAULT 0,
  synthetic BOOLEAN NOT NULL DEFAULT FALSE
) ON COMMIT DROP;

INSERT INTO migration_package_source (
  source_order_id, target_order_id, source_subscription_id, target_user_id, source_user_id,
  weekly_credit_usd, starts_at, expires_at, refresh_count, credited_count, status, next_credit_at,
  issued_credit_usd, used_credit_usd, remaining_credit_usd
)
SELECT
  po.id,
  po.id,
  po.subscription_id,
  m.target_user_id,
  po.user_id,
  plan.weekly_credit_usd,
  start_at,
  end_at,
  refresh_count,
  credited_count,
  CASE
    WHEN po.status IN ('REFUNDED', 'PARTIALLY_REFUNDED') THEN 'refunded'
    WHEN end_at <= now() OR credited_count >= refresh_count THEN 'completed'
    ELSE 'active'
  END,
  CASE
    WHEN po.status IN ('REFUNDED', 'PARTIALLY_REFUNDED') OR end_at <= now() OR credited_count >= refresh_count THEN NULL
    ELSE start_at + make_interval(days => credited_count * 7)
  END,
  CASE WHEN po.status IN ('REFUNDED', 'PARTIALLY_REFUNDED') THEN 0 ELSE plan.weekly_credit_usd * credited_count END,
  used_credit_usd,
  CASE
    WHEN po.status IN ('REFUNDED', 'PARTIALLY_REFUNDED') OR end_at <= now() THEN 0
    ELSE GREATEST(plan.weekly_credit_usd * credited_count - used_credit_usd, 0)
  END
FROM (
  SELECT
    po.*,
    COALESCE(po.paid_at, po.created_at) AS start_at,
    COALESCE(
      LEAD(COALESCE(po.paid_at, po.created_at)) OVER (PARTITION BY COALESCE(po.subscription_id, po.id) ORDER BY COALESCE(po.paid_at, po.created_at), po.id),
      us.expires_at,
      COALESCE(po.paid_at, po.created_at) + interval '28 days'
    ) AS end_at
  FROM migration_src.payment_orders po
  LEFT JOIN migration_src.user_subscriptions us ON us.id = po.subscription_id
  WHERE po.order_type = 'subscription'
    AND (po.id % 100) < current_setting('migration.sample_percent')::BIGINT
    AND po.status IN ('COMPLETED', 'REFUNDED', 'PARTIALLY_REFUNDED', 'REFUND_FAILED')
) po
JOIN migration_user_map m ON m.source_user_id = po.user_id
JOIN LATERAL (
  SELECT b.id, b.weekly_credit_usd
  FROM balance_package_plans b
  WHERE b.price_cny = po.amount
  ORDER BY b.id
  LIMIT 1
) plan ON TRUE
CROSS JOIN LATERAL (
  SELECT GREATEST(1, CEIL(EXTRACT(EPOCH FROM (po.end_at - po.start_at)) / 604800.0)::INTEGER) AS refresh_count,
         LEAST(
           GREATEST(1, CEIL(EXTRACT(EPOCH FROM (po.end_at - po.start_at)) / 604800.0)::INTEGER),
           CASE WHEN now() < po.start_at THEN 0 ELSE FLOOR(EXTRACT(EPOCH FROM (LEAST(now(), po.end_at) - po.start_at)) / 604800.0)::INTEGER + 1 END
         ) AS credited_count,
         COALESCE((
           SELECT SUM(ul.actual_cost)
           FROM migration_src.usage_logs ul
           WHERE po.subscription_id IS NOT NULL
             AND ul.subscription_id = po.subscription_id
             AND ul.created_at >= po.start_at
             AND ul.created_at < po.end_at
         ), 0)::NUMERIC(20,8) AS used_credit_usd
) metrics;

-- 没有付费订单但仍在有效期内的人工订阅，转成迁移订单后继续按周到账；无限额旧分组不映射为余额套餐。
DO $$
DECLARE
  subscription_row RECORD;
  synthetic_order_id BIGINT;
  plan_row RECORD;
  manual_refresh_count INTEGER;
BEGIN
  FOR subscription_row IN
    SELECT us.*, g.weekly_limit_usd, m.target_user_id, u.email, u.username
    FROM migration_src.user_subscriptions us
    JOIN migration_src.groups g ON g.id = us.group_id
    JOIN migration_user_map m ON m.source_user_id = us.user_id
    JOIN users u ON u.id = m.target_user_id
    WHERE us.status = 'active'
      AND us.deleted_at IS NULL
      AND us.expires_at > now()
      AND g.weekly_limit_usd IS NOT NULL
      AND (us.id % 100) < current_setting('migration.sample_percent')::BIGINT
      AND NOT EXISTS (
        SELECT 1
        FROM migration_src.payment_orders po
        WHERE po.subscription_id = us.id
          AND po.order_type = 'subscription'
          AND po.status IN ('COMPLETED', 'REFUNDED', 'PARTIALLY_REFUNDED', 'REFUND_FAILED')
      )
    ORDER BY us.id
  LOOP
    synthetic_order_id := NULL;
    manual_refresh_count := GREATEST(1, CEIL(EXTRACT(EPOCH FROM (subscription_row.expires_at - subscription_row.starts_at)) / 604800.0)::INTEGER);

    INSERT INTO payment_orders (
      user_id, user_email, user_name, user_notes, amount, pay_amount, fee_rate, recharge_code,
      payment_type, payment_trade_no, order_type, status, refund_amount, force_refund,
      expires_at, paid_at, completed_at, client_ip, src_host, out_trade_no,
      balance_package_plan_id, balance_package_weekly_credit_usd,
      balance_package_refresh_count, balance_package_refresh_interval_days,
      balance_package_validity_days, created_at, updated_at
    )
    SELECT
      subscription_row.target_user_id, subscription_row.email, COALESCE(subscription_row.username, ''),
      '18080 legacy subscription migration', 0, 0, 0, 'migration_subscription_' || subscription_row.id,
      'migration', '', 'balance_subscription', 'COMPLETED', 0, false,
      subscription_row.expires_at, subscription_row.starts_at, subscription_row.starts_at, '', '18080',
      'migration_subscription_' || subscription_row.id,
      b.id, subscription_row.weekly_limit_usd, manual_refresh_count, 7,
      GREATEST(1, CEIL(EXTRACT(EPOCH FROM (subscription_row.expires_at - subscription_row.starts_at)) / 86400.0)::INTEGER),
      subscription_row.created_at, subscription_row.updated_at
    FROM balance_package_plans b
    WHERE b.weekly_credit_usd = subscription_row.weekly_limit_usd
    ORDER BY b.id
    LIMIT 1
    RETURNING id INTO synthetic_order_id;

    IF synthetic_order_id IS NULL THEN
      RAISE EXCEPTION '人工订阅 % 无法映射到余额套餐', subscription_row.id;
    END IF;

    INSERT INTO migration_package_source (
      source_order_id, target_order_id, source_subscription_id, target_user_id, source_user_id,
      weekly_credit_usd, starts_at, expires_at, refresh_count, credited_count, status, next_credit_at,
      issued_credit_usd, used_credit_usd, remaining_credit_usd, synthetic
    )
    SELECT
      NULL, synthetic_order_id, subscription_row.id, subscription_row.target_user_id, subscription_row.user_id,
      subscription_row.weekly_limit_usd, subscription_row.starts_at, subscription_row.expires_at,
      manual_refresh_count,
      LEAST(manual_refresh_count, CASE WHEN now() < subscription_row.starts_at THEN 0 ELSE FLOOR(EXTRACT(EPOCH FROM (LEAST(now(), subscription_row.expires_at) - subscription_row.starts_at)) / 604800.0)::INTEGER + 1 END),
      CASE WHEN subscription_row.expires_at <= now() THEN 'completed' ELSE 'active' END,
      CASE WHEN subscription_row.expires_at <= now() THEN NULL ELSE subscription_row.starts_at + make_interval(days => LEAST(manual_refresh_count, CASE WHEN now() < subscription_row.starts_at THEN 0 ELSE FLOOR(EXTRACT(EPOCH FROM (LEAST(now(), subscription_row.expires_at) - subscription_row.starts_at)) / 604800.0)::INTEGER + 1 END) * 7) END,
      subscription_row.weekly_limit_usd * LEAST(manual_refresh_count, CASE WHEN now() < subscription_row.starts_at THEN 0 ELSE FLOOR(EXTRACT(EPOCH FROM (LEAST(now(), subscription_row.expires_at) - subscription_row.starts_at)) / 604800.0)::INTEGER + 1 END),
      COALESCE((SELECT SUM(ul.actual_cost) FROM migration_src.usage_logs ul WHERE ul.subscription_id = subscription_row.id AND ul.created_at >= subscription_row.starts_at AND ul.created_at < subscription_row.expires_at), 0),
      CASE WHEN subscription_row.expires_at <= now() THEN 0 ELSE GREATEST(subscription_row.weekly_limit_usd * LEAST(manual_refresh_count, CASE WHEN now() < subscription_row.starts_at THEN 0 ELSE FLOOR(EXTRACT(EPOCH FROM (LEAST(now(), subscription_row.expires_at) - subscription_row.starts_at)) / 604800.0)::INTEGER + 1 END) - COALESCE((SELECT SUM(ul.actual_cost) FROM migration_src.usage_logs ul WHERE ul.subscription_id = subscription_row.id AND ul.created_at >= subscription_row.starts_at AND ul.created_at < subscription_row.expires_at), 0), 0) END,
      true;
  END LOOP;
END;
$$;
SELECT migration_reset_sequence('payment_orders');

INSERT INTO user_balance_packages (
  user_id, plan_id, payment_order_id, weekly_credit_usd, credited_count, refresh_count,
  refresh_interval_days, starts_at, next_credit_at, expires_at, status, created_at, updated_at
)
SELECT
  p.target_user_id,
  o.balance_package_plan_id,
  p.target_order_id,
  p.weekly_credit_usd,
  p.credited_count,
  p.refresh_count,
  7,
  p.starts_at,
  p.next_credit_at,
  p.expires_at,
  p.status,
  o.created_at,
  o.updated_at
FROM migration_package_source p
JOIN payment_orders o ON o.id = p.target_order_id;

-- 18080 的用户余额不含旧订阅池额度；只把当前仍可用的余额套餐剩余额度加入余额。
UPDATE users u
   SET balance = u.balance + COALESCE((SELECT SUM(p.remaining_credit_usd) FROM migration_package_source p WHERE p.target_user_id = u.id), 0),
       total_recharged = u.total_recharged + COALESCE((SELECT SUM(p.issued_credit_usd) FROM migration_package_source p WHERE p.target_user_id = u.id), 0);

INSERT INTO payment_audit_logs(order_id, action, detail, operator, created_at)
SELECT
  p.target_order_id::TEXT,
  'MIGRATION_BALANCE_PACKAGE',
  json_build_object(
    'source_subscription_id', p.source_subscription_id,
    'source_order_id', p.source_order_id,
    'weekly_credit_usd', p.weekly_credit_usd,
    'refresh_count', p.refresh_count,
    'credited_count', p.credited_count,
    'synthetic', p.synthetic
  )::TEXT,
  'system', now()
FROM migration_package_source p
WHERE p.synthetic;
SELECT migration_reset_sequence('payment_audit_logs');

-- 迁移核验：输出的只是数量和汇总，不输出密码哈希、API Key、头像内容或支付凭证。
SELECT 'source_users' AS metric, COUNT(*)::BIGINT AS value FROM migration_src.users
UNION ALL SELECT 'target_mapped_users', COUNT(*) FROM migration_user_map
UNION ALL SELECT 'target_users', COUNT(*) FROM users
UNION ALL SELECT 'source_non_traffic_orders', COUNT(*) FROM migration_src.payment_orders WHERE order_type <> 'traffic_pack' AND (id % 100) < current_setting('migration.sample_percent')::BIGINT
UNION ALL SELECT 'target_orders', COUNT(*) FROM payment_orders
UNION ALL SELECT 'source_non_traffic_audits', COUNT(*) FROM migration_src.payment_audit_logs pal WHERE EXISTS (SELECT 1 FROM migration_src.payment_orders po WHERE po.id::text = pal.order_id AND po.order_type <> 'traffic_pack' AND (po.id % 100) < current_setting('migration.sample_percent')::BIGINT)
UNION ALL SELECT 'target_audits', COUNT(*) FROM payment_audit_logs
UNION ALL SELECT 'source_usage_logs', COUNT(*) FROM migration_src.usage_logs WHERE (id % 100) < current_setting('migration.sample_percent')::BIGINT
UNION ALL SELECT 'target_usage_logs', COUNT(*) FROM usage_logs
UNION ALL SELECT 'source_api_keys', COUNT(*) FROM migration_src.api_keys
UNION ALL SELECT 'target_api_keys', COUNT(*) FROM api_keys
UNION ALL SELECT 'target_balance_packages', COUNT(*) FROM user_balance_packages
UNION ALL SELECT 'target_synthetic_packages', COUNT(*) FROM migration_package_source WHERE synthetic
ORDER BY metric;

DROP FUNCTION migration_copy_common(TEXT, TEXT, TEXT, JSONB);
DROP FUNCTION migration_reset_sequence(TEXT);
DROP SERVER migration_18080 CASCADE;
DROP SCHEMA migration_src CASCADE;

\if :commit
COMMIT;
\else
ROLLBACK;
\echo dry-run complete; all changes rolled back
\endif
