#!/usr/bin/env bash
# 一键重启 Sub2API 本体；默认不碰 Postgres、Redis、CLIProxyAPI、nginx 或 Cloudflare Tunnel。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

BACKEND="${SUB2API_RESTART_BACKEND:-auto}"
COMPOSE_FILE="${SUB2API_COMPOSE_FILE:-}"
SERVICE="${SUB2API_SERVICE:-sub2api}"
CONTAINER="${SUB2API_CONTAINER:-sub2api}"
SYSTEMD_UNIT="${SUB2API_SYSTEMD_UNIT:-sub2api}"
HEALTH_URL="${SUB2API_HEALTH_URL:-http://127.0.0.1:18080/health}"
TIMEOUT_SECONDS="${SUB2API_RESTART_TIMEOUT:-60}"
INTERVAL_SECONDS="${SUB2API_RESTART_INTERVAL:-2}"
BUILD=false
YES=false
DRY_RUN=false
NO_HEALTH_CHECK=false

usage() {
  cat <<'EOF'
用法：
  deploy/restart-sub2api.sh [选项]

说明：
  重启 Sub2API 会短暂影响 https://api.aaccx.pw/v1/*。
  正在运行的 Codex 流式请求可能断开；请在确认不需要当前会话继续操作时执行。

默认行为：
  - 只重启 Sub2API 本体。
  - 不重启 Postgres、Redis、CLIProxyAPI、nginx、Cloudflare Tunnel。
  - 重启后检查 http://127.0.0.1:18080/health。

选项：
  --yes                       跳过交互确认
  --dry-run                   只打印将执行的命令，不执行重启
  --build                     Docker Compose 模式下重建并重启 Sub2API
                              仅在 Compose 服务包含 build 配置时会从源码重建
  --backend auto|compose|container|systemd
                              指定重启后端，默认 auto
  --compose-file PATH         指定 docker-compose 文件
  --service NAME              指定 Compose 服务名，默认 sub2api
  --container NAME            指定 Docker 容器名，默认 sub2api
  --unit NAME                 指定 systemd unit，默认 sub2api
  --health-url URL            指定健康检查 URL
  --timeout SECONDS           指定健康检查超时秒数，默认 60
  --interval SECONDS          指定健康检查间隔秒数，默认 2
  --no-health-check           跳过重启后的健康检查
  -h, --help                  显示帮助

示例：
  deploy/restart-sub2api.sh --dry-run --yes
  deploy/restart-sub2api.sh --yes
  deploy/restart-sub2api.sh --yes --build --compose-file deploy/docker-compose.local.yml
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

format_cmd() {
  local arg
  local rendered=""
  for arg in "$@"; do
    if [[ -z "${rendered}" ]]; then
      printf -v rendered '%q' "${arg}"
    else
      local quoted
      printf -v quoted '%q' "${arg}"
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

resolve_compose_file() {
  if [[ -n "${COMPOSE_FILE}" ]]; then
    if [[ "${COMPOSE_FILE}" != /* ]]; then
      COMPOSE_FILE="${REPO_ROOT}/${COMPOSE_FILE}"
    fi
    return
  fi

  if [[ -f "${REPO_ROOT}/deploy/docker-compose.local.yml" ]]; then
    COMPOSE_FILE="${REPO_ROOT}/deploy/docker-compose.local.yml"
    return
  fi

  if [[ -f "${REPO_ROOT}/deploy/docker-compose.yml" ]]; then
    COMPOSE_FILE="${REPO_ROOT}/deploy/docker-compose.yml"
    return
  fi
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
      --build)
        BUILD=true
        shift
        ;;
      --backend)
        [[ $# -ge 2 ]] || die "--backend 需要参数"
        BACKEND="$2"
        shift 2
        ;;
      --compose-file)
        [[ $# -ge 2 ]] || die "--compose-file 需要参数"
        COMPOSE_FILE="$2"
        shift 2
        ;;
      --service)
        [[ $# -ge 2 ]] || die "--service 需要参数"
        SERVICE="$2"
        shift 2
        ;;
      --container)
        [[ $# -ge 2 ]] || die "--container 需要参数"
        CONTAINER="$2"
        shift 2
        ;;
      --unit)
        [[ $# -ge 2 ]] || die "--unit 需要参数"
        SYSTEMD_UNIT="$2"
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

validate_args() {
  case "${BACKEND}" in
    auto|compose|container|systemd) ;;
    *) die "--backend 只能是 auto、compose、container 或 systemd" ;;
  esac

  is_positive_integer "${TIMEOUT_SECONDS}" || die "--timeout 必须是正整数"
  is_positive_integer "${INTERVAL_SECONDS}" || die "--interval 必须是正整数"
}

detect_backend() {
  if [[ "${BACKEND}" != "auto" ]]; then
    printf '%s' "${BACKEND}"
    return
  fi

  if [[ -n "${COMPOSE_FILE}" && -f "${COMPOSE_FILE}" ]]; then
    printf 'compose'
    return
  fi

  if command_exists docker; then
    printf 'container'
    return
  fi

  if command_exists systemctl; then
    printf 'systemd'
    return
  fi

  die "无法自动识别重启方式：未找到可用的 Docker Compose、Docker 容器或 systemd"
}

validate_runtime() {
  local selected_backend="$1"

  case "${selected_backend}" in
    compose)
      [[ -n "${COMPOSE_FILE}" ]] || die "Compose 模式需要 --compose-file 或 deploy/docker-compose.local.yml"
      [[ -f "${COMPOSE_FILE}" ]] || die "Compose 文件不存在：${COMPOSE_FILE}"
      if [[ "${DRY_RUN}" != true ]]; then
        command_exists docker || die "未找到 docker 命令"
      fi
      ;;
    container)
      [[ "${BUILD}" != true ]] || die "container 模式不支持 --build；请使用 --backend compose"
      if [[ "${DRY_RUN}" != true ]]; then
        command_exists docker || die "未找到 docker 命令"
      fi
      ;;
    systemd)
      [[ "${BUILD}" != true ]] || die "systemd 模式不支持 --build；请先构建并替换二进制"
      if [[ "${DRY_RUN}" != true ]]; then
        command_exists systemctl || die "未找到 systemctl 命令"
      fi
      ;;
    *)
      die "未知重启方式：${selected_backend}"
      ;;
  esac

  if [[ "${NO_HEALTH_CHECK}" != true && "${DRY_RUN}" != true ]]; then
    command_exists curl || die "未找到 curl 命令，无法执行健康检查"
  fi
}

print_plan() {
  local selected_backend="$1"

  warn "即将重启 Sub2API，本操作会短暂影响 https://api.aaccx.pw/v1/*。"
  warn "正在进行的 Codex 流式请求可能断开。"
  log "重启方式：${selected_backend}"

  case "${selected_backend}" in
    compose)
      log "Compose 文件：${COMPOSE_FILE}"
      log "Compose 服务：${SERVICE}"
      ;;
    container)
      log "Docker 容器：${CONTAINER}"
      ;;
    systemd)
      log "systemd unit：${SYSTEMD_UNIT}"
      ;;
  esac

  if [[ "${NO_HEALTH_CHECK}" == true ]]; then
    log "健康检查：跳过"
  else
    log "健康检查：${HEALTH_URL}"
  fi
}

confirm_restart() {
  if [[ "${YES}" == true || "${DRY_RUN}" == true ]]; then
    return
  fi

  printf '确认现在重启 Sub2API？请输入 yes 继续：'
  local reply
  read -r reply
  if [[ "${reply}" != "yes" ]]; then
    warn "已取消。"
    exit 130
  fi
}

restart_with_compose() {
  if [[ "${BUILD}" == true ]]; then
    warn "--build 仅在 Compose 服务包含 build 配置时会从源码重建；image-only Compose 可能只会重建已有配置。"
    run_cmd docker compose -f "${COMPOSE_FILE}" up -d --build --no-deps "${SERVICE}"
    return
  fi

  run_cmd docker compose -f "${COMPOSE_FILE}" restart "${SERVICE}"
}

restart_with_container() {
  run_cmd docker restart "${CONTAINER}"
}

restart_with_systemd() {
  local cmd=(systemctl restart "${SYSTEMD_UNIT}")

  if [[ "${DRY_RUN}" != true && "${EUID}" -ne 0 ]] && command_exists sudo; then
    cmd=(sudo "${cmd[@]}")
  fi

  run_cmd "${cmd[@]}"
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

  local start_time="${SECONDS}"
  local deadline=$((start_time + TIMEOUT_SECONDS))

  while (( SECONDS <= deadline )); do
    if curl -fsS --max-time 5 "${HEALTH_URL}" >/dev/null; then
      log "健康检查通过：${HEALTH_URL}"
      return
    fi
    sleep "${INTERVAL_SECONDS}"
  done

  die "健康检查超时：${HEALTH_URL}。请查看 Sub2API 日志。"
}

main() {
  parse_args "$@"
  resolve_compose_file
  validate_args

  local selected_backend
  selected_backend="$(detect_backend)"
  validate_runtime "${selected_backend}"
  print_plan "${selected_backend}"
  confirm_restart

  case "${selected_backend}" in
    compose)
      restart_with_compose
      ;;
    container)
      restart_with_container
      ;;
    systemd)
      restart_with_systemd
      ;;
  esac

  wait_for_health
  log "Sub2API 重启流程结束。"
}

main "$@"
