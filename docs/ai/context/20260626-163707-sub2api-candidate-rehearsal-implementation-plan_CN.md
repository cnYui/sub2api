# Sub2API 候选预演环境 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增一套本地候选预演流程，让新镜像先在 `127.0.0.1:18081` 连接生产 DB 克隆完成启动、migration、健康检查和关键只读 API 验证，再允许替换公网 `sub2api` 容器。

**Architecture:** 公网 `sub2api` 继续运行在 `127.0.0.1:18080`；候选环境使用独立 compose、独立 Postgres、独立 Redis、独立端口和独立镜像 tag。候选脚本先构建 `sub2api-candidate:<timestamp>-<sha>`，再从生产 Postgres 只读 dump 到候选 Postgres，执行候选 DB 脱敏/隔离 SQL，启动候选应用并输出发布门禁结果。

**Tech Stack:** Docker Desktop CLI、Docker Compose、Bash、PostgreSQL `pg_dump/psql/pg_restore`、Sub2API Go 后端、现有 `deploy/docker-compose.local.yml` 运行形态。

---

## 文件结构

- Create: `deploy/docker-compose.candidate.yml`
  - 只定义 `sub2api-candidate`、`sub2api-candidate-postgres`、`sub2api-candidate-redis`。
  - 使用 `CANDIDATE_IMAGE`，禁止默认指向 `weishaw/sub2api:latest`。
  - 端口固定默认 `127.0.0.1:18081->8080`。

- Create: `deploy/.env.candidate.local.example`
  - 只放变量名、示例值和说明。
  - 不包含真实密码、API Key、支付密钥、SMTP 密码。

- Create: `deploy/sql/candidate-sanitize.sql`
  - 只对候选 DB 执行。
  - 关闭支付、SMTP、运维监控、通道监控、通知类外部副作用。
  - 不进入 `backend/migrations/`，不写 `schema_migrations`。

- Create: `deploy/rehearse-sub2api-candidate.sh`
  - 主预演脚本：定位 main worktree、构建候选镜像、dump 生产 DB、恢复候选 DB、启动候选容器、执行 smoke test。
  - 默认全量重建候选 DB。
  - 默认不调用公网 `/v1`，也不调用候选 `/v1`。

- Create: `deploy/promote-sub2api-candidate.sh`
  - 只接受已验证的候选镜像 tag。
  - 将候选镜像 retag 为 `weishaw/sub2api:latest` 后，只重建公网 `sub2api` 服务。
  - 发布前必须确认公网旧镜像 ID 和候选镜像 ID。

- Modify: `deploy/redeploy-sub2api-image.sh`
  - 只更新帮助文案，提示常规上线先跑候选预演。
  - 不改变当前脚本默认行为，避免影响已有应急流程。

- Create: `docs/ai/context/YYYYMMDD-HHMMSS-sub2api-candidate-rehearsal-result_CN.md`
  - 由执行阶段新建结果文档，记录镜像 tag、DB 克隆时间、候选健康检查、是否允许发布。

---

### Task 1: 候选 Compose

**Files:**
- Create: `deploy/docker-compose.candidate.yml`
- Test: `docker compose --env-file deploy/.env.candidate.local.example -f deploy/docker-compose.candidate.yml config`

- [ ] **Step 1: 写候选 compose**

创建 `deploy/docker-compose.candidate.yml`，内容使用独立容器名、网络、数据目录和端口。

```yaml
services:
  sub2api-candidate:
    image: ${CANDIDATE_IMAGE:?CANDIDATE_IMAGE is required}
    container_name: sub2api-candidate
    restart: "no"
    ulimits:
      nofile:
        soft: 100000
        hard: 100000
    ports:
      - "127.0.0.1:${CANDIDATE_SERVER_PORT:-18081}:8080"
    volumes:
      - ./candidate/data:/app/data:Z
    environment:
      - AUTO_SETUP=true
      - SERVER_HOST=0.0.0.0
      - SERVER_PORT=8080
      - SERVER_MODE=${SERVER_MODE:-release}
      - RUN_MODE=${RUN_MODE:-candidate}
      - OPS_ENABLED=${OPS_ENABLED:-false}
      - OPS_CLEANUP_ENABLED=${OPS_CLEANUP_ENABLED:-false}
      - OPS_AGGREGATION_ENABLED=${OPS_AGGREGATION_ENABLED:-false}
      - DATABASE_HOST=sub2api-candidate-postgres
      - DATABASE_PORT=5432
      - DATABASE_USER=${POSTGRES_USER:-sub2api}
      - DATABASE_PASSWORD=${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}
      - DATABASE_DBNAME=${POSTGRES_DB:-sub2api}
      - DATABASE_SSLMODE=disable
      - DATABASE_MAX_OPEN_CONNS=${DATABASE_MAX_OPEN_CONNS:-20}
      - DATABASE_MAX_IDLE_CONNS=${DATABASE_MAX_IDLE_CONNS:-5}
      - DATABASE_CONN_MAX_LIFETIME_MINUTES=${DATABASE_CONN_MAX_LIFETIME_MINUTES:-30}
      - DATABASE_CONN_MAX_IDLE_TIME_MINUTES=${DATABASE_CONN_MAX_IDLE_TIME_MINUTES:-5}
      - REDIS_HOST=sub2api-candidate-redis
      - REDIS_PORT=6379
      - REDIS_PASSWORD=${REDIS_PASSWORD:-}
      - REDIS_DB=${REDIS_DB:-0}
      - REDIS_POOL_SIZE=${REDIS_POOL_SIZE:-64}
      - REDIS_MIN_IDLE_CONNS=${REDIS_MIN_IDLE_CONNS:-4}
      - REDIS_ENABLE_TLS=false
      - ADMIN_EMAIL=${ADMIN_EMAIL:-admin@sub2api.local}
      - ADMIN_PASSWORD=${ADMIN_PASSWORD:-}
      - JWT_SECRET=${JWT_SECRET:-candidate-local-jwt-secret-change-me}
      - JWT_EXPIRE_HOUR=${JWT_EXPIRE_HOUR:-24}
      - TOTP_ENCRYPTION_KEY=${TOTP_ENCRYPTION_KEY:-candidate-local-totp-key-change-me}
      - TZ=${TZ:-Asia/Shanghai}
      - SECURITY_URL_ALLOWLIST_ENABLED=${SECURITY_URL_ALLOWLIST_ENABLED:-false}
      - SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP=${SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP:-false}
      - SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS=${SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS:-false}
      - UPDATE_PROXY_URL=
      - GATEWAY_IMAGE_CONCURRENCY_ENABLED=${GATEWAY_IMAGE_CONCURRENCY_ENABLED:-false}
    depends_on:
      sub2api-candidate-postgres:
        condition: service_healthy
      sub2api-candidate-redis:
        condition: service_healthy
    networks:
      - sub2api-candidate-network
    healthcheck:
      test: ["CMD", "wget", "-q", "-T", "5", "-O", "/dev/null", "http://localhost:8080/health"]
      interval: 10s
      timeout: 5s
      retries: 6
      start_period: 20s

  sub2api-candidate-postgres:
    image: postgres:18-alpine
    container_name: sub2api-candidate-postgres
    restart: "no"
    ulimits:
      nofile:
        soft: 100000
        hard: 100000
    volumes:
      - ./candidate/postgres_data:/var/lib/postgresql/data:Z
      - ./candidate/dumps:/candidate/dumps:Z
      - ./sql:/candidate/sql:ro
    environment:
      - POSTGRES_USER=${POSTGRES_USER:-sub2api}
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}
      - POSTGRES_DB=${POSTGRES_DB:-sub2api}
      - PGDATA=/var/lib/postgresql/data
      - TZ=${TZ:-Asia/Shanghai}
    networks:
      - sub2api-candidate-network
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-sub2api} -d ${POSTGRES_DB:-sub2api}"]
      interval: 5s
      timeout: 5s
      retries: 12
      start_period: 5s

  sub2api-candidate-redis:
    image: redis:8-alpine
    container_name: sub2api-candidate-redis
    restart: "no"
    volumes:
      - ./candidate/redis_data:/data:Z
    command: >
      sh -c '
        redis-server
        --save ""
        --appendonly no
        ${REDIS_PASSWORD:+--requirepass "$REDIS_PASSWORD"}'
    environment:
      - TZ=${TZ:-Asia/Shanghai}
      - REDISCLI_AUTH=${REDIS_PASSWORD:-}
    networks:
      - sub2api-candidate-network
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 12
      start_period: 5s

networks:
  sub2api-candidate-network:
    name: sub2api-candidate-network
    driver: bridge
```

- [ ] **Step 2: 运行 compose 配置验证**

```bash
cp deploy/.env.candidate.local.example /tmp/sub2api-candidate.env
PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH" \
  docker compose --env-file /tmp/sub2api-candidate.env \
  -f deploy/docker-compose.candidate.yml config >/tmp/sub2api-candidate-compose.yml
```

Expected: 命令退出码为 0，`/tmp/sub2api-candidate-compose.yml` 中出现 `sub2api-candidate`、`sub2api-candidate-postgres`、`sub2api-candidate-redis`。

- [ ] **Step 3: 提交候选 compose**

```bash
git add deploy/docker-compose.candidate.yml
git commit -m "deploy: add sub2api candidate compose"
```

---

### Task 2: 候选 Env Example

**Files:**
- Create: `deploy/.env.candidate.local.example`
- Test: `docker compose --env-file deploy/.env.candidate.local.example -f deploy/docker-compose.candidate.yml config`

- [ ] **Step 1: 写 env example**

创建 `deploy/.env.candidate.local.example`。

```dotenv
# Candidate rehearsal environment. Copy to deploy/.env.candidate.local and fill secrets locally.
# Do not commit deploy/.env.candidate.local.

CANDIDATE_IMAGE=sub2api-candidate:example
CANDIDATE_SERVER_PORT=18081

POSTGRES_USER=sub2api
POSTGRES_PASSWORD=change-me-candidate-only
POSTGRES_DB=sub2api

REDIS_PASSWORD=

SERVER_MODE=release
RUN_MODE=candidate
TZ=Asia/Shanghai

OPS_ENABLED=false
OPS_CLEANUP_ENABLED=false
OPS_AGGREGATION_ENABLED=false

DATABASE_MAX_OPEN_CONNS=20
DATABASE_MAX_IDLE_CONNS=5
DATABASE_CONN_MAX_LIFETIME_MINUTES=30
DATABASE_CONN_MAX_IDLE_TIME_MINUTES=5

JWT_SECRET=candidate-local-jwt-secret-change-me
JWT_EXPIRE_HOUR=24
TOTP_ENCRYPTION_KEY=candidate-local-totp-key-change-me

ADMIN_EMAIL=admin@sub2api.local
ADMIN_PASSWORD=

SECURITY_URL_ALLOWLIST_ENABLED=false
SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP=false
SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS=false
GATEWAY_IMAGE_CONCURRENCY_ENABLED=false
```

- [ ] **Step 2: 确认真实 env 被忽略**

```bash
git check-ignore deploy/.env.candidate.local || true
```

Expected: 如果未输出路径，检查 `.gitignore`。若 `.gitignore` 未覆盖，追加：

```gitignore
deploy/.env.candidate.local
deploy/candidate/
```

- [ ] **Step 3: 验证 compose 可解析 env example**

```bash
PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH" \
  docker compose --env-file deploy/.env.candidate.local.example \
  -f deploy/docker-compose.candidate.yml config >/tmp/sub2api-candidate-compose.yml
```

Expected: 命令退出码为 0，且输出中 `127.0.0.1:18081:8080` 存在。

- [ ] **Step 4: 提交 env example**

```bash
git add deploy/.env.candidate.local.example .gitignore
git commit -m "deploy: document candidate environment variables"
```

---

### Task 3: 候选 DB 脱敏与副作用隔离 SQL

**Files:**
- Create: `deploy/sql/candidate-sanitize.sql`
- Test: 在临时候选 DB 中执行 `psql -v ON_ERROR_STOP=1 -f /candidate/sql/candidate-sanitize.sql`

- [ ] **Step 1: 创建 SQL 目录和文件**

创建 `deploy/sql/candidate-sanitize.sql`。该文件不是 migration，不写入 `schema_migrations`。

```sql
-- Candidate-only sanitization. Never run this against production DB.
-- It disables external side effects after restoring a production DB clone.

DO $$
BEGIN
  IF to_regclass('public.settings') IS NOT NULL THEN
    INSERT INTO settings (key, value, updated_at)
    VALUES
      ('payment_enabled', 'false', NOW()),
      ('payment_visible_method_alipay_enabled', 'false', NOW()),
      ('payment_visible_method_wxpay_enabled', 'false', NOW()),
      ('ENABLED_PAYMENT_TYPES', '[]', NOW()),
      ('smtp_host', '', NOW()),
      ('smtp_username', '', NOW()),
      ('smtp_password', '', NOW()),
      ('smtp_from', '', NOW()),
      ('smtp_from_name', 'Sub2API Candidate', NOW()),
      ('smtp_use_tls', 'false', NOW()),
      ('ops_monitoring_enabled', 'false', NOW()),
      ('ops_realtime_monitoring_enabled', 'false', NOW()),
      ('channel_monitor_enabled', 'false', NOW()),
      ('available_channels_enabled', 'false', NOW()),
      ('subscription_expiry_notify_enabled', 'false', NOW()),
      ('balance_low_notify_enabled', 'false', NOW()),
      ('account_quota_notify_enabled', 'false', NOW())
    ON CONFLICT (key) DO UPDATE
      SET value = EXCLUDED.value,
          updated_at = NOW();
  END IF;

  IF to_regclass('public.payment_provider_instances') IS NOT NULL THEN
    EXECUTE 'UPDATE payment_provider_instances SET enabled = false';
  END IF;

  IF to_regclass('public.channel_monitors') IS NOT NULL THEN
    EXECUTE 'UPDATE channel_monitors SET enabled = false';
  END IF;

  IF to_regclass('public.ops_alert_rules') IS NOT NULL THEN
    EXECUTE 'UPDATE ops_alert_rules SET enabled = false';
  END IF;
END $$;
```

- [ ] **Step 2: 静态检查 SQL 不引用生产连接**

```bash
rg -n "api\\.aaccx\\.pw|127\\.0\\.0\\.1:18080|DATABASE_PASSWORD|POSTGRES_PASSWORD|sk-|secret" deploy/sql/candidate-sanitize.sql
```

Expected: `rg` 退出码为 1，无匹配。

- [ ] **Step 3: 提交 SQL**

```bash
git add deploy/sql/candidate-sanitize.sql
git commit -m "deploy: add candidate db sanitization sql"
```

---

### Task 4: 候选预演脚本骨架与安全门禁

**Files:**
- Create: `deploy/rehearse-sub2api-candidate.sh`
- Test: `bash -n deploy/rehearse-sub2api-candidate.sh`

- [ ] **Step 1: 写脚本骨架**

创建 `deploy/rehearse-sub2api-candidate.sh`，先实现参数解析、安全检查和 dry-run。

```bash
#!/usr/bin/env bash
# 构建并启动 Sub2API 候选预演环境；不替换公网容器，不访问公网 /v1。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

DOCKER_BIN="${SUB2API_DOCKER_BIN:-/Applications/Docker.app/Contents/Resources/bin/docker}"
COMPOSE_FILE="${REPO_ROOT}/deploy/docker-compose.candidate.yml"
ENV_FILE="${REPO_ROOT}/deploy/.env.candidate.local"
PUBLIC_COMPOSE_FILE="${REPO_ROOT}/deploy/docker-compose.local.yml"
PUBLIC_ENV_FILE="${REPO_ROOT}/deploy/.env.scheme-a.local"
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
  - 默认只做本地只读验证，不访问 https://api.aaccx.pw/v1。

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
      --docker-bin) DOCKER_BIN="$2"; shift 2 ;;
      --env-file) ENV_FILE="$2"; shift 2 ;;
      --candidate-port) CANDIDATE_PORT="$2"; shift 2 ;;
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

main() {
  parse_args "$@"
  [[ -x "${DOCKER_BIN}" ]] || die "Docker CLI 不可执行：${DOCKER_BIN}"
  [[ -f "${COMPOSE_FILE}" ]] || die "候选 compose 不存在：${COMPOSE_FILE}"
  [[ -f "${ENV_FILE}" ]] || die "候选 env 不存在：${ENV_FILE}"
  [[ -f "${PUBLIC_COMPOSE_FILE}" ]] || die "公网 compose 不存在：${PUBLIC_COMPOSE_FILE}"
  [[ -f "${PUBLIC_ENV_FILE}" ]] || die "公网 env 不存在：${PUBLIC_ENV_FILE}"
  [[ "${CANDIDATE_PORT}" != "18080" ]] || die "候选端口不能使用公网 18080"
  log "安全检查通过。后续任务会补齐 build、clone、restore、smoke test。"
}

main "$@"
```

- [ ] **Step 2: 语法检查**

```bash
bash -n deploy/rehearse-sub2api-candidate.sh
```

Expected: 无输出，退出码为 0。

- [ ] **Step 3: dry-run 检查不会碰公网**

```bash
deploy/rehearse-sub2api-candidate.sh --dry-run --reset-db
```

Expected: 输出安全检查通过，不出现 `up -d --force-recreate sub2api`，不出现 `api.aaccx.pw/v1`。

- [ ] **Step 4: 提交脚本骨架**

```bash
chmod +x deploy/rehearse-sub2api-candidate.sh
git add deploy/rehearse-sub2api-candidate.sh
git commit -m "deploy: add candidate rehearsal script skeleton"
```

---

### Task 5: 候选镜像构建与 Main Worktree 定位

**Files:**
- Modify: `deploy/rehearse-sub2api-candidate.sh`
- Test: `deploy/rehearse-sub2api-candidate.sh --dry-run --reset-db`

- [ ] **Step 1: 增加 main worktree 定位函数**

在脚本中加入：

```bash
find_main_worktree() {
  local current wt branch
  current=""
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
  local status
  status="$(git -C "${MAIN_WT}" status --short --branch)"
  printf '%s\n' "${status}"
  if printf '%s\n' "${status}" | rg -q '^(UU|AA|DD|DU|UD|AU|UA|!!|[ MADRCU?]{1,2}) '; then
    die "main worktree 存在改动或冲突；先处理后再预演"
  fi
}
```

- [ ] **Step 2: 增加候选镜像 tag 计算与构建函数**

```bash
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
```

- [ ] **Step 3: 在 main 中串联定位与 dry-run 输出**

在 `main()` 安全检查后加入：

```bash
find_main_worktree
ensure_main_worktree_clean
compute_candidate_image
log "Main worktree：${MAIN_WT}"
log "候选镜像：${CANDIDATE_IMAGE}"
build_candidate_image
```

- [ ] **Step 4: 运行 dry-run**

```bash
deploy/rehearse-sub2api-candidate.sh --dry-run --reset-db
```

Expected: 输出 `sub2api-candidate:<timestamp>-<sha>`；不出现 `weishaw/sub2api:latest` build；不访问公网。

- [ ] **Step 5: 提交构建逻辑**

```bash
git add deploy/rehearse-sub2api-candidate.sh
git commit -m "deploy: build tagged candidate image for rehearsal"
```

---

### Task 6: 生产 DB Dump 与候选 DB Restore

**Files:**
- Modify: `deploy/rehearse-sub2api-candidate.sh`
- Test: `deploy/rehearse-sub2api-candidate.sh --dry-run --reset-db`

- [ ] **Step 1: 增加候选目录与 reset 逻辑**

```bash
prepare_candidate_dirs() {
  mkdir -p "${REPO_ROOT}/deploy/candidate/dumps" "${REPO_ROOT}/deploy/candidate/logs"
  if [[ "${RESET_DB}" == true ]]; then
    run_cmd "${DOCKER_BIN}" compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" down --remove-orphans
    run_cmd rm -rf "${REPO_ROOT}/deploy/candidate/postgres_data" "${REPO_ROOT}/deploy/candidate/redis_data" "${REPO_ROOT}/deploy/candidate/data"
  elif [[ ! -d "${REPO_ROOT}/deploy/candidate/postgres_data" ]]; then
    die "候选 DB 不存在；首次运行必须加 --reset-db"
  fi
  mkdir -p "${REPO_ROOT}/deploy/candidate/postgres_data" "${REPO_ROOT}/deploy/candidate/redis_data" "${REPO_ROOT}/deploy/candidate/data"
}
```

- [ ] **Step 2: 增加生产 DB dump 函数**

使用生产 Postgres 容器本地 dump，不打印 env 内容。

```bash
dump_public_db() {
  local dump_file
  dump_file="${REPO_ROOT}/deploy/candidate/dumps/sub2api-public-$(date +%Y%m%d-%H%M%S).dump"
  run_cmd "${DOCKER_BIN}" exec sub2api-postgres sh -lc \
    "pg_dump -U \"\$POSTGRES_USER\" -d \"\$POSTGRES_DB\" --format=custom --no-owner --no-privileges --file=/tmp/sub2api-candidate.dump"
  run_cmd "${DOCKER_BIN}" cp sub2api-postgres:/tmp/sub2api-candidate.dump "${dump_file}"
  run_cmd "${DOCKER_BIN}" exec sub2api-postgres rm -f /tmp/sub2api-candidate.dump
  printf '%s' "${dump_file}" > "${REPO_ROOT}/deploy/candidate/dumps/latest-dump-path.txt"
  log "生产 DB dump 已保存到：${dump_file}"
}
```

- [ ] **Step 3: 增加候选 Postgres 启动与 restore 函数**

```bash
start_candidate_db() {
  CANDIDATE_IMAGE="${CANDIDATE_IMAGE}" CANDIDATE_SERVER_PORT="${CANDIDATE_PORT}" \
    run_cmd "${DOCKER_BIN}" compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" up -d sub2api-candidate-postgres sub2api-candidate-redis
}

restore_candidate_db() {
  local dump_file dump_base
  dump_file="$(cat "${REPO_ROOT}/deploy/candidate/dumps/latest-dump-path.txt")"
  dump_base="$(basename "${dump_file}")"
  run_cmd cp "${dump_file}" "${REPO_ROOT}/deploy/candidate/dumps/${dump_base}"
  run_cmd "${DOCKER_BIN}" exec sub2api-candidate-postgres sh -lc \
    "pg_restore -U \"\$POSTGRES_USER\" -d \"\$POSTGRES_DB\" --clean --if-exists --no-owner --no-privileges \"/candidate/dumps/${dump_base}\""
  run_cmd "${DOCKER_BIN}" exec sub2api-candidate-postgres sh -lc \
    "psql -U \"\$POSTGRES_USER\" -d \"\$POSTGRES_DB\" -v ON_ERROR_STOP=1 -f /candidate/sql/candidate-sanitize.sql"
}
```

- [ ] **Step 4: 在 main 中串联 DB 流程**

在 `build_candidate_image` 后加入：

```bash
prepare_candidate_dirs
dump_public_db
start_candidate_db
restore_candidate_db
```

- [ ] **Step 5: dry-run 检查**

```bash
deploy/rehearse-sub2api-candidate.sh --dry-run --reset-db
```

Expected: 输出包含 `pg_dump`、`pg_restore`、`candidate-sanitize.sql`；不包含公网 `sub2api` 容器重建命令。

- [ ] **Step 6: 提交 DB 克隆逻辑**

```bash
git add deploy/rehearse-sub2api-candidate.sh
git commit -m "deploy: clone production db into candidate rehearsal"
```

---

### Task 7: 候选启动与本地 Smoke Test

**Files:**
- Modify: `deploy/rehearse-sub2api-candidate.sh`
- Test: `deploy/rehearse-sub2api-candidate.sh --dry-run --reset-db`

- [ ] **Step 1: 增加候选应用启动函数**

```bash
start_candidate_app() {
  CANDIDATE_IMAGE="${CANDIDATE_IMAGE}" CANDIDATE_SERVER_PORT="${CANDIDATE_PORT}" \
    run_cmd "${DOCKER_BIN}" compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" up -d sub2api-candidate
}
```

- [ ] **Step 2: 增加健康检查函数**

```bash
wait_candidate_health() {
  local url deadline
  url="http://127.0.0.1:${CANDIDATE_PORT}/health"
  deadline=$((SECONDS + 120))
  if [[ "${DRY_RUN}" == true ]]; then
    log "DRY-RUN: would wait for ${url}"
    return 0
  fi
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
```

- [ ] **Step 3: 增加只读 HTTP smoke test**

```bash
smoke_candidate_http() {
  local base path
  base="http://127.0.0.1:${CANDIDATE_PORT}"
  for path in /health / /dashboard /purchase /usage-guide /api/v1/settings/public /api/v1/payment/checkout-info; do
    run_cmd curl -fsS --max-time 10 "${base}${path}" -o /dev/null
  done
  if [[ "${ALLOW_GATEWAY_SMOKE}" == true ]]; then
    warn "允许候选 /v1 smoke test，但实现阶段必须使用专用候选测试 Key。"
  else
    log "跳过 /v1 smoke test，默认不触发上游请求。"
  fi
}
```

- [ ] **Step 4: 增加 migration mismatch 日志检查**

```bash
check_candidate_logs() {
  if [[ "${DRY_RUN}" == true ]]; then
    log "DRY-RUN: would check candidate logs"
    return 0
  fi
  if "${DOCKER_BIN}" logs --tail 300 sub2api-candidate 2>&1 | rg -i 'checksum mismatch|migration .*failed|panic|failed to initialize application'; then
    die "候选日志包含启动失败或 migration 风险"
  fi
}
```

- [ ] **Step 5: 在 main 中串联启动验证**

在 `restore_candidate_db` 后加入：

```bash
start_candidate_app
wait_candidate_health
smoke_candidate_http
check_candidate_logs
log "候选预演通过。候选镜像：${CANDIDATE_IMAGE}"
```

- [ ] **Step 6: dry-run 检查**

```bash
deploy/rehearse-sub2api-candidate.sh --dry-run --reset-db
```

Expected: 输出会验证 `127.0.0.1:18081`；不出现 `https://api.aaccx.pw/v1`。

- [ ] **Step 7: 提交候选验证逻辑**

```bash
git add deploy/rehearse-sub2api-candidate.sh
git commit -m "deploy: verify candidate app before public promotion"
```

---

### Task 8: 候选发布提升脚本

**Files:**
- Create: `deploy/promote-sub2api-candidate.sh`
- Test: `bash -n deploy/promote-sub2api-candidate.sh`

- [ ] **Step 1: 写发布脚本**

创建 `deploy/promote-sub2api-candidate.sh`。

```bash
#!/usr/bin/env bash
# 将已验证候选镜像提升为公网 latest，并只重建公网 sub2api 容器。

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOCKER_BIN="${SUB2API_DOCKER_BIN:-/Applications/Docker.app/Contents/Resources/bin/docker}"
COMPOSE_FILE="${REPO_ROOT}/deploy/docker-compose.local.yml"
ENV_FILE="${REPO_ROOT}/deploy/.env.scheme-a.local"
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
    --candidate-image) CANDIDATE_IMAGE="$2"; shift 2 ;;
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
```

- [ ] **Step 2: 语法检查**

```bash
bash -n deploy/promote-sub2api-candidate.sh
```

Expected: 无输出，退出码为 0。

- [ ] **Step 3: 参数保护检查**

```bash
deploy/promote-sub2api-candidate.sh --candidate-image weishaw/sub2api:latest --yes
```

Expected: 失败，输出 `只允许发布 sub2api-candidate:* 镜像`。

- [ ] **Step 4: 提交发布脚本**

```bash
chmod +x deploy/promote-sub2api-candidate.sh
git add deploy/promote-sub2api-candidate.sh
git commit -m "deploy: promote verified candidate image"
```

---

### Task 9: 更新现有重部署脚本文案

**Files:**
- Modify: `deploy/redeploy-sub2api-image.sh`
- Test: `deploy/redeploy-sub2api-image.sh --help`

- [ ] **Step 1: 修改 help 文案**

在 `usage()` 的“说明”下方增加：

```text
建议：
  常规公网发布先执行 deploy/rehearse-sub2api-candidate.sh --reset-db，
  候选预演通过后再使用 deploy/promote-sub2api-candidate.sh 发布。
  本脚本保留为应急直接替换入口。
```

- [ ] **Step 2: 查看帮助输出**

```bash
deploy/redeploy-sub2api-image.sh --help | rg '候选预演|promote-sub2api-candidate|应急直接替换'
```

Expected: 三个关键词均能匹配。

- [ ] **Step 3: 提交文案**

```bash
git add deploy/redeploy-sub2api-image.sh
git commit -m "docs: point redeploy script to candidate rehearsal"
```

---

### Task 10: 首次候选预演验收

**Files:**
- Create: `docs/ai/context/YYYYMMDD-HHMMSS-sub2api-candidate-rehearsal-result_CN.md`
- Runtime: 候选 Docker、候选 DB、候选 Redis

- [ ] **Step 1: 准备本地候选 env**

```bash
cp deploy/.env.candidate.local.example deploy/.env.candidate.local
```

手工编辑 `deploy/.env.candidate.local`，只填候选环境密码。不要把该文件加入 git。

- [ ] **Step 2: 执行完整候选预演**

```bash
PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH" \
  deploy/rehearse-sub2api-candidate.sh --reset-db
```

Expected:

- 候选镜像 tag 形如 `sub2api-candidate:YYYYMMDD-HHMMSS-<sha>`。
- `sub2api-candidate-postgres` healthy。
- `sub2api-candidate-redis` healthy。
- `sub2api-candidate` healthy。
- `http://127.0.0.1:18081/health` 通过。
- 日志无 `checksum mismatch`。

- [ ] **Step 3: 核对公网容器未被替换**

```bash
PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH" \
  docker ps --filter name='^/sub2api$' --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
```

Expected: `sub2api` 仍运行在 `127.0.0.1:18080->8080/tcp`，没有被候选脚本重建。

- [ ] **Step 4: 写结果上下文**

创建结果文档，格式如下：

```markdown
# Sub2API 候选预演结果

## 结论

- 候选镜像：
- main worktree：
- HEAD：
- 候选 DB dump：
- 候选容器状态：
- 候选健康检查：
- 候选只读 API：
- 是否允许公网发布：

## 重要日志

- migration checksum：
- 前端资源 hash：
- 失败项：

## 后续动作

-
```

- [ ] **Step 5: 提交结果文档**

```bash
git add docs/ai/context/*-sub2api-candidate-rehearsal-result_CN.md
git commit -m "docs: record candidate rehearsal result"
```

---

## 自查清单

- 设计覆盖：计划覆盖候选 compose、env example、生产 DB 克隆、候选 DB 隔离、候选启动、健康检查、关键只读 API、候选提升发布、失败记录。
- 公网隔离：候选脚本默认只使用 `127.0.0.1:18081`，不访问 `https://api.aaccx.pw/v1`，不重建 `sub2api`。
- 数据隔离：候选应用只连 `sub2api-candidate-postgres`，不直接写生产 DB。
- 镜像隔离：构建阶段只打 `sub2api-candidate:*`，不覆盖 `weishaw/sub2api:latest`。
- 副作用隔离：候选 DB restore 后执行 `candidate-sanitize.sql`，关闭支付、SMTP、监控和通知类开关。
- 发布门禁：只有 `promote-sub2api-candidate.sh` 会 retag `latest` 并替换公网容器，且必须显式传 `--candidate-image sub2api-candidate:*`。
