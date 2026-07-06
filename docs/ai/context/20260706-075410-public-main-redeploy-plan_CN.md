# 公网 Sub2API 替换为本地 main 最新代码计划

## 目标

- 将当前公网上运行的 Sub2API 前后端替换为本地 `main` 最新代码。
- 当前本地 `main` HEAD：`9966c6f5b feat: finish automatic api key effective group`。
- 当前公网应用容器：`sub2api-candidate`，镜像 `sub2api-main-preview:20260702-local-main-6ef887a8d`，端口 `127.0.0.1:18084->8080`。
- 当前公网数据层容器：`sub2api-candidate-postgres`、`sub2api-candidate-redis`。
- 本次只替换应用容器，不改 nginx，不切端口，不覆盖 DB/Redis 数据目录。

## 当前只读确认

- 本地分支：`main`。
- 当前 `18084/health` 和 `8080/health` 均为 200。
- 运行容器：
  - `sub2api-candidate`
  - `sub2api-candidate-postgres`
  - `sub2api-candidate-redis`
- 当前应用容器连接：
  - Docker network：`sub2api-candidate-network`
  - DB host：`sub2api-candidate-postgres`
  - Redis host：`sub2api-candidate-redis`
  - 应用数据 bind mount：`/Users/wujianxiang/CodeSpace/sub2api/.worktrees/codex-sub2api-candidate-rehearsal-20260626/deploy/candidate/data:/app/data`
- Postgres/Redis 当前 bind mount 位于旧 candidate worktree 下；不能用当前仓库 `deploy/docker-compose.candidate.yml` 直接 `up`，否则会挂到当前仓库空的 `deploy/candidate/*` 目录。

## 风险点

- 新版本包含 `159_auto_api_key_effective_group.sql`，应用启动时会自动迁移公网 DB：
  - 创建/修正 `traffic-pack-openai` 分组。
  - 给未删除 OpenAI 上游账号绑定该分组。
  - 将未删除旧 OpenAI API Key 的 `group_id` 置为 `NULL`。
- 如果新应用启动并成功跑完迁移，之后再回滚到旧应用镜像，旧应用可能不能正确处理这些自动 Key。
- 因此回滚分两类：
  - **应用未成功跑迁移前失败**：可只恢复旧应用容器。
  - **迁移已进入 DB 后失败**：需要使用本轮备份恢复 Postgres，再恢复旧应用容器。

## 执行前原则

- 不执行 `git pull`，不推送远端。
- 不停止 Postgres/Redis。
- 不修改 nginx。
- 不使用当前仓库 `deploy/docker-compose.candidate.yml` 重建数据层。
- 所有备份文件保存到 `deploy/backups/`，权限设为 `600`。
- 不在文档、日志摘要或提交中记录完整 API Key、JWT secret、SMTP 密码等敏感值。

## Task 1：执行前本地和公网只读检查

```bash
cd /Users/wujianxiang/CodeSpace/sub2api
git branch --show-current
git rev-parse --short HEAD
git status --short --untracked-files=all
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
curl -fsS -o /dev/null -w '18084=%{http_code}\n' http://127.0.0.1:18084/health
curl -fsS -o /dev/null -w '8080=%{http_code}\n' http://127.0.0.1:8080/health
curl -fsS -o /dev/null -w 'public=%{http_code}\n' https://api.aaccx.pw/health
```

预期：

- 当前分支为 `main`。
- HEAD 为 `9966c6f5b` 或之后明确确认过的新提交。
- 除 `.superpowers/...` 临时文件外，没有未提交源码改动。
- `sub2api-candidate*` 三个容器 healthy。
- 三个 health 检查均为 200。

## Task 2：构建本地 main 新镜像

```bash
cd /Users/wujianxiang/CodeSpace/sub2api
export TS="$(date +%Y%m%d-%H%M%S)"
export SHA="$(git rev-parse --short HEAD)"
export NEW_IMAGE="sub2api-candidate:${TS}-${SHA}-local-main"

docker build \
  --build-arg GOPROXY=https://goproxy.cn,direct \
  --build-arg GOSUMDB=sum.golang.google.cn \
  -t "${NEW_IMAGE}" \
  -f Dockerfile \
  .

docker image inspect "${NEW_IMAGE}" --format '{{.Id}} {{json .RepoTags}}'
```

预期：

- Docker build 成功。
- 新镜像 tag 形如 `sub2api-candidate:20260706-HHMMSS-9966c6f-local-main`。

## Task 3：备份公网候选数据层

```bash
cd /Users/wujianxiang/CodeSpace/sub2api
export TS="${TS:-$(date +%Y%m%d-%H%M%S)}"
mkdir -p deploy/backups

export PG_BACKUP="deploy/backups/${TS}-sub2api-candidate-postgres-before-main-redeploy.dump"
export REDIS_BACKUP="deploy/backups/${TS}-sub2api-candidate-redis-before-main-redeploy.rdb"

docker exec sub2api-candidate-postgres sh -lc \
  'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --no-owner --no-privileges --file=/tmp/sub2api-before-main-redeploy.dump'
docker cp sub2api-candidate-postgres:/tmp/sub2api-before-main-redeploy.dump "${PG_BACKUP}"
docker exec sub2api-candidate-postgres rm -f /tmp/sub2api-before-main-redeploy.dump
chmod 600 "${PG_BACKUP}"
pg_restore -l "${PG_BACKUP}" >/dev/null

docker exec sub2api-candidate-redis sh -lc 'redis-cli SAVE'
docker cp sub2api-candidate-redis:/data/dump.rdb "${REDIS_BACKUP}"
chmod 600 "${REDIS_BACKUP}"
ls -lh "${PG_BACKUP}" "${REDIS_BACKUP}"
```

预期：

- Postgres dump 可被 `pg_restore -l` 读取。
- Redis RDB 文件存在。
- 两个备份权限为 `600`。

## Task 4：记录发布前数据库快照

```bash
docker exec sub2api-candidate-postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "
SELECT
  (SELECT COUNT(*) FROM users WHERE deleted_at IS NULL) AS active_users,
  (SELECT COUNT(*) FROM api_keys WHERE deleted_at IS NULL) AS active_keys,
  (SELECT COUNT(*) FROM schema_migrations) AS migrations,
  (SELECT COALESCE(MAX(filename), '''') FROM schema_migrations) AS latest_migration;
"'

docker exec sub2api-candidate-postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "
SELECT id, name, platform, subscription_type, status
FROM groups
WHERE name = ''traffic-pack-openai''
ORDER BY id;
"'
```

预期：

- 记录当前用户数、Key 数、migration 数。
- 发布前通常还没有 `159_auto_api_key_effective_group.sql`，也可能没有 `traffic-pack-openai`。

## Task 5：保存旧应用容器配置并替换应用容器

先把旧容器 env 保存到临时文件。该文件含运行态 secret，只放 `/tmp`，不提交，不写入文档正文。

```bash
cd /Users/wujianxiang/CodeSpace/sub2api
export TS="${TS:-$(date +%Y%m%d-%H%M%S)}"
export NEW_IMAGE="${NEW_IMAGE:?NEW_IMAGE is required}"
export OLD_CONTAINER="sub2api-candidate-before-${TS}"
export ENV_FILE="/tmp/sub2api-candidate-env-${TS}.env"

docker inspect sub2api-candidate --format '{{range .Config.Env}}{{println .}}{{end}}' > "${ENV_FILE}"
chmod 600 "${ENV_FILE}"

docker inspect sub2api-candidate --format 'old_image={{.Config.Image}} old_image_id={{.Image}}'
docker stop sub2api-candidate
docker rename sub2api-candidate "${OLD_CONTAINER}"

docker run -d \
  --name sub2api-candidate \
  --restart=no \
  --ulimit nofile=100000:100000 \
  --env-file "${ENV_FILE}" \
  -p 127.0.0.1:18084:8080 \
  -v /Users/wujianxiang/CodeSpace/sub2api/.worktrees/codex-sub2api-candidate-rehearsal-20260626/deploy/candidate/data:/app/data:rw \
  --network sub2api-candidate-network \
  --health-cmd 'wget -q -T 5 -O /dev/null http://localhost:8080/health' \
  --health-interval 10s \
  --health-timeout 5s \
  --health-retries 6 \
  --health-start-period 20s \
  "${NEW_IMAGE}"
```

预期：

- 只停止并替换 `sub2api-candidate` 应用容器。
- `sub2api-candidate-postgres` 和 `sub2api-candidate-redis` 不停止、不重建。
- 旧应用容器保留为 `sub2api-candidate-before-${TS}`，便于快速应用层回滚。

## Task 6：等待新应用健康并验证 migration

```bash
for i in $(seq 1 30); do
  status="$(docker inspect sub2api-candidate --format '{{.State.Health.Status}}' 2>/dev/null || true)"
  echo "health=${status}"
  if [ "${status}" = "healthy" ]; then
    break
  fi
  sleep 3
done

docker logs --tail 200 sub2api-candidate

curl -fsS -o /dev/null -w '18084=%{http_code}\n' http://127.0.0.1:18084/health
curl -fsS -o /dev/null -w '8080=%{http_code}\n' http://127.0.0.1:8080/health
curl -fsS -o /dev/null -w 'public=%{http_code}\n' https://api.aaccx.pw/health

docker exec sub2api-candidate-postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "
SELECT filename
FROM schema_migrations
ORDER BY filename DESC
LIMIT 5;
"'

docker exec sub2api-candidate-postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "
SELECT id, name, platform, subscription_type, is_exclusive, status, allow_image_generation
FROM groups
WHERE name = ''traffic-pack-openai''
  AND deleted_at IS NULL;
"'

docker exec sub2api-candidate-postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "
SELECT COUNT(*)
FROM account_groups ag
JOIN groups g ON g.id = ag.group_id
JOIN accounts a ON a.id = ag.account_id
WHERE g.name = ''traffic-pack-openai''
  AND a.platform = ''openai''
  AND a.deleted_at IS NULL;
"'
```

预期：

- 新容器变为 healthy。
- 三个 health 检查均为 200。
- `schema_migrations` 包含 `159_auto_api_key_effective_group.sql`。
- `traffic-pack-openai` 为 active/openai/standard/exclusive，允许生图。
- 至少一个未删除 OpenAI 上游账号绑定到 `traffic-pack-openai`；如果当前没有 OpenAI 上游账号，则该计数可能为 0，需要结合 `accounts` 表确认。

## Task 7：公网页面与 API smoke test

```bash
curl -fsS -o /dev/null -w 'dashboard=%{http_code}\n' https://aaccx.pw/dashboard
curl -fsS -o /dev/null -w 'purchase=%{http_code}\n' https://aaccx.pw/purchase
curl -fsS -o /dev/null -w 'api_models_unauth=%{http_code}\n' https://api.aaccx.pw/v1/models
curl -fsS -o /dev/null -w 'responses_unauth=%{http_code}\n' https://api.aaccx.pw/v1/responses
docker logs --tail 300 sub2api-candidate | rg -i 'checksum mismatch|migration .*failed|panic|failed to initialize application' || true
```

预期：

- 页面路由返回 200。
- 未鉴权 API 路由返回 401 或 405/400 等受控错误，不返回 502/503。
- 日志中没有 migration failed、checksum mismatch、panic。

## Task 8：失败处理与回滚

### 8.1 新容器启动失败且 159 未进入 DB

判断：

```bash
docker exec sub2api-candidate-postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "
SELECT COUNT(*)
FROM schema_migrations
WHERE filename = ''159_auto_api_key_effective_group.sql'';
"'
```

如果返回 `0`，可只恢复旧应用容器：

```bash
docker logs --tail 300 sub2api-candidate || true
docker rm -f sub2api-candidate || true
docker rename "${OLD_CONTAINER}" sub2api-candidate
docker start sub2api-candidate
curl -fsS -o /dev/null -w '18084=%{http_code}\n' http://127.0.0.1:18084/health
curl -fsS -o /dev/null -w '8080=%{http_code}\n' http://127.0.0.1:8080/health
```

### 8.2 159 已进入 DB 或自动 Key 数据已变更后需要回滚

如果 `159_auto_api_key_effective_group.sql` 已应用，旧应用可能不能正确处理 `group_id=NULL` 的 OpenAI Key。此时需要恢复 Postgres 备份，再恢复旧应用容器。

```bash
export PG_BACKUP="${PG_BACKUP:?PG_BACKUP is required}"
docker rm -f sub2api-candidate || true

docker cp "${PG_BACKUP}" sub2api-candidate-postgres:/tmp/sub2api-rollback.dump
docker exec sub2api-candidate-postgres sh -lc \
  'dropdb -U "$POSTGRES_USER" "$POSTGRES_DB" --force && createdb -U "$POSTGRES_USER" "$POSTGRES_DB"'
docker exec sub2api-candidate-postgres sh -lc \
  'pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists --no-owner --no-privileges /tmp/sub2api-rollback.dump'
docker exec sub2api-candidate-postgres rm -f /tmp/sub2api-rollback.dump

docker rename "${OLD_CONTAINER}" sub2api-candidate
docker start sub2api-candidate
curl -fsS -o /dev/null -w '18084=%{http_code}\n' http://127.0.0.1:18084/health
curl -fsS -o /dev/null -w '8080=%{http_code}\n' http://127.0.0.1:8080/health
```

Redis 通常不需要恢复；如果确认 session/cache 层也要完全回到发布前，再执行：

```bash
export REDIS_BACKUP="${REDIS_BACKUP:?REDIS_BACKUP is required}"
docker cp "${REDIS_BACKUP}" sub2api-candidate-redis:/data/dump.rdb
docker restart sub2api-candidate-redis
```

## Task 9：成功后清理与记录

成功稳定后再删除旧应用容器和临时 env 文件：

```bash
docker rm "${OLD_CONTAINER}"
rm -f "${ENV_FILE}"
docker image inspect "${NEW_IMAGE}" --format '{{.Id}} {{json .RepoTags}}'
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
```

然后新建结果文档：

- `docs/ai/context/YYYYMMDD-HHMMSS-public-main-redeploy-result_CN.md`

结果文档必须记录：

- 新镜像 tag 与 image id。
- Postgres/Redis 备份文件路径和权限。
- 发布前后 migration 数、最新 migration。
- health/smoke test 结果。
- 是否执行回滚。
- 明确说明未修改 nginx，未重建 Postgres/Redis。

同时追加 `AGENTS.md` 运行态索引。

## 执行确认点

执行前需要用户确认：

1. 允许我构建新 Docker 镜像。
2. 允许我备份当前公网 `sub2api-candidate-postgres` 与 `sub2api-candidate-redis`。
3. 允许我短暂停止并替换 `sub2api-candidate` 应用容器。
4. 接受新应用启动时自动应用 `159_auto_api_key_effective_group.sql` 到公网 DB。
