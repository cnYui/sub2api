# 18080 Main Preview Public Promotion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将当前已蓝绿验证的 `sub2api-main-preview` 18080 栈切换为公网 Sub2API 入口，并以 18084 当前公网数据库作为最终事实源完成数据同步和迁移。

**Architecture:** 采用“停写后最终克隆”方式：先备份两套数据库，停止 18084 应用阻断公网写入，再从 18084 DB 导出最终 dump 覆盖恢复到 18080 DB，启动 18080 新版应用自动执行缺失迁移，最后将 nginx upstream 从 18084 切到 18080。18084 DB/Redis 保留为短期回滚资产，18084 应用在切换窗口内保持停止，除非需要回滚。

**Tech Stack:** Docker Desktop CLI、PostgreSQL `pg_dump/pg_restore`、Redis `redis-cli`、Homebrew nginx、Sub2API Go 后端和内嵌前端、CLIProxyAPI 本地上游。

---

## 执行前固定变量

执行计划时先在 shell 中设置这些变量。不要把变量输出到日志或文档。

```bash
cd /Users/wujianxiang/CodeSpace/sub2api
DOCKER=/Applications/Docker.app/Contents/Resources/bin/docker
NGINX=/opt/homebrew/bin/nginx
TS="$(date +%Y%m%d-%H%M%S)"
BACKUP_DIR=/Users/wujianxiang/CodeSpace/sub2api/deploy/backups
NGINX_BACKUP_DIR=/opt/homebrew/etc/nginx/backups
mkdir -p "${BACKUP_DIR}" "${NGINX_BACKUP_DIR}"
```

期望：

- 当前目录是 `/Users/wujianxiang/CodeSpace/sub2api`。
- `echo "${TS}"` 是形如 `20260629-101521` 的时间戳。
- 不打印任何 API Key、SMTP 密码、内部 token 或 dump 内容。

## 文件与运行态责任

- Create: `docs/ai/context/YYYYMMDD-HHMMSS-promote-18080-main-preview-to-public-result_CN.md`，记录执行结果、备份文件名、迁移数、验证结果。
- Modify: `/opt/homebrew/etc/nginx/servers/cliproxy.conf`，将 `api.aaccx.pw` 的 Sub2API 代理从 `127.0.0.1:18084` 改到 `127.0.0.1:18080`。
- Modify: `/opt/homebrew/etc/nginx/servers/aaccx-root.conf`，将 `aaccx.pw` 上 `/v1/*`、`/api/*`、裸 OpenAI 路由、Sub2API 前端路由和资产代理从 `127.0.0.1:18084` 改到 `127.0.0.1:18080`。
- Runtime write: `sub2api-main-preview-postgres`，被 18084 最终 dump 覆盖恢复，并由 18080 应用执行迁移。
- Runtime write: `sub2api-main-preview-redis`，恢复 18080 DB 后执行 `FLUSHALL` 清缓存和会话态。
- Runtime stop: `sub2api-candidate`，停写窗口内停止；`sub2api-candidate-postgres` 和 `sub2api-candidate-redis` 不停止。
- Backup only: `deploy/backups/*.dump`，敏感运行态数据库备份，不提交到 git。

---

### Task 1: 切换前只读预检

**Files:**

- Read: `/opt/homebrew/etc/nginx/servers/cliproxy.conf`
- Read: `/opt/homebrew/etc/nginx/servers/aaccx-root.conf`
- Read: `docs/ai/context/20260629-100845-promote-18080-main-preview-to-public-design_CN.md`

- [ ] **Step 1: 确认 Docker CLI、nginx 和容器状态**

```bash
"${DOCKER}" version --format '{{.Server.Version}}'
"${NGINX}" -t
"${DOCKER}" ps --format '{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
```

期望：

- Docker version 命令成功。
- nginx 输出包含 `syntax is ok` 和 `test is successful`。
- 运行容器包含 `sub2api-candidate`、`sub2api-candidate-postgres`、`sub2api-candidate-redis`、`sub2api-main-preview`、`sub2api-main-preview-postgres`、`sub2api-main-preview-redis`。
- `sub2api-candidate` 端口为 `127.0.0.1:18084->8080/tcp`。
- `sub2api-main-preview` 端口为 `127.0.0.1:18080->8080/tcp`。

- [ ] **Step 2: 确认 nginx 当前仍指向 18084**

```bash
rg -o '127\.0\.0\.1:18084' /opt/homebrew/etc/nginx/servers/cliproxy.conf | wc -l | tr -d ' '
rg -o '127\.0\.0\.1:18084' /opt/homebrew/etc/nginx/servers/aaccx-root.conf | wc -l | tr -d ' '
rg -n '127\.0\.0\.1:18080|127\.0\.0\.1:18084' /opt/homebrew/etc/nginx/servers/cliproxy.conf /opt/homebrew/etc/nginx/servers/aaccx-root.conf
```

期望：

- `cliproxy.conf` 中 `127.0.0.1:18084` 数量为 `6`。
- `aaccx-root.conf` 中 `127.0.0.1:18084` 数量为 `9`。
- 两个文件中没有 Sub2API 代理遗漏到其他端口；`127.0.0.1:4173` 是 yui.web 静态站，不修改。

- [ ] **Step 3: 记录两套数据库迁移和核心表数量**

```bash
"${DOCKER}" exec sub2api-candidate-postgres psql -U sub2api -d sub2api -Atc "
select 'candidate_migrations', count(*) from schema_migrations
union all select 'candidate_users', count(*) from users
union all select 'candidate_api_keys', count(*) from api_keys
union all select 'candidate_user_subscriptions', count(*) from user_subscriptions
union all select 'candidate_payment_orders', count(*) from payment_orders
union all select 'candidate_user_traffic_credits', count(*) from user_traffic_credits
union all select 'candidate_traffic_credit_ledger', count(*) from traffic_credit_ledger;
select filename from schema_migrations order by filename desc limit 8;
"

"${DOCKER}" exec sub2api-main-preview-postgres psql -U sub2api -d sub2api -Atc "
select 'preview_migrations', count(*) from schema_migrations
union all select 'preview_users', count(*) from users
union all select 'preview_api_keys', count(*) from api_keys
union all select 'preview_user_subscriptions', count(*) from user_subscriptions
union all select 'preview_payment_orders', count(*) from payment_orders
union all select 'preview_user_traffic_credits', count(*) from user_traffic_credits
union all select 'preview_traffic_credit_ledger', count(*) from traffic_credit_ledger;
select filename from schema_migrations order by filename desc limit 8;
"
```

期望：

- 18084 候选库当前为 `191` migrations，最新迁移不超过 `155_seed_codex_subscription_plans_baseline.sql`。
- 18080 预览库当前为 `194` migrations，最新包含 `158_enable_affiliate_default.sql`。
- 若数量与设计文档差异明显，先停在本步骤，记录差异并重新判断是否仍以 18084 为事实源。

- [ ] **Step 4: 记录切换前 health**

```bash
curl -fsS http://127.0.0.1:18084/health
curl -fsS http://127.0.0.1:18080/health
curl -fsS http://127.0.0.1:8080/health
curl -fsS https://api.aaccx.pw/health
```

期望：

- 四个请求都返回 200。
- body 包含 `{"status":"ok"}` 或等价健康状态。

---

### Task 2: 停写前备份两套数据库

**Files:**

- Create: `deploy/backups/${TS}-18084-candidate-before-promote.dump`
- Create: `deploy/backups/${TS}-18080-preview-before-overwrite.dump`

- [ ] **Step 1: 备份 18084 候选库**

```bash
"${DOCKER}" exec sub2api-candidate-postgres pg_dump -U sub2api -d sub2api -Fc -f "/tmp/${TS}-18084-candidate-before-promote.dump"
"${DOCKER}" cp "sub2api-candidate-postgres:/tmp/${TS}-18084-candidate-before-promote.dump" "${BACKUP_DIR}/${TS}-18084-candidate-before-promote.dump"
"${DOCKER}" exec sub2api-candidate-postgres rm -f "/tmp/${TS}-18084-candidate-before-promote.dump"
ls -lh "${BACKUP_DIR}/${TS}-18084-candidate-before-promote.dump"
```

期望：

- 备份文件存在。
- 文件大小接近近期 18084 dump，当前量级约 15MB。

- [ ] **Step 2: 备份 18080 预览库**

```bash
"${DOCKER}" exec sub2api-main-preview-postgres pg_dump -U sub2api -d sub2api -Fc -f "/tmp/${TS}-18080-preview-before-overwrite.dump"
"${DOCKER}" cp "sub2api-main-preview-postgres:/tmp/${TS}-18080-preview-before-overwrite.dump" "${BACKUP_DIR}/${TS}-18080-preview-before-overwrite.dump"
"${DOCKER}" exec sub2api-main-preview-postgres rm -f "/tmp/${TS}-18080-preview-before-overwrite.dump"
ls -lh "${BACKUP_DIR}/${TS}-18080-preview-before-overwrite.dump"
```

期望：

- 备份文件存在。
- 文件大小接近近期 18080 dump，当前量级约 15MB。

- [ ] **Step 3: 确认备份不会进入 git**

```bash
git status --short deploy/backups
git check-ignore -v deploy/backups/"${TS}-18084-candidate-before-promote.dump" deploy/backups/"${TS}-18080-preview-before-overwrite.dump"
```

期望：

- `git status --short deploy/backups` 不显示新 dump，或只显示已知不应提交的 ignored/未跟踪备份。
- `git check-ignore` 显示这两个 dump 被忽略。
- 如果 dump 未被忽略，立即停止执行，不要继续切换；先修正 ignore 规则或把备份移到 git 外部安全路径。

---

### Task 3: 进入停写窗口并导出最终事实源 dump

**Files:**

- Create: `deploy/backups/${TS}-18084-candidate-final-after-stopwrite.dump`
- Runtime stop: `sub2api-candidate`
- Runtime stop: `sub2api-main-preview`

- [ ] **Step 1: 停止 18084 公网应用阻断写入**

```bash
"${DOCKER}" stop sub2api-candidate
"${DOCKER}" ps --format '{{.Names}}\t{{.Status}}\t{{.Ports}}' | rg 'sub2api-candidate|sub2api-main-preview'
```

期望：

- `sub2api-candidate` 不再出现在 `docker ps` 运行列表中。
- `sub2api-candidate-postgres` 和 `sub2api-candidate-redis` 仍在运行。
- 从此刻到 nginx 切到 18080 之前，公网 API 和控制台可能不可用，这是预期停写窗口。

- [ ] **Step 2: 停止 18080 应用，避免恢复目标库时仍有连接**

```bash
"${DOCKER}" stop sub2api-main-preview
"${DOCKER}" ps --format '{{.Names}}\t{{.Status}}\t{{.Ports}}' | rg 'sub2api-main-preview|sub2api-candidate' || true
```

期望：

- `sub2api-main-preview` 不再出现在运行列表中。
- 两套 Postgres 和 Redis 仍在运行。

- [ ] **Step 3: 停写后从 18084 导出最终 dump**

```bash
"${DOCKER}" exec sub2api-candidate-postgres pg_dump -U sub2api -d sub2api -Fc -f "/tmp/${TS}-18084-candidate-final-after-stopwrite.dump"
"${DOCKER}" cp "sub2api-candidate-postgres:/tmp/${TS}-18084-candidate-final-after-stopwrite.dump" "${BACKUP_DIR}/${TS}-18084-candidate-final-after-stopwrite.dump"
"${DOCKER}" exec sub2api-candidate-postgres rm -f "/tmp/${TS}-18084-candidate-final-after-stopwrite.dump"
ls -lh "${BACKUP_DIR}/${TS}-18084-candidate-final-after-stopwrite.dump"
```

期望：

- 最终 dump 文件存在。
- 文件大小与停写前 18084 备份接近。

- [ ] **Step 4: 确认最终 dump 可被 pg_restore 读取**

```bash
"${DOCKER}" run --rm -v "${BACKUP_DIR}:/backups:ro" postgres:18-alpine pg_restore -l "/backups/${TS}-18084-candidate-final-after-stopwrite.dump" | head -20
```

期望：

- 命令成功。
- 输出包含 PostgreSQL archive TOC 条目。

---

### Task 4: 用 18084 最终 dump 覆盖恢复 18080 DB

**Files:**

- Runtime write: `sub2api-main-preview-postgres`
- Runtime write: `sub2api-main-preview-redis`

- [ ] **Step 1: 删除并重建 18080 目标数据库**

```bash
"${DOCKER}" exec sub2api-main-preview-postgres psql -U sub2api -d postgres -v ON_ERROR_STOP=1 \
  -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = 'sub2api' AND pid <> pg_backend_pid();" \
  -c "DROP DATABASE IF EXISTS sub2api;" \
  -c "CREATE DATABASE sub2api OWNER sub2api;"
```

期望：

- `DROP DATABASE` 和 `CREATE DATABASE` 成功。
- 如果提示仍有连接，确认 `sub2api-main-preview` 已停止后重试本步骤一次。

- [ ] **Step 2: 恢复 18084 最终 dump 到 18080 目标库**

```bash
"${DOCKER}" cp "${BACKUP_DIR}/${TS}-18084-candidate-final-after-stopwrite.dump" "sub2api-main-preview-postgres:/tmp/${TS}-18084-candidate-final-after-stopwrite.dump"
"${DOCKER}" exec sub2api-main-preview-postgres pg_restore -U sub2api -d sub2api --no-owner --role=sub2api "/tmp/${TS}-18084-candidate-final-after-stopwrite.dump"
"${DOCKER}" exec sub2api-main-preview-postgres rm -f "/tmp/${TS}-18084-candidate-final-after-stopwrite.dump"
```

期望：

- `pg_restore` 退出码为 0。
- 不出现 `ERROR:`。

- [ ] **Step 3: 清空 18080 Redis**

```bash
"${DOCKER}" exec sub2api-main-preview-redis redis-cli FLUSHALL
```

期望：

- 输出 `OK`。

- [ ] **Step 4: 验证 18080 DB 已恢复到 18084 停写数据**

```bash
"${DOCKER}" exec sub2api-main-preview-postgres psql -U sub2api -d sub2api -Atc "
select 'preview_restored_migrations', count(*) from schema_migrations
union all select 'preview_restored_users', count(*) from users
union all select 'preview_restored_api_keys', count(*) from api_keys
union all select 'preview_restored_user_subscriptions', count(*) from user_subscriptions
union all select 'preview_restored_payment_orders', count(*) from payment_orders
union all select 'preview_restored_user_traffic_credits', count(*) from user_traffic_credits
union all select 'preview_restored_traffic_credit_ledger', count(*) from traffic_credit_ledger;
select filename from schema_migrations order by filename desc limit 8;
"
```

期望：

- 此时 18080 DB 仍应是 191 migrations，因为新版应用尚未启动迁移。
- users、api_keys、payment_orders、traffic_credit_ledger 等数量应与 Task 3 最终 dump 对应的 18084 候选库一致。

---

### Task 5: 启动 18080 新版应用并验证迁移

**Files:**

- Runtime start: `sub2api-main-preview`
- Runtime read: `sub2api-main-preview` logs

- [ ] **Step 1: 启动 18080 应用**

```bash
"${DOCKER}" start sub2api-main-preview
for i in {1..60}; do
  if curl -fsS http://127.0.0.1:18080/health; then
    break
  fi
  sleep 2
done
curl -fsS http://127.0.0.1:18080/health
```

期望：

- 最终 `curl` 返回 200。
- body 包含 `{"status":"ok"}` 或等价健康状态。

- [ ] **Step 2: 查看启动日志中是否有迁移错误**

```bash
"${DOCKER}" logs --tail 240 sub2api-main-preview | rg -i 'migration|migrate|error|panic|fatal|checksum|failed' || true
```

期望：

- 可以看到迁移相关日志。
- 不出现 `panic`、`fatal`、`checksum mismatch`、`migration failed`。
- 如果出现迁移失败，立即进入 Task 9 的恢复路径，不切 nginx。

- [ ] **Step 3: 验证 18080 DB 已迁移到新版 schema**

```bash
"${DOCKER}" exec sub2api-main-preview-postgres psql -U sub2api -d sub2api -Atc "
select 'preview_migrated_migrations', count(*) from schema_migrations;
select filename from schema_migrations order by filename desc limit 8;
"
```

期望：

- migration 数为 `194`。
- 最新迁移包含：
  - `158_enable_affiliate_default.sql`
  - `157_fix_codex_79_subscription_plan_base_price.sql`
  - `156_seed_codex_79_subscription_plan.sql`

- [ ] **Step 4: 验证关键表数量和套餐 seed**

```bash
"${DOCKER}" exec sub2api-main-preview-postgres psql -U sub2api -d sub2api -Atc "
select 'preview_migrated_users', count(*) from users
union all select 'preview_migrated_api_keys', count(*) from api_keys
union all select 'preview_migrated_user_subscriptions', count(*) from user_subscriptions
union all select 'preview_migrated_payment_orders', count(*) from payment_orders
union all select 'preview_migrated_user_traffic_credits', count(*) from user_traffic_credits
union all select 'preview_migrated_traffic_credit_ledger', count(*) from traffic_credit_ledger;
select id, name, price, duration_days, group_id, enabled from subscription_plans order by id;
select id, name, price, credit_amount, enabled from traffic_packs order by id;
"
```

期望：

- 业务表数量与恢复前基本一致；新增迁移只应影响 seed/config，不应批量删除用户、Key、订单、ledger。
- `79 元订阅池` 存在且基础价为 `79.00`，对应 `codex-pool-69-usd` 的 group。

---

### Task 6: 18080 本地业务验证

**Files:**

- Runtime read: `sub2api-main-preview-postgres`
- Runtime read: `sub2api-main-preview` logs

- [ ] **Step 1: 验证公开设置和购买页**

```bash
curl -fsS http://127.0.0.1:18080/api/v1/settings/public > /tmp/sub2api-18080-public-settings.json
python3 -m json.tool /tmp/sub2api-18080-public-settings.json >/tmp/sub2api-18080-public-settings.pretty.json
rg -n '"payment|email|register|affiliate|site|name|enabled' /tmp/sub2api-18080-public-settings.pretty.json || true
curl -fsS -o /tmp/sub2api-18080-purchase.html -w '%{http_code}\n' http://127.0.0.1:18080/purchase
```

期望：

- public settings 返回 200 且能被 `python3 -m json.tool` 解析。
- `/purchase` HTTP 状态为 `200`。
- 不在终端输出 SMTP 密码或支付商户密钥；public settings 不应包含这些敏感值。

- [ ] **Step 2: 从 18080 DB 安全取一个 active API Key 到 shell 变量，不打印**

```bash
API_KEY="$("${DOCKER}" exec sub2api-main-preview-postgres psql -U sub2api -d sub2api -Atc "select key from api_keys where deleted_at is null and status = 'active' order by id limit 1")"
test -n "${API_KEY}"
```

期望：

- `test -n "${API_KEY}"` 成功。
- 不执行 `echo "${API_KEY}"`。

- [ ] **Step 3: 验证 18080 OpenAI 兼容 API 真实链路**

```bash
HTTP_STATUS="$(curl -sS -o /tmp/sub2api-18080-responses.json -w '%{http_code}' \
  http://127.0.0.1:18080/v1/responses \
  -H "Authorization: Bearer ${API_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-5.5","input":"请只回复 pong"}')"
printf 'HTTP_STATUS=%s\n' "${HTTP_STATUS}"
test "${HTTP_STATUS}" = "200"
python3 - <<'PY'
import json
from pathlib import Path
data = json.loads(Path('/tmp/sub2api-18080-responses.json').read_text())
print('response_keys=' + ','.join(sorted(data.keys())[:8]))
print('id_present=' + str(bool(data.get('id'))))
PY
```

期望：

- `HTTP_STATUS=200`。
- Python 输出 `id_present=True`。
- 不打印 API Key。

- [ ] **Step 4: 验证认证失败路径仍由 Sub2API 返回**

```bash
curl -sS -o /tmp/sub2api-18080-auth-fail.json -w '%{http_code}\n' \
  http://127.0.0.1:18080/v1/models \
  -H 'Authorization: Bearer invalid-key-for-cutover-check'
cat /tmp/sub2api-18080-auth-fail.json
```

期望：

- HTTP 状态为 `401` 或 `403`。
- body 是 Sub2API 风格错误，不是 nginx 502、yui.web HTML 或 Cloudflare 错误页。

---

### Task 7: 切换 nginx 到 18080

**Files:**

- Modify: `/opt/homebrew/etc/nginx/servers/cliproxy.conf`
- Modify: `/opt/homebrew/etc/nginx/servers/aaccx-root.conf`
- Create: `/opt/homebrew/etc/nginx/backups/cliproxy.conf.${TS}.before-18080-promote.bak`
- Create: `/opt/homebrew/etc/nginx/backups/aaccx-root.conf.${TS}.before-18080-promote.bak`

- [ ] **Step 1: 备份 nginx 配置**

```bash
cp /opt/homebrew/etc/nginx/servers/cliproxy.conf "${NGINX_BACKUP_DIR}/cliproxy.conf.${TS}.before-18080-promote.bak"
cp /opt/homebrew/etc/nginx/servers/aaccx-root.conf "${NGINX_BACKUP_DIR}/aaccx-root.conf.${TS}.before-18080-promote.bak"
ls -lh "${NGINX_BACKUP_DIR}/cliproxy.conf.${TS}.before-18080-promote.bak" "${NGINX_BACKUP_DIR}/aaccx-root.conf.${TS}.before-18080-promote.bak"
```

期望：

- 两个备份文件存在。

- [ ] **Step 2: 将两个 nginx 文件中的 Sub2API upstream 从 18084 改为 18080**

```bash
perl -0pi -e 's/127\.0\.0\.1:18084/127.0.0.1:18080/g' \
  /opt/homebrew/etc/nginx/servers/cliproxy.conf \
  /opt/homebrew/etc/nginx/servers/aaccx-root.conf

rg -o '127\.0\.0\.1:18084' /opt/homebrew/etc/nginx/servers/cliproxy.conf /opt/homebrew/etc/nginx/servers/aaccx-root.conf | wc -l | tr -d ' '
rg -o '127\.0\.0\.1:18080' /opt/homebrew/etc/nginx/servers/cliproxy.conf | wc -l | tr -d ' '
rg -o '127\.0\.0\.1:18080' /opt/homebrew/etc/nginx/servers/aaccx-root.conf | wc -l | tr -d ' '
```

期望：

- 18084 总数量为 `0`。
- `cliproxy.conf` 中 18080 数量为 `6`。
- `aaccx-root.conf` 中 18080 数量为 `9`。
- `127.0.0.1:4173` 保持不变。

- [ ] **Step 3: 测试并 reload nginx**

```bash
"${NGINX}" -t
"${NGINX}" -s reload
```

期望：

- nginx 测试通过。
- reload 命令退出码为 0。

---

### Task 8: 公网验证

**Files:**

- Runtime read: nginx response
- Runtime read: `sub2api-main-preview` logs

- [ ] **Step 1: 验证本机 nginx 8080 已进入 18080**

```bash
curl -fsS http://127.0.0.1:8080/health
curl -fsS -o /tmp/sub2api-8080-purchase.html -w '%{http_code}\n' http://127.0.0.1:8080/purchase
curl -sS -o /tmp/sub2api-8080-auth-fail.json -w '%{http_code}\n' \
  http://127.0.0.1:8080/v1/models \
  -H 'Host: api.aaccx.pw' \
  -H 'Authorization: Bearer invalid-key-for-cutover-check'
```

期望：

- `/health` 返回 200。
- `/purchase` 返回 200。
- 无效 Key 返回 401 或 403，且 body 是 Sub2API 风格错误。

- [ ] **Step 2: 验证公网域名**

```bash
curl -fsS https://api.aaccx.pw/health
curl -fsS -o /tmp/sub2api-public-purchase.html -w '%{http_code}\n' https://aaccx.pw/purchase
curl -fsS -o /tmp/sub2api-public-dashboard.html -w '%{http_code}\n' https://aaccx.pw/dashboard
curl -fsS -o /tmp/sub2api-public-usage-guide.html -w '%{http_code}\n' https://aaccx.pw/usage-guide
```

期望：

- `api.aaccx.pw/health` 返回 200。
- `aaccx.pw/purchase`、`/dashboard`、`/usage-guide` 都返回 200。

- [ ] **Step 3: 验证公网裸 OpenAI 兼容路由仍进 Sub2API**

```bash
curl -sS -o /tmp/sub2api-public-bare-responses-auth-fail.json -w '%{http_code}\n' \
  https://aaccx.pw/responses \
  -H 'Authorization: Bearer invalid-key-for-cutover-check' \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-5.5","input":"ping"}'

curl -sS -o /tmp/sub2api-public-v1-models-auth-fail.json -w '%{http_code}\n' \
  https://api.aaccx.pw/v1/models \
  -H 'Authorization: Bearer invalid-key-for-cutover-check'
```

期望：

- 两个请求都返回 401 或 403。
- body 是 Sub2API 风格错误，不是 404、502、Cloudflare 页面或 yui.web HTML。

- [ ] **Step 4: 验证公网真实 LLM 请求**

```bash
HTTP_STATUS="$(curl -sS -o /tmp/sub2api-public-responses.json -w '%{http_code}' \
  https://api.aaccx.pw/v1/responses \
  -H "Authorization: Bearer ${API_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-5.5","input":"请只回复 pong"}')"
printf 'HTTP_STATUS=%s\n' "${HTTP_STATUS}"
test "${HTTP_STATUS}" = "200"
python3 - <<'PY'
import json
from pathlib import Path
data = json.loads(Path('/tmp/sub2api-public-responses.json').read_text())
print('response_keys=' + ','.join(sorted(data.keys())[:8]))
print('id_present=' + str(bool(data.get('id'))))
PY
```

期望：

- `HTTP_STATUS=200`。
- Python 输出 `id_present=True`。
- 不打印 API Key。

- [ ] **Step 5: 验证 18084 应用仍停止且 18084 DB/Redis 保留**

```bash
"${DOCKER}" ps --format '{{.Names}}\t{{.Status}}\t{{.Ports}}' | rg 'sub2api-candidate|sub2api-main-preview'
"${DOCKER}" ps -a --format '{{.Names}}\t{{.Status}}\t{{.Ports}}' | rg 'sub2api-candidate|sub2api-main-preview'
```

期望：

- `sub2api-main-preview` 运行且 healthy。
- `sub2api-main-preview-postgres` 和 `sub2api-main-preview-redis` 运行。
- `sub2api-candidate` 在 `docker ps -a` 中存在但处于 Exited。
- `sub2api-candidate-postgres` 和 `sub2api-candidate-redis` 运行，用作短期回滚资产。

---

### Task 9: 回滚路径

**Files:**

- Modify: `/opt/homebrew/etc/nginx/servers/cliproxy.conf`
- Modify: `/opt/homebrew/etc/nginx/servers/aaccx-root.conf`
- Runtime start: `sub2api-candidate`

- [ ] **Step 1: 如果 Task 5 或 Task 6 失败，尚未改 nginx 时恢复 18084 公网应用**

```bash
"${DOCKER}" start sub2api-candidate
for i in {1..60}; do
  if curl -fsS http://127.0.0.1:18084/health; then
    break
  fi
  sleep 2
done
curl -fsS http://127.0.0.1:18084/health
curl -fsS http://127.0.0.1:8080/health
```

期望：

- 18084 health 返回 200。
- 8080 仍指向 18084，health 返回 200。
- 停止执行后续切换步骤，记录失败原因。

- [ ] **Step 2: 如果 Task 7 或 Task 8 失败，使用 nginx 备份恢复到 18084**

```bash
cp "${NGINX_BACKUP_DIR}/cliproxy.conf.${TS}.before-18080-promote.bak" /opt/homebrew/etc/nginx/servers/cliproxy.conf
cp "${NGINX_BACKUP_DIR}/aaccx-root.conf.${TS}.before-18080-promote.bak" /opt/homebrew/etc/nginx/servers/aaccx-root.conf
"${DOCKER}" start sub2api-candidate
"${NGINX}" -t
"${NGINX}" -s reload
curl -fsS http://127.0.0.1:18084/health
curl -fsS http://127.0.0.1:8080/health
curl -fsS https://api.aaccx.pw/health
```

期望：

- nginx 重新指向 18084。
- 18084、8080、公网 health 都返回 200。

- [ ] **Step 3: 如果切到 18080 后已经产生新写入，再决定是否回滚数据库**

```bash
"${DOCKER}" exec sub2api-main-preview-postgres psql -U sub2api -d sub2api -Atc "
select 'preview_latest_payment_order', coalesce(max(created_at)::text, '') from payment_orders
union all select 'preview_latest_usage_log', coalesce(max(created_at)::text, '') from usage_logs
union all select 'preview_latest_traffic_ledger', coalesce(max(created_at)::text, '') from traffic_credit_ledger;
"
```

期望：

- 如果 18080 已有切换后的新订单、用量或 ledger，不能无脑切回 18084 旧库，否则会丢写入。
- 这种情况先停止公网写入，再从 18080 dump，同步回 18084 或单独导出差异后再回滚。
- 当前用户确认没有真实使用，正常烟测窗口内应没有用户新写入。

---

### Task 10: 记录结果上下文

**Files:**

- Create: `docs/ai/context/YYYYMMDD-HHMMSS-promote-18080-main-preview-to-public-result_CN.md`

- [ ] **Step 1: 新建结果文档**

```bash
RESULT_TS="$(date +%Y%m%d-%H%M%S)"
RESULT_DOC="docs/ai/context/${RESULT_TS}-promote-18080-main-preview-to-public-result_CN.md"
printf '%s\n' \
  '# 18080 main-preview 切换为公网入口结果' \
  '' \
  '## 执行结论' \
  '' \
  '- 公网 nginx upstream 已从 18084 切换到 18080。' \
  '- 18084 公网候选库最终 dump 已覆盖恢复到 18080 预览库。' \
  '- 18080 新版应用已完成迁移并通过本地与公网验证。' \
  '- 18084 DB/Redis 保留为短期回滚资产，18084 应用保持停止。' \
  '' \
  '## 备份文件' \
  '' \
  "- 18084 停写前备份：deploy/backups/${TS}-18084-candidate-before-promote.dump" \
  "- 18080 覆盖前备份：deploy/backups/${TS}-18080-preview-before-overwrite.dump" \
  "- 18084 停写后最终 dump：deploy/backups/${TS}-18084-candidate-final-after-stopwrite.dump" \
  "- nginx cliproxy 备份：${NGINX_BACKUP_DIR}/cliproxy.conf.${TS}.before-18080-promote.bak" \
  "- nginx aaccx-root 备份：${NGINX_BACKUP_DIR}/aaccx-root.conf.${TS}.before-18080-promote.bak" \
  '' \
  '## 验证结果' \
  '' \
  '- 18080 /health：200。' \
  '- 18080 migration 数：194。' \
  '- 8080 /health：200。' \
  '- api.aaccx.pw /health：200。' \
  '- aaccx.pw /purchase、/dashboard、/usage-guide：200。' \
  '- 公网真实 /v1/responses：200。' \
  '' \
  '## 运行态' \
  '' \
  '- 当前公网链路：Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> sub2api-main-preview 127.0.0.1:18080 -> CLIProxyAPI 127.0.0.1:8317。' \
  '- 当前事实源 DB：sub2api-main-preview-postgres。' \
  '- 当前 Redis：sub2api-main-preview-redis。' \
  '- 旧 18084 回滚资产：sub2api-candidate-postgres、sub2api-candidate-redis。' \
  '' \
  '## 安全说明' \
  '' \
  '- 未在文档记录完整 API Key、SMTP 密码、支付密钥、HMAC secret、内部 token 或 dump 内容。' \
  '- deploy/backups/ 下 dump 不提交到 git。' \
  > "${RESULT_DOC}"

sed -n '1,220p' "${RESULT_DOC}"
```

期望：

- 结果文档创建成功。
- 文档不包含明文 API Key、SMTP 密码、支付密钥、内部 token 或 dump 内容。

- [ ] **Step 2: 检查 git 状态和敏感信息**

```bash
git status --short
rg -n 'sk-|AIza|smtp_password|HMAC|secret|token|BEGIN .*PRIVATE|App Password' "${RESULT_DOC}" docs/ai/context/20260629-101521-promote-18080-main-preview-to-public-plan_CN.md || true
git status --short deploy/backups
```

期望：

- 新增计划和结果文档可见。
- 敏感扫描只允许命中“不要记录敏感信息”这类说明文字。
- `deploy/backups` 不出现需要提交的 dump。

---

## 执行完成后的分支处理

执行成功后先不要删除 18084 DB/Redis。建议观察一个短窗口后再单独确认是否停止旧数据层。

若用户要求提交上下文文档：

```bash
git add docs/ai/context/20260629-100845-promote-18080-main-preview-to-public-design_CN.md \
        docs/ai/context/20260629-101521-promote-18080-main-preview-to-public-plan_CN.md \
        docs/ai/context/"${RESULT_TS}-promote-18080-main-preview-to-public-result_CN.md"
git commit -m "docs: plan 18080 public promotion"
```

提交前必须重新运行：

```bash
git ls-files --others --exclude-standard docs/ai/context
git status --short deploy/backups
```

期望：

- 没有遗漏本轮必须提交的上下文文档。
- 没有 dump、密钥或运行态敏感文件进入提交范围。
