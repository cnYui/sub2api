#!/usr/bin/env bash
# 验证候选预演脚本的 dry-run 安全边界，不启动 Docker 服务。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

assert_contains() {
  local haystack="$1"
  local needle="$2"
  if [[ "${haystack}" != *"${needle}"* ]]; then
    printf '[FAIL] expected output to contain: %s\n' "${needle}" >&2
    exit 1
  fi
}

assert_not_contains() {
  local haystack="$1"
  local needle="$2"
  if [[ "${haystack}" == *"${needle}"* ]]; then
    printf '[FAIL] expected output not to contain: %s\n' "${needle}" >&2
    exit 1
  fi
}

main() {
  local compose_output dry_run_output promote_output promote_status sanitize_sql

  sanitize_sql="$(<"${REPO_ROOT}/deploy/sql/candidate-sanitize.sql")"
  assert_contains "${sanitize_sql}" "('payment_enabled', 'true'"
  assert_contains "${sanitize_sql}" "('payment_visible_method_alipay_enabled', 'true'"
  assert_contains "${sanitize_sql}" "('payment_visible_method_alipay_source', 'easypay_alipay'"
  assert_contains "${sanitize_sql}" "('ENABLED_PAYMENT_TYPES', 'alipay'"
  assert_contains "${sanitize_sql}" "name = 'ZPay Alipay'"
  assert_not_contains "${sanitize_sql}" "('payment_enabled', 'false'"
  assert_not_contains "${sanitize_sql}" "('payment_visible_method_alipay_enabled', 'false'"
  assert_not_contains "${sanitize_sql}" "UPDATE payment_provider_instances SET enabled = false"

  compose_output="$("${DOCKER_BIN:-/Applications/Docker.app/Contents/Resources/bin/docker}" compose \
    --env-file "${REPO_ROOT}/deploy/.env.candidate.local.example" \
    -f "${REPO_ROOT}/deploy/docker-compose.candidate.yml" \
    config 2>&1)"
  assert_contains "${compose_output}" "name: sub2api-candidate-rehearsal"
  assert_contains "${compose_output}" "TOTP_ENCRYPTION_KEY: 00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
  assert_not_contains "${compose_output}" "candidate-local-totp-key-change-me"

  dry_run_output="$("${REPO_ROOT}/deploy/rehearse-sub2api-candidate.sh" --dry-run --reset-db 2>&1)"
  assert_contains "${dry_run_output}" "sub2api-candidate:"
  assert_contains "${dry_run_output}" "127.0.0.1:18081"
  assert_contains "${dry_run_output}" "-p sub2api-candidate-rehearsal"
  assert_contains "${dry_run_output}" "pg_isready"
  assert_contains "${dry_run_output}" "inspect public container compose project labels"
  assert_not_contains "${dry_run_output}" "https://api.aaccx.pw/v1"
  assert_not_contains "${dry_run_output}" "down --remove-orphans"
  assert_not_contains "${dry_run_output}" "up -d --no-deps --force-recreate sub2api"
  assert_not_contains "${dry_run_output}" "weishaw/sub2api:latest -f"

  set +e
  promote_output="$("${REPO_ROOT}/deploy/promote-sub2api-candidate.sh" --candidate-image weishaw/sub2api:latest --yes 2>&1)"
  promote_status=$?
  set -e
  if [[ ${promote_status} -eq 0 ]]; then
    printf '[FAIL] promote script accepted weishaw/sub2api:latest\n' >&2
    exit 1
  fi
  assert_contains "${promote_output}" "只允许发布 sub2api-candidate:* 镜像"

  printf '[PASS] candidate rehearsal script safety checks passed\n'
}

main "$@"
