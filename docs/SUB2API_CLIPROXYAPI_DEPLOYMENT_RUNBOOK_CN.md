# Sub2API + CLIProxyAPI 部署 Runbook

本文给新人执行 Sub2API 公网链路重新部署、验收和 `auth_unavailable` / 502 排障使用。按本文操作前，先确认本次目标是“部署文档里的流程”，不是临时改运行态。

## 适用范围

- 适用于当前单机公网链路：`Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> sub2api-candidate 127.0.0.1:18084 -> CLIProxyAPI 127.0.0.1:8317`。
- 适用于只替换 Sub2API 应用容器的常规发布：保留 `sub2api-candidate-postgres`、`sub2api-candidate-redis`、Nginx 和 Cloudflare Tunnel。
- 不覆盖跨机器迁移、整库恢复、支付密钥轮换、CLIProxyAPI OAuth 账号池迁移。这些需要单独计划、备份和授权。

## 职责边界

| 组件 | 职责 | 不能混淆的点 |
|---|---|---|
| Sub2API | 唯一公网 API 入口、唯一用户 Key、订阅/余额/流量卡计费、`usage_facts` 和 `usage_logs` 事实源 | 不管理真实 Codex OAuth 账号池；不要把 Sub2API 用户 Key 拿去直接请求 CLIProxyAPI |
| CLIProxyAPI | 内网聚合上游、Codex OAuth 账号池、模型 provider 映射、协议转换、账号调度和轮询 | 不是公网用户系统；它只认识自己的内部转发密钥 |
| Nginx | 本机反向代理，把 Cloudflare Tunnel 流量转给 `127.0.0.1:18084` | 不能凭旧端口记忆判断当前运行态 |
| Cloudflare Tunnel | 公网入口到本机 Nginx | 不直接连 Docker 容器 |
| PostgreSQL / Redis | Sub2API 业务事实和调度缓存 | 任何修改前必须备份并验证备份可读 |

当前关键事实：

- 当前公网应用容器：`sub2api-candidate`
- 当前公网数据库容器：`sub2api-candidate-postgres`
- 当前公网 Redis 容器：`sub2api-candidate-redis`
- 当前公网宿主端口：`127.0.0.1:18084 -> 容器 8080`
- Sub2API 容器内访问 CLIProxyAPI：`https://host.docker.internal:8317/v1`
- CLIProxyAPI 8317 当前是 HTTPS/TLS。用 HTTP 打 8317 出现 `Empty reply` 只是协议错，不代表模型失败。

## 部署红线

- 不要在未确认目标环境时执行任何会停止、删除、重建 DB/Redis/Nginx/Cloudflare Tunnel 的命令。
- 不要运行 `docker compose down -v`、删除 volume、删除 `deploy/candidate/*` 数据目录，除非已有单独授权和可读备份。
- 电脑空间不足时，先清旧容器和无用镜像，再重新部署；不要通过删除 DB/Redis volume 或业务备份来换空间。
- 不要把完整用户 API Key、CLIProxyAPI 内部转发密钥、HMAC secret、SMTP 密码、支付密钥写进文档、提交、日志或聊天。
- 不要用 Sub2API 用户 Key 直接打 CLIProxyAPI；CLIProxyAPI 返回 `Invalid API key` 在这种情况下是正常结果。
- 不要把历史 HTTP 阶段当作当前事实。当前 8317 是 TLS，Sub2API upstream `base_url` 应是 `https://host.docker.internal:8317/v1`。
- 数据层回滚必须单独授权；应用发布失败时只做应用容器回滚，不擅自恢复整库。

## 部署前确认

### 1. 确认代码和工作区

```bash
cd /Users/wujianxiang/CodeSpace/sub2api
git status --short --branch
git log --oneline --decorate -5
git ls-files --others --exclude-standard docs/ai/context
```

要求：

- 发布用 worktree 应尽量干净；如果有未提交改动，必须确认它们就是本次要发布的内容。
- `docs/ai/context` 未跟踪文件需要确认无敏感信息，按任务决定是否提交。

### 2. 确认当前运行容器

```bash
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}' | rg 'sub2api|postgres|redis'
docker inspect sub2api-candidate --format '{{ index .Config.Labels "com.docker.compose.project" }}'
docker inspect sub2api-candidate --format '{{range .Mounts}}{{println .Source "->" .Destination}}{{end}}'
docker inspect sub2api-candidate --format '{{range .NetworkSettings.Networks}}{{println .NetworkID}}{{end}}'
```

要求：

- `sub2api-candidate`、`sub2api-candidate-postgres`、`sub2api-candidate-redis` 存在且是当前公网事实源。
- 应用端口应包含 `127.0.0.1:18084->8080/tcp`。
- Compose project、network、volume 不得和预演环境混淆。

### 3. 确认公网指向

```bash
curl -fsS http://127.0.0.1:18084/health
curl -fsS http://127.0.0.1:8080/health
curl -fsS https://api.aaccx.pw/health
pgrep -af cloudflared
nginx -T 2>/dev/null | rg 'api\.aaccx\.pw|aaccx\.pw|127\.0\.0\.1:18084|127\.0\.0\.1:8080'
```

要求：

- 三个 health 都返回 200。
- Nginx 实际指向 `127.0.0.1:18084`，不要凭 18080、18082、18085 的历史记忆操作。

### 4. 清理旧容器和无用镜像释放空间

空间不足时，必须先完成本节，再构建镜像或重新部署。清理前已经完成上面的“当前运行容器”和“公网指向”确认，所以能明确保护当前生产栈。

先看磁盘和 Docker 占用：

```bash
df -h /
docker system df -v
docker ps -a --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
docker images --format 'table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.CreatedSince}}\t{{.Size}}' | sed -n '1,80p'
```

必须保护这些当前生产容器，不允许删除：

- `sub2api-candidate`
- `sub2api-candidate-postgres`
- `sub2api-candidate-redis`

删除历史 `before-promote` 容器后，将失去对应旧版本的应用容器回滚点；当前正在运行的 `sub2api-candidate` 不受影响，下一次 `promote` 仍会把它重命名为新的回滚容器。

先列出已停止的历史容器候选：

```bash
OLD_CONTAINERS="$(
  docker ps -a --filter status=exited --filter status=created --format '{{.Names}}' |
    rg '^(sub2api-candidate-before|sub2api-before|sub2api-main-preview|sub2api-smtp-test|sub2api-.*preview|sub2api-.*test)' || true
)"

if [ -z "$OLD_CONTAINERS" ]; then
  echo '没有匹配到可清理的已停止旧容器'
else
  printf '将清理以下已停止旧容器：\n%s\n' "$OLD_CONTAINERS"
fi
```

确认列表里没有当前生产容器后，再删除：

```bash
if [ -n "$OLD_CONTAINERS" ]; then
  printf '%s\n' "$OLD_CONTAINERS" |
    while IFS= read -r name; do
      case "$name" in
        sub2api-candidate|sub2api-candidate-postgres|sub2api-candidate-redis)
          echo "跳过受保护容器：$name"
          ;;
        *)
          docker rm "$name"
          ;;
      esac
    done
fi
```

再清理 dangling image 和旧 build cache：

```bash
docker image prune -f
docker builder prune -f --filter 'until=168h'
docker system df
```

如果空间仍不足，再删除未被任何容器引用的旧 `sub2api-candidate:*` 镜像。先只读列出：

```bash
docker images 'sub2api-candidate' --format 'table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.CreatedSince}}\t{{.Size}}'
docker ps -a --format '{{.Image}}' | sort -u
```

然后手动选择不在任何容器 `IMAGE` 列中的旧镜像标签删除：

```bash
docker image rm sub2api-candidate:<旧标签>
docker system df
df -h /
```

禁止执行：

- `docker volume prune`
- `docker system prune --volumes`
- `docker compose down -v`
- `rm -rf deploy/candidate/postgres_data deploy/candidate/redis_data deploy/backups`

清理完成要求：

- `df -h /` 至少有足够空间容纳一次 Postgres 备份和一次新镜像构建。
- `docker ps` 里当前生产三容器仍存在且状态正常。
- `curl -fsS http://127.0.0.1:18084/health` 仍返回 200。

### 5. 备份 Postgres 并验证可读

```bash
TS="$(date +%Y%m%d-%H%M%S)"
mkdir -p deploy/backups
PG_DUMP="deploy/backups/${TS}-sub2api-candidate-postgres-before-deploy.dump"

docker exec sub2api-candidate-postgres sh -lc \
  'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --no-owner --no-privileges --file=/tmp/sub2api-before-deploy.dump'
docker cp sub2api-candidate-postgres:/tmp/sub2api-before-deploy.dump "$PG_DUMP"
docker exec sub2api-candidate-postgres rm -f /tmp/sub2api-before-deploy.dump

docker cp "$PG_DUMP" sub2api-candidate-postgres:/tmp/sub2api-verify.dump
docker exec sub2api-candidate-postgres sh -lc 'pg_restore --list /tmp/sub2api-verify.dump >/dev/null'
docker exec sub2api-candidate-postgres rm -f /tmp/sub2api-verify.dump
ls -lh "$PG_DUMP"
```

要求：

- `pg_restore --list` 成功。
- 只记录备份路径、大小、权限，不复制 dump 内容。

### 6. 备份 Redis 并验证可读

```bash
REDIS_RDB="deploy/backups/${TS}-sub2api-candidate-redis-before-deploy.rdb"

docker exec sub2api-candidate-redis sh -lc \
  'redis-cli SAVE >/dev/null && cp /data/dump.rdb /tmp/sub2api-before-deploy.rdb && redis-check-rdb /tmp/sub2api-before-deploy.rdb >/dev/null'
docker cp sub2api-candidate-redis:/tmp/sub2api-before-deploy.rdb "$REDIS_RDB"
docker exec sub2api-candidate-redis rm -f /tmp/sub2api-before-deploy.rdb
ls -lh "$REDIS_RDB"
```

要求：

- `redis-check-rdb` 成功。
- Redis 备份只用于应急；不要在应用回滚时自动恢复 Redis。

## 配置核对

### 1. 核对 Sub2API 上游账号

只读检查，不打印完整内部转发密钥：

```bash
docker exec -i sub2api-candidate-postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -P pager=off' <<'SQL'
SELECT
  id,
  name,
  platform,
  type,
  status,
  schedulable,
  credentials->>'base_url' AS base_url,
  credentials ? 'api_key' AS has_upstream_key,
  credentials->>'pool_mode' AS pool_mode,
  temp_unschedulable_until,
  temp_unschedulable_reason
FROM accounts
WHERE id = 1;
SQL
```

要求：

- `id=1` 名称应是 `cliproxy-local-openai` 或当前等价的 CLIProxyAPI 内网上游入口。
- `base_url` 应为 `https://host.docker.internal:8317/v1`。
- `has_upstream_key` 应为 `t`。
- `pool_mode` 应为 `true` 或配置语义等价。
- `status=active` 且 `schedulable=true`。
- `temp_unschedulable_until` 应为空，或已过期。

如需修正内部转发密钥，优先通过管理后台填写。必须走 SQL 时，先完成备份，不要把真实密钥写进脚本文件或文档。

### 2. 核对 CLIProxyAPI TLS

当前 8317 是 TLS。先做协议探针：

```bash
openssl s_client -connect 127.0.0.1:8317 -servername localhost </dev/null 2>/dev/null | openssl x509 -noout -subject -issuer -dates
curl -k -sS -o /dev/null -w '%{http_code}\n' https://127.0.0.1:8317/v1/models
```

解释：

- `openssl` 能读到证书，说明端口是 TLS。
- 未带 CLIProxyAPI 内部转发密钥时，`/v1/models` 返回 401 是可接受结果。
- 如果用 `http://127.0.0.1:8317` 返回 `Empty reply`，不要误判为 CLIProxyAPI 挂了。

再从 Sub2API 容器内验证 TLS 和连通性：

```bash
docker exec sub2api-candidate sh -lc \
  'wget -S -O /dev/null --timeout=10 https://host.docker.internal:8317/v1/models 2>&1 | sed -n "1,20p"'
```

解释：

- 如果看到 401，说明容器能完成 TLS 并到达 CLIProxyAPI。
- 如果出现 `x509: certificate signed by unknown authority` 或证书校验失败，先修 Sub2API 镜像/容器信任的 CLIProxyAPI CA，不要改成 HTTP 绕过。
- 如果出现 `connection refused`，检查 CLIProxyAPI 进程、端口和监听地址。

## 构建和发布

### 1. 构建候选镜像

```bash
cd /Users/wujianxiang/CodeSpace/sub2api
IMAGE="sub2api-candidate:$(date +%Y%m%d-%H%M%S)-$(git rev-parse --short=12 HEAD)"
docker build \
  --build-arg GOPROXY=https://goproxy.cn,direct \
  --build-arg GOSUMDB=sum.golang.google.cn \
  -t "$IMAGE" \
  -f Dockerfile \
  .
docker image inspect "$IMAGE" --format '{{.Id}} {{.RepoTags}}'
```

### 2. dry-run 发布计划

```bash
deploy/promote-sub2api-candidate.sh --candidate-image "$IMAGE" --dry-run --yes
```

要求：

- 输出目标是 `public_candidate_18084`。
- 只替换 `sub2api-candidate` 应用容器。
- 不重建 `sub2api-candidate-postgres`、`sub2api-candidate-redis`、Nginx 或 Cloudflare Tunnel。

### 3. 正式发布

```bash
deploy/promote-sub2api-candidate.sh --candidate-image "$IMAGE" --yes
```

脚本会：

- 导出现有 `sub2api-candidate` 环境变量到临时文件。
- 停止并重命名旧应用容器为 `sub2api-candidate-before-promote-<timestamp>`。
- 用新镜像启动同名 `sub2api-candidate`。
- 等待 `http://127.0.0.1:18084/health` 通过。

不要在脚本外手动重建 Postgres 或 Redis。

### 4. 候选预演脚本的使用边界

`deploy/rehearse-sub2api-candidate.sh` 可用于构建并启动本地候选预演环境，但运行前必须打开脚本确认生产 dump 来源。历史上当前公网事实源曾从 `sub2api`/18080 迁移到 `sub2api-candidate`/18084，不能让脚本从旧库 dump。

建议只先执行：

```bash
deploy/rehearse-sub2api-candidate.sh --dry-run --reset-db --candidate-port 18081
```

确认 dump 源、目标 project、容器名、端口和数据目录都正确后，才允许真实运行。

## 发布后验收

### 1. 健康检查

```bash
curl -fsS http://127.0.0.1:18084/health
curl -fsS http://127.0.0.1:8080/health
curl -fsS https://api.aaccx.pw/health
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}' | rg 'sub2api-candidate'
docker logs --tail 300 sub2api-candidate | rg -i 'panic|migration .*failed|failed to initialize|x509|invalid url scheme|account_select_failed|auth_unavailable' || true
```

要求：

- 三个 health 都返回 200。
- 新 `sub2api-candidate` 为 healthy。
- 最近日志没有 panic、migration 失败、DB/Redis 初始化失败。

### 2. 公网真实 API 验证

使用一把有效 Sub2API 用户 Key。不要把完整 Key 写进终端命令历史；用 `read -s` 临时输入：

```bash
read -rsp 'Sub2API user key: ' SUB2API_USER_KEY; echo

curl -sS -o /tmp/sub2api-public-smoke.json -w '%{http_code}\n' \
  https://api.aaccx.pw/v1/chat/completions \
  -H "Authorization: Bearer ${SUB2API_USER_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-5.5",
    "messages": [
      {"role": "user", "content": "请只回复 pong"}
    ],
    "stream": false
  }'

jq '.choices[0].message.content // .error' /tmp/sub2api-public-smoke.json
unset SUB2API_USER_KEY
```

要求：

- HTTP 状态为 200。
- 返回内容正常。
- 如果这里 401，先检查用户 Key 是否属于 Sub2API，别去改 CLIProxyAPI。

### 3. 计费事实验证

```bash
docker exec -i sub2api-candidate-postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -P pager=off' <<'SQL'
SELECT
  id,
  api_key_id,
  user_id,
  account_id,
  billing_status,
  payload->'usage_log'->>'model' AS model,
  payload->'effects'->>'actual_cost' AS actual_cost,
  completed_at,
  settled_at,
  left(last_error, 120) AS last_error
FROM usage_facts
ORDER BY id DESC
LIMIT 5;
SQL
```

要求：

- 最新 smoke test 对应记录进入 `usage_facts`。
- `billing_status` 最终应为 `settled`。
- `account_id` 应指向 Sub2API 的 CLIProxyAPI 上游入口，当前通常是 `1`。

## `auth_unavailable` / 502 排障

### 历史事故定论

此前 `auth_unavailable` / 502 的闭环结论：

- CLIProxyAPI 本地 HTTPS 8317 请求 `gpt-5.5` 成功。
- Sub2API 容器内直连 CLIProxyAPI 成功。
- 公网 502 不是 CLIProxyAPI 调度器坏，也不是 Codex auth 全部没额度。
- 真正根因是 Sub2API 多次失败后把 account 1 放进失败/不可调度状态；Redis 调度快照中该账号被排除，Sub2API 日志出现 `excluded_account_count=1`。
- 清理 account 1 的临时失败调度状态、重建 Redis 调度快照、重启 Sub2API 后，本地 `127.0.0.1:18084` 和公网 `https://api.aaccx.pw/v1/chat/completions` 都返回 200，`usage_facts` 增加且最终 `settled`。

这类问题的重点是分段定位，不要一看到 `auth_unavailable` 就去改 CLIProxyAPI 账号池。

### 快速判断表

| 现象 | 优先判断 | 下一步 |
|---|---|---|
| `http://127.0.0.1:8317` 是 `Empty reply` | 协议错，8317 当前是 TLS | 改用 `https://127.0.0.1:8317` |
| 用 Sub2API 用户 Key 打 CLIProxyAPI 返回 `Invalid API key` | 正常，Key 类型错 | 用 CLIProxyAPI 内部转发密钥验证，或回到 Sub2API 公网入口测用户 Key |
| CLIProxyAPI HTTPS 直连 200，但 Sub2API 公网 502 | 问题在 Sub2API 到 CLIProxyAPI 或 Sub2API 调度状态 | 查 `base_url`、证书信任、account 1 临时失败状态、Redis 调度快照 |
| 日志 `excluded_account_count=1` 且只有 account 1 | Sub2API 已把唯一上游账号排除 | 查 DB `temp_unschedulable_until`、`schedulable` 和 Redis `sched:*` |
| 日志 `x509: certificate signed by unknown authority` | Sub2API 不信任 CLIProxyAPI 自签 CA | 修镜像/容器 CA，保持 HTTPS |
| 日志 `invalid url scheme: http` | Sub2API 安全策略拒绝 HTTP | 当前应使用 HTTPS，不建议为了绕过改 HTTP |
| 日志 `connect: connection refused` | CLIProxyAPI 未监听或地址错 | 查 CLIProxyAPI 进程、8317 监听、Docker host gateway |
| CLIProxyAPI 自身返回 `auth_unavailable: no auth available` | CLIProxyAPI 账号池确实无可用 auth | 去 CLIProxyAPI 日志查 OAuth 账号状态、quota cooldown、模型映射 |

### 分段排查步骤

1. 拿到请求时间、模型、入口路径、HTTP 状态和 request_id。不要记录完整用户 Key。
2. 查 Sub2API 日志：

   ```bash
   docker logs --since 20m sub2api-candidate | rg 'request_id|account_select|excluded_account_count|auth_unavailable|x509|invalid url scheme|host.docker.internal|8317'
   ```

3. 查 Sub2API 上游账号只读状态：

```bash
docker exec -i sub2api-candidate-postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -P pager=off' <<'SQL'
SELECT id, name, status, schedulable, temp_unschedulable_until, temp_unschedulable_reason,
       credentials->>'base_url' AS base_url,
       credentials ? 'api_key' AS has_upstream_key,
       credentials->>'pool_mode' AS pool_mode
FROM accounts
WHERE id = 1;
SQL
```

4. 查 Redis 调度快照，只读查看：

   ```bash
   docker exec sub2api-candidate-redis sh -lc 'redis-cli SMEMBERS sched:buckets'
   docker exec sub2api-candidate-redis sh -lc 'redis-cli GET sched:meta:1'
   ```

   如果需要查看某个 bucket 的快照，先从 `SMEMBERS sched:buckets` 选择 bucket，再查：

   ```bash
   BUCKET='<从 sched:buckets 输出中选择，例如 2:openai:single>'
   docker exec -e BUCKET="$BUCKET" sub2api-candidate-redis sh -lc '
     VER="$(redis-cli GET "sched:active:${BUCKET}")"
     echo "active_version=${VER}"
     test -n "$VER" && redis-cli ZRANGE "sched:${BUCKET}:v${VER}" 0 -1 WITHSCORES
   '
   ```

5. 如果确认是 account 1 临时不可调度，优先用管理接口清理临时状态：

   ```bash
   read -rsp 'Sub2API admin bearer: ' ADMIN_BEARER; echo
   curl -sS -X DELETE \
     http://127.0.0.1:18084/api/v1/admin/accounts/1/temp-unschedulable \
     -H "Authorization: Bearer ${ADMIN_BEARER}"
   unset ADMIN_BEARER
   ```

6. 清理后重启应用容器，让调度快照启动重建：

   ```bash
   docker restart sub2api-candidate
   curl -fsS http://127.0.0.1:18084/health
   ```

7. 如果管理接口不可用或 Redis 快照仍明显陈旧，不要让新人直接删除 `sched:*`。升级给熟悉 Sub2API 调度缓存的人处理；处理原则是先备份 Redis，只清 account 1 的临时调度缓存或对应 bucket 快照，不删业务数据、用户 Key、订阅、余额、流量卡或 usage facts。

## 回滚边界

应用容器回滚：

```bash
docker ps -a --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}' | rg 'sub2api-candidate-before-promote|sub2api-candidate'
OLD_CONTAINER='<上一步发布脚本保留的旧容器名>'

docker stop sub2api-candidate
docker rm sub2api-candidate
docker rename "$OLD_CONTAINER" sub2api-candidate
docker start sub2api-candidate
curl -fsS http://127.0.0.1:18084/health
curl -fsS https://api.aaccx.pw/health
```

边界：

- 这只回滚应用镜像和容器环境，不回滚数据库迁移、业务数据、Redis 状态或外部支付状态。
- 如果新版本已经跑过迁移，数据库回滚必须另行评估和授权。
- 如果回滚后公网 API 仍失败，回到“分段排查步骤”，不要连续多次重启。

## 新人执行检查清单

- [ ] 已确认本次目标环境是 `public_candidate_18084`。
- [ ] 已确认 Nginx 指向 `127.0.0.1:18084`。
- [ ] 已确认 `sub2api-candidate-postgres` 是当前事实源。
- [ ] 已清理已停止旧容器和无用镜像释放空间，未删除 DB/Redis volume、业务备份或当前生产容器。
- [ ] 已完成 Postgres 备份，并用 `pg_restore --list` 验证可读。
- [ ] 已完成 Redis 备份，并用 `redis-check-rdb` 验证可读。
- [ ] 已确认 CLIProxyAPI 8317 是 HTTPS/TLS。
- [ ] 已确认 Sub2API 上游 `base_url` 是 `https://host.docker.internal:8317/v1`。
- [ ] 已确认未在文档、命令输出或提交里暴露完整密钥。
- [ ] 已 dry-run `promote-sub2api-candidate.sh`，输出只替换应用容器。
- [ ] 已正式发布并通过 18084、8080、公网 health。
- [ ] 已用有效 Sub2API 用户 Key 跑公网 `/v1/chat/completions`。
- [ ] 已确认最新 `usage_facts.billing_status=settled`。
- [ ] 已检查 Sub2API 日志没有新的 panic、migration 失败、证书错误或账号选择失败。
