# Sub2API 上游账号并发调整为 100 设计

时间：2026-07-11 21:32 JST

## 背景

当前模型请求链路为：

```text
用户请求
  -> Sub2API API Key 鉴权与计费准入
  -> Sub2API API Key 并发槽（当前运行数据为每 Key 5）
  -> Sub2API 上游账号槽 cliproxy-local-openai（当前 10）
  -> CLIProxyAPI inbound-limits（普通请求 100，图片请求 10）
  -> CLIProxyAPI 内部账号池与模型上游
```

当前唯一 Sub2API 上游账号 `accounts.id=1 / cliproxy-local-openai` 的并发为 10。即使多个用户拥有多把 API Key，全站普通模型请求在进入 CLIProxyAPI 前仍会被 Sub2API 账号槽限制为约 10，无法使用 CLIProxyAPI 已配置的普通请求 100 并发容量。

用户已确认希望把 Sub2API 该账号并发调整为 100，与 CLIProxyAPI 普通请求全局并发 100 对齐。

## 必须满足的目标

- 把运行态 Sub2API `accounts.id=1` 的 `concurrency` 从 10 调整为 100。
- 保留 Sub2API API Key 维度并发控制，不改变用户 Key 的现有并发语义。
- 保留 Sub2API 账号级保护层，但把其上限提高到 100，不设置为无限制。
- CLIProxyAPI 普通请求继续保持 `global=100/per-api-key=100`。
- CLIProxyAPI 图片生成和编辑继续保持 `global=10/per-api-key=10`。
- 不改代码、不构建镜像、不重启 Sub2API/CLIProxyAPI、不改 nginx、计费、订阅或用户数据。

## 方案比较

### 方案一：把 Sub2API 上游账号并发从 10 调整为 100（推荐）

优点：

- 与 CLIProxyAPI 普通请求全局并发上限一致。
- 保留 Sub2API 账号级背压，CLIProxyAPI 配置异常时仍有明确上限。
- 只修改一条运行态账号配置，不需要代码、迁移或服务重启。
- 管理接口更新会立即同步调度器缓存，生效路径清晰。

缺点：

- 单机 Sub2API、CLIProxyAPI、数据库和上游账号池会承受更高瞬时压力。
- CLIProxyAPI 或真实模型上游可能在达到 100 前先触发 429、账号不可用或网络瓶颈。

### 方案二：把 Sub2API 上游账号并发设为 0/无限制

不采用。`maxConcurrency <= 0` 在 Sub2API 并发服务中表示不限流，会删除 Sub2API 最后一层账号级背压；CLIProxyAPI 配置错误、未加载或被绕过时，可能出现无界并发。

### 方案三：复制多个指向同一 CLIProxyAPI 的 Sub2API 账号

不采用。多个账号记录仍指向同一个 CLIProxyAPI 聚合入口，只会增加调度、分组绑定和粘性会话复杂度，并不能创造新的真实上游容量。

## 推荐设计

使用 Sub2API 管理接口更新 `accounts.id=1`：

```http
PUT /api/v1/admin/accounts/1
Content-Type: application/json

{"concurrency":100}
```

也可以通过项目现有管理员 CLI 调用同一管理接口：

```bash
node skills/sub2api-admin/scripts/sub2api-admin.js accounts update 1 --json '{"concurrency":100}'
```

不推荐直接执行 SQL。管理接口会经过 `AdminService.UpdateAccount()` 和 `AccountRepository.Update()`：

- 更新 PostgreSQL `accounts.concurrency`。
- 写入 scheduler outbox 的 `account_changed` 事件。
- 立即刷新 Redis 单账号调度快照 `sched:acc:1`。

现有 `concurrency:account:1` 是正在处理请求的 Sorted Set，最大并发值不保存在该键中；每次抢槽都会使用最新账号快照传入的 `maxConcurrency`。因此从 10 调到 100 不需要删除当前槽位，也不应清空 Redis。

## 调整后的有效并发关系

```text
单把 Sub2API API Key
  -> 当前默认/运行值 5

所有 Sub2API API Key 合计
  -> Sub2API cliproxy-local-openai 账号上限 100

CLIProxyAPI 普通请求
  -> 全局 100，共用上游 Key 100

CLIProxyAPI 图片请求
  -> 独立全局 10，共用上游 Key 10
```

这意味着普通请求的 Sub2API 账号闸门和 CLIProxyAPI 入站闸门对齐为 100。图片请求即使通过 Sub2API 的账号 100，仍会被 CLIProxyAPI 图片 endpoint override 限制为 10，符合现有图片保护策略。

## 实施前检查

- 确认公网应用容器仍为 `sub2api-candidate`，端口为 18084。
- 确认目标账号仍是 `accounts.id=1 / cliproxy-local-openai / status=active`。
- 确认当前并发为 10，且没有第二个指向 CLIProxyAPI 的 active 上游账号。
- 确认 CLIProxyAPI 当前普通入站配置仍是 100/100，图片仍是 10/10。
- 确认 18084、nginx 8080 和公网 `/health` 均正常。
- 修改前备份 PostgreSQL。Redis 仅保存可重建缓存和运行中槽位，本次不重启、不清缓存，因此不需要恢复 Redis 快照。

## 生效与验收

更新后必须同时验证：

1. PostgreSQL `accounts.id=1.concurrency = 100`。
2. Redis `sched:acc:1` 中账号快照的 `concurrency = 100`。
3. `concurrency:account:1` 当前活跃成员未被删除，运行中请求不受破坏。
4. `concurrency:api_key:*` 仍按 API Key 独立存在，未恢复成 `concurrency:user:*`。
5. 现有用户的 `users.concurrency` 不被修改。
6. CLIProxyAPI `config.yaml` 仍为普通 100/100、图片 10/10。
7. 18084、8080 和公网 `/health` 均返回 200。
8. Sub2API 与 CLIProxyAPI 日志没有新增 scheduler cache、Redis、账号选择或 inbound limiter 配置错误。

本次配置更新不直接发起 100 路真实模型压测。真实并发验证应单独安排低峰测试，并先明确成本、上游 429 风险和停止阈值。最小运行态验证可在低峰使用多把 Key 发起超过 10、但明显低于 100 的受控请求，确认不再被旧的账号并发 10 卡住。

## 风险与控制

### 本机资源压力

账号并发从 10 提高到 100 后，Sub2API、CLIProxyAPI、TLS、流式连接和日志的峰值资源占用可能显著增加。CLIProxyAPI 的 100 是保护上限，不代表当前 M1/8GB 和真实上游一定能稳定承受 100 个长流请求。

控制方式：先完成配置对齐，再单独采用阶梯压测验证 20、40、60、80、100，并观察 CPU、内存、连接数、TTFT、错误率和队列等待。

### 上游账号池容量

CLIProxyAPI 的全局 100 只限制入站请求数量，不保证内部 Codex/OpenAI 账号池具有 100 个可用并发。真实上游可能先返回 429、usage limit、token invalidated 或 no available accounts。

控制方式：并发测试必须同时观察 CLIProxyAPI 账号选择与上游错误分类，不能把上游限额误判为 Sub2API CPU 瓶颈。

### 单 Key 上限语义

本次只调整上游账号并发，不修复“每 Key 硬上限 5”与 `users.concurrency` 可修改之间的语义差异。当前运行库所有用户都是 5，因此当前每 Key 为 5；若后续管理员修改用户并发，每 Key 上限仍会跟随变化。

## 回滚

出现异常时，通过同一管理接口把账号并发恢复为 10：

```http
PUT /api/v1/admin/accounts/1
Content-Type: application/json

{"concurrency":10}
```

回滚后验证 PostgreSQL 和 `sched:acc:1` 均恢复为 10。无需重启容器，也不删除 `concurrency:account:1` 或 `concurrency:api_key:*`。

## 非目标

- 不把每 Key 并发硬编码为 5。
- 不修改用户 `users.concurrency`。
- 不修改 CLIProxyAPI 普通或图片入站限制。
- 不修改 Sub2API DB/Redis 连接池。
- 不执行真实高并发压测。
- 不构建、发布或重启任何服务。

## 后续实施门槛

用户确认本设计后，再单独编写实施计划并执行运行态更新。实施时必须先完成 PostgreSQL 备份和前置检查，再通过管理接口修改，最后按验收项逐项复核。
