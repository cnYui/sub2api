# LOCAL API Key 无上限并发设置结果

时间：2026-07-14 21:25 JST

## 结论

已将目标 LOCAL API Key 所属用户 `users.id=13 / xiaobianfuai@gmail.com` 的并发从 `5` 设置为 `0`。目标用户当前只有一把未删除 Key：

- `api_keys.id=32`
- 名称：`local unlimited key sk-LOCAL-454...e28804`
- 状态：`active`
- 分组：自动选择（`group_id=NULL`）

当前真实语义是：Sub2API 仍按 `api_key_id` 独立处理并发，但每把 Key 的上限取自所属用户的 `users.concurrency`。`ConcurrencyService.AcquireAPIKeySlot()` 对 `maxConcurrency <= 0` 直接放行，因此 `0` 表示跳过 Sub2API API Key 级并发限制，不会因数值为 0 报错。

该用户未来新增的 Key 也会继承不限并发。套餐、余额、流量卡、RPM、模型权限、计费准入和下游保护仍然生效。

## 设计与执行取舍

原计划优先通过正式管理员接口只提交 `concurrency=0`。实际执行发现：

- 后端 `PUT /api/v1/admin/users/:id` 和并发服务支持 0。
- 管理后台 `UserEditModal.vue` 额外硬编码 `form.concurrency < 1`，因此页面拒绝 0，数据库未发生写入。
- 当前运行态未配置可复用的全局 `admin-` API Key。
- 浏览器安全策略禁止通过页面脚本绕过表单校验。

经用户明确授权，最终改用带精确前置条件的 PostgreSQL 单事务，避免生成新的全局管理员凭据。事务在写入前锁定目标用户，并断言：用户仍为 active admin、并发仍为 5、仍只有一把未删除 Key、`api_keys.id=32` 仍 active；任一条件变化都会整体失败。

## 修改前保护

修改前备份：

`deploy/backups/20260714-204110-sub2api-candidate-before-user-13-unlimited-concurrency.dump`

- 大小：`74,190,246` 字节。
- 权限：`600`。
- PostgreSQL 自定义格式。
- `pg_restore -l` 验证可读。

## 数据库结果

单事务完成：

- `users.id=13.concurrency: 5 -> 0`。
- 新增 `redeem_codes.id=25`。
- 审计类型：`admin_concurrency`。
- 审计值：`-5.00000000`。
- 状态：`used`。
- `used_by=13`。
- 备注：`将 LOCAL API Key 所属用户并发调整为 0（不限流）`。

最终用户分布为：

- `concurrency=0`：1 个用户。
- `concurrency=5`：104 个用户。

目标用户仍为 active admin，RPM 为 0，余额为 0；目标 Key 的状态、分组和额度未改变。

## 缓存与接口验证

已按目标 Key 的 SHA-256 标识精准处理鉴权缓存：

- Redis L2 `apikey:auth:*` 删除返回 `1`。
- 向 `auth:cache:invalidate` 发布 L1 失效消息，订阅者数量返回 `1`。
- 未删除 `concurrency:api_key:32`；修改前两个运行中槽位自然结束，最终槽数为 0。

随后使用目标 Key 请求不产生模型费用的本地 `/v1/models`：

- HTTP 状态：`200`。
- 回源后 Redis 鉴权快照中的用户并发：`0`。
- 新 L2 缓存正常生成并具有 TTL。

这证明后续模型请求读取到的并发值为 0，会走不限流分支。

## 健康与下游限制

最终健康检查均成功：

- Sub2API `127.0.0.1:18084/health`。
- Nginx `127.0.0.1:8080/health`。
- 公网 `https://api.aaccx.pw/health`。
- CLIProxyAPI `127.0.0.1:8317/healthz`。

下游保护未改变：

- Sub2API 上游账号 `accounts.id=1` 仍为 active、schedulable、并发 100。
- CLIProxyAPI 普通请求仍为 `global=100 / per-api-key=100`。
- 图片生成和编辑仍为 `10/10`。

最近日志未发现匹配的数据库、Redis、鉴权缓存、并发控制、panic 或 fatal 关键错误。

## 未执行事项

- 未修改业务代码或数据库 schema。
- 未修改完整 API Key、状态、分组、额度或有效期。
- 未修改套餐、余额、流量卡、RPM 或计费规则。
- 未修改上游账号或 CLIProxyAPI 配置。
- 未清 Redis 全库或强制删除运行中并发槽。
- 未重启 Sub2API、PostgreSQL、Redis、Nginx、Cloudflare Tunnel 或 CLIProxyAPI。
- 未执行真实模型生成或高并发压测。

## 后续设计问题

管理后台“并发最小为 1”与后端“0 表示不限流”存在明确契约冲突。若后续需要从页面管理不限并发，应单独修复前端校验和展示，并补充 `0=不限` 的契约测试；本轮不扩大为代码发布任务。

## 回滚

如需恢复限制，应在事务中将 `users.id=13.concurrency` 从 0 改回 5，新增 `admin_concurrency=5` 审计记录，并再次精准失效目标 Key 的 L2/L1 鉴权缓存。未经再次授权，不使用整库备份覆盖回滚后的新增业务数据。
