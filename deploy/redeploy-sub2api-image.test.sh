#!/usr/bin/env bash
# redeploy-sub2api-image.sh 的轻量回归测试；只使用 dry-run 和 mock 命令，不触碰真实 Docker。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT_UNDER_TEST="${SCRIPT_DIR}/redeploy-sub2api-image.sh"

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

make_temp_deploy() {
  local temp_dir="$1"
  mkdir -p "${temp_dir}/deploy"
  touch "${temp_dir}/Dockerfile"
  touch "${temp_dir}/deploy/docker-compose.local.yml"
  printf 'POSTGRES_PASSWORD=dummy\n' > "${temp_dir}/deploy/.env.scheme-a.local"
}

test_help_mentions_detach_and_api_interruption() {
  local output
  output="$(bash "${SCRIPT_UNDER_TEST}" --help)"
  assert_contains "${output}" "https://api.aaccx.pw/v1"
  assert_contains "${output}" "detached"
  assert_contains "${output}" "--dry-run"
}

test_dry_run_builds_image_and_recreates_only_sub2api() {
  local temp_dir output
  temp_dir="$(mktemp -d)"
  trap 'rm -rf "${temp_dir}"' RETURN
  make_temp_deploy "${temp_dir}"

  output="$(
    bash "${SCRIPT_UNDER_TEST}" \
      --dry-run \
      --yes \
      --foreground \
      --context "${temp_dir}" \
      --dockerfile "${temp_dir}/Dockerfile" \
      --compose-file "${temp_dir}/deploy/docker-compose.local.yml" \
      --env-file "${temp_dir}/deploy/.env.scheme-a.local" \
      --docker-bin docker \
      --image "weishaw/sub2api:latest"
  )"

  assert_contains "${output}" "docker build --build-arg GOPROXY=https://goproxy.cn\\,direct --build-arg GOSUMDB=sum.golang.google.cn -t weishaw/sub2api:latest -f ${temp_dir}/Dockerfile ${temp_dir}"
  assert_contains "${output}" "docker compose --env-file ${temp_dir}/deploy/.env.scheme-a.local -f ${temp_dir}/deploy/docker-compose.local.yml up -d --no-deps --force-recreate sub2api"
  assert_not_contains "${output}" "postgres"
  assert_not_contains "${output}" "redis"
  assert_not_contains "${output}" "cli-proxy"
  assert_not_contains "${output}" "nginx"
}

test_dry_run_default_detached_prints_log_path() {
  local temp_dir output
  temp_dir="$(mktemp -d)"
  trap 'rm -rf "${temp_dir}"' RETURN
  make_temp_deploy "${temp_dir}"

  output="$(
    bash "${SCRIPT_UNDER_TEST}" \
      --dry-run \
      --yes \
      --context "${temp_dir}" \
      --dockerfile "${temp_dir}/Dockerfile" \
      --compose-file "${temp_dir}/deploy/docker-compose.local.yml" \
      --env-file "${temp_dir}/deploy/.env.scheme-a.local" \
      --docker-bin docker
  )"

  assert_contains "${output}" "detached"
  assert_contains "${output}" "deploy/logs/redeploy-sub2api-"
  assert_contains "${output}" "--foreground --yes"
}

test_mocked_foreground_redeploy_and_health_check() {
  local temp_dir output docker_log curl_log
  temp_dir="$(mktemp -d)"
  trap 'rm -rf "${temp_dir}"' RETURN
  make_temp_deploy "${temp_dir}"
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
        --foreground \
        --yes \
        --context "${temp_dir}" \
        --dockerfile "${temp_dir}/Dockerfile" \
        --compose-file "${temp_dir}/deploy/docker-compose.local.yml" \
        --env-file "${temp_dir}/deploy/.env.scheme-a.local" \
        --health-url "http://127.0.0.1:18080/health" \
        --timeout 1 \
        --interval 1
  )"

  assert_contains "$(cat "${docker_log}")" "build --build-arg GOPROXY=https://goproxy.cn,direct --build-arg GOSUMDB=sum.golang.google.cn -t weishaw/sub2api:latest -f ${temp_dir}/Dockerfile ${temp_dir}"
  assert_contains "$(cat "${docker_log}")" "compose --env-file ${temp_dir}/deploy/.env.scheme-a.local -f ${temp_dir}/deploy/docker-compose.local.yml up -d --no-deps --force-recreate sub2api"
  assert_contains "$(cat "${curl_log}")" "http://127.0.0.1:18080/health"
  assert_contains "${output}" "健康检查通过"
}

test_help_mentions_detach_and_api_interruption
test_dry_run_builds_image_and_recreates_only_sub2api
test_dry_run_default_detached_prints_log_path
test_mocked_foreground_redeploy_and_health_check

echo "redeploy-sub2api-image tests passed"
