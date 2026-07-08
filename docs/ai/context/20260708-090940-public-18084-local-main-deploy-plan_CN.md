# 本地 main 发布到公网 18084 Implementation Plan

> 给执行代理：按本文逐项执行，步骤使用 checkbox 追踪；发布前必须再次确认当前工作区就是要发布的版本。

## 目标

将当前本地 `main` 的 Sub2API 代码发布到公网 18084 链路：

`Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> sub2api-candidate 127.0.0.1:18084 -> CLIProxyAPI 127.0.0.1:8317`

本次只替换 `sub2api-candidate` 应用容器，保留：

- `sub2api-candidate-postgres`
- `sub2api-candidate-redis`
- nginx / Cloudflare Tunnel
- 当前 18084 端口和 Docker network

不使用当前仓库的 `deploy/docker-compose.candidate.yml` 直接 `up`，因为当前 18084 数据挂载仍来自旧 candidate worktree；直接 compose 可能挂到错误目录。

## 当前基线

2026-07-08 09:09 左右只读核对：

- 本地分支：`main`
- 本地 HEAD：`6f00a311a`
- 本地状态：`main...personal/main [ahead 13]`，且存在未提交 `AGENTS.md` 与若干未跟踪上下文文档。发布前必须确认这些不是未完成源码改动。
- 当前公网应用容器：
  - 容器：`sub2api-candidate`
  - 镜像：`sub2api-candidate:20260707-084458-74e5a4bb0-subscription-window-refresh`
  - 状态：`Up ... (healthy)`
  - 端口：`127.0.0.1:18084->8080/tcp`
  - 网络：`sub2api-candidate-network`
  - `/app/data` 挂载：`/Users/wujianxiang/CodeSpace/sub2api/.worktrees/codex-sub2api-candidate-rehearsal-20260626/deploy/candidate/data`
- 当前公网 DB：
  - `schema_migrations`：195
  - 最新 migration：`159_auto_api_key_effective_group.sql`
  - `160_rmb_balance_payment_affiliate_defaults.sql` 尚未应用
  - `payment_orders.payment_type = 'balance'` 订单数为 0
- 当前 health：
  - `http://127.0.0.1:18084/health`：200
  - `http://127.0.0.1:8080/health`：200
  - `https://api.aaccx.pw/health`：200
  - `https://aaccx.pw/dashboard`：200
  - `https://aaccx.pw/purchase`：200

## 本次发布包含的关键变化

- 人民币余额充值：支付宝 only，1:1 人民币入账，`amount=pay_amount`，`fee_rate=0`。
- 用户购买页只展示支付宝和余额；余额不足进入充值确认页，不做混合支付。
- 新增余额支付商品接口 `POST /api/v1/payment/orders/balance-pay`，支持套餐和流量包。
- 余额支付创建 `payment_type=balance` 完成态订单，事务内条件扣减 `users.balance >= pay_amount`。
- 余额支付不产生邀请返利。
- 支付宝完成的余额充值、套餐、流量包按 `amount` 产生邀请返利。
- 新 migration `160_rmb_balance_payment_affiliate_defaults.sql` 会覆盖运行态返利 settings：
  - `affiliate_rebate_rate=8`
  - `affiliate_rebate_freeze_hours=24`
  - `affiliate_rebate_duration_days=365`
  - `affiliate_rebate_per_invitee_cap=100`

## 发布原则

- 不执行 `git pull`，不推送远端。
- 不重建 Postgres / Redis，不覆盖数据目录。
- 不修改 nginx / Cloudflare Tunnel。
- 不提交或记录完整 API Key、JWT secret、HMAC secret、SMTP 密码。
- 所有备份放到 `deploy/backups/`，权限设为 `600`。
- 正式替换应用容器前必须完成 Postgres custom dump 和 Redis RDB 备份。
- 如果发布后已有真实订单、用量或余额扣减，数据层回滚会丢失这些运行态写入，必须先二次备份故障态数据。

## 方案取舍

推荐方案：构建本地 `main` 镜像，备份公网数据层，然后用 `deploy/promote-sub2api-candidate.sh` 替换 18084 应用容器。

原因：

- 脚本会从当前 `sub2api-candidate` 读取 env、Docker network 和 `/app/data` mount，避免手写旧 worktree 路径出错。
- 只替换应用容器，数据库和 Redis 容器持续运行。
- 旧应用容器会保留为 `sub2api-candidate-before-promote-${TS}`，便于快速应用层回滚。

不采用：

- 不用 `docker compose up` 重建 candidate 栈：当前挂载路径不在当前仓库。
- 不把 18080 或其他环境 DB 覆盖到 18084：18084 的 `sub2api-candidate-postgres` 是公网事实源。
- 不直接运行 `deploy/redeploy-sub2api-image.sh` 默认配置：默认偏向 legacy/local compose，不是当前 18084 标准目标。

## Task 1：发布前版本确认

- [ ] **Step 1：确认本地分支、提交和脏区**

```bash
cd /Users/wujianxiang/CodeSpace/sub2api

git branch --show-current
git rev-parse --short HEAD
git log --oneline -5
git status --short --untracked-files=all
git diff --stat
git diff --name-only -- backend frontend Dockerfile deploy docs/legal
```

预期：

- 当前分支为 `main`。
- HEAD 为本次要发布的提交。
- `backend`、`frontend`、`Dockerfile`、`deploy`、`docs/legal` 没有未确认的源码变更。
- 如果 `git diff --name-only -- backend frontend Dockerfile deploy docs/legal` 有输出，先确认这些变更就是要发布的内容。

- [ ] **Step 2：确认公网当前仍指向 18084**

```bash
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
docker inspect sub2api-candidate --format 'image={{.Config.Image}} image_id={{.Image}} restart={{json .HostConfig.RestartPolicy}}'
docker inspect sub2api-candidate --format '{{range $name, $_ := .NetworkSettings.Networks}}{{println $name}}{{end}}'
docker inspect sub2api-candidate --format '{{range .Mounts}}{{println .Source "->" .Destination}}{{end}}'

curl -fsS -o /dev/null -w '18084=%{http_code}\n' http://127.0.0.1:18084/health
curl -fsS -o /dev/null -w '8080=%{http_code}\n' http://127.0.0.1:8080/health
curl -fsS -o /dev/null -w 'api=%{http_code}\n' https://api.aaccx.pw/health
curl -fsS -o /dev/null -w 'dashboard=%{http_code}\n' https://aaccx.pw/dashboard
curl -fsS -o /dev/null -w 'purchase=%{http_code}\n' https://aaccx.pw/purchase
```

预期：

- `sub2api-candidate`、`sub2api-candidate-postgres`、`sub2api-candidate-redis` 均 healthy。
- `sub2api-candidate` 端口为 `127.0.0.1:18084->8080/tcp`。
- 18084、8080、公网 health 和页面路由均为 200。

## Task 2：发布前自动化验证

- [ ] **Step 1：运行后端验证**

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/backend

go test -count=1 -tags=unit ./internal/payment ./internal/service ./internal/handler ./internal/server
go test -count=1 ./migrations
go test -count=1 ./cmd/server
```

预期：全部通过。

- [ ] **Step 2：运行前端验证**

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/frontend

pnpm typecheck
pnpm vitest run \
  src/api/__tests__/payment.spec.ts \
  src/components/payment/__tests__/paymentFlow.spec.ts \
  src/views/user/__tests__/PaymentView.spec.ts \
  src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts \
  src/components/user/profile/__tests__/ProfileInfoCard.spec.ts \
  src/views/admin/__tests__/RedeemView.batchUpdate.spec.ts
pnpm build
```

预期：

- `typecheck` 通过。
- 目标 vitest 通过。
- `pnpm build` 通过；Vite 既有 chunk/dynamic import 警告不阻断发布。

- [ ] **Step 3：运行 diff 空白检查**

```bash
cd /Users/wujianxiang/CodeSpace/sub2api
git diff --check
```

预期：无输出。

## Task 3：构建本地 main 镜像

- [ ] **Step 1：设置发布变量**

```bash
cd /Users/wujianxiang/CodeSpace/sub2api

export TS="$(date +%Y%m%d-%H%M%S)"
export SHA="$(git rev-parse --short HEAD)"
export COMMIT="$(git rev-parse HEAD)"
export BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
export NEW_IMAGE="sub2api-candidate:${TS}-${SHA}-rmb-balance-affiliate"

printf 'TS=%s\nSHA=%s\nNEW_IMAGE=%s\n' "$TS" "$SHA" "$NEW_IMAGE"
```

预期：`NEW_IMAGE` 形如 `sub2api-candidate:20260708-HHMMSS-6f00a311a-rmb-balance-affiliate`。

- [ ] **Step 2：构建镜像**

```bash
docker build \
  --build-arg GOPROXY=https://goproxy.cn,direct \
  --build-arg GOSUMDB=sum.golang.google.cn \
  --build-arg COMMIT="${COMMIT}" \
  --build-arg DATE="${BUILD_DATE}" \
  -t "${NEW_IMAGE}" \
  -f Dockerfile \
  .

docker image inspect "${NEW_IMAGE}" --format 'image_id={{.Id}} tags={{json .RepoTags}}'
```

预期：

- Docker build 成功。
- 新镜像 tag 为 `sub2api-candidate:*rmb-balance-affiliate`。

## Task 4：备份公网数据层

- [ ] **Step 1：创建备份目录**

```bash
cd /Users/wujianxiang/CodeSpace/sub2api

export TS="${TS:-$(date +%Y%m%d-%H%M%S)}"
mkdir -p deploy/backups
chmod 700 deploy/backups
```

- [ ] **Step 2：备份 Postgres**

```bash
export PG_BACKUP="deploy/backups/${TS}-sub2api-candidate-postgres-before-rmb-balance-affiliate.dump"

docker exec sub2api-candidate-postgres sh -lc \
  'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --no-owner --no-privileges --file=/tmp/sub2api-before-rmb-balance-affiliate.dump'

docker cp sub2api-candidate-postgres:/tmp/sub2api-before-rmb-balance-affiliate.dump "${PG_BACKUP}"
docker exec sub2api-candidate-postgres rm -f /tmp/sub2api-before-rmb-balance-affiliate.dump

chmod 600 "${PG_BACKUP}"
pg_restore -l "${PG_BACKUP}" >/dev/null
ls -lh "${PG_BACKUP}"
```

如果宿主机没有 `pg_restore`，用容器验证：

```bash
docker run --rm \
  -v "$PWD/deploy/backups:/backups:ro" \
  postgres:18-alpine \
  pg_restore -l "/backups/$(basename "${PG_BACKUP}")" >/dev/null
```

预期：

- dump 可被 `pg_restore -l` 读取。
- 文件权限为 `600`。

- [ ] **Step 3：备份 Redis**

```bash
export REDIS_BACKUP="deploy/backups/${TS}-sub2api-candidate-redis-before-rmb-balance-affiliate.rdb"

docker exec sub2api-candidate-redis sh -lc 'redis-cli SAVE'
docker cp sub2api-candidate-redis:/data/dump.rdb "${REDIS_BACKUP}"

chmod 600 "${REDIS_BACKUP}"
ls -lh "${REDIS_BACKUP}"
```

预期：

- RDB 文件存在。
- 文件权限为 `600`。

## Task 5：记录发布前数据库快照

- [ ] **Step 1：记录核心计数和 migration 状态**

```bash
docker exec -i sub2api-candidate-postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -At' <<'SQL'
SELECT
  (SELECT COUNT(*) FROM users WHERE deleted_at IS NULL) AS active_users,
  (SELECT COUNT(*) FROM api_keys WHERE deleted_at IS NULL) AS active_keys,
  (SELECT COUNT(*) FROM user_subscriptions WHERE deleted_at IS NULL AND status = 'active' AND expires_at > NOW()) AS active_subscriptions,
  (SELECT COUNT(*) FROM schema_migrations) AS migrations,
  (SELECT MAX(filename) FROM schema_migrations) AS latest_migration;

SELECT COUNT(*)
FROM schema_migrations
WHERE filename = '160_rmb_balance_payment_affiliate_defaults.sql';
SQL
```

预期：

- 当前 18084 通常为 `195` migrations，最新 `159_auto_api_key_effective_group.sql`。
- `160_rmb_balance_payment_affiliate_defaults.sql` 计数为 `0`。

- [ ] **Step 2：记录返利 settings 发布前值**

```bash
docker exec -i sub2api-candidate-postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -At' <<'SQL'
SELECT key, value
FROM settings
WHERE key IN (
  'affiliate_enabled',
  'affiliate_rebate_rate',
  'affiliate_rebate_freeze_hours',
  'affiliate_rebate_duration_days',
  'affiliate_rebate_per_invitee_cap'
)
ORDER BY key;
SQL
```

预期：

- `affiliate_enabled` 为 `true`。
- 返利四个默认值可能不存在或不是目标值；发布后会被 migration 160 写成 `8/24/365/100`。

- [ ] **Step 3：记录余额支付订单基线**

```bash
docker exec -i sub2api-candidate-postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -At' <<'SQL'
SELECT COUNT(*)
FROM payment_orders
WHERE payment_type = 'balance';
SQL
```

预期：

- 当前为 `0`。发布后如用户使用余额购买套餐/流量包，会新增 `payment_type=balance` 的完成态订单。

## Task 6：发布应用容器

- [ ] **Step 1：先 dry-run promote 脚本**

```bash
cd /Users/wujianxiang/CodeSpace/sub2api

deploy/promote-sub2api-candidate.sh \
  --candidate-image "${NEW_IMAGE}" \
  --dry-run \
  --yes
```

预期：

- 目标为 `public_candidate_18084`。
- 应用容器为 `sub2api-candidate`。
- Postgres 为 `sub2api-candidate-postgres`。
- Redis 为 `sub2api-candidate-redis`。
- 宿主端口为 `18084`。
- Docker 网络为 `sub2api-candidate-network`。

- [ ] **Step 2：正式替换应用容器**

```bash
export OLD_CONTAINER="sub2api-candidate-before-promote-${TS}"

SUB2API_PROMOTE_TS="${TS}" \
deploy/promote-sub2api-candidate.sh \
  --candidate-image "${NEW_IMAGE}" \
  --yes
```

预期：

- 仅 `sub2api-candidate` 被停止、重命名、重新创建。
- 旧应用容器保留为 `${OLD_CONTAINER}`。
- `sub2api-candidate-postgres` 和 `sub2api-candidate-redis` 未停止、未重建。
- 脚本健康检查通过。

## Task 7：发布后健康检查

- [ ] **Step 1：检查容器和日志**

```bash
for i in $(seq 1 40); do
  status="$(docker inspect sub2api-candidate --format '{{.State.Health.Status}}' 2>/dev/null || true)"
  echo "health=${status}"
  if [ "${status}" = "healthy" ]; then
    break
  fi
  sleep 3
done

docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
docker inspect sub2api-candidate --format 'new_image={{.Config.Image}} image_id={{.Image}}'
docker logs --tail 300 sub2api-candidate
docker logs --tail 300 sub2api-candidate | rg -i 'checksum mismatch|migration .*failed|panic|failed to initialize application|listen tcp' || true
```

预期：

- `sub2api-candidate` 为 healthy。
- 新镜像为 `${NEW_IMAGE}`。
- 日志中没有 migration failed、checksum mismatch、panic、端口冲突。

- [ ] **Step 2：检查本地和公网路由**

```bash
curl -fsS -o /dev/null -w '18084=%{http_code}\n' http://127.0.0.1:18084/health
curl -fsS -o /dev/null -w '8080=%{http_code}\n' http://127.0.0.1:8080/health
curl -fsS -o /dev/null -w 'api=%{http_code}\n' https://api.aaccx.pw/health
curl -fsS -o /dev/null -w 'dashboard=%{http_code}\n' https://aaccx.pw/dashboard
curl -fsS -o /dev/null -w 'purchase=%{http_code}\n' https://aaccx.pw/purchase
```

预期：全部返回 200。

## Task 8：发布后数据库验证

- [ ] **Step 1：确认 migration 160 已应用**

```bash
docker exec -i sub2api-candidate-postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -At' <<'SQL'
SELECT COUNT(*), MAX(filename)
FROM schema_migrations;

SELECT COUNT(*)
FROM schema_migrations
WHERE filename = '160_rmb_balance_payment_affiliate_defaults.sql';
SQL
```

预期：

- migration 数从 `195` 增加到至少 `196`。
- `160_rmb_balance_payment_affiliate_defaults.sql` 计数为 `1`。

- [ ] **Step 2：确认返利默认 settings**

```bash
docker exec -i sub2api-candidate-postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -At' <<'SQL'
SELECT key, value
FROM settings
WHERE key IN (
  'affiliate_rebate_rate',
  'affiliate_rebate_freeze_hours',
  'affiliate_rebate_duration_days',
  'affiliate_rebate_per_invitee_cap'
)
ORDER BY key;
SQL
```

预期：

```text
affiliate_rebate_duration_days|365
affiliate_rebate_freeze_hours|24
affiliate_rebate_per_invitee_cap|100
affiliate_rebate_rate|8
```

- [ ] **Step 3：确认余额支付订单仍未异常增长**

```bash
docker exec -i sub2api-candidate-postgres sh -lc \
  'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -At' <<'SQL'
SELECT payment_type, order_type, status, COUNT(*)
FROM payment_orders
WHERE created_at > NOW() - INTERVAL '30 minutes'
GROUP BY payment_type, order_type, status
ORDER BY payment_type, order_type, status;
SQL
```

预期：

- 发布动作本身不应创建订单。
- 如出现新订单，必须能对应真实用户操作。

## Task 9：业务 smoke test

- [ ] **Step 1：未鉴权路由 smoke test**

```bash
curl -fsS -o /dev/null -w 'models_unauth=%{http_code}\n' https://api.aaccx.pw/v1/models
curl -fsS -o /dev/null -w 'responses_unauth=%{http_code}\n' https://api.aaccx.pw/v1/responses
curl -fsS -o /tmp/sub2api-purchase.html https://aaccx.pw/purchase
rg -o 'assets/[^"]+\.(js|css)' /tmp/sub2api-purchase.html | sort -u
rm -f /tmp/sub2api-purchase.html
```

预期：

- 未鉴权 API 返回 401、400 或 405 等受控错误，不返回 502/503。
- `/purchase` 能返回 SPA HTML 和静态资源引用。

- [ ] **Step 2：真实 API smoke test**

使用操作者手动输入的测试 Key，不在命令历史、文档或总结里记录完整 Key。

```bash
read -rsp 'TEST_API_KEY=' TEST_API_KEY; echo

curl -sS https://api.aaccx.pw/v1/responses \
  -H "Authorization: Bearer ${TEST_API_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-5.5","input":"reply with ok"}' \
  -o /tmp/sub2api-rmb-balance-smoke.json \
  -w 'api_http=%{http_code}\n'

python3 - <<'PY'
import json
p = "/tmp/sub2api-rmb-balance-smoke.json"
with open(p, "r", encoding="utf-8") as f:
    data = json.load(f)
print(json.dumps({
    "id": data.get("id"),
    "status": data.get("status"),
    "error": data.get("error"),
}, ensure_ascii=False, indent=2))
PY

unset TEST_API_KEY
rm -f /tmp/sub2api-rmb-balance-smoke.json
```

预期：

- HTTP 返回 200。
- 返回体有响应 id/status，无上游账号不可用、计费异常或 5xx。

- [ ] **Step 3：支付页面人工验收**

使用浏览器登录测试账号，检查但不付款：

- `/purchase` 第一张卡是余额充值。
- 余额充值金额默认 1 元，限制 1-100 整数。
- 余额充值只显示支付宝。
- 套餐/流量包确认页只显示支付宝和余额。
- 余额不足进入充值确认页。
- 订单列表和余额文案为人民币口径。

## Task 10：失败处理与回滚

### 10.1 应用层回滚

适用场景：

- 新容器未 healthy。
- 新镜像日志出现启动失败、panic、checksum mismatch、migration failed。
- 发布后页面/API 立即 502/503。
- 尚未发生大量真实余额支付订单或其他关键写入。

命令：

```bash
export OLD_CONTAINER="${OLD_CONTAINER:?OLD_CONTAINER is required}"

docker logs --tail 300 sub2api-candidate || true
docker rm -f sub2api-candidate || true
docker rename "${OLD_CONTAINER}" sub2api-candidate
docker start sub2api-candidate

for i in $(seq 1 40); do
  status="$(docker inspect sub2api-candidate --format '{{.State.Health.Status}}' 2>/dev/null || true)"
  echo "health=${status}"
  if [ "${status}" = "healthy" ]; then
    break
  fi
  sleep 3
done

curl -fsS -o /dev/null -w '18084=%{http_code}\n' http://127.0.0.1:18084/health
curl -fsS -o /dev/null -w '8080=%{http_code}\n' http://127.0.0.1:8080/health
curl -fsS -o /dev/null -w 'api=%{http_code}\n' https://api.aaccx.pw/health
```

说明：

- migration 160 只改 settings，不改 schema；大多数启动失败可先应用层回滚。
- 如果 migration 160 已应用，应用层回滚不会恢复返利 settings。
- 如果已经有 `payment_type=balance` 商品订单，旧镜像可能不能完整理解这些新订单语义，继续观察后台订单页和用户余额。

### 10.2 数据层回滚前二次备份故障态

只有在新代码造成错误 DB 写入，且应用层回滚不足以恢复时，才做数据层回滚。执行前必须先备份当前故障态数据。

```bash
cd /Users/wujianxiang/CodeSpace/sub2api

export ROLLBACK_TS="$(date +%Y%m%d-%H%M%S)"
export FAILURE_PG_BACKUP="deploy/backups/${ROLLBACK_TS}-sub2api-candidate-postgres-before-data-rollback.dump"
export FAILURE_REDIS_BACKUP="deploy/backups/${ROLLBACK_TS}-sub2api-candidate-redis-before-data-rollback.rdb"

docker exec sub2api-candidate-postgres sh -lc \
  'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --no-owner --no-privileges --file=/tmp/sub2api-before-data-rollback.dump'
docker cp sub2api-candidate-postgres:/tmp/sub2api-before-data-rollback.dump "${FAILURE_PG_BACKUP}"
docker exec sub2api-candidate-postgres rm -f /tmp/sub2api-before-data-rollback.dump
chmod 600 "${FAILURE_PG_BACKUP}"
pg_restore -l "${FAILURE_PG_BACKUP}" >/dev/null

docker exec sub2api-candidate-redis sh -lc 'redis-cli SAVE'
docker cp sub2api-candidate-redis:/data/dump.rdb "${FAILURE_REDIS_BACKUP}"
chmod 600 "${FAILURE_REDIS_BACKUP}"
ls -lh "${FAILURE_PG_BACKUP}" "${FAILURE_REDIS_BACKUP}"
```

### 10.3 恢复 Postgres 到发布前备份

警告：这会丢失发布后产生的真实订单、余额扣减、用量、Key、注册等写入。

```bash
export PG_BACKUP="${PG_BACKUP:?PG_BACKUP is required}"
export OLD_CONTAINER="${OLD_CONTAINER:?OLD_CONTAINER is required}"

docker stop sub2api-candidate || true

docker cp "${PG_BACKUP}" sub2api-candidate-postgres:/tmp/sub2api-rollback.dump
docker exec sub2api-candidate-postgres sh -lc \
  'dropdb -U "$POSTGRES_USER" "$POSTGRES_DB" --force && createdb -U "$POSTGRES_USER" "$POSTGRES_DB"'
docker exec sub2api-candidate-postgres sh -lc \
  'pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --no-owner --no-privileges /tmp/sub2api-rollback.dump'
docker exec sub2api-candidate-postgres rm -f /tmp/sub2api-rollback.dump

docker rm -f sub2api-candidate || true
docker rename "${OLD_CONTAINER}" sub2api-candidate || true
docker start sub2api-candidate
```

### 10.4 可选恢复 Redis 到发布前备份

只有在缓存/session 状态也必须回到发布前时执行。

```bash
export REDIS_BACKUP="${REDIS_BACKUP:?REDIS_BACKUP is required}"

docker stop sub2api-candidate || true
docker stop sub2api-candidate-redis
docker cp "${REDIS_BACKUP}" sub2api-candidate-redis:/data/dump.rdb
docker start sub2api-candidate-redis
docker start sub2api-candidate
```

数据层回滚后重新执行 Task 7 和 Task 8 的 health/DB 验证。

## Task 11：发布成功后的记录

- [ ] **Step 1：清理和记录**

```bash
docker inspect sub2api-candidate --format 'new_image={{.Config.Image}} image_id={{.Image}}'
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
ls -lh "${PG_BACKUP}" "${REDIS_BACKUP}"
```

旧容器 `${OLD_CONTAINER}` 建议保留到完成业务验收后再删除：

```bash
docker rm "${OLD_CONTAINER}"
```

- [ ] **Step 2：新建结果文档**

文件名：

```text
docs/ai/context/YYYYMMDD-HHMMSS-public-18084-rmb-balance-affiliate-redeploy-result_CN.md
```

结果文档必须记录：

- 发布时间。
- 本地 HEAD 和新镜像 tag/image id。
- Postgres/Redis 备份文件路径、大小、权限。
- 发布前后 migration 数和最新 migration。
- migration 160 是否应用，返利 settings 是否为 `8/24/365/100`。
- health、页面、API smoke test 结果。
- 是否执行真实余额支付人工验收。
- 是否执行回滚。
- 明确说明未修改 nginx，未重建 Postgres/Redis。

- [ ] **Step 3：追加 AGENTS.md 长期记忆**

在 `AGENTS.md` 顶部运行态/最高优先级区域追加一条简短结果索引，不能记录完整密钥、token 或敏感运行态配置。

## 执行确认点

真正执行发布前，需要用户明确确认：

1. 允许使用当前本地 `main` 工作区内容构建公网镜像。
2. 允许备份 `sub2api-candidate-postgres` 和 `sub2api-candidate-redis`。
3. 允许短暂停止并替换 `sub2api-candidate` 应用容器。
4. 接受新应用启动时在公网 DB 应用 `160_rmb_balance_payment_affiliate_defaults.sql`，将返利默认值写为 `8/24/365/100`。
