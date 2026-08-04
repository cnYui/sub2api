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
    accounts,
    auth_identities,
    api_keys,
    usage_logs,
    payment_orders,
    payment_audit_logs,
    traffic_packs,
    user_traffic_credits,
    traffic_credit_ledger,
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
       AND (
         (source_user.deleted_at IS NULL AND u.deleted_at IS NULL)
         OR (source_user.deleted_at IS NOT NULL AND u.deleted_at IS NOT NULL)
       )
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
             -- 18080 的 users.balance 是旧钱包语义，18082 不继承该值。
             balance = 0,
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
        source_user.email, source_user.password_hash, source_user.role, 0, 0,
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

CREATE OR REPLACE FUNCTION migration_sample_selected(source_id BIGINT)
RETURNS BOOLEAN
LANGUAGE SQL
STABLE AS $$
  SELECT (source_id % 100) < current_setting('migration.sample_percent')::BIGINT;
$$;

CREATE OR REPLACE FUNCTION migration_source_user_is_admin(source_user_id BIGINT)
RETURNS BOOLEAN
LANGUAGE SQL
STABLE AS $$
  SELECT EXISTS (
    SELECT 1
      FROM migration_src.users
     WHERE id = source_user_id
       AND lower(trim(email)) = 'xiaobianfuai@gmail.com'
       AND role = 'admin'
       AND deleted_at IS NULL
  );
$$;

-- 18080 的新版本保留了额度预留字段；目标旧表没有时补齐，避免迁移中丢失已预留额度。
ALTER TABLE user_traffic_credits
  ADD COLUMN IF NOT EXISTS reserved_usd DECIMAL(20, 10) NOT NULL DEFAULT 0;

-- 目标库可能已清空业务数据；迁移前恢复 18082 已确认的目录基线，但不覆盖现有配置。
INSERT INTO balance_package_plans
  (code, name, price_cny, weekly_credit_usd, validity_days, refresh_count, refresh_interval_days, for_sale, sort_order)
VALUES
  ('balance-29',  '余额套餐 ¥29',  29,  76, 28, 4, 7, TRUE, 10),
  ('balance-39',  '余额套餐 ¥39',  39, 102, 28, 4, 7, TRUE, 20),
  ('balance-49',  '余额套餐 ¥49',  49, 128, 28, 4, 7, TRUE, 30),
  ('balance-59',  '余额套餐 ¥59',  59, 154, 28, 4, 7, TRUE, 40),
  ('balance-79',  '余额套餐 ¥79',  79, 206, 28, 4, 7, TRUE, 50),
  ('balance-99',  '余额套餐 ¥99',  99, 258, 28, 4, 7, TRUE, 60),
  ('balance-149', '余额套餐 ¥149', 149, 389, 28, 4, 7, TRUE, 70),
  ('balance-199', '余额套餐 ¥199', 199, 520, 28, 4, 7, TRUE, 80),
  ('balance-249', '余额套餐 ¥249', 249, 651, 28, 4, 7, TRUE, 90),
  ('balance-299', '余额套餐 ¥299', 299, 781, 28, 4, 7, TRUE, 100)
ON CONFLICT (code) DO NOTHING;

INSERT INTO traffic_packs
  (code, name, description, price, credit_usd, validity_days, platform, for_sale, sort_order)
VALUES
  ('gpt_traffic_5usd_2cny', 'GPT 流量包 5 刀', '2 元购买 5 USD GPT 额度，可用于 GPT 写代码和生图。', 2, 5, 28, 'openai', TRUE, 10),
  ('gpt_traffic_10usd_3cny', 'GPT 流量包 10 刀', '3 元购买 10 USD GPT 额度，可用于 GPT 写代码和生图。', 3, 10, 28, 'openai', TRUE, 20),
  ('gpt_traffic_20usd_5cny', 'GPT 流量包 20 刀', '5 元购买 20 USD GPT 额度，可用于 GPT 写代码和生图。', 5, 20, 28, 'openai', TRUE, 30)
ON CONFLICT (code) DO NOTHING;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
      FROM migration_src.users
     WHERE lower(trim(email)) = 'xiaobianfuai@gmail.com'
       AND role = 'admin'
       AND deleted_at IS NULL
  ) THEN
    RAISE EXCEPTION '源库缺少管理员账号 xiaobianfuai@gmail.com';
  END IF;

  IF EXISTS (
    SELECT 1
      FROM migration_src.traffic_packs source_pack
     WHERE NOT EXISTS (
       SELECT 1 FROM traffic_packs target_pack WHERE target_pack.code = source_pack.code
     )
  ) THEN
    RAISE EXCEPTION '目标库缺少源流量包 code，无法安全映射';
  END IF;
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
  traffic_credit_ledger,
  user_traffic_credits,
  accounts,
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

-- 复用已有软删除账号后，清理此前抽样迁移留下的同邮箱软删除副本，保证用户数与源库一一对应。
DELETE FROM users u
 WHERE u.deleted_at IS NOT NULL
   AND NOT EXISTS (
     SELECT 1 FROM migration_user_map m WHERE m.target_user_id = u.id
   )
   AND EXISTS (
     SELECT 1
       FROM migration_src.users source_user
      WHERE source_user.deleted_at IS NOT NULL
        AND lower(trim(source_user.email)) = lower(trim(u.email))
   );

-- 目标套餐目录由 18082 自身维护；历史额度按 source traffic pack code 映射，保留目标 28 天配置。

-- 普通订单抽样；流量订单同时纳入抽样额度所依赖的订单，并强制纳入管理员流量卡关联订单。
SELECT migration_copy_common(
  'payment_orders',
  'payment_orders',
  $$
    (
      s.order_type <> 'traffic_pack'
      AND migration_sample_selected(s.id)
    )
    OR (
      s.order_type = 'traffic_pack'
      AND (
        migration_sample_selected(s.id)
        OR migration_source_user_is_admin(s.user_id)
        OR EXISTS (
          SELECT 1
            FROM migration_src.user_traffic_credits c
           WHERE c.order_id = s.id
             AND (
               migration_sample_selected(c.id)
               OR migration_source_user_is_admin(c.user_id)
             )
        )
      )
    )
  $$,
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

-- 支付审计与已抽样的普通/流量订单保持一致。
SELECT migration_copy_common(
  'payment_audit_logs',
  'payment_audit_logs',
  $$EXISTS (
       SELECT 1
         FROM migration_src.payment_orders po
        WHERE po.id::text = s.order_id
          AND (
            (
              po.order_type <> 'traffic_pack'
              AND migration_sample_selected(po.id)
            )
            OR (
              po.order_type = 'traffic_pack'
              AND (
                migration_sample_selected(po.id)
                OR migration_source_user_is_admin(po.user_id)
                OR EXISTS (
                  SELECT 1
                    FROM migration_src.user_traffic_credits c
                   WHERE c.order_id = po.id
                     AND (
                       migration_sample_selected(c.id)
                       OR migration_source_user_is_admin(c.user_id)
                     )
                )
              )
            )
          )
     )$$
);
SELECT migration_reset_sequence('payment_audit_logs');

-- API Key 的分组属于 18080 的模型池，18082 已重建渠道分组，无法按 ID 复用，因此只保留 Key 本身并解除旧分组绑定。
SELECT migration_copy_common('api_keys', 'api_keys', '', jsonb_build_object('group_id', 'NULL'));
SELECT migration_reset_sequence('api_keys');

-- 使用记录依赖账号 ID；账号没有用户外键，完整复制并保留源 ID。
SELECT migration_copy_common('accounts', 'accounts');
SELECT migration_reset_sequence('accounts');

-- 使用记录保留原始计费字段；旧分组和旧订阅关联解除，账户 ID 仅在两端都存在时复用。
SELECT migration_copy_common(
  'usage_logs',
  'usage_logs',
  $$((s.id % 100) < current_setting('migration.sample_percent')::BIGINT)$$,
  jsonb_build_object(
    'group_id', 'NULL',
    'subscription_id', 'NULL'
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
         AND (
           (
             po.order_type <> 'traffic_pack'
             AND migration_sample_selected(po.id)
           )
           OR (
             po.order_type = 'traffic_pack'
             AND (
               migration_sample_selected(po.id)
               OR migration_source_user_is_admin(po.user_id)
               OR EXISTS (
                 SELECT 1
                   FROM migration_src.user_traffic_credits c
                  WHERE c.order_id = po.id
                    AND (
                      migration_sample_selected(c.id)
                      OR migration_source_user_is_admin(c.user_id)
                    )
               )
             )
           )
         )
    ) THEN s.source_order_id ELSE NULL END$$
  )
);
SELECT migration_reset_sequence('user_affiliate_ledger');
SELECT migration_copy_common('redeem_codes', 'redeem_codes', '', jsonb_build_object('group_id', 'NULL'));
SELECT migration_reset_sequence('redeem_codes');
SELECT migration_copy_common('usage_dashboard_daily_users', 'usage_dashboard_daily_users');
SELECT migration_copy_common('usage_dashboard_hourly_users', 'usage_dashboard_hourly_users');

-- 流量卡额度保留源库的实际剩余值、预留值和过期时间；pack_id 按 code 映射到目标目录。
SELECT migration_copy_common(
  'user_traffic_credits',
  'user_traffic_credits',
  $$migration_sample_selected(s.id) OR migration_source_user_is_admin(s.user_id)$$,
  jsonb_build_object(
    'pack_id', $$CASE WHEN s.pack_id IS NULL THEN NULL ELSE (
      SELECT target_pack.id
        FROM migration_src.traffic_packs source_pack
        JOIN traffic_packs target_pack ON target_pack.code = source_pack.code
       WHERE source_pack.id = s.pack_id
    ) END$$
  )
);
SELECT migration_reset_sequence('user_traffic_credits');

-- 流水跟随抽样额度、管理员额度或抽样订单，外键在目标库中重新绑定。
SELECT migration_copy_common(
  'traffic_credit_ledger',
  'traffic_credit_ledger',
  $$
    migration_sample_selected(s.id)
    OR migration_source_user_is_admin(s.user_id)
    OR EXISTS (
      SELECT 1
        FROM migration_src.user_traffic_credits c
       WHERE c.id = s.credit_id
         AND (
           migration_sample_selected(c.id)
           OR migration_source_user_is_admin(c.user_id)
         )
    )
    OR EXISTS (
      SELECT 1
        FROM migration_src.payment_orders po
       WHERE po.id = s.order_id
         AND (
           migration_sample_selected(po.id)
           OR migration_source_user_is_admin(po.user_id)
         )
    )
  $$,
  jsonb_build_object(
    'credit_id', $$CASE WHEN EXISTS (SELECT 1 FROM user_traffic_credits c WHERE c.id = s.credit_id) THEN s.credit_id ELSE NULL END$$,
    'order_id', $$CASE WHEN EXISTS (SELECT 1 FROM payment_orders o WHERE o.id = s.order_id) THEN s.order_id ELSE NULL END$$
  )
);
SELECT migration_reset_sequence('traffic_credit_ledger');

-- 以源库当前有效日/周窗口重算剩余额度，避免直接使用停服前可能尚未刷新的缓存字段。
CREATE TEMP TABLE migration_current_quota ON COMMIT DROP AS
WITH active_subscriptions AS (
  SELECT
    us.*,
    g.daily_limit_usd,
    g.weekly_limit_usd,
    m.target_user_id,
    CASE
      WHEN us.expires_at <= now() OR us.starts_at > now() THEN NULL::timestamptz
      WHEN us.daily_window_start IS NOT NULL
       AND now() >= us.daily_window_start
       AND now() < us.daily_window_start + interval '24 hours'
        THEN us.daily_window_start
      ELSE date_trunc('day', now())
    END AS current_daily_start,
    CASE
      WHEN us.expires_at <= now() OR us.starts_at > now() THEN NULL::timestamptz
      WHEN us.weekly_anchor_at IS NOT NULL AND us.weekly_anchor_at <= now()
        THEN us.weekly_anchor_at
          + floor(EXTRACT(EPOCH FROM (now() - us.weekly_anchor_at)) / 604800.0)::INTEGER * interval '7 days'
      WHEN us.weekly_window_start IS NOT NULL
       AND now() >= us.weekly_window_start
       AND now() < us.weekly_window_start + interval '7 days'
        THEN us.weekly_window_start
      ELSE us.starts_at
        + floor(EXTRACT(EPOCH FROM (now() - us.starts_at)) / 604800.0)::INTEGER * interval '7 days'
    END AS current_weekly_start
  FROM migration_src.user_subscriptions us
  JOIN migration_src.groups g ON g.id = us.group_id
  JOIN migration_user_map m ON m.source_user_id = us.user_id
  WHERE us.deleted_at IS NULL
    AND us.status = 'active'
), windows AS (
  SELECT
    s.*,
    CASE
      WHEN s.current_daily_start IS NULL THEN NULL::timestamptz
      ELSE LEAST(s.current_daily_start + interval '24 hours', s.expires_at)
    END AS current_daily_end,
    CASE
      WHEN s.current_weekly_start IS NULL THEN NULL::timestamptz
      ELSE LEAST(s.current_weekly_start + interval '7 days', s.expires_at)
    END AS current_weekly_end
  FROM active_subscriptions s
)
SELECT
  w.id AS source_subscription_id,
  w.target_user_id,
  CASE
    WHEN w.daily_limit_usd IS NULL OR w.daily_limit_usd <= 0
      OR w.current_daily_start IS NULL
      OR w.current_daily_end <= w.current_daily_start
      THEN 0::NUMERIC(20,8)
    ELSE GREATEST(
      w.daily_limit_usd - COALESCE((
        SELECT SUM(ul.actual_cost)
        FROM migration_src.usage_logs ul
        WHERE ul.subscription_id = w.id
          AND ul.created_at >= w.current_daily_start
          AND ul.created_at < w.current_daily_end
      ), 0),
      0
    )::NUMERIC(20,8)
  END AS daily_remaining_usd,
  CASE
    WHEN w.weekly_limit_usd IS NULL OR w.weekly_limit_usd <= 0
      OR w.current_weekly_start IS NULL
      OR w.current_weekly_end <= w.current_weekly_start
      THEN 0::NUMERIC(20,8)
    ELSE GREATEST(
      w.weekly_limit_usd
        * EXTRACT(EPOCH FROM (w.current_weekly_end - w.current_weekly_start)) / 604800.0
        - COALESCE((
          SELECT SUM(ul.actual_cost)
          FROM migration_src.usage_logs ul
          WHERE ul.subscription_id = w.id
            AND ul.created_at >= w.current_weekly_start
            AND ul.created_at < w.current_weekly_end
        ), 0),
      0
    )::NUMERIC(20,8)
  END AS weekly_remaining_usd
FROM windows w;

CREATE TEMP TABLE migration_daily_remaining ON COMMIT DROP AS
SELECT target_user_id, SUM(daily_remaining_usd)::NUMERIC(20,8) AS remaining_usd
FROM migration_current_quota
GROUP BY target_user_id;

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
  current_remaining_credit_usd NUMERIC(20,8) NOT NULL DEFAULT 0,
  synthetic BOOLEAN NOT NULL DEFAULT FALSE
) ON COMMIT DROP;

INSERT INTO migration_package_source (
  source_order_id, target_order_id, source_subscription_id, target_user_id, source_user_id,
  weekly_credit_usd, starts_at, expires_at, refresh_count, credited_count, status, next_credit_at,
  issued_credit_usd, used_credit_usd, remaining_credit_usd, current_remaining_credit_usd
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
  END,
  CASE
    WHEN po.status IN ('REFUNDED', 'PARTIALLY_REFUNDED')
      OR po.start_at > now()
      OR po.end_at <= now()
      THEN 0
    ELSE COALESCE(cq.weekly_remaining_usd, 0)
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
    AND migration_sample_selected(po.id)
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
) metrics
LEFT JOIN migration_current_quota cq ON cq.source_subscription_id = po.subscription_id;

-- 历史订单只保留审计，不得因源订阅失效或缺失而继续在目标按周到账。
UPDATE migration_package_source p
   SET status = 'completed',
       next_credit_at = NULL,
       current_remaining_credit_usd = 0
 WHERE p.status = 'active'
   AND NOT EXISTS (
     SELECT 1
       FROM migration_current_quota q
      WHERE q.source_subscription_id = p.source_subscription_id
   );

UPDATE migration_package_source
   SET current_remaining_credit_usd = 0
 WHERE status <> 'active';

-- 为仍有效但没有可续期迁移包的订阅补建迁移订单，保留剩余天数和后续周刷新；无限额旧分组不映射为余额套餐。
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
          FROM migration_package_source p
         WHERE p.source_subscription_id = us.id
           AND p.status = 'active'
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
      issued_credit_usd, used_credit_usd, remaining_credit_usd, current_remaining_credit_usd, synthetic
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
      COALESCE((SELECT q.weekly_remaining_usd FROM migration_current_quota q WHERE q.source_subscription_id = subscription_row.id), 0),
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

-- 源 users.balance 是旧钱包，直接废弃；目标余额只承接当前日窗口剩余和当前周窗口剩余。
UPDATE users u
   SET balance = COALESCE((SELECT d.remaining_usd FROM migration_daily_remaining d WHERE d.target_user_id = u.id), 0)
               + COALESCE((SELECT SUM(p.current_remaining_credit_usd) FROM migration_package_source p WHERE p.target_user_id = u.id), 0);

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
UNION ALL SELECT 'source_sampled_orders', COUNT(*) FROM migration_src.payment_orders po WHERE (migration_sample_selected(po.id) OR (po.order_type = 'traffic_pack' AND (migration_source_user_is_admin(po.user_id) OR EXISTS (SELECT 1 FROM migration_src.user_traffic_credits c WHERE c.order_id = po.id AND (migration_sample_selected(c.id) OR migration_source_user_is_admin(c.user_id))))))
UNION ALL SELECT 'target_orders', COUNT(*) FROM payment_orders
UNION ALL SELECT 'source_sampled_audits', COUNT(*) FROM migration_src.payment_audit_logs pal WHERE EXISTS (SELECT 1 FROM migration_src.payment_orders po WHERE po.id::text = pal.order_id AND (migration_sample_selected(po.id) OR (po.order_type = 'traffic_pack' AND (migration_source_user_is_admin(po.user_id) OR EXISTS (SELECT 1 FROM migration_src.user_traffic_credits c WHERE c.order_id = po.id AND (migration_sample_selected(c.id) OR migration_source_user_is_admin(c.user_id)))))))
UNION ALL SELECT 'target_audits', COUNT(*) FROM payment_audit_logs
UNION ALL SELECT 'source_usage_logs', COUNT(*) FROM migration_src.usage_logs WHERE migration_sample_selected(id)
UNION ALL SELECT 'target_usage_logs', COUNT(*) FROM usage_logs
UNION ALL SELECT 'source_api_keys', COUNT(*) FROM migration_src.api_keys
UNION ALL SELECT 'target_api_keys', COUNT(*) FROM api_keys
UNION ALL SELECT 'source_accounts', COUNT(*) FROM migration_src.accounts
UNION ALL SELECT 'target_accounts', COUNT(*) FROM accounts
UNION ALL SELECT 'source_traffic_credits', COUNT(*) FROM migration_src.user_traffic_credits WHERE migration_sample_selected(id) OR migration_source_user_is_admin(user_id)
UNION ALL SELECT 'target_traffic_credits', COUNT(*) FROM user_traffic_credits
UNION ALL SELECT 'source_traffic_ledger', COUNT(*) FROM migration_src.traffic_credit_ledger WHERE migration_sample_selected(id) OR migration_source_user_is_admin(user_id) OR EXISTS (SELECT 1 FROM migration_src.user_traffic_credits c WHERE c.id = credit_id AND (migration_sample_selected(c.id) OR migration_source_user_is_admin(c.user_id))) OR EXISTS (SELECT 1 FROM migration_src.payment_orders po WHERE po.id = order_id AND (migration_sample_selected(po.id) OR migration_source_user_is_admin(po.user_id)))
UNION ALL SELECT 'target_traffic_ledger', COUNT(*) FROM traffic_credit_ledger
UNION ALL SELECT 'admin_target_users', COUNT(*) FROM users WHERE lower(trim(email)) = 'xiaobianfuai@gmail.com' AND role = 'admin' AND deleted_at IS NULL
UNION ALL SELECT 'admin_unspent_traffic_usd', COALESCE(SUM(remaining_usd), 0)::BIGINT FROM user_traffic_credits c JOIN users u ON u.id = c.user_id WHERE lower(trim(u.email)) = 'xiaobianfuai@gmail.com' AND u.role = 'admin' AND c.remaining_usd > 0 AND c.expires_at > now()
UNION ALL SELECT 'target_balance_packages', COUNT(*) FROM user_balance_packages
UNION ALL SELECT 'target_synthetic_packages', COUNT(*) FROM migration_package_source WHERE synthetic
UNION ALL SELECT 'source_current_daily_remaining_usd', COALESCE(SUM(daily_remaining_usd), 0)::BIGINT FROM migration_current_quota
UNION ALL SELECT 'source_current_weekly_remaining_usd', COALESCE(SUM(weekly_remaining_usd), 0)::BIGINT FROM migration_current_quota
UNION ALL SELECT 'target_current_package_remaining_usd', COALESCE(SUM(current_remaining_credit_usd), 0)::BIGINT FROM migration_package_source
UNION ALL SELECT 'target_balance_usd', COALESCE(SUM(balance), 0)::BIGINT FROM users
ORDER BY metric;

DROP FUNCTION migration_copy_common(TEXT, TEXT, TEXT, JSONB);
DROP FUNCTION migration_reset_sequence(TEXT);
DROP FUNCTION migration_sample_selected(BIGINT);
DROP FUNCTION migration_source_user_is_admin(BIGINT);
DROP SERVER migration_18080 CASCADE;
DROP SCHEMA migration_src CASCADE;

\if :commit
COMMIT;
\else
ROLLBACK;
\echo dry-run complete; all changes rolled back
\endif
