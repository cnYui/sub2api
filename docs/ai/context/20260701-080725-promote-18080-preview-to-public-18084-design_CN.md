# 2026-07-01 18080 preview 发布到公网 18084 设计文档

## 目标

- 将当前本地蓝绿测试入口 `127.0.0.1:18080` 已验证的 Sub2API 应用镜像发布到公网候选入口 `127.0.0.1:18084`。
- 保持公网入口拓扑不变：`nginx 8080 -> sub2api-candidate 18084`。
- 保持公网事实数据源不变：继续使用 `sub2api-candidate-postgres` 与 `sub2api-candidate-redis`。
- 处理新应用对应的数据库 migration 差异，避免新代码访问旧 DB 时出现 schema 或 seed 数据不一致。

## 当前事实

- 18080 preview 应用：
  - 容器：`sub2api-main-preview`
  - 镜像：`sub2api-main-preview:20260629-092226-ddd4fb9a9`
  - image id：`sha256:1cc424ad95735188f6c707c5c1b19af42b670a2c332c863ef8e8a9cc4ddb563b`
  - 端口：`127.0.0.1:18080->8080`
  - 状态：healthy
- 18084 公网候选应用：
  - 容器：`sub2api-candidate`
  - 镜像：`sub2api-candidate:20260627-221441-traffic-card-fix`
  - image id：`sha256:299560875687ba0fc7c9b9703a5bece639a832c35720fb6ce47f8dd222483e22`
  - 端口：`127.0.0.1:18084->8080`
  - 状态：healthy
- 18084 公网候选数据层：
  - PostgreSQL：`sub2api-candidate-postgres`
  - Redis：`sub2api-candidate-redis`
  - PostgreSQL 已备份：`deploy/backups/20260701-080310-sub2api-candidate-postgres-before-app-promote.dump`
  - Redis 已备份：`deploy/backups/20260701-080310-sub2api-candidate-redis-before-app-promote.rdb`

## 数据库差异

18084 当前 `schema_migrations`：

- 数量：`191`
- 最新迁移：`155_seed_codex_subscription_plans_baseline.sql`

18080 当前 `schema_migrations`：

- 数量：`194`
- 最新迁移：
  - `158_enable_affiliate_default.sql`
  - `157_fix_codex_79_subscription_plan_base_price.sql`
  - `156_seed_codex_79_subscription_plan.sql`

因此，本次发布需要让 18084 PostgreSQL 补齐以下三条 migration：

1. `156_seed_codex_79_subscription_plan.sql`
   - 新增或更新 `groups.name='codex-pool-69-usd'`
   - 新增或更新 `subscription_plans.name='79 元订阅池'`
   - 初始 price 写入 `79.79`
2. `157_fix_codex_79_subscription_plan_base_price.sql`
   - 将 `79 元订阅池` 的基础价从 `79.79` 修正为 `79.00`
   - 目的：避免运行态 1% 手续费二次加费
3. `158_enable_affiliate_default.sql`
   - 写入或更新 `settings.key='affiliate_enabled'` 为 `true`

本轮实际差异主要是 seed/update 数据，不是新增表字段。后续如果新镜像包含 DDL migration，仍按同一机制执行：新应用启动时由 migration runner 在 `sub2api-candidate-postgres` 上按文件名顺序应用未执行 migration。

## 设计结论

### 推荐方案：替换 18084 应用容器，保留 18084 DB/Redis

执行方式：

- 不修改 nginx。
- 不替换 PostgreSQL。
- 不替换 Redis。
- 用 18080 已验证镜像 `sub2api-main-preview:20260629-092226-ddd4fb9a9` 重建 `sub2api-candidate` 应用容器。
- 新 `sub2api-candidate` 启动时连接 `sub2api-candidate-postgres`，自动执行 `156/157/158` 三条 migration。

理由：

- 18084 PostgreSQL 是当前公网事实源，不能被 18080 preview DB 覆盖。
- 18080 preview DB 可能包含测试数据，且不包含备份之后公网新增写入。
- 当前 migration runner 会在应用初始化阶段执行 SQL migrations；migration 失败时应用初始化失败，不会进入正常服务状态。
- 保留 nginx 指向 18084，可以避免把公网流量直接切到 preview 数据库。

### 不推荐方案：修改 nginx 直接指向 18080

问题：

- 公网写入会进入 `sub2api-main-preview-postgres`，导致公网事实源从 18084 DB 分叉。
- 支付订单、用户、订阅、用量、验证码、Redis 队列都会落到 preview 数据层。
- 回滚时需要处理两个数据库之间的数据差异，风险高。

### 不推荐方案：用 18080 PostgreSQL/Redis 替换 18084 数据层

问题：

- 会把 preview 数据覆盖到公网。
- 可能丢失 18084 在克隆之后产生的用户、订单、用量和配置。
- Redis 替换会影响会话、验证码、队列和缓存状态。

## 发布前检查

发布前执行以下只读检查：

```bash
PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH" \
docker ps -a \
  --filter 'name=sub2api-main-preview' \
  --filter 'name=sub2api-candidate' \
  --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'

curl -fsS http://127.0.0.1:18080/health
curl -fsS http://127.0.0.1:18084/health
curl -fsS http://127.0.0.1:8080/health

PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH" \
docker exec sub2api-candidate-postgres \
  psql -U sub2api -d sub2api -Atc \
  "select count(*) from schema_migrations; select filename from schema_migrations order by filename desc limit 10;"
```

预期：

- `sub2api-main-preview` healthy，镜像为 `sub2api-main-preview:20260629-092226-ddd4fb9a9`
- `sub2api-candidate` healthy，替换前镜像为 `sub2api-candidate:20260627-221441-traffic-card-fix`
- `18080/health`、`18084/health`、`8080/health` 均返回 `{"status":"ok"}`
- 18084 DB 迁移数为 `191`，最新到 `155`

如果距离上次备份已产生新的公网写入，必须重新备份 `sub2api-candidate-postgres`。当前用户声明“现在没人使用”，但执行发布前仍应做一次 health 和容器状态确认。

## 发布步骤设计

1. 记录旧应用容器镜像与启动时间：

```bash
PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH" \
docker inspect sub2api-candidate \
  --format 'image={{.Config.Image}} image_id={{.Image}} started_at={{.State.StartedAt}}'
```

2. 从旧应用容器导出 env 到临时文件，不打印内容：

```bash
PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH" \
docker inspect sub2api-candidate \
  --format '{{range .Config.Env}}{{println .}}{{end}}' \
  > /tmp/sub2api-candidate-env-promote-20260701
```

3. 记录旧容器网络、端口、volume：

```bash
PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH" \
docker inspect sub2api-candidate \
  --format 'network={{range $name, $_ := .NetworkSettings.Networks}}{{$name}}{{end}} ports={{json .HostConfig.PortBindings}} mounts={{range .Mounts}}{{.Name}}:{{.Destination}} {{end}}'
```

4. 停止并删除应用容器，仅限 `sub2api-candidate`：

```bash
PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH" docker stop sub2api-candidate
PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH" docker rm sub2api-candidate
```

5. 用 18080 已验证镜像重建 `sub2api-candidate`，沿用原网络、端口与 env；当前旧容器没有 app data volume：

```bash
PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH" \
docker run -d \
  --name sub2api-candidate \
  --network sub2api-candidate-network \
  --env-file /tmp/sub2api-candidate-env-promote-20260701 \
  --publish 127.0.0.1:18084:8080 \
  sub2api-main-preview:20260629-092226-ddd4fb9a9
```

当前 `sub2api-candidate` 应用容器没有 app data volume 挂载；实际执行前仍必须以 `docker inspect sub2api-candidate` 的输出为准确认 network、port 和 mount，发现新增 mount 时按旧容器配置保留。

6. 等待应用健康：

```bash
for i in $(seq 1 60); do
  curl -fsS http://127.0.0.1:18084/health && exit 0
  sleep 2
done
exit 1
```

7. 删除临时 env 文件：

```bash
rm -f /tmp/sub2api-candidate-env-promote-20260701
```

## 发布后数据库验证

发布后必须验证 18084 PostgreSQL 已自动应用 migration：

```bash
PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH" \
docker exec sub2api-candidate-postgres \
  psql -U sub2api -d sub2api -Atc \
  "select count(*) from schema_migrations; select filename from schema_migrations order by filename desc limit 5;"
```

预期：

- migration 数量从 `191` 变为 `194`
- 最新 migration 为 `158_enable_affiliate_default.sql`

验证 79 元套餐：

```bash
PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH" \
docker exec sub2api-candidate-postgres \
  psql -U sub2api -d sub2api -Atc \
  "select sp.id,sp.group_id,g.name,sp.name,sp.price,sp.validity_days,sp.for_sale,sp.sort_order from subscription_plans sp left join groups g on g.id=sp.group_id where sp.name='79 元订阅池' or g.name='codex-pool-69-usd';"
```

预期：

- 存在 `codex-pool-69-usd`
- 存在 `79 元订阅池`
- `price=79.00`
- `validity_days=30`
- `for_sale=t`
- `sort_order=79`

验证邀请返利开关：

```bash
PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH" \
docker exec sub2api-candidate-postgres \
  psql -U sub2api -d sub2api -Atc \
  "select key,value from settings where key='affiliate_enabled';"
```

预期：

- `affiliate_enabled|true`

验证入口：

```bash
curl -fsS http://127.0.0.1:18084/health
curl -fsS http://127.0.0.1:8080/health
curl -fsS https://api.aaccx.pw/health
curl -sS https://aaccx.pw/dashboard | rg 'assets/(index|app-index)-|index-.*css'
```

## 回滚设计

### 应用层回滚

如果新应用容器启动失败，且 migration 未执行或未产生不兼容数据，可直接用旧镜像重建应用容器：

- 旧镜像：`sub2api-candidate:20260627-221441-traffic-card-fix`
- 端口仍为 `127.0.0.1:18084:8080`
- DB/Redis 仍为 `sub2api-candidate-postgres` / `sub2api-candidate-redis`

### 数据层精确回滚

如果 migration 已执行，并且需要恢复到发布前精确数据状态，使用备份恢复：

- PostgreSQL：`deploy/backups/20260701-080310-sub2api-candidate-postgres-before-app-promote.dump`
- Redis：`deploy/backups/20260701-080310-sub2api-candidate-redis-before-app-promote.rdb`

恢复前必须停止 `sub2api-candidate` 应用容器，避免恢复过程中有写入。PostgreSQL 恢复会覆盖当前公网 DB，只有在确认无人使用且需要精确回退时执行。

### 本轮 migration 的回滚判断

当前待执行的 `156/157/158` 主要新增和更新套餐、分组、settings 数据，没有删除旧表或旧字段。旧应用大概率可以容忍这些额外行，但精确回滚仍应以 PostgreSQL dump 为准。

## 风险与控制

- 风险：`156` 依赖 `codex-pool-49-usd` 复制部分配置。
  - 当前 18084 已存在 `codex-pool-49-usd`，检查结果正常。
- 风险：`156` 先写 `79.79`，`157` 再修正为 `79.00`。
  - 两条 migration 必须连续应用，不能只手工执行其中一条。
- 风险：旧应用回滚后仍看到新增 `79 元订阅池`。
  - 如果不能接受新增套餐残留，必须恢复 PostgreSQL dump。
- 风险：Redis 缓存结构不兼容。
  - 默认不替换 Redis；如发布后出现缓存问题，优先清具体缓存或重启应用，只有在明确需要恢复会话/队列状态时使用 Redis RDB。
- 风险：nginx 切错目标。
  - 本方案不改 nginx，避免把公网直接打到 18080 preview DB。

## 明确不做

- 不把 `sub2api-main-preview-postgres` 覆盖到 `sub2api-candidate-postgres`。
- 不把 `sub2api-main-preview-redis` 覆盖到 `sub2api-candidate-redis`。
- 不修改 nginx 的 `8080 -> 18084` 反代。
- 不手工编辑 `schema_migrations`。
- 不跳过应用启动 migration runner。
- 不提交 `deploy/backups/` 下的 dump/RDB 文件。

## 结论

公网数据库的正确修改方式是：保留 `sub2api-candidate-postgres`，让新 `sub2api-candidate` 应用容器启动时自动应用 `156/157/158` migration。当前不应手工把 18080 的数据库复制到公网，也不应只改 nginx 指向 18080。
