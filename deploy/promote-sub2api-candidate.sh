#!/usr/bin/env bash
# 将已验证候选镜像提升为公网 latest，并只重建公网 sub2api 容器。

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOCKER_BIN="${SUB2API_DOCKER_BIN:-/Applications/Docker.app/Contents/Resources/bin/docker}"
COMPOSE_FILE="${REPO_ROOT}/deploy/docker-compose.local.yml"
ENV_FILE="${SUB2API_PUBLIC_ENV_FILE:-/Users/wujianxiang/CodeSpace/sub2api/deploy/.env.scheme-a.local}"
CANDIDATE_IMAGE=""
YES=false

log() { printf '[INFO] %s\n' "$*"; }
warn() { printf '[WARN] %s\n' "$*" >&2; }
die() { printf '[ERROR] %s\n' "$*" >&2; exit 1; }

usage() {
  cat <<'EOF'
用法：
  deploy/promote-sub2api-candidate.sh --candidate-image sub2api-candidate:YYYYMMDD-HHMMSS-sha --yes

说明：
  - 只发布已通过候选预演的镜像。
  - 会短暂影响 https://api.aaccx.pw/v1/*。
  - 不构建镜像，不 dump DB，不修改候选 DB。
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --candidate-image)
      [[ $# -ge 2 ]] || die "--candidate-image 需要参数"
      CANDIDATE_IMAGE="$2"
      shift 2
      ;;
    --yes) YES=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "未知参数：$1" ;;
  esac
done

[[ -n "${CANDIDATE_IMAGE}" ]] || die "必须传 --candidate-image"
[[ "${CANDIDATE_IMAGE}" == sub2api-candidate:* ]] || die "只允许发布 sub2api-candidate:* 镜像"
[[ -x "${DOCKER_BIN}" ]] || die "Docker CLI 不可执行：${DOCKER_BIN}"
[[ -f "${COMPOSE_FILE}" ]] || die "公网 compose 不存在：${COMPOSE_FILE}"
[[ -f "${ENV_FILE}" ]] || die "公网 env 不存在：${ENV_FILE}"

old_image_id="$("${DOCKER_BIN}" inspect sub2api --format '{{.Image}}' 2>/dev/null || true)"
candidate_id="$("${DOCKER_BIN}" image inspect "${CANDIDATE_IMAGE}" --format '{{.Id}}')"

log "当前公网容器镜像 ID：${old_image_id}"
log "候选镜像 ID：${candidate_id}"
warn "即将替换公网 sub2api 容器，正在运行的流式请求可能断开。"

if [[ "${YES}" != true ]]; then
  printf '请输入 yes 继续：'
  read -r reply
  [[ "${reply}" == "yes" ]] || die "已取消"
fi

"${DOCKER_BIN}" tag "${CANDIDATE_IMAGE}" weishaw/sub2api:latest
"${DOCKER_BIN}" compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" up -d --no-deps --force-recreate sub2api
curl -fsS --max-time 5 http://127.0.0.1:18080/health >/dev/null
curl -fsS --max-time 10 https://api.aaccx.pw/health >/dev/null

log "公网发布完成。old=${old_image_id} new=${candidate_id}"
