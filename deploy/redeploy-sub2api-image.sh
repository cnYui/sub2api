#!/usr/bin/env bash
# 构建新 Sub2API 镜像并只重建 Sub2API 容器；默认 detached，避免 API 重启导致调用方断联后流程中断。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

IMAGE="${SUB2API_IMAGE:-weishaw/sub2api:latest}"
COMPOSE_FILE="${SUB2API_COMPOSE_FILE:-}"
ENV_FILE="${SUB2API_ENV_FILE:-}"
SERVICE="${SUB2API_SERVICE:-sub2api}"
DOCKERFILE="${SUB2API_DOCKERFILE:-}"
BUILD_CONTEXT="${SUB2API_BUILD_CONTEXT:-}"
HEALTH_URL="${SUB2API_HEALTH_URL:-http://127.0.0.1:18080/health}"
TIMEOUT_SECONDS="${SUB2API_REDEPLOY_TIMEOUT:-90}"
INTERVAL_SECONDS="${SUB2API_REDEPLOY_INTERVAL:-2}"
LOG_DIR="${SUB2API_REDEPLOY_LOG_DIR:-}"
DOCKER_BIN="${SUB2API_DOCKER_BIN:-}"
YES=false
DRY_RUN=false
FOREGROUND=false
NO_HEALTH_CHECK=false

usage() {
  cat <<'EOF'
用法：
  deploy/redeploy-sub2api-image.sh [选项]

说明：
  本脚本会构建新镜像并重建 Sub2API 容器，因此会短暂影响 https://api.aaccx.pw/v1/*。
  正在运行的 Codex 流式请求可能断开。

建议：
  常规公网发布先执行 deploy/rehearse-sub2api-candidate.sh --reset-db，
  候选预演通过后再使用 deploy/promote-sub2api-candidate.sh 发布。
  本脚本保留为应急直接替换入口。

默认行为：
  - 默认 detached 后台执行，日志写入 deploy/logs/redeploy-sub2api-*.log。
  - 构建镜像 weishaw/sub2api:latest。
  - 使用 deploy/docker-compose.local.yml 重建 sub2api 服务。
  - 自动读取 deploy/.env；若不存在则读取 deploy/.env.scheme-a.local。
  - 只重建 Sub2API，不重启 Postgres、Redis、CLIProxyAPI、nginx、Cloudflare Tunnel。

选项：
  --yes                       跳过交互确认
  --dry-run                   只打印将执行的命令，不构建、不重启
  --foreground                前台执行；默认真实运行会 detached 到后台
  --image IMAGE               镜像标签，默认 weishaw/sub2api:latest
  --compose-file PATH         指定 docker-compose 文件
  --env-file PATH             指定 Compose env 文件
  --service NAME              指定 Compose 服务名，默认 sub2api
  --dockerfile PATH           指定 Dockerfile，默认仓库根目录 Dockerfile
  --context PATH              指定 docker build context，默认仓库根目录
  --health-url URL            指定健康检查 URL
  --timeout SECONDS           指定健康检查超时秒数，默认 90
  --interval SECONDS          指定健康检查间隔秒数，默认 2
  --log-dir PATH              指定 detached 日志目录，默认 deploy/logs
  --docker-bin PATH           指定 Docker CLI 路径
  --no-health-check           跳过重建后的健康检查
  -h, --help                  显示帮助

常用：
  ./deploy/redeploy-sub2api-image.sh --dry-run --yes
  ./deploy/redeploy-sub2api-image.sh --yes
  ./deploy/redeploy-sub2api-image.sh --yes --foreground
EOF
}

log() {
  printf '[INFO] %s\n' "$*"
}

warn() {
  printf '[WARN] %s\n' "$*" >&2
}

die() {
  printf '[ERROR] %s\n' "$*" >&2
  exit 1
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

is_positive_integer() {
  [[ "$1" =~ ^[1-9][0-9]*$ ]]
}

abs_path() {
  local path="$1"
  if [[ "${path}" == /* ]]; then
    printf '%s' "${path}"
    return
  fi
  printf '%s/%s' "${REPO_ROOT}" "${path}"
}

format_cmd() {
  local arg rendered quoted
  rendered=""
  for arg in "$@"; do
    printf -v quoted '%q' "${arg}"
    if [[ -z "${rendered}" ]]; then
      rendered="${quoted}"
    else
      rendered="${rendered} ${quoted}"
    fi
  done
  printf '%s' "${rendered}"
}

run_cmd() {
  if [[ "${DRY_RUN}" == true ]]; then
    printf '[DRY-RUN] %s\n' "$(format_cmd "$@")"
    return 0
  fi

  printf '+ %s\n' "$(format_cmd "$@")"
  "$@"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --yes)
        YES=true
        shift
        ;;
      --dry-run)
        DRY_RUN=true
        shift
        ;;
      --foreground)
        FOREGROUND=true
        shift
        ;;
      --image)
        [[ $# -ge 2 ]] || die "--image 需要参数"
        IMAGE="$2"
        shift 2
        ;;
      --compose-file)
        [[ $# -ge 2 ]] || die "--compose-file 需要参数"
        COMPOSE_FILE="$2"
        shift 2
        ;;
      --env-file)
        [[ $# -ge 2 ]] || die "--env-file 需要参数"
        ENV_FILE="$2"
        shift 2
        ;;
      --service)
        [[ $# -ge 2 ]] || die "--service 需要参数"
        SERVICE="$2"
        shift 2
        ;;
      --dockerfile)
        [[ $# -ge 2 ]] || die "--dockerfile 需要参数"
        DOCKERFILE="$2"
        shift 2
        ;;
      --context)
        [[ $# -ge 2 ]] || die "--context 需要参数"
        BUILD_CONTEXT="$2"
        shift 2
        ;;
      --health-url)
        [[ $# -ge 2 ]] || die "--health-url 需要参数"
        HEALTH_URL="$2"
        shift 2
        ;;
      --timeout)
        [[ $# -ge 2 ]] || die "--timeout 需要参数"
        TIMEOUT_SECONDS="$2"
        shift 2
        ;;
      --interval)
        [[ $# -ge 2 ]] || die "--interval 需要参数"
        INTERVAL_SECONDS="$2"
        shift 2
        ;;
      --log-dir)
        [[ $# -ge 2 ]] || die "--log-dir 需要参数"
        LOG_DIR="$2"
        shift 2
        ;;
      --docker-bin)
        [[ $# -ge 2 ]] || die "--docker-bin 需要参数"
        DOCKER_BIN="$2"
        shift 2
        ;;
      --no-health-check)
        NO_HEALTH_CHECK=true
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die "未知参数：$1"
        ;;
    esac
  done
}

resolve_paths() {
  [[ -n "${COMPOSE_FILE}" ]] || COMPOSE_FILE="${REPO_ROOT}/deploy/docker-compose.local.yml"
  [[ -n "${DOCKERFILE}" ]] || DOCKERFILE="${REPO_ROOT}/Dockerfile"
  [[ -n "${BUILD_CONTEXT}" ]] || BUILD_CONTEXT="${REPO_ROOT}"
  [[ -n "${LOG_DIR}" ]] || LOG_DIR="${REPO_ROOT}/deploy/logs"

  COMPOSE_FILE="$(abs_path "${COMPOSE_FILE}")"
  DOCKERFILE="$(abs_path "${DOCKERFILE}")"
  BUILD_CONTEXT="$(abs_path "${BUILD_CONTEXT}")"
  LOG_DIR="$(abs_path "${LOG_DIR}")"

  if [[ -n "${ENV_FILE}" ]]; then
    ENV_FILE="$(abs_path "${ENV_FILE}")"
  elif [[ -f "${REPO_ROOT}/deploy/.env" ]]; then
    ENV_FILE="${REPO_ROOT}/deploy/.env"
  elif [[ -f "${REPO_ROOT}/deploy/.env.scheme-a.local" ]]; then
    ENV_FILE="${REPO_ROOT}/deploy/.env.scheme-a.local"
  fi
}

resolve_docker_bin() {
  if [[ -n "${DOCKER_BIN}" ]]; then
    [[ "${DRY_RUN}" == true || -x "${DOCKER_BIN}" ]] || die "Docker CLI 不可执行：${DOCKER_BIN}"
    return
  fi

  if command_exists docker; then
    DOCKER_BIN="$(command -v docker)"
    return
  fi

  if [[ -x "/Applications/Docker.app/Contents/Resources/bin/docker" ]]; then
    DOCKER_BIN="/Applications/Docker.app/Contents/Resources/bin/docker"
    return
  fi

  die "未找到 docker 命令；可用 --docker-bin 指定 Docker CLI 路径"
}

validate_args() {
  is_positive_integer "${TIMEOUT_SECONDS}" || die "--timeout 必须是正整数"
  is_positive_integer "${INTERVAL_SECONDS}" || die "--interval 必须是正整数"
  [[ -f "${COMPOSE_FILE}" ]] || die "Compose 文件不存在：${COMPOSE_FILE}"
  [[ -f "${DOCKERFILE}" ]] || die "Dockerfile 不存在：${DOCKERFILE}"
  [[ -d "${BUILD_CONTEXT}" ]] || die "build context 不存在：${BUILD_CONTEXT}"
  [[ -n "${ENV_FILE}" ]] || die "未找到 Compose env 文件；请传 --env-file"
  [[ -f "${ENV_FILE}" ]] || die "Compose env 文件不存在：${ENV_FILE}"
}

print_plan() {
  warn "即将构建新镜像并重建 Sub2API 容器，本操作会短暂影响 https://api.aaccx.pw/v1/*。"
  warn "正在进行的 Codex 流式请求可能断开。"
  log "镜像：${IMAGE}"
  log "Dockerfile：${DOCKERFILE}"
  log "Build context：${BUILD_CONTEXT}"
  log "Compose 文件：${COMPOSE_FILE}"
  log "Compose env：${ENV_FILE}"
  log "Compose 服务：${SERVICE}"
  if [[ "${NO_HEALTH_CHECK}" == true ]]; then
    log "健康检查：跳过"
  else
    log "健康检查：${HEALTH_URL}"
  fi
}

confirm_redeploy() {
  if [[ "${YES}" == true || "${DRY_RUN}" == true ]]; then
    return
  fi

  printf '确认现在构建镜像并替换 Sub2API 容器？请输入 yes 继续：'
  local reply
  read -r reply
  if [[ "${reply}" != "yes" ]]; then
    warn "已取消。"
    exit 130
  fi
}

detached_args() {
  local args=()
  args+=(--foreground --yes)
  args+=(--image "${IMAGE}")
  args+=(--compose-file "${COMPOSE_FILE}")
  args+=(--env-file "${ENV_FILE}")
  args+=(--service "${SERVICE}")
  args+=(--dockerfile "${DOCKERFILE}")
  args+=(--context "${BUILD_CONTEXT}")
  args+=(--health-url "${HEALTH_URL}")
  args+=(--timeout "${TIMEOUT_SECONDS}")
  args+=(--interval "${INTERVAL_SECONDS}")
  args+=(--log-dir "${LOG_DIR}")
  args+=(--docker-bin "${DOCKER_BIN}")
  if [[ "${NO_HEALTH_CHECK}" == true ]]; then
    args+=(--no-health-check)
  fi
  printf '%s\0' "${args[@]}"
}

maybe_spawn_detached() {
  if [[ "${FOREGROUND}" == true ]]; then
    return
  fi

  mkdir -p "${LOG_DIR}"
  local log_file="${LOG_DIR}/redeploy-sub2api-$(date +%Y%m%d-%H%M%S).log"
  local args=()
  while IFS= read -r -d '' arg; do
    args+=("${arg}")
  done < <(detached_args)

  if [[ "${DRY_RUN}" == true ]]; then
    log "默认真实运行会以 detached 后台方式执行。"
    log "日志路径：${log_file}"
    printf '[DRY-RUN] nohup %s > %s 2>&1 &\n' \
      "$(format_cmd "$0" "${args[@]}")" \
      "$(format_cmd "${log_file}")"
    return
  fi

  log "将以 detached 后台方式执行，当前连接断开也会继续。"
  log "日志路径：${log_file}"
  nohup "$0" "${args[@]}" > "${log_file}" 2>&1 &
  local pid="$!"
  log "后台 PID：${pid}"
  log "查看日志：tail -f ${log_file}"
  exit 0
}

build_image() {
  run_cmd "${DOCKER_BIN}" build \
    --build-arg "GOPROXY=https://goproxy.cn,direct" \
    --build-arg "GOSUMDB=sum.golang.google.cn" \
    -t "${IMAGE}" \
    -f "${DOCKERFILE}" \
    "${BUILD_CONTEXT}"
}

recreate_sub2api() {
  run_cmd "${DOCKER_BIN}" compose \
    --env-file "${ENV_FILE}" \
    -f "${COMPOSE_FILE}" \
    up -d --no-deps --force-recreate "${SERVICE}"
}

wait_for_health() {
  if [[ "${NO_HEALTH_CHECK}" == true ]]; then
    log "已跳过健康检查。"
    return
  fi

  if [[ "${DRY_RUN}" == true ]]; then
    printf '[DRY-RUN] wait until healthy: curl -fsS --max-time 5 %s\n' "${HEALTH_URL}"
    return
  fi

  command_exists curl || die "未找到 curl 命令，无法执行健康检查"

  local start_time="${SECONDS}"
  local deadline=$((start_time + TIMEOUT_SECONDS))

  while (( SECONDS <= deadline )); do
    if curl -fsS --max-time 5 "${HEALTH_URL}" >/dev/null; then
      log "健康检查通过：${HEALTH_URL}"
      return
    fi
    sleep "${INTERVAL_SECONDS}"
  done

  die "健康检查超时：${HEALTH_URL}。请查看日志和 Sub2API 容器状态。"
}

main() {
  parse_args "$@"
  resolve_paths
  resolve_docker_bin
  validate_args
  print_plan
  confirm_redeploy
  maybe_spawn_detached

  build_image
  recreate_sub2api
  wait_for_health
  log "Sub2API 镜像替换发布流程结束。"
}

main "$@"
