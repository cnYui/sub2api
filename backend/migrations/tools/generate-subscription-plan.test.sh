#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT_UNDER_TEST="${SCRIPT_DIR}/generate-subscription-plan.sh"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  [[ "${haystack}" == *"${needle}"* ]] || fail "expected output to contain: ${needle}"
}

output="$(
  bash "${SCRIPT_UNDER_TEST}" \
    --price-cny 109 \
    --plan-label "109 元订阅池" \
    --group-name codex-pool-99-usd \
    --daily-limit-usd 99 \
    --sort-order 109 \
    --validity-days 30 \
    --template-group codex-pool-49-usd \
    --bind-openai-accounts
)"

assert_contains "${output}" "INSERT INTO groups"
assert_contains "${output}" "'codex-pool-99-usd'"
assert_contains "${output}" "daily_limit_usd = 99"
assert_contains "${output}" "INSERT INTO subscription_plans"
assert_contains "${output}" "'109 元订阅池'"
assert_contains "${output}" "price = 109.00"
assert_contains "${output}" "INSERT INTO account_groups"
assert_contains "${output}" "WHERE a.platform = 'openai'"
assert_contains "${output}" "ON CONFLICT (account_id, group_id) DO NOTHING"

echo "generate-subscription-plan tests passed"
