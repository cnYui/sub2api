#!/usr/bin/env bash
# 构建并启动 Sub2API 候选预演环境；不替换公网容器，不访问公网 /v1。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

DOCKER_BIN="${SUB2API_DOCKER_BIN:-/Applications/Docker.app/Contents/Resources/bin/docker}"
COMPOSE_FILE="${REPO_ROOT}/deploy/docker-compose.candidate.yml"
COMPOSE_PROJECT_NAME="${SUB2API_CANDIDATE_PROJECT_NAME:-sub2api-candidate-rehearsal}"
ENV_FILE="${REPO_ROOT}/deploy/.env.candidate.local"
ENV_EXAMPLE_FILE="${REPO_ROOT}/deploy/.env.candidate.local.example"
PUBLIC_COMPOSE_FILE="${REPO_ROOT}/deploy/docker-compose.local.yml"
MAIN_WT=""
CANDIDATE_IMAGE=""
CANDIDATE_PORT="${CANDIDATE_PORT:-18081}"
RESET_DB=false
DRY_RUN=false
ALLOW_GATEWAY_SMOKE=false

log() { printf '[INFO] %s\n' "$*"; }
warn() { printf '[WARN] %s\n' "$*" >&2; }
die() { printf '[ERROR] %s\n' "$*" >&2; exit 1; }

usage() {
  cat <<'EOF'
用法：
  deploy/rehearse-sub2api-candidate.sh --reset-db

说明：
  - 构建 sub2api-candidate:<timestamp>-<sha>。
  - 从当前生产 Postgres dump 数据到 deploy/candidate/dumps/。
  - 恢复到 sub2api-candidate-postgres。
  - 执行 deploy/sql/candidate-sanitize.sql。
  - 启动 sub2api-candidate 到 127.0.0.1:18081。
  - 默认只做本地只读验证，不访问公网 /v1，也不访问候选 /v1。

选项：
  --reset-db                  删除并重建候选 DB 数据目录；首次运行必须使用
  --dry-run                   打印计划，不构建、不启动、不 dump
  --allow-gateway-smoke       允许对候选 18081 执行显式 /v1 smoke test
  --docker-bin PATH           指定 Docker CLI
  --env-file PATH             指定候选 env 文件
  --candidate-port PORT       指定候选宿主端口，默认 18081
  -h, --help                  显示帮助
EOF
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --reset-db) RESET_DB=true; shift ;;
      --dry-run) DRY_RUN=true; shift ;;
      --allow-gateway-smoke) ALLOW_GATEWAY_SMOKE=true; shift ;;
      --docker-bin)
        [[ $# -ge 2 ]] || die "--docker-bin 需要参数"
        DOCKER_BIN="$2"
        shift 2
        ;;
      --env-file)
        [[ $# -ge 2 ]] || die "--env-file 需要参数"
        ENV_FILE="$2"
        shift 2
        ;;
      --candidate-port)
        [[ $# -ge 2 ]] || die "--candidate-port 需要参数"
        CANDIDATE_PORT="$2"
        shift 2
        ;;
      -h|--help) usage; exit 0 ;;
      *) die "未知参数：$1" ;;
    esac
  done
}

run_cmd() {
  if [[ "${DRY_RUN}" == true ]]; then
    printf '[DRY-RUN]'
    printf ' %q' "$@"
    printf '\n'
    return 0
  fi

  printf '+'
  printf ' %q' "$@"
  printf '\n'
  "$@"
}

require_files() {
  [[ -x "${DOCKER_BIN}" ]] || die "Docker CLI 不可执行：${DOCKER_BIN}"
  [[ -f "${COMPOSE_FILE}" ]] || die "候选 compose 不存在：${COMPOSE_FILE}"
  if [[ ! -f "${ENV_FILE}" ]]; then
    if [[ "${DRY_RUN}" == true && -f "${ENV_EXAMPLE_FILE}" ]]; then
      warn "候选 env 不存在，dry-run 使用 example：${ENV_EXAMPLE_FILE}"
      ENV_FILE="${ENV_EXAMPLE_FILE}"
    else
      die "候选 env 不存在：${ENV_FILE}；请复制 deploy/.env.candidate.local.example"
    fi
  fi
  [[ -f "${PUBLIC_COMPOSE_FILE}" ]] || die "公网 compose 不存在：${PUBLIC_COMPOSE_FILE}"
  [[ "${CANDIDATE_PORT}" =~ ^[0-9]+$ ]] || die "候选端口必须是数字"
  [[ "${CANDIDATE_PORT}" != "18080" ]] || die "候选端口不能使用公网 18080"
}

find_main_worktree() {
  local current="" wt="" branch
  while IFS= read -r line; do
    case "${line}" in
      worktree\ *) wt="${line#worktree }" ;;
      branch\ refs/heads/main)
        current="${wt}"
        break
        ;;
    esac
  done < <(git -C "${REPO_ROOT}" worktree list --porcelain)

  if [[ -z "${current}" ]]; then
    branch="$(git -C "${REPO_ROOT}" rev-parse --abbrev-ref HEAD)"
    [[ "${branch}" == "main" ]] || die "找不到签出 main 的 worktree"
    current="${REPO_ROOT}"
  fi

  MAIN_WT="${current}"
}

ensure_main_worktree_clean() {
  local status conflicts
  status="$(git -C "${MAIN_WT}" status --short --branch)"
  printf '%s\n' "${status}"
  conflicts="$(git -C "${MAIN_WT}" diff --name-only --diff-filter=U)"
  [[ -z "${conflicts}" ]] || die "main worktree 存在冲突：${conflicts}"
}

compute_candidate_image() {
  local sha ts
  sha="$(git -C "${MAIN_WT}" rev-parse --short=12 HEAD)"
  ts="$(date +%Y%m%d-%H%M%S)"
  CANDIDATE_IMAGE="sub2api-candidate:${ts}-${sha}"
}

build_candidate_image() {
  run_cmd "${DOCKER_BIN}" build \
    --build-arg "GOPROXY=https://goproxy.cn,direct" \
    --build-arg "GOSUMDB=sum.golang.google.cn" \
    -t "${CANDIDATE_IMAGE}" \
    -f "${MAIN_WT}/Dockerfile" \
    "${MAIN_WT}"
}

candidate_compose() {
	env CANDIDATE_IMAGE="${CANDIDATE_IMAGE}" CANDIDATE_SERVER_PORT="${CANDIDATE_PORT}" \
		"${DOCKER_BIN}" compose -p "${COMPOSE_PROJECT_NAME}" --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" "$@"
}

run_candidate_compose() {
  if [[ "${DRY_RUN}" == true ]]; then
	printf '[DRY-RUN] env CANDIDATE_IMAGE=%q CANDIDATE_SERVER_PORT=%q %q compose -p %q --env-file %q -f %q' \
		"${CANDIDATE_IMAGE}" "${CANDIDATE_PORT}" "${DOCKER_BIN}" "${COMPOSE_PROJECT_NAME}" "${ENV_FILE}" "${COMPOSE_FILE}"
    printf ' %q' "$@"
    printf '\n'
    return 0
  fi
  candidate_compose "$@"
}

assert_public_container_isolation() {
  local public_containers=(sub2api sub2api-postgres sub2api-redis)
  local container project

  if [[ "${DRY_RUN}" == true ]]; then
    printf '[DRY-RUN] inspect public container compose project labels:'
    printf ' %s' "${public_containers[@]}"
    printf '\n'
    return 0
  fi

  for container in "${public_containers[@]}"; do
    project="$("${DOCKER_BIN}" inspect --format '{{ index .Config.Labels "com.docker.compose.project" }}' "${container}" 2>/dev/null || true)"
    [[ -n "${project}" ]] || die "公网容器不存在或缺少 compose project label：${container}"
    [[ "${project}" != "${COMPOSE_PROJECT_NAME}" ]] || die "候选 project 与公网容器 ${container} 相同：${project}"
  done

  log "公网容器 project label 已确认不属于候选 project：${COMPOSE_PROJECT_NAME}"
}

remove_candidate_containers() {
  local candidate_containers=(sub2api-candidate sub2api-candidate-postgres sub2api-candidate-redis)
  local container

  for container in "${candidate_containers[@]}"; do
    if [[ "${DRY_RUN}" == true ]]; then
      printf '[DRY-RUN] %q rm -f %q\n' "${DOCKER_BIN}" "${container}"
      continue
    fi

    if "${DOCKER_BIN}" container inspect "${container}" >/dev/null 2>&1; then
      run_cmd "${DOCKER_BIN}" rm -f "${container}"
    fi
  done
}

prepare_candidate_dirs() {
  mkdir -p "${REPO_ROOT}/deploy/candidate/dumps" "${REPO_ROOT}/deploy/candidate/logs"
  if [[ "${RESET_DB}" == true ]]; then
    remove_candidate_containers
    run_cmd rm -rf \
      "${REPO_ROOT}/deploy/candidate/postgres_data" \
      "${REPO_ROOT}/deploy/candidate/redis_data" \
      "${REPO_ROOT}/deploy/candidate/data"
  elif [[ ! -d "${REPO_ROOT}/deploy/candidate/postgres_data" ]]; then
    die "候选 DB 不存在；首次运行必须加 --reset-db"
  fi
  run_cmd mkdir -p \
    "${REPO_ROOT}/deploy/candidate/postgres_data" \
    "${REPO_ROOT}/deploy/candidate/redis_data" \
    "${REPO_ROOT}/deploy/candidate/data"
}

dump_public_db() {
  local dump_file
  dump_file="${REPO_ROOT}/deploy/candidate/dumps/sub2api-public-$(date +%Y%m%d-%H%M%S).dump"
  run_cmd "${DOCKER_BIN}" exec sub2api-postgres sh -lc \
    "pg_dump -U \"\$POSTGRES_USER\" -d \"\$POSTGRES_DB\" --format=custom --no-owner --no-privileges --file=/tmp/sub2api-candidate.dump"
  run_cmd "${DOCKER_BIN}" cp sub2api-postgres:/tmp/sub2api-candidate.dump "${dump_file}"
  run_cmd "${DOCKER_BIN}" exec sub2api-postgres rm -f /tmp/sub2api-candidate.dump
  if [[ "${DRY_RUN}" != true ]]; then
    printf '%s' "${dump_file}" > "${REPO_ROOT}/deploy/candidate/dumps/latest-dump-path.txt"
  else
    log "DRY-RUN: would save latest dump path: ${dump_file}"
  fi
  log "生产 DB dump 路径：${dump_file}"
}

start_candidate_db() {
	run_candidate_compose up -d sub2api-candidate-postgres sub2api-candidate-redis
}

wait_candidate_db() {
	if [[ "${DRY_RUN}" == true ]]; then
		printf '[DRY-RUN] %q exec sub2api-candidate-postgres pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"\n' "${DOCKER_BIN}"
		return 0
	fi

	local deadline
	deadline=$((SECONDS + 60))
	while (( SECONDS <= deadline )); do
		if "${DOCKER_BIN}" exec sub2api-candidate-postgres sh -lc 'pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"' >/dev/null; then
			log "候选 Postgres 已就绪"
			return 0
		fi
		sleep 1
	done
	die "候选 Postgres 等待超时"
}

restore_candidate_db() {
  local dump_file dump_base
  if [[ "${DRY_RUN}" == true ]]; then
    dump_file="${REPO_ROOT}/deploy/candidate/dumps/sub2api-public-dry-run.dump"
	else
		dump_file="$(cat "${REPO_ROOT}/deploy/candidate/dumps/latest-dump-path.txt")"
	fi
	dump_base="$(basename "${dump_file}")"
	run_cmd "${DOCKER_BIN}" exec sub2api-candidate-postgres sh -lc \
		"pg_restore -U \"\$POSTGRES_USER\" -d \"\$POSTGRES_DB\" --clean --if-exists --no-owner --no-privileges \"/candidate/dumps/${dump_base}\""
  run_cmd "${DOCKER_BIN}" exec sub2api-candidate-postgres sh -lc \
    "psql -U \"\$POSTGRES_USER\" -d \"\$POSTGRES_DB\" -v ON_ERROR_STOP=1 -f /candidate/sql/candidate-sanitize.sql"
}

start_candidate_app() {
  run_candidate_compose up -d sub2api-candidate
}

wait_candidate_health() {
  local url deadline
  url="http://127.0.0.1:${CANDIDATE_PORT}/health"
  if [[ "${DRY_RUN}" == true ]]; then
    log "DRY-RUN: would wait for ${url}"
    return 0
  fi

  deadline=$((SECONDS + 120))
  while (( SECONDS <= deadline )); do
    if curl -fsS --max-time 5 "${url}" >/dev/null; then
      log "候选健康检查通过：${url}"
      return 0
    fi
    sleep 2
  done

  "${DOCKER_BIN}" logs --tail 120 sub2api-candidate >&2 || true
  die "候选健康检查超时：${url}"
}

curl_expect_success() {
	local url status
	url="$1"
	if [[ "${DRY_RUN}" == true ]]; then
		printf '[DRY-RUN] curl -fsS --max-time 10 %q -o /dev/null\n' "${url}"
		return 0
	fi
  status="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 10 "${url}")"
  case "${status}" in
    200|301|302|401) log "候选 HTTP ${status}: ${url}" ;;
    *) die "候选 HTTP 异常 ${status}: ${url}" ;;
  esac
}

smoke_candidate_http() {
  local base path
  base="http://127.0.0.1:${CANDIDATE_PORT}"
  for path in /health / /dashboard /purchase /usage-guide /api/v1/settings/public; do
    curl_expect_success "${base}${path}"
  done
  if [[ "${ALLOW_GATEWAY_SMOKE}" == true ]]; then
    warn "允许候选 /v1 smoke test，但当前脚本不内置真实 Key；请单独使用专用候选测试 Key。"
  else
    log "跳过 /v1 smoke test，默认不触发上游请求。"
  fi
}

check_candidate_logs() {
  if [[ "${DRY_RUN}" == true ]]; then
    log "DRY-RUN: would check candidate logs"
    return 0
  fi
  if "${DOCKER_BIN}" logs --tail 300 sub2api-candidate 2>&1 | rg -i 'checksum mismatch|migration .*failed|panic|failed to initialize application'; then
    die "候选日志包含启动失败或 migration 风险"
  fi
}

main() {
  parse_args "$@"
  require_files
  assert_public_container_isolation
  find_main_worktree
  ensure_main_worktree_clean
  compute_candidate_image
  log "Main worktree：${MAIN_WT}"
  log "候选镜像：${CANDIDATE_IMAGE}"
  log "候选端口：http://127.0.0.1:${CANDIDATE_PORT}"
  build_candidate_image
  prepare_candidate_dirs
	dump_public_db
	start_candidate_db
	wait_candidate_db
	restore_candidate_db
  start_candidate_app
  wait_candidate_health
  smoke_candidate_http
  check_candidate_logs
  log "候选预演通过。候选镜像：${CANDIDATE_IMAGE}"
}

main "$@"
