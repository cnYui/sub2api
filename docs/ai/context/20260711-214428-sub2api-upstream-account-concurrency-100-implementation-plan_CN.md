# Sub2API 上游账号并发调整为 100 实施计划

时间：2026-07-11 21:44 JST

对应设计：`docs/ai/context/20260711-213214-sub2api-upstream-account-concurrency-100-design_CN.md`

## 目标

把运行态 Sub2API 唯一上游账号 `accounts.id=1 / cliproxy-local-openai` 的并发从 10 调整为 100，使 Sub2API 普通请求账号级闸门与 CLIProxyAPI 普通入站限制 100/100 对齐。

本次只修改运行态账号配置，不改代码、数据库 schema、用户 API Key 并发、CLIProxyAPI 配置、nginx 或容器。

## 执行约束

- 必须先备份 PostgreSQL，并验证备份可读。
- 必须通过 Sub2API 管理接口更新，不直接执行 SQL。
- 不重启 Sub2API、Postgres、Redis、CLIProxyAPI、nginx 或 Cloudflare Tunnel。
- 不删除 `concurrency:account:1`、`concurrency:api_key:*` 或 scheduler Redis key。
- 不执行 100 路真实模型压测。
- 管理 API Key、用户 API Key、上游 Key 和账号凭据不得写入文档或日志摘要。
- 任一前置检查失败时停止，不继续写入。

## Task 1：确认当前运行态和目标对象

- [ ] 确认公网应用容器与健康状态：

```bash
docker ps --filter name='^/sub2api-candidate$' --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
curl -fsS http://127.0.0.1:18084/health
curl -fsS http://127.0.0.1:8080/health
curl -fsS https://api.aaccx.pw/health
```

- [ ] 只读确认目标账号唯一且当前值为 10：

```bash
docker exec sub2api-candidate-postgres psql -U sub2api -d sub2api -P pager=off -c "select id,name,platform,type,status,schedulable,concurrency from accounts where deleted_at is null order by id;"
```

预期：只有一个 active/schedulable 上游账号，`id=1`、`name=cliproxy-local-openai`、`concurrency=10`。

- [ ] 确认用户并发分布不变：

```bash
docker exec sub2api-candidate-postgres psql -U sub2api -d sub2api -P pager=off -c "select concurrency,count(*) from users where deleted_at is null group by concurrency order by concurrency;"
```

- [ ] 确认 CLIProxyAPI 当前限制仍匹配设计：

```bash
sed -n '63,75p' /Users/wujianxiang/CodeSpace/CLIProxyAPI/config.yaml
lsof -nP -iTCP:8317 -sTCP:LISTEN
```

预期：普通请求 100/100，图片生成和编辑 10/10，8317 正常监听。

- [ ] 记录修改前 Redis 状态，不输出账号凭据：

```bash
docker exec sub2api-candidate-redis sh -lc 'unset REDISCLI_AUTH; redis-cli --raw GET sched:acc:1' | jq '{id,name,concurrency,status,schedulable}'
docker exec sub2api-candidate-redis sh -lc 'unset REDISCLI_AUTH; redis-cli ZCARD concurrency:account:1'
docker exec sub2api-candidate-redis sh -lc 'unset REDISCLI_AUTH; redis-cli --scan --pattern "concurrency:api_key:*"'
docker exec sub2api-candidate-redis sh -lc 'unset REDISCLI_AUTH; redis-cli --scan --pattern "concurrency:user:*"'
```

## Task 2：备份 PostgreSQL

- [ ] 生成本轮唯一时间戳和备份路径：

```bash
STAMP=$(date +%Y%m%d-%H%M%S)
BACKUP="deploy/backups/${STAMP}-sub2api-candidate-before-account-concurrency-100.dump"
```

- [ ] 导出自定义格式备份并限制权限：

```bash
docker exec sub2api-candidate-postgres pg_dump -U sub2api -d sub2api -Fc > "$BACKUP"
chmod 600 "$BACKUP"
```

- [ ] 验证备份非空且 `pg_restore` 可读：

```bash
test -s "$BACKUP"
docker exec -i sub2api-candidate-postgres pg_restore -l < "$BACKUP"
ls -lh "$BACKUP"
```

停止条件：备份为空、不可读或命令非 0 时停止，不执行账号更新。

## Task 3：通过管理接口更新账号并发

- [ ] 确认管理员 CLI 环境变量已配置，不打印值：

```bash
test -n "$SUB2API_ADMIN_API_KEY"
export SUB2API_BASE_URL='http://127.0.0.1:18084'
```

- [ ] 使用只读命令再次确认目标账号：

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/skills/sub2api-admin
node scripts/sub2api-admin.js accounts get 1
```

- [ ] 执行单字段更新：

```bash
node scripts/sub2api-admin.js accounts update 1 --json '{"concurrency":100}'
```

预期：返回账号 `id=1`、`concurrency=100`。不得同时提交 credentials、extra、group_ids、status 等无关字段。

## Task 4：验证数据库和调度快照已同步

- [ ] 验证 PostgreSQL：

```bash
docker exec sub2api-candidate-postgres psql -U sub2api -d sub2api -P pager=off -c "select id,name,status,schedulable,concurrency,updated_at from accounts where id=1;"
```

- [ ] 验证 Redis 单账号调度快照：

```bash
docker exec sub2api-candidate-redis sh -lc 'unset REDISCLI_AUTH; redis-cli --raw GET sched:acc:1' | jq '{id,name,concurrency,status,schedulable}'
```

预期：数据库和 `sched:acc:1` 均为 100。

- [ ] 验证 scheduler outbox 已产生或被 worker 消费：

```bash
docker exec sub2api-candidate-postgres psql -U sub2api -d sub2api -P pager=off -c "select id,event_type,account_id,processed_at,created_at from scheduler_outbox where account_id=1 order by id desc limit 5;"
```

停止条件：DB 已是 100 但 `sched:acc:1` 仍为 10，执行 Task 7 回滚，再排查 scheduler cache。

## Task 5：确认现有并发槽和用户配置未被破坏

- [ ] 对比修改前后的账号活跃槽计数：

```bash
docker exec sub2api-candidate-redis sh -lc 'unset REDISCLI_AUTH; redis-cli ZCARD concurrency:account:1'
```

活跃请求自然完成会导致计数变化，不要求数值相等；只确认没有人为删除或 Redis 类型错误。

- [ ] 确认 API Key 槽维度仍正常：

```bash
docker exec sub2api-candidate-redis sh -lc 'unset REDISCLI_AUTH; redis-cli --scan --pattern "concurrency:api_key:*"'
docker exec sub2api-candidate-redis sh -lc 'unset REDISCLI_AUTH; redis-cli --scan --pattern "concurrency:user:*"'
```

- [ ] 再次确认用户并发未变：

```bash
docker exec sub2api-candidate-postgres psql -U sub2api -d sub2api -P pager=off -c "select concurrency,count(*) from users where deleted_at is null group by concurrency order by concurrency;"
```

## Task 6：健康和日志验收

- [ ] 验证三层健康：

```bash
curl -fsS http://127.0.0.1:18084/health
curl -fsS http://127.0.0.1:8080/health
curl -fsS https://api.aaccx.pw/health
```

- [ ] 查看 Sub2API 修改后日志窗口：

```bash
docker logs --since 10m sub2api-candidate
```

重点检查 scheduler cache/outbox、Redis acquire/release、账号选择、panic、fatal 和数据库错误。

- [ ] 确认 CLIProxyAPI 8317 仍监听，配置未修改：

```bash
lsof -nP -iTCP:8317 -sTCP:LISTEN
sed -n '63,75p' /Users/wujianxiang/CodeSpace/CLIProxyAPI/config.yaml
```

- [ ] 本轮不做 100 并发真实请求。若需要证明旧 10 限制已解除，另开低峰压测任务，使用 20 以下受控并发并设置成本与错误率停止阈值。

## Task 7：回滚路径

满足以下任一条件立即回滚：

- Redis `sched:acc:1` 未同步到 100。
- 18084、8080 或公网 health 异常。
- scheduler cache、Redis 或数据库出现持续错误。
- 账号选择错误或 CLIProxyAPI 429/5xx 在无压测情况下明显增加。

- [ ] 使用管理 CLI 恢复为 10：

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/skills/sub2api-admin
node scripts/sub2api-admin.js accounts update 1 --json '{"concurrency":10}'
```

- [ ] 验证 DB 和 Redis 快照均恢复为 10：

```bash
docker exec sub2api-candidate-postgres psql -U sub2api -d sub2api -P pager=off -c "select id,name,status,schedulable,concurrency from accounts where id=1;"
docker exec sub2api-candidate-redis sh -lc 'unset REDISCLI_AUTH; redis-cli --raw GET sched:acc:1' | jq '{id,name,concurrency,status,schedulable}'
```

一般不恢复整库备份。整库恢复会覆盖本轮之后产生的用户、订单和 usage 数据，未经用户再次明确授权不得执行。

## Task 8：结果记录

- [ ] 新建 `docs/ai/context/YYYYMMDD-HHMMSS-sub2api-upstream-account-concurrency-100-result_CN.md`。
- [ ] 记录备份路径、修改前后值、DB/Redis 快照、health 和日志结论。
- [ ] 更新 `AGENTS.md`，明确运行态账号并发已经变为 100，CLIProxyAPI 普通 100/100、图片 10/10 未变。
- [ ] 执行 `git diff --check`。
- [ ] 不提交或推送 git，除非用户明确要求。

## 完成标准

- `accounts.id=1.concurrency=100`。
- `sched:acc:1.concurrency=100`。
- 用户 `users.concurrency` 未改变。
- API Key 槽维度未改变。
- CLIProxyAPI 普通 100/100 和图片 10/10 未改变。
- 18084、8080、公网 health 正常。
- 无新增关键运行错误。
- 结果文档和 AGENTS 记忆已落盘。
