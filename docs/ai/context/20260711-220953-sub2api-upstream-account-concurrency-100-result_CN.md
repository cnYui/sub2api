# Sub2API 上游账号并发调整为 100 结果

时间：2026-07-11 22:09 JST

对应设计：`docs/ai/context/20260711-213214-sub2api-upstream-account-concurrency-100-design_CN.md`

对应计划：`docs/ai/context/20260711-214428-sub2api-upstream-account-concurrency-100-implementation-plan_CN.md`

## 执行结果

已通过 Sub2API 正式管理员登录和账号管理接口，将运行态唯一上游账号：

- `accounts.id=1`
- `name=cliproxy-local-openai`
- `concurrency: 10 -> 100`

更新请求只包含 `concurrency=100`，未修改账号凭据、状态、分组绑定或其他字段。管理员密码、JWT、API Key 和上游凭据均未输出或写入文档。

## 修改前保护

修改前已备份运行态 PostgreSQL：

```text
deploy/backups/20260711-215209-sub2api-candidate-before-account-concurrency-100.dump
```

- 文件大小约 53MB。
- 文件权限为 `600`。
- 文件非空，`pg_restore -l` 可正常读取，自定义格式目录包含 930 个条目。

## 最终验证

### PostgreSQL

`accounts.id=1` 最终状态：

```text
name=cliproxy-local-openai
status=active
schedulable=true
concurrency=100
```

未删除用户的并发分布仍为：

```text
concurrency=5, count=95
```

本次没有修改任何用户的 `users.concurrency`。

### Redis 调度快照与并发槽

`sched:acc:1` 已同步为：

```text
ID=1
Name=cliproxy-local-openai
Concurrency=100
Status=active
Schedulable=true
```

最终检查时：

- `concurrency:account:1` 有 7 个活跃槽，未被清理。
- 存在 4 个 `concurrency:api_key:*` key。
- 不存在 `concurrency:user:*` key。

活跃槽和 API Key key 数量会随实时请求自然变化，只用于证明本次没有清理运行中的并发状态，且入口并发维度仍是 API Key。

### Scheduler outbox

实施计划中的示例查询引用了不存在的 `processed_at` 字段。运行态 `scheduler_outbox` 实际字段为：

```text
id,event_type,account_id,group_id,payload,created_at,dedup_key
```

最终查询没有账号 1 的待处理事件。结合 PostgreSQL 已为 100 且 Redis `sched:acc:1` 已同步为 100，可确认事件已被 worker 消费并删除。历史计划文档保持不变，本结果文档记录实际 schema 差异。

### CLIProxyAPI

`/Users/wujianxiang/CodeSpace/CLIProxyAPI/config.yaml` 未修改，最终仍为：

- 普通请求：`global-concurrency=100`、`per-api-key-concurrency=100`。
- 图片生成：`global-concurrency=10`、`per-api-key-concurrency=10`。
- 图片编辑：`global-concurrency=10`、`per-api-key-concurrency=10`。
- 8317 端口由 `cli-proxy` 正常监听。

### 健康与日志

最终健康检查均返回 HTTP 200：

- Sub2API 18084：`/health`
- nginx 8080：`/health`
- 公网 `https://api.aaccx.pw/health`
- CLIProxyAPI 8317：`/healthz`

Sub2API 最近 10 分钟日志未发现 scheduler、outbox、Redis、数据库、panic、fatal、账号选择失败或并发 acquire/release 关键错误。

## 调整后的并发关系

```text
单把 Sub2API API Key：当前运行值 5
所有 Key 汇聚到 Sub2API 上游账号：100
CLIProxyAPI 普通请求：100/100
CLIProxyAPI 图片请求：10/10
```

因此，原先 Sub2API 全站共享的账号并发 10 已解除，普通请求链路现在与 CLIProxyAPI 的 100 并发保护上限对齐。图片请求仍由 CLIProxyAPI endpoint override 限制为 10。

## 未执行事项

- 未修改业务代码或数据库 schema。
- 未修改 CLIProxyAPI 配置。
- 未修改用户、API Key、套餐、计费或订阅数据。
- 未清理 Redis 并发槽或 scheduler key。
- 未重启 Sub2API、PostgreSQL、Redis、nginx、Cloudflare Tunnel 或 CLIProxyAPI。
- 未执行真实 100 路模型并发压测。
- 未提交或推送 Git。

## 回滚方式

若后续观察到本机资源、上游账号池或错误率无法承受当前设置，应通过同一账号管理接口将 `accounts.id=1.concurrency` 恢复为 10，并同时确认 PostgreSQL 与 `sched:acc:1` 均恢复。除非另行明确授权，不使用整库备份覆盖回滚后的新增业务数据。
