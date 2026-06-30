# 79/99 元套餐绑定上游修改面设计说明

时间：2026-06-30 22:44 JST

## 目标

说明要让 79 元和 99 元套餐都能访问 OpenAI 上游，需要修改哪些部分，并区分运行态数据库、源码迁移、后端逻辑、前端文案。

## 当前事实

公网 18084：

- 在售订阅套餐只有 4 个：29、39、59、99。
- 99 元套餐存在，对应 `groups.id=8 / codex-pool-89-usd`，每日 89 USD。
- `codex-pool-89-usd` 没有任何 `account_groups` 绑定，因此请求在账号选择阶段失败：`no available accounts`。
- 公网 18084 尚未应用 `156_seed_codex_79_subscription_plan.sql` 和 `157_fix_codex_79_subscription_plan_base_price.sql`，所以没有 79 元套餐和 `codex-pool-69-usd`。

本地 main-preview 18080：

- 在售订阅套餐有 5 个：29、39、59、79、99。
- 79 元套餐对应 `codex-pool-69-usd`，每日 69 USD。
- 99 元套餐对应 `codex-pool-89-usd`，每日 89 USD。
- `codex-pool-69-usd` 与 `codex-pool-89-usd` 都没有上游绑定。

## 需要修改的部分

### 1. 公网数据库：补 79 元套餐和 group

公网 18084 需要应用或等价执行：

- `backend/migrations/156_seed_codex_79_subscription_plan.sql`
- `backend/migrations/157_fix_codex_79_subscription_plan_base_price.sql`

作用：

- 创建/更新 `groups.name='codex-pool-69-usd'`
- 创建/更新 `subscription_plans.name='79 元订阅池'`
- 将 79 元套餐基础价修正为 `79.00`

### 2. 公网数据库：绑定 79/99 group 到上游账号

需要写入 `account_groups`：

- `codex-pool-69-usd` -> `cliproxy-local-openai`
- `codex-pool-89-usd` -> `cliproxy-local-openai`

当前公网唯一可用 OpenAI 上游是：

- `accounts.id=1`
- `accounts.name='cliproxy-local-openai'`
- `status='active'`
- `schedulable=true`
- `concurrency=10`

### 3. 调度缓存 / outbox

后端存在 `SchedulerSnapshotService`，账号分组绑定通过仓储方法会写 `scheduler_outbox`。因此推荐通过后端管理接口/仓储路径绑定。

如果直接 SQL 写 `account_groups`，还需要二选一：

1. 同步写 `scheduler_outbox` 的 group/account 变更事件，等待调度快照刷新。
2. 或重启 `sub2api-candidate`，让调度快照初始重建。

只插 `account_groups` 而不刷新调度快照，可能出现数据库已绑定但请求仍短时间 `no available accounts`。

### 4. 后端逻辑

当前不需要改后端业务逻辑。

原因：

- `/purchase` 的套餐来自 `subscription_plans`。
- 订单履约使用 `subscription_plans.group_id` 给用户发订阅。
- API 请求调度使用 `api_keys.group_id -> account_groups -> accounts`。
- 这条链路已经存在；缺的是运行态数据和绑定关系。

只有在希望“以后新增套餐自动复制 49 元套餐上游绑定”时，才需要新增后端/脚本能力。当前不建议把环境特定的 `accounts.id=1` 写进通用迁移，因为不同环境账号 ID 不可靠。

### 5. 前端

购买页不需要改，因为它从后端 `/api/v1/payment/plans` 读在售套餐。

可选修改：

- `frontend/src/views/user/UsageGuideView.vue` 当前文案仍写 `29/39/59/99 元套餐已支持生图和图生图`，如果 79 上线公网，应改成 `29/39/59/79/99 元套餐...`。
- 对应测试 `frontend/src/views/user/__tests__/UsageGuideView.spec.ts` 也要同步。

## 推荐执行顺序

1. 备份公网 18084 数据库。
2. 在 18084 应用 156/157 或执行等价 SQL，补齐 79 元套餐。
3. 使用后端管理接口/仓储路径绑定 79/99 group 到 `cliproxy-local-openai`；如果走直接 SQL，则同步刷新调度快照。
4. 验证 `account_groups` 中 69/89 两个 group 都有绑定。
5. 用 `2864533153@qq.com` 的 99 元 Key 重新请求 `gpt-5.4-mini`，确认 HTTP 200、`usage_logs` 和订阅用量正常增长。
6. 如果要检查 79 元链路，新建或选择 79 元订阅用户生成 Key 后请求同一模型验证。

## 结论

必须改的是公网运行态数据库：

- 补 `79 元订阅池 / codex-pool-69-usd`
- 绑定 `codex-pool-69-usd` 和 `codex-pool-89-usd` 到 `cliproxy-local-openai`
- 刷新调度快照

暂时不需要改后端代码；前端只需要可选更新 usage guide 文案。
