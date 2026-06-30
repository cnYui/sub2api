# 79/99 元套餐上游绑定本地代码设计

时间：2026-07-01 07:50 JST

## 背景

公网 18084 当前 99 元套餐对应 `codex-pool-89-usd`，但没有上游绑定，导致 `2864533153@qq.com` 的 API Key 请求在账号选择阶段失败：`no available accounts`。本地 main-preview 已有 79 元套餐 `codex-pool-69-usd`，但 79 和 99 对应 group 也都没有绑定上游。

现有 29/39/59 元套餐的设计是：

- 迁移只创建/更新 `subscription_plans` 和 `groups`。
- 上游账号绑定保存在运行态表 `account_groups`。
- 后端仓储绑定路径会写 `scheduler_outbox`，触发调度快照刷新。
- 迁移回归测试明确要求 154/155/156/157 不包含 `INSERT INTO account_groups` / `UPDATE account_groups`。

因此，本次本地代码改动应沿用同一职责划分，不把具体账号 ID 或账号名硬塞进通用迁移。

## 目标

新增一个可复用运行态脚本，用于将 Codex 高档订阅 group 绑定到指定 OpenAI 上游账号：

- 默认目标 group：`codex-pool-69-usd`、`codex-pool-89-usd`
- 默认源账号：`cliproxy-local-openai`
- 默认 PostgreSQL 容器：`sub2api-candidate-postgres`
- 默认 dry-run，不写数据库
- `--apply` 后才写入

同时把用户使用说明中套餐文案从 `29/39/59/99` 更新为 `29/39/59/79/99`。

## 非目标

- 不改后端 API 路由或计费逻辑。
- 不改 154/155/156/157 迁移的“无账号绑定”设计。
- 不写死 `accounts.id=1`。
- 不在脚本输出里打印任何 API Key 或账号 credentials。
- 不直接在本轮写公网 18084 运行态数据库，除非用户后续明确要求执行。

## 设计

新增 `scripts/bind-codex-subscription-upstreams.mjs`。

### 输入

CLI 参数：

- `--apply`：执行写入；默认 dry-run。
- `--dry-run`：只输出摘要和 SQL，不写入。
- `--pg-container=NAME`：PostgreSQL 容器名，默认 `sub2api-candidate-postgres`。
- `--account-name=NAME`：目标上游账号名，默认 `cliproxy-local-openai`。
- `--group=NAME`：可重复传入，覆盖默认 group 列表。

### SQL 行为

脚本按名称解析账号和 group：

1. 查找 `accounts.name = account_name`、`deleted_at IS NULL`。
2. 要求账号 `platform='openai'`、`status='active'`、`schedulable=true`。
3. 查找每个目标 group，要求 `platform='openai'`、`status='active'`、`deleted_at IS NULL`。
4. 对每个 group 写入：
   - `account_groups(account_id, group_id, priority, created_at)`
   - `priority = 1`
   - `ON CONFLICT (account_id, group_id) DO UPDATE SET priority = LEAST(account_groups.priority, EXCLUDED.priority)`
5. 写入 `scheduler_outbox`，事件类型使用 `group_changed`，每个 group 一条，带去重 key，避免调度快照不刷新。

如果账号或 group 缺失，SQL 应失败并回滚。

### 文案修改

修改：

- `frontend/src/views/user/UsageGuideView.vue`
- `frontend/src/views/user/__tests__/UsageGuideView.spec.ts`

文案从 `29/39/59/99 元套餐已支持生图和图生图` 改为 `29/39/59/79/99 元套餐已支持生图和图生图`。

## 验证

脚本测试：

- 新增 `scripts/__tests__/bind-codex-subscription-upstreams.test.mjs`
- 用 `node --test` 覆盖：
  - 默认 dry-run 不写入。
  - 默认 SQL 包含 69/89 两个 group。
  - SQL 按名称查 account/group，不写死账号 ID。
  - SQL 写 `account_groups` 和 `scheduler_outbox`。
  - `--group` 可覆盖默认 group。

前端测试：

- 更新 `frontend/src/views/user/__tests__/UsageGuideView.spec.ts` 断言。
- 运行 `pnpm --dir frontend vitest run src/views/user/__tests__/UsageGuideView.spec.ts`。

本轮不执行 `--apply`。真正发布时另走运行态执行和公网验证。
