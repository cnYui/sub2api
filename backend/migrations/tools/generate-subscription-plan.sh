#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE' >&2
用法:
  generate-subscription-plan.sh \
    --price-cny 109 \
    --plan-label "109 元订阅池" \
    --group-name codex-pool-99-usd \
    --daily-limit-usd 99 \
    --sort-order 109 \
    --validity-days 30 \
    --template-group codex-pool-49-usd \
    [--bind-openai-accounts]
USAGE
}

sql_quote() {
  local value="$1"
  value="${value//\'/\'\'}"
  printf "'%s'" "${value}"
}

require_value() {
  local name="$1"
  local value="$2"
  if [[ -z "${value}" ]]; then
    echo "缺少参数: ${name}" >&2
    usage
    exit 2
  fi
}

require_number() {
  local name="$1"
  local value="$2"
  if [[ ! "${value}" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
    echo "参数必须是非负数字: ${name}" >&2
    exit 2
  fi
}

require_int() {
  local name="$1"
  local value="$2"
  if [[ ! "${value}" =~ ^[0-9]+$ ]]; then
    echo "参数必须是非负整数: ${name}" >&2
    exit 2
  fi
}

price_cny=""
plan_label=""
group_name=""
daily_limit_usd=""
sort_order=""
validity_days=""
template_group=""
bind_openai_accounts="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --price-cny)
      price_cny="${2:-}"
      shift 2
      ;;
    --plan-label)
      plan_label="${2:-}"
      shift 2
      ;;
    --group-name)
      group_name="${2:-}"
      shift 2
      ;;
    --daily-limit-usd)
      daily_limit_usd="${2:-}"
      shift 2
      ;;
    --sort-order)
      sort_order="${2:-}"
      shift 2
      ;;
    --validity-days)
      validity_days="${2:-}"
      shift 2
      ;;
    --template-group)
      template_group="${2:-}"
      shift 2
      ;;
    --bind-openai-accounts)
      bind_openai_accounts="true"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "未知参数: $1" >&2
      usage
      exit 2
      ;;
  esac
done

require_value "--price-cny" "${price_cny}"
require_value "--plan-label" "${plan_label}"
require_value "--group-name" "${group_name}"
require_value "--daily-limit-usd" "${daily_limit_usd}"
require_value "--sort-order" "${sort_order}"
require_value "--validity-days" "${validity_days}"
require_value "--template-group" "${template_group}"

require_number "--price-cny" "${price_cny}"
require_number "--daily-limit-usd" "${daily_limit_usd}"
require_int "--sort-order" "${sort_order}"
require_int "--validity-days" "${validity_days}"

price_fmt="$(LC_ALL=C printf '%.2f' "${price_cny}")"
plan_label_sql="$(sql_quote "${plan_label}")"
group_name_sql="$(sql_quote "${group_name}")"
template_group_sql="$(sql_quote "${template_group}")"
group_description="${plan_label}，每日 ${daily_limit_usd} USD，${validity_days} 天有效期"
group_description_sql="$(sql_quote "${group_description}")"
plan_description="月度订阅-时间 ${validity_days}天，日限额 ${daily_limit_usd}刀，24点刷新"
plan_description_sql="$(sql_quote "${plan_description}")"

cat <<SQL
-- 自动生成订阅套餐 seed：${plan_label} / ${group_name}
-- 只输出 SQL，请人工审阅后保存为新的 migration；不要覆写已发布 migration。

DO \$\$
DECLARE
    target_group_id BIGINT;
BEGIN
    INSERT INTO groups (
        name,
        description,
        rate_multiplier,
        is_exclusive,
        status,
        platform,
        subscription_type,
        daily_limit_usd,
        weekly_limit_usd,
        monthly_limit_usd,
        default_validity_days,
        allow_image_generation,
        image_rate_independent,
        image_rate_multiplier,
        image_price_1k,
        image_price_2k,
        image_price_4k,
        allow_messages_dispatch,
        require_oauth_only,
        require_privacy_set,
        default_mapped_model,
        messages_dispatch_model_config,
        models_list_config,
        supported_model_scopes,
        model_routing,
        model_routing_enabled,
        sort_order,
        created_at,
        updated_at
    )
    SELECT
        ${group_name_sql},
        ${group_description_sql},
        1.0,
        FALSE,
        'active',
        'openai',
        'subscription',
        ${daily_limit_usd},
        NULL,
        NULL,
        ${validity_days},
        TRUE,
        COALESCE((SELECT image_rate_independent FROM groups WHERE name = ${template_group_sql} AND deleted_at IS NULL ORDER BY id LIMIT 1), FALSE),
        COALESCE((SELECT image_rate_multiplier FROM groups WHERE name = ${template_group_sql} AND deleted_at IS NULL ORDER BY id LIMIT 1), 1.0),
        0.10,
        0.20,
        0.40,
        COALESCE((SELECT allow_messages_dispatch FROM groups WHERE name = ${template_group_sql} AND deleted_at IS NULL ORDER BY id LIMIT 1), FALSE),
        COALESCE((SELECT require_oauth_only FROM groups WHERE name = ${template_group_sql} AND deleted_at IS NULL ORDER BY id LIMIT 1), FALSE),
        COALESCE((SELECT require_privacy_set FROM groups WHERE name = ${template_group_sql} AND deleted_at IS NULL ORDER BY id LIMIT 1), FALSE),
        COALESCE((SELECT default_mapped_model FROM groups WHERE name = ${template_group_sql} AND deleted_at IS NULL ORDER BY id LIMIT 1), ''),
        COALESCE((SELECT messages_dispatch_model_config FROM groups WHERE name = ${template_group_sql} AND deleted_at IS NULL ORDER BY id LIMIT 1), '{}'::jsonb),
        COALESCE((SELECT models_list_config FROM groups WHERE name = ${template_group_sql} AND deleted_at IS NULL ORDER BY id LIMIT 1), '{}'::jsonb),
        COALESCE((SELECT supported_model_scopes FROM groups WHERE name = ${template_group_sql} AND deleted_at IS NULL ORDER BY id LIMIT 1), '[]'::jsonb),
        COALESCE((SELECT model_routing FROM groups WHERE name = ${template_group_sql} AND deleted_at IS NULL ORDER BY id LIMIT 1), '{}'::jsonb),
        COALESCE((SELECT model_routing_enabled FROM groups WHERE name = ${template_group_sql} AND deleted_at IS NULL ORDER BY id LIMIT 1), FALSE),
        ${sort_order},
        NOW(),
        NOW()
    WHERE NOT EXISTS (
        SELECT 1 FROM groups WHERE name = ${group_name_sql} AND deleted_at IS NULL
    )
    RETURNING id INTO target_group_id;

    SELECT id INTO target_group_id
    FROM groups
    WHERE name = ${group_name_sql} AND deleted_at IS NULL
    ORDER BY id
    LIMIT 1;

    UPDATE groups
    SET
        description = ${group_description_sql},
        rate_multiplier = 1.0,
        is_exclusive = FALSE,
        status = 'active',
        platform = 'openai',
        subscription_type = 'subscription',
        daily_limit_usd = ${daily_limit_usd},
        weekly_limit_usd = NULL,
        monthly_limit_usd = NULL,
        default_validity_days = ${validity_days},
        allow_image_generation = TRUE,
        image_rate_independent = COALESCE((SELECT image_rate_independent FROM groups WHERE name = ${template_group_sql} AND deleted_at IS NULL ORDER BY id LIMIT 1), FALSE),
        image_rate_multiplier = COALESCE((SELECT image_rate_multiplier FROM groups WHERE name = ${template_group_sql} AND deleted_at IS NULL ORDER BY id LIMIT 1), 1.0),
        image_price_1k = 0.10,
        image_price_2k = 0.20,
        image_price_4k = 0.40,
        allow_messages_dispatch = COALESCE((SELECT allow_messages_dispatch FROM groups WHERE name = ${template_group_sql} AND deleted_at IS NULL ORDER BY id LIMIT 1), FALSE),
        require_oauth_only = COALESCE((SELECT require_oauth_only FROM groups WHERE name = ${template_group_sql} AND deleted_at IS NULL ORDER BY id LIMIT 1), FALSE),
        require_privacy_set = COALESCE((SELECT require_privacy_set FROM groups WHERE name = ${template_group_sql} AND deleted_at IS NULL ORDER BY id LIMIT 1), FALSE),
        default_mapped_model = COALESCE((SELECT default_mapped_model FROM groups WHERE name = ${template_group_sql} AND deleted_at IS NULL ORDER BY id LIMIT 1), ''),
        messages_dispatch_model_config = COALESCE((SELECT messages_dispatch_model_config FROM groups WHERE name = ${template_group_sql} AND deleted_at IS NULL ORDER BY id LIMIT 1), '{}'::jsonb),
        models_list_config = COALESCE((SELECT models_list_config FROM groups WHERE name = ${template_group_sql} AND deleted_at IS NULL ORDER BY id LIMIT 1), '{}'::jsonb),
        supported_model_scopes = COALESCE((SELECT supported_model_scopes FROM groups WHERE name = ${template_group_sql} AND deleted_at IS NULL ORDER BY id LIMIT 1), '[]'::jsonb),
        model_routing = COALESCE((SELECT model_routing FROM groups WHERE name = ${template_group_sql} AND deleted_at IS NULL ORDER BY id LIMIT 1), '{}'::jsonb),
        model_routing_enabled = COALESCE((SELECT model_routing_enabled FROM groups WHERE name = ${template_group_sql} AND deleted_at IS NULL ORDER BY id LIMIT 1), FALSE),
        sort_order = ${sort_order},
        updated_at = NOW()
    WHERE id = target_group_id;

    UPDATE subscription_plans
    SET
        group_id = target_group_id,
        name = ${plan_label_sql},
        description = ${plan_description_sql},
        price = ${price_fmt},
        original_price = NULL,
        validity_days = ${validity_days},
        validity_unit = 'day',
        features = '',
        product_name = ${plan_label_sql},
        for_sale = TRUE,
        sort_order = ${sort_order},
        updated_at = NOW()
    WHERE group_id = target_group_id
       OR name = ${plan_label_sql}
       OR product_name = ${plan_label_sql};

    INSERT INTO subscription_plans (
        group_id,
        name,
        description,
        price,
        original_price,
        validity_days,
        validity_unit,
        features,
        product_name,
        for_sale,
        sort_order,
        created_at,
        updated_at
    )
    SELECT
        target_group_id,
        ${plan_label_sql},
        ${plan_description_sql},
        ${price_fmt},
        NULL,
        ${validity_days},
        'day',
        '',
        ${plan_label_sql},
        TRUE,
        ${sort_order},
        NOW(),
        NOW()
    WHERE NOT EXISTS (
        SELECT 1
        FROM subscription_plans
        WHERE group_id = target_group_id
           OR name = ${plan_label_sql}
           OR product_name = ${plan_label_sql}
    );
END \$\$;
SQL

if [[ "${bind_openai_accounts}" == "true" ]]; then
  cat <<SQL

INSERT INTO account_groups (account_id, group_id, priority, created_at)
SELECT a.id, g.id, a.priority, NOW()
FROM accounts a
JOIN groups g
  ON g.name = ${group_name_sql}
 AND g.deleted_at IS NULL
WHERE a.platform = 'openai'
  AND a.deleted_at IS NULL
ON CONFLICT (account_id, group_id) DO NOTHING;
SQL
fi
