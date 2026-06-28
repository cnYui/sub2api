#!/usr/bin/env bash
# restart-sub2api.sh 的轻量回归测试；只使用 dry-run 和 mock 命令，不触碰真实服务。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT_UNDER_TEST="${SCRIPT_DIR}/restart-sub2api.sh"

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

test_help_mentions_api_interruption() {
  local output
  output="$(bash "${SCRIPT_UNDER_TEST}" --help)"
  assert_contains "${output}" "https://api.aaccx.pw/v1"
  assert_contains "${output}" "--dry-run"
}

test_compose_dry_run_restarts_only_sub2api() {
  local temp_dir output
  temp_dir="$(mktemp -d)"
  trap 'rm -rf "${temp_dir}"' RETURN
  touch "${temp_dir}/docker-compose.yml"

  output="$(bash "${SCRIPT_UNDER_TEST}" --dry-run --yes --backend compose --compose-file "${temp_dir}/docker-compose.yml")"

  assert_contains "${output}" "docker compose -f ${temp_dir}/docker-compose.yml restart sub2api"
  assert_not_contains "${output}" "postgres"
  assert_not_contains "${output}" "redis"
  assert_not_contains "${output}" "cli-proxy"
  assert_not_contains "${output}" "nginx"
}

test_compose_build_dry_run_uses_no_deps() {
  local temp_dir output
  temp_dir="$(mktemp -d)"
  trap 'rm -rf "${temp_dir}"' RETURN
  touch "${temp_dir}/docker-compose.yml"

  output="$(bash "${SCRIPT_UNDER_TEST}" --dry-run --yes --backend compose --compose-file "${temp_dir}/docker-compose.yml" --build)"

  assert_contains "${output}" "docker compose -f ${temp_dir}/docker-compose.yml up -d --build --no-deps sub2api"
}

test_systemd_dry_run_restarts_unit() {
  local output
  output="$(bash "${SCRIPT_UNDER_TEST}" --dry-run --yes --backend systemd --unit sub2api --no-health-check)"
  assert_contains "${output}" "systemctl restart sub2api"
}

test_mocked_compose_restart_and_health_check() {
  local temp_dir output docker_log curl_log
  temp_dir="$(mktemp -d)"
  trap 'rm -rf "${temp_dir}"' RETURN
  touch "${temp_dir}/docker-compose.yml"
  docker_log="${temp_dir}/docker.log"
  curl_log="${temp_dir}/curl.log"

  cat > "${temp_dir}/docker" <<'MOCK_DOCKER'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "${MOCK_DOCKER_LOG}"
exit 0
MOCK_DOCKER
  chmod +x "${temp_dir}/docker"

  cat > "${temp_dir}/curl" <<'MOCK_CURL'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "${MOCK_CURL_LOG}"
exit 0
MOCK_CURL
  chmod +x "${temp_dir}/curl"

  output="$(
    PATH="${temp_dir}:${PATH}" \
      MOCK_DOCKER_LOG="${docker_log}" \
      MOCK_CURL_LOG="${curl_log}" \
      bash "${SCRIPT_UNDER_TEST}" \
        --backend compose \
        --compose-file "${temp_dir}/docker-compose.yml" \
        --yes \
        --health-url "http://127.0.0.1:18080/health" \
        --timeout 1 \
        --interval 1
  )"

  assert_contains "$(cat "${docker_log}")" "compose -f ${temp_dir}/docker-compose.yml restart sub2api"
  assert_contains "$(cat "${curl_log}")" "http://127.0.0.1:18080/health"
  assert_contains "${output}" "健康检查通过"
}

test_help_mentions_api_interruption
test_compose_dry_run_restarts_only_sub2api
test_compose_build_dry_run_uses_no_deps
test_systemd_dry_run_restarts_unit
test_mocked_compose_restart_and_health_check

echo "restart-sub2api tests passed"
