-- 回填历史 active 订阅的权益周期，并把订阅窗口用量校准到 usage_facts/usage_logs 的事实口径。

WITH paid_orders AS (
    SELECT
        po.id AS order_id,
        po.user_id,
        po.subscription_id,
        po.subscription_group_id AS group_id,
        GREATEST(COALESCE(po.subscription_days, 30), 1) AS period_days,
        COALESCE(po.paid_at, po.completed_at, po.created_at) AS paid_at,
        SUM(GREATEST(COALESCE(po.subscription_days, 30), 1)) OVER (
            PARTITION BY po.subscription_id
            ORDER BY COALESCE(po.paid_at, po.completed_at, po.created_at), po.id
            ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING
        ) AS prior_days
    FROM payment_orders po
    JOIN user_subscriptions us ON us.id = po.subscription_id
    WHERE po.order_type = 'subscription'
      AND po.subscription_id IS NOT NULL
      AND po.status IN ('COMPLETED', 'REFUND_FAILED', 'PARTIALLY_REFUNDED')
      AND us.deleted_at IS NULL
),
paid_periods AS (
    SELECT
        po.user_id,
        po.subscription_id,
        po.group_id,
        'payment_order'::varchar(40) AS source_type,
        po.order_id::text AS source_id,
        us.starts_at + (COALESCE(po.prior_days, 0) * INTERVAL '1 day') AS starts_at,
        LEAST(
            us.starts_at + ((COALESCE(po.prior_days, 0) + po.period_days) * INTERVAL '1 day'),
            us.expires_at
        ) AS expires_at,
        po.period_days,
        g.daily_limit_usd
    FROM paid_orders po
    JOIN user_subscriptions us ON us.id = po.subscription_id
    JOIN groups g ON g.id = po.group_id
),
insert_paid AS (
    INSERT INTO subscription_entitlement_periods (
        user_id,
        subscription_id,
        group_id,
        source_type,
        source_id,
        starts_at,
        expires_at,
        period_days,
        daily_limit_usd,
        status
    )
    SELECT
        user_id,
        subscription_id,
        group_id,
        source_type,
        source_id,
        starts_at,
        expires_at,
        GREATEST(FLOOR(EXTRACT(EPOCH FROM (expires_at - starts_at)) / 86400)::int, 1),
        daily_limit_usd,
        'active'
    FROM paid_periods
    WHERE expires_at > starts_at
    ON CONFLICT (source_type, source_id) DO NOTHING
    RETURNING 1
),
legacy_periods AS (
    SELECT
        us.user_id,
        us.id AS subscription_id,
        us.group_id,
        'legacy_subscription'::varchar(40) AS source_type,
        us.id::text AS source_id,
        us.starts_at,
        us.expires_at,
        GREATEST(FLOOR(EXTRACT(EPOCH FROM (us.expires_at - us.starts_at)) / 86400)::int, 1) AS period_days,
        g.daily_limit_usd
    FROM user_subscriptions us
    JOIN groups g ON g.id = us.group_id
    WHERE us.deleted_at IS NULL
      AND us.status = 'active'
      AND us.starts_at <= NOW()
      AND us.expires_at > NOW()
      AND NOT EXISTS (
          SELECT 1
          FROM subscription_entitlement_periods sep
          WHERE sep.subscription_id = us.id
            AND sep.status = 'active'
            AND sep.starts_at <= NOW()
            AND sep.expires_at > NOW()
      )
)
INSERT INTO subscription_entitlement_periods (
    user_id,
    subscription_id,
    group_id,
    source_type,
    source_id,
    starts_at,
    expires_at,
    period_days,
    daily_limit_usd,
    status
)
SELECT
    user_id,
    subscription_id,
    group_id,
    source_type,
    source_id,
    starts_at,
    expires_at,
    period_days,
    daily_limit_usd,
    'active'
FROM legacy_periods
WHERE expires_at > starts_at
ON CONFLICT (source_type, source_id) DO NOTHING;

WITH bounds AS (
    SELECT
        NOW() AS now_at,
        (date_trunc('day', NOW() AT TIME ZONE 'Asia/Shanghai') AT TIME ZONE 'Asia/Shanghai') AS daily_start,
        (
            date_trunc(
                'day',
                (NOW() AT TIME ZONE 'Asia/Shanghai')
                - (((EXTRACT(DOW FROM (NOW() AT TIME ZONE 'Asia/Shanghai'))::int + 6) % 7) * INTERVAL '1 day')
            ) AT TIME ZONE 'Asia/Shanghai'
        ) AS weekly_start
),
active_subs AS (
    SELECT
        us.id,
        us.user_id,
        us.group_id,
        b.now_at,
        b.daily_start,
        b.weekly_start,
        CASE
            WHEN us.monthly_window_start IS NULL OR b.now_at - us.monthly_window_start >= INTERVAL '30 days'
                THEN b.now_at
            ELSE us.monthly_window_start
        END AS monthly_start
    FROM user_subscriptions us
    CROSS JOIN bounds b
    WHERE us.deleted_at IS NULL
      AND us.status = 'active'
      AND us.starts_at <= b.now_at
      AND us.expires_at > b.now_at
),
fact_source AS (
    SELECT
        uf.user_id,
        NULLIF(NULLIF(uf.payload #>> '{billing_command,SubscriptionID}', ''), 'null')::bigint AS subscription_id,
        COALESCE(
            NULLIF(NULLIF(uf.payload #>> '{effects,group_id}', ''), 'null')::bigint,
            NULLIF(NULLIF(uf.payload #>> '{usage_log,GroupID}', ''), 'null')::bigint,
            NULLIF(NULLIF(uf.payload #>> '{usage_log,group_id}', ''), 'null')::bigint
        ) AS group_id,
        uf.request_id,
        uf.api_key_id,
        uf.completed_at AS occurred_at,
        COALESCE(
            NULLIF(uf.payload #>> '{usage_log,ActualCost}', '')::numeric,
            NULLIF(uf.payload #>> '{usage_log,actual_cost}', '')::numeric,
            NULLIF(uf.payload #>> '{effects,actual_cost}', '')::numeric,
            0
        ) AS actual_cost
    FROM usage_facts uf
    WHERE uf.billing_status IN ('pending', 'settling', 'settled', 'debt')
),
fact_rows AS (
    SELECT
        a.id AS subscription_id,
        fs.request_id,
        fs.api_key_id,
        fs.occurred_at,
        SUM(fs.actual_cost) AS actual_cost
    FROM active_subs a
    JOIN fact_source fs ON fs.user_id = a.user_id
        AND (fs.subscription_id = a.id OR (fs.subscription_id IS NULL AND fs.group_id = a.group_id))
        AND fs.occurred_at >= LEAST(a.daily_start, a.weekly_start, a.monthly_start)
        AND fs.occurred_at < a.now_at
    GROUP BY a.id, fs.request_id, fs.api_key_id, fs.occurred_at
),
log_rows AS (
    SELECT
        a.id AS subscription_id,
        ul.request_id,
        ul.api_key_id,
        ul.created_at AS occurred_at,
        ul.actual_cost::numeric AS actual_cost
    FROM active_subs a
    JOIN usage_logs ul ON ul.user_id = a.user_id
        AND (ul.subscription_id = a.id OR (ul.subscription_id IS NULL AND ul.group_id = a.group_id))
        AND ul.created_at >= LEAST(a.daily_start, a.weekly_start, a.monthly_start)
        AND ul.created_at < a.now_at
        AND ul.actual_cost > 0
    WHERE NOT EXISTS (
        SELECT 1
        FROM usage_facts uf
        WHERE uf.request_id = ul.request_id
          AND uf.api_key_id = ul.api_key_id
          AND uf.billing_status IN ('pending', 'settling', 'settled', 'debt')
    )
),
source_rows AS (
    SELECT * FROM fact_rows
    UNION ALL
    SELECT * FROM log_rows
),
usage_totals AS (
    SELECT
        a.id AS subscription_id,
        COALESCE(SUM(sr.actual_cost) FILTER (WHERE sr.occurred_at >= a.daily_start), 0) AS daily_usage_usd,
        COALESCE(SUM(sr.actual_cost) FILTER (WHERE sr.occurred_at >= a.weekly_start), 0) AS weekly_usage_usd,
        COALESCE(SUM(sr.actual_cost) FILTER (WHERE sr.occurred_at >= a.monthly_start), 0) AS monthly_usage_usd
    FROM active_subs a
    LEFT JOIN source_rows sr ON sr.subscription_id = a.id
    GROUP BY a.id
)
UPDATE user_subscriptions us
SET
    daily_usage_usd = ut.daily_usage_usd,
    weekly_usage_usd = ut.weekly_usage_usd,
    monthly_usage_usd = ut.monthly_usage_usd,
    daily_window_start = a.daily_start,
    weekly_window_start = a.weekly_start,
    monthly_window_start = a.monthly_start,
    updated_at = a.now_at
FROM usage_totals ut
JOIN active_subs a ON a.id = ut.subscription_id
WHERE us.id = ut.subscription_id;
