#!/usr/bin/env bash
# 将已验证候选镜像发布到目标应用容器；默认只替换 18084 candidate 应用容器。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# shellcheck source=lib/common.sh
source "${REPO_ROOT}/deploy/lib/common.sh"
# shellcheck source=lib/targets.sh
source "${REPO_ROOT}/deploy/lib/targets.sh"

DOCKER_BIN="${SUB2API_DOCKER_BIN:-/Applications/Docker.app/Contents/Resources/bin/docker}"
CANDIDATE_IMAGE=""
YES=false
DRY_RUN=false
PROMOTE_TS="${SUB2API_PROMOTE_TS:-$(date +%Y%m%d-%H%M%S)}"
OLD_CONTAINER=""
ENV_FILE=""
TARGET_NETWORK=""
DATA_MOUNT=""

usage() {
  cat <<'EOF'
用法：
  deploy/promote-sub2api-candidate.sh --candidate-image sub2api-candidate:YYYYMMDD-HHMMSS-sha --yes

说明：
  - 只发布已通过候选预演的 sub2api-candidate:* 镜像。
  - 默认目标是 public_candidate_18084：只替换 sub2api-candidate 应用容器。
  - 不重建 Postgres、Redis、nginx 或 Cloudflare Tunnel。
  - 会短暂影响 https://api.aaccx.pw/v1/*，正在运行的流式请求可能断开。

选项：
  --candidate-image IMAGE     候选镜像，必须是 sub2api-candidate:*。
  --yes                       跳过交互确认。
  --dry-run                   只打印目标摘要和将执行的命令。
  --docker-bin PATH           指定 Docker CLI。
  -h, --help                  显示帮助。

环境变量：
  DEPLOY_TARGET               默认 public_candidate_18084；legacy_18080 需要额外确认。
  SUB2API_ALLOW_LEGACY_TARGET 使用 legacy_18080 时必须显式设置 true。
  SUB2API_TARGET_DATA_MOUNT   覆盖 /app/data bind mount，格式为 source:/app/data:rw。
  SUB2API_TARGET_NETWORK      覆盖 Docker 网络。
EOF
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --candidate-image)
        [[ $# -ge 2 ]] || deploy_die "--candidate-image 需要参数"
        CANDIDATE_IMAGE="$2"
        shift 2
        ;;
      --yes)
        YES=true
        shift
        ;;
      --dry-run)
        DRY_RUN=true
        shift
        ;;
      --docker-bin)
        [[ $# -ge 2 ]] || deploy_die "--docker-bin 需要参数"
        DOCKER_BIN="$2"
        shift 2
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        deploy_die "未知参数：$1"
        ;;
    esac
  done
}

require_inputs() {
  [[ -n "${CANDIDATE_IMAGE}" ]] || deploy_die "必须传 --candidate-image"
  [[ "${CANDIDATE_IMAGE}" == sub2api-candidate:* ]] || deploy_die "只允许发布 sub2api-candidate:* 镜像"

  if [[ "${DRY_RUN}" != true ]]; then
    [[ -x "${DOCKER_BIN}" ]] || deploy_die "Docker CLI 不可执行：${DOCKER_BIN}"
  fi
}

first_container_network() {
  "${DOCKER_BIN}" inspect "${DEPLOY_TARGET_APP_CONTAINER}" \
    --format '{{range $name, $_ := .NetworkSettings.Networks}}{{println $name}}{{end}}' |
    sed -n '1p'
}

app_data_mount() {
  "${DOCKER_BIN}" inspect "${DEPLOY_TARGET_APP_CONTAINER}" \
    --format '{{range .Mounts}}{{if eq .Destination "/app/data"}}{{println .Source ":" .Destination ":rw"}}{{end}}{{end}}' |
    tr -d ' ' |
    sed -n '1p'
}

resolve_container_metadata() {
  OLD_CONTAINER="${DEPLOY_TARGET_APP_CONTAINER}-before-promote-${PROMOTE_TS}"
  ENV_FILE="${SUB2API_PROMOTE_ENV_FILE:-/tmp/${DEPLOY_TARGET_APP_CONTAINER}-env-${PROMOTE_TS}.env}"
  TARGET_NETWORK="${SUB2API_TARGET_NETWORK:-${DEPLOY_TARGET_NETWORK}}"
  DATA_MOUNT="${SUB2API_TARGET_DATA_MOUNT:-}"

  if [[ "${DRY_RUN}" == true ]]; then
    [[ -n "${DATA_MOUNT}" ]] || DATA_MOUNT="/tmp/${DEPLOY_TARGET_APP_CONTAINER}-data:/app/data:rw"
    deploy_log "DRY-RUN: would export ${DEPLOY_TARGET_APP_CONTAINER} env to ${ENV_FILE}"
    return 0
  fi

  "${DOCKER_BIN}" container inspect "${DEPLOY_TARGET_APP_CONTAINER}" >/dev/null
  "${DOCKER_BIN}" image inspect "${CANDIDATE_IMAGE}" >/dev/null

  deploy_log "当前应用镜像 ID：$("${DOCKER_BIN}" inspect "${DEPLOY_TARGET_APP_CONTAINER}" --format '{{.Image}}')"
  deploy_log "候选镜像 ID：$("${DOCKER_BIN}" image inspect "${CANDIDATE_IMAGE}" --format '{{.Id}}')"

  "${DOCKER_BIN}" inspect "${DEPLOY_TARGET_APP_CONTAINER}" --format '{{range .Config.Env}}{{println .}}{{end}}' > "${ENV_FILE}"
  chmod 600 "${ENV_FILE}"

  if [[ -z "${TARGET_NETWORK}" ]]; then
    TARGET_NETWORK="$(first_container_network)"
  fi
  [[ -n "${TARGET_NETWORK}" ]] || deploy_die "无法解析目标容器 Docker 网络"

  if [[ -z "${DATA_MOUNT}" ]]; then
    DATA_MOUNT="$(app_data_mount)"
  fi
  [[ -n "${DATA_MOUNT}" ]] || deploy_die "无法解析目标容器 /app/data mount，请设置 SUB2API_TARGET_DATA_MOUNT"
}

confirm_promote() {
  deploy_warn "即将替换 ${DEPLOY_TARGET_APP_CONTAINER} 应用容器，正在运行的流式请求可能断开。"
  deploy_warn "不会重建 ${DEPLOY_TARGET_POSTGRES_CONTAINER} 或 ${DEPLOY_TARGET_REDIS_CONTAINER}。"

  if [[ "${YES}" == true ]]; then
    return 0
  fi

  printf '请输入 yes 继续：'
  read -r reply
  [[ "${reply}" == "yes" ]] || deploy_die "已取消"
}

run_new_container() {
  deploy_run_cmd "${DOCKER_BIN}" run -d \
    --name "${DEPLOY_TARGET_APP_CONTAINER}" \
    --restart=no \
    --ulimit nofile=100000:100000 \
    --env-file "${ENV_FILE}" \
    -p "127.0.0.1:${DEPLOY_TARGET_PORT}:8080" \
    -v "${DATA_MOUNT}" \
    --network "${TARGET_NETWORK}" \
    --health-cmd "wget -q -T 5 -O /dev/null http://localhost:8080/health" \
    --health-interval 10s \
    --health-timeout 5s \
    --health-retries 6 \
    --health-start-period 20s \
    "${CANDIDATE_IMAGE}"
}

replace_app_container() {
  deploy_run_cmd "${DOCKER_BIN}" stop "${DEPLOY_TARGET_APP_CONTAINER}"
  deploy_run_cmd "${DOCKER_BIN}" rename "${DEPLOY_TARGET_APP_CONTAINER}" "${OLD_CONTAINER}"
  run_new_container
}

wait_health() {
  if [[ "${DRY_RUN}" == true ]]; then
    deploy_run_cmd curl -fsS --max-time 5 "${DEPLOY_TARGET_HEALTH_URL}"
    return 0
  fi

  local deadline
  deadline=$((SECONDS + 120))
  while (( SECONDS <= deadline )); do
    if curl -fsS --max-time 5 "${DEPLOY_TARGET_HEALTH_URL}" >/dev/null; then
      deploy_log "健康检查通过：${DEPLOY_TARGET_HEALTH_URL}"
      return 0
    fi
    sleep 2
  done

  "${DOCKER_BIN}" logs --tail 120 "${DEPLOY_TARGET_APP_CONTAINER}" >&2 || true
  deploy_die "健康检查超时：${DEPLOY_TARGET_HEALTH_URL}"
}

cleanup_env_file() {
  [[ "${DRY_RUN}" == true ]] && return 0
  rm -f "${ENV_FILE}"
}

main() {
  parse_args "$@"
  require_inputs
  load_deploy_target
  require_legacy_target_confirmation
  print_deploy_target_summary
  resolve_container_metadata
  confirm_promote
  replace_app_container
  wait_health
  cleanup_env_file
  deploy_log "公网发布完成。old_container=${OLD_CONTAINER} new_image=${CANDIDATE_IMAGE}"
}

main "$@"
