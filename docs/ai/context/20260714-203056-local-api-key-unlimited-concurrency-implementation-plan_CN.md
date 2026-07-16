# LOCAL API Key 无上限并发实施计划

> **面向执行型智能体：** 必须使用 `superpowers:executing-plans` 按任务逐项实施本计划，并使用复选框（`- [ ]`）跟踪进度。

**目标：** 将 `api_keys.id=32` 所属用户 `users.id=13` 的并发从 5 调整为 0，使该用户当前唯一 API Key 在 Sub2API 入口层不限并发。

**架构：** 不改代码或 schema。先备份 PostgreSQL，再使用 Chrome 中现有管理员 JWT 会话通过 `/admin/users` 正式管理界面提交单字段并发更新；后端 `AdminService.UpdateUser()` 负责写库、生成 `admin_concurrency` 审计记录并按用户失效 API Key 鉴权缓存。

**技术栈：** Sub2API 管理后台、Gin 管理 API、PostgreSQL 18、Redis 8、Docker、Chrome 管理员会话。

---

对应设计：`docs/ai/context/20260714-202436-local-api-key-unlimited-concurrency-design_CN.md`

## 文件范围

- 新建：`docs/ai/context/YYYYMMDD-HHMMSS-local-api-key-unlimited-concurrency-result_CN.md`，记录运行态结果。
- 修改：`AGENTS.md`，追加本次运行态定论索引。
- 不修改业务代码、数据库 schema、前端代码或配置文件。

### Task 1：确认目标和修改前状态

- [ ] **Step 1：确认容器和四层健康状态**

```bash
docker ps --filter name='^/sub2api-candidate$' --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
curl -fsS http://127.0.0.1:18084/health
curl -fsS http://127.0.0.1:8080/health
curl -fsS https://api.aaccx.pw/health
curl -kfsS https://127.0.0.1:8317/healthz
```

预期：四个健康端点均返回成功，Sub2API 容器保持运行。

- [ ] **Step 2：只读确认 Key、用户和影响面**

```bash
docker exec sub2api-candidate-postgres sh -lc 'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -P pager=off -c "SELECT k.id,k.name,k.status,k.user_id,u.email,u.role,u.status,u.concurrency,(SELECT count(*) FROM api_keys k2 WHERE k2.user_id=k.user_id AND k2.deleted_at IS NULL) AS active_key_count FROM api_keys k JOIN users u ON u.id=k.user_id WHERE k.id=32 AND k.deleted_at IS NULL;"'
```

预期：`api_keys.id=32` active，所属 `users.id=13 / xiaobianfuai@gmail.com` 为 active admin，`concurrency=5`，未删除 Key 数量为 1。

- [ ] **Step 3：记录修改前缓存和并发槽状态**

```bash
docker exec sub2api-candidate-redis sh -lc 'unset REDISCLI_AUTH; redis-cli --scan --pattern "concurrency:api_key:32"'
docker exec sub2api-candidate-redis sh -lc 'unset REDISCLI_AUTH; redis-cli ZCARD concurrency:api_key:32'
docker exec sub2api-candidate-redis sh -lc 'unset REDISCLI_AUTH; redis-cli --scan --pattern "*api*auth*" | head -100'
```

活跃槽会随请求自然变化；不得删除现有槽。

### Task 2：备份 PostgreSQL

- [ ] **Step 1：创建自定义格式备份**

```bash
STAMP=$(date +%Y%m%d-%H%M%S)
BACKUP="deploy/backups/${STAMP}-sub2api-candidate-before-user-13-unlimited-concurrency.dump"
docker exec sub2api-candidate-postgres sh -lc 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > "$BACKUP"
chmod 600 "$BACKUP"
```

- [ ] **Step 2：验证备份可恢复读取**

```bash
test -s "$BACKUP"
docker exec -i sub2api-candidate-postgres pg_restore -l < "$BACKUP" >/dev/null
stat -f '%N|%z bytes|%Sp' "$BACKUP"
```

预期：文件非空、权限为 `-rw-------`、`pg_restore -l` 返回 0。任一条件失败时停止，不执行写入。

### Task 3：通过正式管理员界面更新并发

- [ ] **Step 1：打开管理员用户页面并确认会话**

在 Chrome 打开 `https://api.aaccx.pw/admin/users`。若现有会话已过期，仅使用浏览器已有登录能力恢复会话；不得在终端或文档输出 JWT、刷新令牌或密码。

- [ ] **Step 2：定位唯一目标用户**

在用户搜索框输入 `xiaobianfuai@gmail.com`，确认结果的用户 ID 为 13、角色为管理员、状态 active、并发为 5。

- [ ] **Step 3：提交单字段更新**

打开用户编辑界面，只把并发从 `5` 改为 `0` 并保存。不得修改邮箱、密码、用户名、备注、状态、RPM、余额、角色、允许分组或专属倍率。

预期：界面保存成功，列表刷新后并发显示为 0 或“不限”。

### Task 4：验证正式更新链路

- [ ] **Step 1：验证 PostgreSQL 最终值和 Key 状态**

```bash
docker exec sub2api-candidate-postgres sh -lc 'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -P pager=off -c "SELECT u.id,u.email,u.role,u.status,u.concurrency,u.rpm_limit,k.id AS api_key_id,k.name,k.status AS api_key_status,k.group_id FROM users u JOIN api_keys k ON k.user_id=u.id AND k.deleted_at IS NULL WHERE u.id=13 ORDER BY k.id;"'
```

预期：`users.id=13.concurrency=0`；角色、状态、RPM 和 Key 字段未改变。

- [ ] **Step 2：验证管理员并发审计记录**

```bash
docker exec sub2api-candidate-postgres sh -lc 'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -P pager=off -c "SELECT id,type,value,status,used_by,used_at,created_at FROM redeem_codes WHERE type='\''admin_concurrency'\'' AND used_by=13 ORDER BY id DESC LIMIT 3;"'
```

预期：最新记录 `value=-5`、`status=used`、`used_by=13`，时间属于本次修改窗口。

- [ ] **Step 3：验证鉴权缓存失效效果**

使用目标 Key 请求不产生模型费用的 `/v1/models`，确认 HTTP 200；随后检查容器日志没有认证缓存删除错误。不得执行模型生成请求或并发压测。

```bash
test -n "$TARGET_API_KEY"
curl -sS -o /tmp/sub2api-user13-models.json -w '%{http_code}\n' \
  -H "Authorization: Bearer ${TARGET_API_KEY}" \
  http://127.0.0.1:18084/v1/models
rm -f /tmp/sub2api-user13-models.json
unset TARGET_API_KEY
```

执行前从用户本轮提供值设置仅存在于当前进程环境的 `TARGET_API_KEY`，不得使用 `echo`、shell tracing 或进程参数打印该值。预期 HTTP 200。

### Task 5：健康与日志验收

- [ ] **Step 1：复核四层健康状态**

```bash
curl -fsS http://127.0.0.1:18084/health
curl -fsS http://127.0.0.1:8080/health
curl -fsS https://api.aaccx.pw/health
curl -kfsS https://127.0.0.1:8317/healthz
```

- [ ] **Step 2：检查修改后日志窗口**

```bash
docker logs --since 10m sub2api-candidate 2>&1 | rg -i 'panic|fatal|database|redis|auth.*cache|concurr|INVALID_ADMIN|error' || true
```

预期：无新增数据库、Redis、鉴权缓存、panic、fatal 或并发控制关键错误。业务侧历史错误需结合时间和请求 ID排除。

- [ ] **Step 3：确认下游保护未改变**

```bash
docker exec sub2api-candidate-postgres sh -lc 'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -P pager=off -c "SELECT id,name,status,schedulable,concurrency FROM accounts WHERE id=1;"'
sed -n '63,75p' /Users/wujianxiang/CodeSpace/CLIProxyAPI/config.yaml
```

预期：Sub2API 上游账号并发仍为 100；CLIProxyAPI 普通 100/100、图片 10/10 保持不变。

### Task 6：异常回滚

- [ ] **Step 1：满足停止条件时通过同一管理员界面回滚**

以下任一条件触发回滚：数据库最终值不是 0、审计记录异常、18084/8080/公网健康失败、鉴权缓存持续报错、更新误改其他用户字段。

在 `/admin/users` 将 `users.id=13` 的并发从 0 改回 5，只提交并发字段。

- [ ] **Step 2：验证回滚结果**

```bash
docker exec sub2api-candidate-postgres sh -lc 'psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -P pager=off -c "SELECT id,email,status,role,concurrency FROM users WHERE id=13;"'
```

预期：`concurrency=5`，并产生对应 `value=5` 的 `admin_concurrency` 审计记录。未经用户再次明确授权，不执行整库恢复。

### Task 7：记录结果

- [ ] **Step 1：新建结果文档**

新建 `docs/ai/context/YYYYMMDD-HHMMSS-local-api-key-unlimited-concurrency-result_CN.md`，记录备份路径、修改前后值、审计记录、缓存验证、健康状态、下游限制和未执行事项。

- [ ] **Step 2：更新长期上下文**

在 `AGENTS.md` 的“最高优先级定论”顶部追加本次结果索引，不覆写历史条目。

- [ ] **Step 3：完成文档校验**

```bash
git diff --check -- AGENTS.md docs/ai/context
git status --short
```

不得在文档中记录完整 API Key、JWT、刷新令牌、管理员密码或数据库凭据。

## 完成标准

- `users.id=13.concurrency=0`。
- `api_keys.id=32` 仍为 active，其他字段未改变。
- 新增 `admin_concurrency=-5` 审计记录。
- 目标 Key `/v1/models` 返回 HTTP 200。
- Sub2API 上游账号仍为 100，CLIProxyAPI 普通 100/100、图片 10/10 未变。
- 18084、8080、公网和 8317 健康正常。
- 没有新增关键运行错误。
- 结果文档和 `AGENTS.md` 长期记忆已落盘。
