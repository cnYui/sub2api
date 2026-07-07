#!/usr/bin/env bash

load_deploy_target() {
  DEPLOY_TARGET_NAME="${DEPLOY_TARGET:-public_candidate_18084}"

  case "${DEPLOY_TARGET_NAME}" in
    public_candidate_18084)
      DEPLOY_TARGET_APP_CONTAINER="${SUB2API_TARGET_APP_CONTAINER:-sub2api-candidate}"
      DEPLOY_TARGET_POSTGRES_CONTAINER="${SUB2API_TARGET_POSTGRES_CONTAINER:-sub2api-candidate-postgres}"
      DEPLOY_TARGET_REDIS_CONTAINER="${SUB2API_TARGET_REDIS_CONTAINER:-sub2api-candidate-redis}"
      DEPLOY_TARGET_PORT="${SUB2API_TARGET_PORT:-18084}"
      DEPLOY_TARGET_HEALTH_URL="${SUB2API_TARGET_HEALTH_URL:-http://127.0.0.1:18084/health}"
      DEPLOY_TARGET_NETWORK="${SUB2API_TARGET_NETWORK:-sub2api-candidate-network}"
      DEPLOY_TARGET_LEGACY=false
      ;;
    legacy_18080)
      DEPLOY_TARGET_APP_CONTAINER="${SUB2API_TARGET_APP_CONTAINER:-sub2api}"
      DEPLOY_TARGET_POSTGRES_CONTAINER="${SUB2API_TARGET_POSTGRES_CONTAINER:-sub2api-postgres}"
      DEPLOY_TARGET_REDIS_CONTAINER="${SUB2API_TARGET_REDIS_CONTAINER:-sub2api-redis}"
      DEPLOY_TARGET_PORT="${SUB2API_TARGET_PORT:-18080}"
      DEPLOY_TARGET_HEALTH_URL="${SUB2API_TARGET_HEALTH_URL:-http://127.0.0.1:18080/health}"
      DEPLOY_TARGET_NETWORK="${SUB2API_TARGET_NETWORK:-sub2api_default}"
      DEPLOY_TARGET_LEGACY=true
      ;;
    *)
      deploy_die "未知部署目标：${DEPLOY_TARGET_NAME}"
      ;;
  esac
}

print_deploy_target_summary() {
  cat <<EOF
部署目标：${DEPLOY_TARGET_NAME}
应用容器：${DEPLOY_TARGET_APP_CONTAINER}
Postgres 容器：${DEPLOY_TARGET_POSTGRES_CONTAINER}
Redis 容器：${DEPLOY_TARGET_REDIS_CONTAINER}
宿主端口：${DEPLOY_TARGET_PORT}
健康检查：${DEPLOY_TARGET_HEALTH_URL}
Docker 网络：${DEPLOY_TARGET_NETWORK}
Legacy：${DEPLOY_TARGET_LEGACY}
EOF
}

require_legacy_target_confirmation() {
  if [[ "${DEPLOY_TARGET_LEGACY:-false}" != true ]]; then
    return 0
  fi
  if [[ "${SUB2API_ALLOW_LEGACY_TARGET:-false}" == true ]]; then
    return 0
  fi
  deploy_die "legacy_18080 不是当前标准公网目标；如确需使用，请设置 SUB2API_ALLOW_LEGACY_TARGET=true"
}
