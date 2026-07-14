# LOCAL API Key 无上限并发设计

时间：2026-07-14 20:24 JST

## 目标

将用户指定的 LOCAL API Key 从当前每 Key 5 并发调整为 Sub2API 入口层不限并发。

目标 Key 只记录脱敏标识：

- `api_keys.id=32`
- 名称：`local unlimited key sk-LOCAL-454...e28804`
- 状态：`active`
- 所属用户：`users.id=13 / xiaobianfuai@gmail.com`
- 当前用户并发：`5`
- 当前未删除 API Key 数量：`1`

## 现有语义

Sub2API 已按 `api_key_id` 独立记录模型请求并发槽，但每把 Key 的上限取自所属用户的 `users.concurrency`。`ConcurrencyService.AcquireAPIKeySlot()` 对 `maxConcurrency <= 0` 直接放行，不创建 Redis 并发槽，因此 `users.concurrency=0` 表示该用户所有 API Key 在 Sub2API 入口层不限并发。

当前 schema 没有 API Key 独立并发字段。由于目标用户目前只有一把未删除 Key，将用户并发设为 0 可以满足当前目标；该用户未来新增的 Key 也会继承不限并发，这是用户已确认接受的影响。

不限并发只解除 Sub2API 的 API Key 入口限制，不绕过下游保护：

- Sub2API 唯一上游账号并发仍为 `100`。
- CLIProxyAPI 普通请求全局/单上游 Key 并发仍为 `100/100`。
- CLIProxyAPI 图片生成和编辑并发仍为 `10/10`。
- 套餐、余额、流量卡、RPM、模型权限和计费准入保持不变。

## 方案比较

### 方案一：正式管理员接口设置用户并发为 0

通过现有管理员认证和用户更新接口，仅提交 `concurrency=0`。该路径复用正式服务层、审计和缓存失效逻辑，不需要重启服务。

这是本次采用的方案。

### 方案二：直接更新 PostgreSQL

可以达到相同数据结果，但会绕过正式管理流程和应用侧缓存失效，不采用。

### 方案三：新增 API Key 独立并发字段

可以让单把 Key 独立不限并发，同时保持同用户其他 Key 的限制，但需要 schema、管理接口、认证缓存和前端控制面改造。当前用户只有一把 Key，不值得扩大变更范围。

## 执行设计

1. 修改前确认目标 Key、用户、当前并发、同用户 Key 数量和运行态健康状态。
2. 创建 PostgreSQL 自定义格式备份，限制文件权限为 `600`，并用 `pg_restore -l` 验证可读。
3. 通过正式管理员接口仅将 `users.id=13.concurrency` 从 `5` 更新为 `0`。
4. 核对 PostgreSQL 最终值、目标 Key 状态、同用户 Key 数量和相关审计记录。
5. 核对 API Key 鉴权缓存已失效或已刷新；不清理正在运行的 Redis 并发槽。
6. 验证 Sub2API 18084、Nginx 8080、公网入口和 CLIProxyAPI 8317 健康状态。
7. 检查近期日志中是否出现数据库、Redis、鉴权、panic、fatal 或并发控制异常。

## 回滚

若修改后需要恢复，通过同一管理员接口将 `users.id=13.concurrency` 改回 `5`，再核对数据库和鉴权缓存。整库备份只用于灾难恢复，不用于覆盖修改后的新增业务数据。

## 不执行事项

- 不修改 API Key 内容、状态、分组或配额。
- 不修改用户套餐、余额、流量卡、RPM 或角色。
- 不修改上游账号并发或 CLIProxyAPI 配置。
- 不清 Redis 全库或现有并发槽。
- 不重启 Sub2API、PostgreSQL、Redis、Nginx、Cloudflare Tunnel 或 CLIProxyAPI。
- 不执行真实高并发模型压测，避免产生费用和影响公网请求。
