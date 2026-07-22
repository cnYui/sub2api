# 双 Sub2API 内层 100 并发实施结果

时间：2026-07-23 08:30

## 结论

内层 latest Sub2API（`18086`）固定转发身份的用户级并发已从 `5` 成功更新为 `100`，管理 API 与数据库回读一致。

本次没有修改任何 Go 源码、Docker Compose、外层用户并发、账号级并发、账号调度算法、分组绑定或计费逻辑。

## 已执行变更

- 目标：内层 `users.id=1`，即内部转发 Key `outer-sub2api-forwarder` 的所属用户。
- 变更前：`concurrency=5`。
- 变更后：`concurrency=100`。
- 写入方式：受控单条 SQL，包含 `WHERE id = 1 AND concurrency = 5` 前置条件；返回 `1|100` 与 `UPDATE 1`。
- 回读：数据库显示 `1|100`；使用内层管理 API 登录后，`GET /api/v1/admin/users/1` 也返回 `concurrency=100`、`status=active`。

容器环境中的初始管理员密码与恢复后的数据库密码不一致，管理 API 的首次登录返回 401；用户提供当前管理员凭证后，管理 API 回读通过。凭证、Token、API Key 和 OAuth 凭证均未写入文档、Git、日志或命令输出。

## 备份与回滚

- 变更前 custom 格式数据库备份：`backups/20260723-081853-upstream-18086-before-concurrency-100.dump`。
- 备份大小约 2.37 MB，`pg_restore -l` 已成功读取 archive 目录。
- 备份未纳入 Git。
- 精确回滚命令：

```powershell
docker exec sub2api-upstream-postgres sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -At -F "|" -c "UPDATE users SET concurrency = 5 WHERE id = 1 AND concurrency = 100 RETURNING id, concurrency;"'
```

该回滚只修改同一用户的一个字段，不应通过整库恢复覆盖后续产生的用量数据。

## 验证结果

- 外层历史 `usage_facts` 仍记录真实 `user_id`、`api_key_id` 和外层 `account_id=2`；内层 `usage_logs` 仍归属固定转发身份 `user_id=1/api_key_id=1`，职责边界未改变。
- 内层账号级并发未调整：账号 `1..10` 仍为 `1`，账号 `11..22` 仍为 `10`；优先级也未改动。
- 内层直接发起 10 路并发 `/v1/responses` 请求时，认证和用户级检查均放行，但均在账号选择阶段返回 503，而不是用户并发限制错误。

## 代码验证

- 未修改 Go 源码；本次是内层运行态用户并发配置变更。
- `go test ./internal/handler/admin -count=1` 通过。
- `go test ./...` 在 124 秒后超时，未产生失败用例输出，不能视为完整后端测试已通过。

## 新发现的阻塞项

当前 18086 无法完成成功的模型请求，根因不是并发：

- 22 个 OpenAI 账号全部为 `status=error`、`schedulable=false`。
- 每个账号的错误均为 `Workspace deactivated (402): workspace has been deactivated`。
- 内层 `internal-openai-upstream` 分组当前没有任何 `account_groups` 绑定。
- 服务日志确认请求已通过内部 Key、用户和分组检查，随后调度器报 `no available accounts` 并返回 503。

因此，100 并发入口已生效，但当前可用模型账号容量为 0，无法用成功模型响应验证 10 路并发的真实吞吐。

## 后续边界

账号恢复需要单独授权和设计：重新认证或替换已停用的 OpenAI 凭证，确认有效后绑定到 `internal-openai-upstream`，再以真实外层用户 Key 做 10 路并发和计费回归。不得直接把当前 22 个失效账号强行设为可调度。
