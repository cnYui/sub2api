# 公网部署订阅窗口刷新修复计划

## 目标

- 将本地 `main` 中的订阅窗口刷新修复部署到公网 `sub2api-candidate`。
- 本地最新提交：
  - `7ca7a7b57 docs: archive July 6 operations context`
  - `d6e337238 fix: refresh subscription windows before billing checks`
- 当前公网应用容器仍是旧代码镜像：
  - 容器：`sub2api-candidate`
  - 镜像：`sub2api-candidate:20260706-080023-9966c6f5b-local-main`
  - 端口：`127.0.0.1:18084->8080`
- 当前公网数据层：
  - `sub2api-candidate-postgres`
  - `sub2api-candidate-redis`
- 本次只替换应用容器，不改 nginx，不重建 Postgres/Redis，不覆盖数据目录。

## 修复内容摘要

- `BillingCacheService` 在订阅套餐限额判断前刷新过期窗口。
- Redis `billing:sub:*` 订阅缓存新增窗口字段：
  - `daily_window_start`
  - `weekly_window_start`
  - `monthly_window_start`
- 旧 Redis 订阅缓存缺窗口字段时回源 DB 自愈。
- 新增 `RefreshExpiredUsageWindows` 条件式 DB 更新，防止并发请求重复清零。
- 本次代码不包含新 migration；公网 DB 目前已是 195 migrations，最新为 `159_auto_api_key_effective_group.sql`。

## 执行原则

- 不执行 `git pull`，不推送远端。
- 不停止或重建 `sub2api-candidate-postgres`、`sub2api-candidate-redis`。
- 不修改 nginx 或 Cloudflare Tunnel。
- 不使用当前仓库 `deploy/docker-compose.candidate.yml` 执行 `up`；当前候选栈的数据 bind mount 仍在旧 candidate worktree 路径下，直接 compose 可能挂错空目录。
- 所有备份放到 `deploy/backups/`，权限设为 `600`。
- 不在文档、提交、终端摘要中记录完整 API Key、JWT secret、HMAC secret、SMTP 密码。

## 当前状态确认

执行前先做只读检查：

```bash
cd /Users/wujianxiang/CodeSpace/sub2api

git branch --show-current
git rev-parse --short HEAD
git log --oneline -3
git status --short --untracked-files=all

docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
docker inspect sub2api-candidate --format 'image={{.Config.Image}} image_id={{.Image}} network={{range $k,$v := .NetworkSettings.Networks}}{{$k}}{{end}}'
docker inspect sub2api-candidate --format '{{range .Mounts}}{{println .Source "->" .Destination}}{{end}}'

curl -fsS -o /dev/null -w '18084=%{http_code}\n' http://127.0.0.1:18084/health
curl -fsS -o /dev/null -w '8080=%{http_code}\n' http://127.0.0.1:8080/health
curl -fsS -o /dev/null -w 'public=%{http_code}\n' https://api.aaccx.pw/health
```

预期：

- 当前分支是 `main`。
- HEAD 是 `7ca7a7b57` 或之后明确确认过的新提交。
- 没有未提交源码改动；允许存在未跟本次发布相关的本地临时目录，例如 `.superpowers/`。
- 三个 health 检查均为 200。
- `sub2api-candidate` 当前镜像仍为 `20260706-080023-9966c6f5b-local-main`，说明订阅窗口刷新修复尚未部署。

## 发布前本地验证

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/backend
go test -count=1 -tags=unit ./internal/service ./internal/repository

cd /Users/wujianxiang/CodeSpace/sub2api
git diff --check
```

预期：

- service/repository unit 测试通过。
- `git diff --check` 无输出。

## 构建新应用镜像

```bash
cd /Users/wujianxiang/CodeSpace/sub2api

export TS="$(date +%Y%m%d-%H%M%S)"
export SHA="$(git rev-parse --short HEAD)"
export NEW_IMAGE="sub2api-candidate:${TS}-${SHA}-subscription-window-refresh"

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
- 新镜像 tag 形如 `sub2api-candidate:20260707-HHMMSS-7ca7a7b57-subscription-window-refresh`。

## 备份公网数据层

### Postgres 备份

```bash
cd /Users/wujianxiang/CodeSpace/sub2api

export TS="${TS:-$(date +%Y%m%d-%H%M%S)}"
mkdir -p deploy/backups

export PG_BACKUP="deploy/backups/${TS}-sub2api-candidate-postgres-before-subscription-window-refresh.dump"

docker exec sub2api-candidate-postgres sh -lc \
  'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --no-owner --no-privileges --file=/tmp/sub2api-before-subscription-window-refresh.dump'

docker cp sub2api-candidate-postgres:/tmp/sub2api-before-subscription-window-refresh.dump "${PG_BACKUP}"
docker exec sub2api-candidate-postgres rm -f /tmp/sub2api-before-subscription-window-refresh.dump

chmod 600 "${PG_BACKUP}"
pg_restore -l "${PG_BACKUP}" >/dev/null
ls -lh "${PG_BACKUP}"
```

预期：

- `pg_restore -l` 能读取 dump。
- dump 文件权限为 `600`。

### Redis 备份

```bash
cd /Users/wujianxiang/CodeSpace/sub2api

export TS="${TS:-$(date +%Y%m%d-%H%M%S)}"
export REDIS_BACKUP="deploy/backups/${TS}-sub2api-candidate-redis-before-subscription-window-refresh.rdb"

docker exec sub2api-candidate-redis sh -lc 'redis-cli SAVE'
docker cp sub2api-candidate-redis:/data/dump.rdb "${REDIS_BACKUP}"
chmod 600 "${REDIS_BACKUP}"
ls -lh "${REDIS_BACKUP}"
```

预期：

- Redis RDB 文件存在。
- RDB 文件权限为 `600`。

## 记录发布前数据库快照

```bash
docker exec sub2api-candidate-postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "
SELECT
  (SELECT COUNT(*) FROM users WHERE deleted_at IS NULL) AS active_users,
  (SELECT COUNT(*) FROM api_keys WHERE deleted_at IS NULL) AS active_keys,
  (SELECT COUNT(*) FROM user_subscriptions WHERE deleted_at IS NULL AND status = ''active'' AND expires_at > NOW()) AS active_subscriptions,
  (SELECT COUNT(*) FROM schema_migrations) AS migrations,
  (SELECT COALESCE(MAX(filename), '''') FROM schema_migrations) AS latest_migration;
"'

docker exec sub2api-candidate-postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "
SELECT COUNT(*)
FROM user_subscriptions us
JOIN groups g ON g.id = us.group_id AND g.deleted_at IS NULL
WHERE us.deleted_at IS NULL
  AND us.status = ''active''
  AND us.expires_at > NOW()
  AND g.daily_limit_usd IS NOT NULL
  AND g.daily_limit_usd > 0
  AND us.daily_window_start < date_trunc(''day'', NOW());
"'
```

预期：

- migration 数应为 195，最新包含 `159_auto_api_key_effective_group.sql`。
- stale 日窗口数应为 0；如果非 0，先暂停发布并重新评估是否需要运行态修复。

## 保存旧应用容器配置

旧容器 env 包含运行态 secret，只保存到 `/tmp`，不要提交，不要复制到文档。

```bash
cd /Users/wujianxiang/CodeSpace/sub2api

export TS="${TS:-$(date +%Y%m%d-%H%M%S)}"
export OLD_CONTAINER="sub2api-candidate-before-window-refresh-${TS}"
export ENV_FILE="/tmp/sub2api-candidate-env-${TS}.env"

docker inspect sub2api-candidate --format '{{range .Config.Env}}{{println .}}{{end}}' > "${ENV_FILE}"
chmod 600 "${ENV_FILE}"

docker inspect sub2api-candidate --format 'old_image={{.Config.Image}} old_image_id={{.Image}}'
docker inspect sub2api-candidate --format 'restart_policy={{json .HostConfig.RestartPolicy}}'
docker inspect sub2api-candidate --format '{{range .Mounts}}{{println .Source "->" .Destination}}{{end}}'
```

预期：

- `/tmp/sub2api-candidate-env-${TS}.env` 存在且权限为 `600`。
- 已记录旧镜像 tag 和 image id，便于应用层回滚。

## 替换应用容器

使用 `docker run` 显式复刻当前候选应用容器，不使用 compose。

```bash
cd /Users/wujianxiang/CodeSpace/sub2api

export NEW_IMAGE="${NEW_IMAGE:?NEW_IMAGE is required}"
export ENV_FILE="${ENV_FILE:?ENV_FILE is required}"
export OLD_CONTAINER="${OLD_CONTAINER:?OLD_CONTAINER is required}"

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

- 只替换 `sub2api-candidate` 应用容器。
- `sub2api-candidate-postgres` 和 `sub2api-candidate-redis` 保持运行。
- 旧应用容器保留为 `${OLD_CONTAINER}`。

## 发布后健康检查

```bash
for i in $(seq 1 30); do
  status="$(docker inspect sub2api-candidate --format '{{.State.Health.Status}}' 2>/dev/null || true)"
  echo "health=${status}"
  if [ "${status}" = "healthy" ]; then
    break
  fi
  sleep 3
done

docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}' | sed -n '1,20p'
docker logs --tail 200 sub2api-candidate

curl -fsS -o /dev/null -w '18084=%{http_code}\n' http://127.0.0.1:18084/health
curl -fsS -o /dev/null -w '8080=%{http_code}\n' http://127.0.0.1:8080/health
curl -fsS -o /dev/null -w 'public=%{http_code}\n' https://api.aaccx.pw/health
curl -fsS -o /dev/null -w 'dashboard=%{http_code}\n' https://aaccx.pw/dashboard
curl -fsS -o /dev/null -w 'purchase=%{http_code}\n' https://aaccx.pw/purchase
```

预期：

- `sub2api-candidate` 为 `healthy`。
- health、dashboard、purchase 均返回 200。
- 日志中没有 `panic`、`migration failed`、`checksum mismatch`、`listen tcp` 端口冲突等错误。

## 发布后数据库与缓存检查

```bash
docker exec sub2api-candidate-postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "
SELECT COUNT(*), MAX(filename)
FROM schema_migrations;
"'

docker exec sub2api-candidate-postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "
SELECT COUNT(*)
FROM user_subscriptions us
JOIN groups g ON g.id = us.group_id AND g.deleted_at IS NULL
WHERE us.deleted_at IS NULL
  AND us.status = ''active''
  AND us.expires_at > NOW()
  AND g.daily_limit_usd IS NOT NULL
  AND g.daily_limit_usd > 0
  AND us.daily_window_start < date_trunc(''day'', NOW());
"'
```

预期：

- migration 数仍为 195；本次修复不应新增 migration。
- stale 日窗口数仍为 0，除非执行期间跨日或另有并发写入。

## 可选真实 API 验证

如果需要做真实 OpenAI 兼容请求验证，使用操作者手动提供的 Key，不在命令历史或文档里回显完整 Key。

```bash
read -rsp 'TEST_API_KEY=' TEST_API_KEY; echo

curl -sS https://api.aaccx.pw/v1/responses \
  -H "Authorization: Bearer ${TEST_API_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-5.5","input":"reply with ok"}' \
  -o /tmp/sub2api-window-refresh-smoke.json \
  -w 'api_http=%{http_code}\n'

python3 - <<'PY'
import json
p = "/tmp/sub2api-window-refresh-smoke.json"
with open(p, "r", encoding="utf-8") as f:
    data = json.load(f)
print(json.dumps({
    "id": data.get("id"),
    "status": data.get("status"),
    "error": data.get("error"),
}, ensure_ascii=False, indent=2))
PY

unset TEST_API_KEY
rm -f /tmp/sub2api-window-refresh-smoke.json
```

预期：

- HTTP 返回 200。
- 不在终端摘要或文档中记录完整 Key。

## 应用层回滚

如果新应用容器启动失败、health 不通过、日志明显异常，优先只回滚应用容器：

```bash
export OLD_CONTAINER="${OLD_CONTAINER:?OLD_CONTAINER is required}"

docker logs --tail 200 sub2api-candidate || true
docker rm -f sub2api-candidate || true
docker rename "${OLD_CONTAINER}" sub2api-candidate
docker start sub2api-candidate

for i in $(seq 1 30); do
  status="$(docker inspect sub2api-candidate --format '{{.State.Health.Status}}' 2>/dev/null || true)"
  echo "health=${status}"
  if [ "${status}" = "healthy" ]; then
    break
  fi
  sleep 3
done

curl -fsS -o /dev/null -w '18084=%{http_code}\n' http://127.0.0.1:18084/health
curl -fsS -o /dev/null -w '8080=%{http_code}\n' http://127.0.0.1:8080/health
curl -fsS -o /dev/null -w 'public=%{http_code}\n' https://api.aaccx.pw/health
```

说明：

- 本次修复没有 schema migration；大多数失败只需要应用层回滚。
- 发布后如果已有真实请求发生，DB 可能包含正常的新 usage 或窗口刷新写入，不应轻易恢复整库备份。

## 数据层回滚

只有在明确确认新代码造成错误 DB 写入，且应用层回滚不足以恢复时，才考虑数据层回滚。执行前必须再次备份当前故障态数据，因为恢复发布前 dump 会丢失发布后的真实订单、用量、Key 变更等运行态数据。

```bash
cd /Users/wujianxiang/CodeSpace/sub2api
export ROLLBACK_TS="$(date +%Y%m%d-%H%M%S)"
export FAILURE_PG_BACKUP="deploy/backups/${ROLLBACK_TS}-sub2api-candidate-postgres-before-data-rollback.dump"

docker exec sub2api-candidate-postgres sh -lc \
  'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --no-owner --no-privileges --file=/tmp/sub2api-before-data-rollback.dump'
docker cp sub2api-candidate-postgres:/tmp/sub2api-before-data-rollback.dump "${FAILURE_PG_BACKUP}"
docker exec sub2api-candidate-postgres rm -f /tmp/sub2api-before-data-rollback.dump
chmod 600 "${FAILURE_PG_BACKUP}"
pg_restore -l "${FAILURE_PG_BACKUP}" >/dev/null
```

Postgres 恢复示例：

```bash
export PG_BACKUP="${PG_BACKUP:?PG_BACKUP is required}"

docker stop sub2api-candidate

docker exec sub2api-candidate-postgres sh -lc \
  'dropdb -U "$POSTGRES_USER" "$POSTGRES_DB" && createdb -U "$POSTGRES_USER" "$POSTGRES_DB"'

docker cp "${PG_BACKUP}" sub2api-candidate-postgres:/tmp/restore.dump
docker exec sub2api-candidate-postgres sh -lc \
  'pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --no-owner --no-privileges /tmp/restore.dump'
docker exec sub2api-candidate-postgres rm -f /tmp/restore.dump

docker start sub2api-candidate
```

Redis 恢复示例：

```bash
export REDIS_BACKUP="${REDIS_BACKUP:?REDIS_BACKUP is required}"

docker stop sub2api-candidate
docker stop sub2api-candidate-redis
docker cp "${REDIS_BACKUP}" sub2api-candidate-redis:/data/dump.rdb
docker start sub2api-candidate-redis
docker start sub2api-candidate
```

数据层回滚后必须重新跑 health、DB 快照和至少一个只读页面 smoke test。

## 收尾

发布成功后：

```bash
rm -f "${ENV_FILE}"
docker inspect sub2api-candidate --format 'new_image={{.Config.Image}} image_id={{.Image}}'
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}' | sed -n '1,20p'
```

建议新建结果文档：

```bash
cd /Users/wujianxiang/CodeSpace/sub2api
date +%Y%m%d-%H%M%S
```

文件名示例：

- `docs/ai/context/YYYYMMDD-HHMMSS-public-subscription-window-refresh-redeploy-result_CN.md`

结果文档至少记录：

- 新镜像 tag 和 image id。
- Postgres/Redis 备份文件路径、权限、大小。
- 发布前后 health 结果。
- migration 数是否保持 195。
- 是否做了真实 API smoke test。
- 是否执行回滚。
