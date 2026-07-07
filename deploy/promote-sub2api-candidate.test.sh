#!/usr/bin/env bash
# promote-sub2api-candidate.sh 的轻量回归测试；只使用 dry-run，不触碰真实 Docker。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT_UNDER_TEST="${SCRIPT_DIR}/promote-sub2api-candidate.sh"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  [[ "${haystack}" == *"${needle}"* ]] || fail "expected output to contain: ${needle}"
}

assert_not_contains() {
  local haystack="$1"
  local needle="$2"
  [[ "${haystack}" != *"${needle}"* ]] || fail "expected output not to contain: ${needle}"
}

test_dry_run_promotes_candidate_18084_app_only() {
  local output

  output="$(
    SUB2API_PROMOTE_TS=20260707-000000 \
    SUB2API_TARGET_DATA_MOUNT="/tmp/sub2api-candidate-data:/app/data:rw" \
      bash "${SCRIPT_UNDER_TEST}" \
        --dry-run \
        --yes \
        --docker-bin docker \
        --candidate-image sub2api-candidate:test-sha
  )"

  assert_contains "${output}" "部署目标：public_candidate_18084"
  assert_contains "${output}" "应用容器：sub2api-candidate"
  assert_contains "${output}" "宿主端口：18084"
  assert_contains "${output}" "docker stop sub2api-candidate"
  assert_contains "${output}" "docker rename sub2api-candidate sub2api-candidate-before-promote-20260707-000000"
  assert_contains "${output}" "docker run -d --name sub2api-candidate"
  assert_contains "${output}" "-p 127.0.0.1:18084:8080"
  assert_contains "${output}" "--network sub2api-candidate-network"
  assert_contains "${output}" "/tmp/sub2api-candidate-data:/app/data:rw"
  assert_contains "${output}" "sub2api-candidate:test-sha"
  assert_contains "${output}" "http://127.0.0.1:18084/health"
  assert_not_contains "${output}" "weishaw/sub2api:latest"
  assert_not_contains "${output}" "127.0.0.1:18080"
  assert_not_contains "${output}" "compose --env-file"
}

test_legacy_target_requires_explicit_confirmation() {
  local status

  set +e
  DEPLOY_TARGET=legacy_18080 \
    bash "${SCRIPT_UNDER_TEST}" \
      --dry-run \
      --yes \
      --docker-bin docker \
      --candidate-image sub2api-candidate:test-sha >/tmp/promote-legacy-test.out 2>&1
  status=$?
  set -e

  [[ "${status}" -ne 0 ]] || fail "legacy target should be rejected without explicit confirmation"
  assert_contains "$(cat /tmp/promote-legacy-test.out)" "legacy_18080 不是当前标准公网目标"
}

test_dry_run_promotes_candidate_18084_app_only
test_legacy_target_requires_explicit_confirmation

echo "promote-sub2api-candidate tests passed"
