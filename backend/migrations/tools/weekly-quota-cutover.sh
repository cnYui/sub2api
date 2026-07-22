#!/usr/bin/env bash
set -euo pipefail

# 公共 Codex 周额度切换工具。默认 dry-run，避免 migration runner 在未盘点历史数据时自动改写权益。
# 用法：DATABASE_URL=postgres://... ./weekly-quota-cutover.sh --migration-at=2026-07-21T23:00:00+09:00 [--apply]

apply=0
migration_at=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --apply) apply=1 ;;
    --migration-at=*) migration_at="${1#*=}" ;;
    --migration-at)
      shift
      migration_at="${1:-}"
      ;;
    *) echo "未知参数: $1" >&2; exit 2 ;;
  esac
  shift
done

: "${DATABASE_URL:?请设置 DATABASE_URL；不得在命令行或日志中记录凭据}"
if [[ -z "$migration_at" ]]; then
  echo "必须指定 --migration-at=RFC3339 时间，保证 dry-run 与 apply 使用同一锚点" >&2
  exit 2
fi
if [[ "$apply" == 1 ]]; then
  : "${REDIS_URL:?--apply 必须设置 REDIS_URL，用于失效 billing:sub:* 订阅缓存；不得在命令行或日志中记录凭据}"
  if ! command -v redis-cli >/dev/null 2>&1; then
    echo "--apply 需要 redis-cli 来失效订阅缓存" >&2
    exit 2
  fi
fi

psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -v migration_at="$migration_at" <<'SQL'
\pset pager off
\echo '=== 公共 Codex active 订阅预览 ==='
WITH plan_limits(group_name, weekly_usd) AS (
  VALUES
    ('codex-pool-19-usd'::text, 58::numeric),
    ('codex-pool-29-usd', 78::numeric),
    ('codex-pool-49-usd', 118::numeric),
    ('codex-pool-69-usd', 158::numeric),
    ('codex-pool-89-usd', 198::numeric),
    ('codex-pool-135-usd', 299::numeric),
    ('codex-pool-179-usd', 400::numeric)
),
public_groups AS (
  SELECT g.id, g.name, p.weekly_usd AS target_weekly_limit_usd
  FROM groups g JOIN plan_limits p ON p.group_name = g.name
  WHERE g.name IN ('codex-pool-19-usd','codex-pool-29-usd','codex-pool-49-usd','codex-pool-69-usd','codex-pool-89-usd','codex-pool-135-usd','codex-pool-179-usd')
    AND subscription_type = 'subscription' AND deleted_at IS NULL
)
SELECT us.id AS subscription_id, us.user_id, pg.name, us.expires_at,
       sep.id AS entitlement_period_id,
       pg.target_weekly_limit_usd AS entitlement_weekly_limit_usd,
       GREATEST(us.expires_at - :'migration_at'::timestamptz, interval '0') AS remaining,
       LEAST(us.expires_at, :'migration_at'::timestamptz + interval '7 days') AS first_window_end,
       ROUND(
         pg.target_weekly_limit_usd
         * EXTRACT(EPOCH FROM GREATEST(
             LEAST(us.expires_at, :'migration_at'::timestamptz + interval '7 days') - :'migration_at'::timestamptz,
             interval '0'
           ))
         / EXTRACT(EPOCH FROM interval '7 days'),
         10
       ) AS first_window_effective_limit_usd,
       us.weekly_anchor_at, us.weekly_window_start
FROM user_subscriptions us JOIN public_groups pg ON pg.id = us.group_id
LEFT JOIN LATERAL (
  SELECT id, weekly_limit_usd
  FROM subscription_entitlement_periods
  WHERE subscription_id = us.id
    AND status = 'active'
    AND starts_at < us.expires_at
    AND expires_at > :'migration_at'::timestamptz
  ORDER BY starts_at ASC, id ASC
  LIMIT 1
) sep ON TRUE
WHERE us.deleted_at IS NULL AND us.status = 'active'
ORDER BY us.id;

\echo '=== 需人工处理的订单与权益 ==='
WITH public_groups AS (
  SELECT id, name FROM groups
  WHERE name IN ('codex-pool-19-usd','codex-pool-29-usd','codex-pool-49-usd','codex-pool-69-usd','codex-pool-89-usd','codex-pool-135-usd','codex-pool-179-usd')
    AND subscription_type = 'subscription' AND deleted_at IS NULL
)
SELECT 'pending_or_paid_order' AS category, po.id::text AS object_id
FROM payment_orders po JOIN public_groups pg ON pg.id = po.subscription_group_id
WHERE po.order_type = 'subscription' AND po.status IN ('PENDING','PAID','RECHARGING')
UNION ALL
SELECT 'refund_in_progress_order', po.id::text
FROM payment_orders po JOIN public_groups pg ON pg.id = po.subscription_group_id
WHERE po.order_type = 'subscription' AND po.status IN ('REFUND_REQUESTED','REFUNDING','REFUND_FAILED')
UNION ALL
SELECT 'completed_without_subscription', po.id::text
FROM payment_orders po
JOIN public_groups pg ON pg.id = po.subscription_group_id
WHERE po.order_type = 'subscription' AND po.status = 'COMPLETED' AND po.subscription_id IS NULL
UNION ALL
SELECT 'completed_without_entitlement', po.id::text
FROM payment_orders po
JOIN public_groups pg ON pg.id = po.subscription_group_id
WHERE po.order_type = 'subscription' AND po.status = 'COMPLETED' AND po.subscription_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM subscription_entitlement_periods sep WHERE sep.source_type = 'payment_order' AND sep.source_id = po.id::text)
UNION ALL
SELECT 'overlapping_entitlement', a.id::text || ':' || b.id::text
FROM subscription_entitlement_periods a JOIN subscription_entitlement_periods b
  ON a.subscription_id = b.subscription_id AND a.id < b.id AND a.status = 'active' AND b.status = 'active'
 AND a.starts_at < b.expires_at AND b.starts_at < a.expires_at
JOIN public_groups pg ON pg.id = a.group_id
UNION ALL
SELECT 'usage_fact_unallocated', uf.id::text
FROM usage_facts uf
JOIN LATERAL (
  SELECT COALESCE(
    NULLIF(uf.payload #>> '{billing_command,SubscriptionID}', ''),
    NULLIF(uf.payload #>> '{billing_command,subscription_id}', ''),
    NULLIF(uf.payload #>> '{usage_log,SubscriptionID}', ''),
    NULLIF(uf.payload #>> '{usage_log,subscription_id}', '')
  ) AS subscription_id_raw
) raw_subscription ON TRUE
JOIN user_subscriptions us ON raw_subscription.subscription_id_raw ~ '^[0-9]+$'
  AND us.id = raw_subscription.subscription_id_raw::bigint
JOIN public_groups pg ON pg.id = us.group_id
LEFT JOIN LATERAL (
  SELECT COUNT(*) AS matches
  FROM subscription_entitlement_periods sep
  WHERE sep.subscription_id = us.id
    AND sep.status = 'active'
    AND uf.completed_at >= sep.starts_at
    AND uf.completed_at < sep.expires_at
) matched ON TRUE
WHERE uf.entitlement_period_id IS NULL
  AND COALESCE(matched.matches, 0) <> 1
ORDER BY category, object_id;
SQL

if [[ "$apply" != 1 ]]; then
  echo 'dry-run 完成；未写入任何订阅、权益、额度或缓存。传入 --apply 才会执行切换。'
  exit 0
fi

psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -v migration_at="$migration_at" <<'SQL'
DO $$
DECLARE
  blocker_count bigint;
BEGIN
  WITH public_groups AS (
    SELECT id, name FROM groups
    WHERE name IN ('codex-pool-19-usd','codex-pool-29-usd','codex-pool-49-usd','codex-pool-69-usd','codex-pool-89-usd','codex-pool-135-usd','codex-pool-179-usd')
      AND subscription_type = 'subscription' AND deleted_at IS NULL
  ),
  blockers AS (
    SELECT 'pending_or_paid_order' AS category, po.id::text AS object_id
    FROM payment_orders po JOIN public_groups pg ON pg.id = po.subscription_group_id
    WHERE po.order_type = 'subscription' AND po.status IN ('PENDING','PAID','RECHARGING')
    UNION ALL
    SELECT 'refund_in_progress_order', po.id::text
    FROM payment_orders po JOIN public_groups pg ON pg.id = po.subscription_group_id
    WHERE po.order_type = 'subscription' AND po.status IN ('REFUND_REQUESTED','REFUNDING','REFUND_FAILED')
    UNION ALL
    SELECT 'completed_without_subscription', po.id::text
    FROM payment_orders po
    JOIN public_groups pg ON pg.id = po.subscription_group_id
    WHERE po.order_type = 'subscription' AND po.status = 'COMPLETED' AND po.subscription_id IS NULL
    UNION ALL
    SELECT 'completed_without_entitlement', po.id::text
    FROM payment_orders po
    JOIN public_groups pg ON pg.id = po.subscription_group_id
    WHERE po.order_type = 'subscription' AND po.status = 'COMPLETED' AND po.subscription_id IS NOT NULL
      AND NOT EXISTS (SELECT 1 FROM subscription_entitlement_periods sep WHERE sep.source_type = 'payment_order' AND sep.source_id = po.id::text)
    UNION ALL
    SELECT 'overlapping_entitlement', a.id::text || ':' || b.id::text
    FROM subscription_entitlement_periods a JOIN subscription_entitlement_periods b
      ON a.subscription_id = b.subscription_id AND a.id < b.id AND a.status = 'active' AND b.status = 'active'
     AND a.starts_at < b.expires_at AND b.starts_at < a.expires_at
    JOIN public_groups pg ON pg.id = a.group_id
    UNION ALL
    SELECT 'usage_fact_unallocated', uf.id::text
    FROM usage_facts uf
    JOIN LATERAL (
      SELECT COALESCE(
        NULLIF(uf.payload #>> '{billing_command,SubscriptionID}', ''),
        NULLIF(uf.payload #>> '{billing_command,subscription_id}', ''),
        NULLIF(uf.payload #>> '{usage_log,SubscriptionID}', ''),
        NULLIF(uf.payload #>> '{usage_log,subscription_id}', '')
      ) AS subscription_id_raw
    ) raw_subscription ON TRUE
    JOIN user_subscriptions us ON raw_subscription.subscription_id_raw ~ '^[0-9]+$'
      AND us.id = raw_subscription.subscription_id_raw::bigint
    JOIN public_groups pg ON pg.id = us.group_id
    LEFT JOIN LATERAL (
      SELECT COUNT(*) AS matches
      FROM subscription_entitlement_periods sep
      WHERE sep.subscription_id = us.id
        AND sep.status = 'active'
        AND uf.completed_at >= sep.starts_at
        AND uf.completed_at < sep.expires_at
    ) matched ON TRUE
    WHERE uf.entitlement_period_id IS NULL
      AND COALESCE(matched.matches, 0) <> 1
  )
  SELECT COUNT(*) INTO blocker_count FROM blockers;

  IF blocker_count > 0 THEN
    RAISE EXCEPTION 'weekly quota cutover has % blocking objects; run dry-run and handle them before --apply', blocker_count;
  END IF;
END $$;

BEGIN;
LOCK TABLE groups, subscription_plans, user_subscriptions, subscription_entitlement_periods IN SHARE ROW EXCLUSIVE MODE;

WITH plan_limits(group_name, weekly_usd, plan_label, price) AS (
  VALUES
    ('codex-pool-19-usd'::text, 58::numeric, '29 元订阅池'::text, 29.00::numeric),
    ('codex-pool-29-usd', 78::numeric, '39 元订阅池', 39.00::numeric),
    ('codex-pool-49-usd', 118::numeric, '59 元订阅池', 59.00::numeric),
    ('codex-pool-69-usd', 158::numeric, '79 元订阅池', 79.00::numeric),
    ('codex-pool-89-usd', 198::numeric, '99 元订阅池', 99.00::numeric),
    ('codex-pool-135-usd', 299::numeric, '149 元订阅池', 149.00::numeric),
    ('codex-pool-179-usd', 400::numeric, '199 元订阅池', 199.00::numeric)
)
UPDATE groups g SET daily_limit_usd = NULL, monthly_limit_usd = NULL, weekly_limit_usd = p.weekly_usd,
  default_validity_days = 28, description = p.plan_label || '，每 7 天 ' || trim_scale(p.weekly_usd) || ' USD，28 天有效期', updated_at = NOW()
FROM plan_limits p WHERE g.name = p.group_name AND g.subscription_type = 'subscription';

UPDATE subscription_plans sp SET
  name = p.plan_label,
  product_name = p.plan_label,
  validity_days = 28,
  validity_unit = 'day',
  description = '28 天订阅，每 7 天刷新周额度，购买时间起滚动计算',
  features = '周额度 ' || p.weekly_usd::text || ' USD' || E'\n28 天有效期' || E'\n购买时间起每 7 天刷新',
  updated_at = NOW()
FROM groups g
JOIN plan_limits p ON p.group_name = g.name
WHERE sp.group_id = g.id AND g.weekly_limit_usd IS NOT NULL
  AND g.name IN ('codex-pool-19-usd','codex-pool-29-usd','codex-pool-49-usd','codex-pool-69-usd','codex-pool-89-usd','codex-pool-135-usd','codex-pool-179-usd');

UPDATE user_subscriptions us SET weekly_anchor_at = :'migration_at'::timestamptz,
  weekly_window_start = :'migration_at'::timestamptz, weekly_usage_usd = 0, updated_at = NOW()
FROM groups g WHERE us.group_id = g.id AND us.deleted_at IS NULL AND us.status = 'active'
  AND us.expires_at > :'migration_at'::timestamptz AND us.weekly_anchor_at IS NULL
  AND g.weekly_limit_usd IS NOT NULL
  AND g.name IN ('codex-pool-19-usd','codex-pool-29-usd','codex-pool-49-usd','codex-pool-69-usd','codex-pool-89-usd','codex-pool-135-usd','codex-pool-179-usd');

UPDATE subscription_entitlement_periods sep SET weekly_limit_usd = g.weekly_limit_usd,
  period_total_quota_usd = g.weekly_limit_usd * 4,
  quota_window_unit = 'week', quota_window_days = 7, updated_at = NOW()
FROM groups g WHERE sep.group_id = g.id AND sep.status = 'active'
  AND sep.expires_at > :'migration_at'::timestamptz
  AND g.weekly_limit_usd IS NOT NULL
  AND g.name IN ('codex-pool-19-usd','codex-pool-29-usd','codex-pool-49-usd','codex-pool-69-usd','codex-pool-89-usd','codex-pool-135-usd','codex-pool-179-usd');

-- 仅回填唯一落入一个权益段的历史事实；跨段、缺少订阅来源或重叠权益保持 NULL，自动退款将转人工审核。
WITH candidates AS (
  SELECT uf.id, MIN(sep.id) AS entitlement_period_id, COUNT(*) AS matches
  FROM usage_facts uf
  JOIN LATERAL (
    SELECT COALESCE(
      NULLIF(uf.payload #>> '{billing_command,SubscriptionID}', ''),
      NULLIF(uf.payload #>> '{billing_command,subscription_id}', ''),
      NULLIF(uf.payload #>> '{usage_log,SubscriptionID}', ''),
      NULLIF(uf.payload #>> '{usage_log,subscription_id}', '')
    ) AS subscription_id_raw
  ) raw_subscription ON TRUE
  JOIN subscription_entitlement_periods sep ON raw_subscription.subscription_id_raw ~ '^[0-9]+$'
    AND sep.subscription_id = raw_subscription.subscription_id_raw::bigint
    AND sep.status = 'active' AND uf.completed_at >= sep.starts_at AND uf.completed_at < sep.expires_at
  WHERE uf.entitlement_period_id IS NULL
  GROUP BY uf.id
)
UPDATE usage_facts uf SET entitlement_period_id = c.entitlement_period_id
FROM candidates c WHERE uf.id = c.id AND c.matches = 1;

-- 已在本地完成有效期抵扣，仅写入审计事实；生产不得使用该分支，而要按锁定事实重新计算。
INSERT INTO subscription_quota_debt_adjustments (
  subscription_id, user_id, group_id, source_key, overage_usd, weekly_limit_usd, daily_equivalent_usd,
  raw_deduction_days, deducted_days, original_expires_at, new_expires_at, application_status, applied_at, notes
)
SELECT us.id, us.user_id, us.group_id, 'weekly_quota_cutover_overage:' || us.id,
  v.overage_usd, 58, 58.0 / 7, v.raw_days, v.deducted_days, us.expires_at, us.expires_at,
  'already_applied', NOW(), '本地开发库已在切换前按历史超额扣减有效期；禁止重复扣减'
FROM (VALUES (21::bigint, 234.1998836::numeric, 28.2655031931::numeric, 28), (53::bigint, 189.9496876::numeric, 22.9249622966::numeric, 22))
  AS v(subscription_id, overage_usd, raw_days, deducted_days)
JOIN user_subscriptions us ON us.id = v.subscription_id
ON CONFLICT (source_key) DO NOTHING;

COMMIT;
SQL

cache_pairs_file="$(mktemp)"
trap 'rm -f "$cache_pairs_file"' EXIT

psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -At -F ' ' <<'SQL' > "$cache_pairs_file"
WITH public_groups AS (
  SELECT id
  FROM groups
  WHERE name IN ('codex-pool-19-usd','codex-pool-29-usd','codex-pool-49-usd','codex-pool-69-usd','codex-pool-89-usd','codex-pool-135-usd','codex-pool-179-usd')
    AND subscription_type = 'subscription'
    AND deleted_at IS NULL
)
SELECT DISTINCT us.user_id, us.group_id
FROM user_subscriptions us
JOIN public_groups pg ON pg.id = us.group_id
WHERE us.deleted_at IS NULL;
SQL

cache_key_count=0
cache_publish_count=0
batch=()
flush_cache_batch() {
  if [[ ${#batch[@]} -eq 0 ]]; then
    return
  fi
  redis-cli -u "$REDIS_URL" DEL "${batch[@]}" >/dev/null
  cache_key_count=$((cache_key_count + ${#batch[@]}))
  batch=()
}

while read -r user_id group_id; do
  if [[ -z "${user_id:-}" || -z "${group_id:-}" ]]; then
    continue
  fi
  batch+=("billing:sub:${user_id}:${group_id}")
  redis-cli -u "$REDIS_URL" PUBLISH "subscription:cache:invalidate" "${user_id}:${group_id}" >/dev/null
  cache_publish_count=$((cache_publish_count + 1))
  if [[ ${#batch[@]} -ge 500 ]]; then
    flush_cache_batch
  fi
done < "$cache_pairs_file"
flush_cache_batch

echo "切换已提交，已失效 ${cache_key_count} 个 Redis 订阅缓存键，并发布 ${cache_publish_count} 条订阅 L1 缓存失效消息。"
