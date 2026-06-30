# 29/39/59 元套餐上游绑定设计核对

时间：2026-06-30 22:48 JST

## 问题

确认当前 29/39/59 元套餐是如何设计和绑定上游的，以判断 79/99 是否应该在迁移里自动绑定上游。

## 代码与迁移设计

现有迁移刻意只 seed 套餐和 group，不 seed 上游绑定：

- `154_seed_codex_99_subscription_plan.sql` 注释写明“只 seed 分组和商品计划，不复制或绑定上游账号”。
- `155_seed_codex_subscription_plans_baseline.sql` 注释写明“只补齐分组和订阅计划，不复制或绑定上游账号”。
- `156_seed_codex_79_subscription_plan.sql` 注释写明“只 seed 分组和商品计划，不复制或绑定上游账号”。
- 回归测试显式断言 154/155/156/157 不包含 `INSERT INTO account_groups` 和 `UPDATE account_groups`。

这说明项目当前设计是：

- `subscription_plans`：售卖商品。
- `groups`：套餐对应的额度、平台、模型配置。
- `account_groups`：运行态上游账号绑定。

三者不是同一个职责。

## 公网 18084 运行态事实

当前 29/39/59 的绑定如下：

| 套餐 | group | 上游账号 | 绑定时间 |
| --- | --- | --- | --- |
| 29 元订阅池 | `codex-pool-19-usd` | `cliproxy-local-openai` | `2026-06-18 07:21:42+08` |
| 39 元订阅池 | `codex-pool-29-usd` | `cliproxy-local-openai` | `2026-06-19 10:29:38+08` |
| 59 元订阅池 | `codex-pool-49-usd` | `cliproxy-local-openai` | `2026-06-19 10:29:38+08` |

这些绑定存在于 `account_groups` 表，优先级为 `1`。

## 后端管理路径

后端已有管理路径支持复制/绑定账号到 group：

- `adminService.CreateGroup` 支持创建 group 时复制源 group 的账号。
- `adminService.UpdateGroup` 支持更新 group 时按源 group 重新绑定账号。
- `groupRepository.BindAccountsToGroup` 会写 `account_groups`，并写 `scheduler_outbox` 触发调度快照刷新。

因此，项目内推荐路径不是把具体账号 ID 写进通用迁移，而是通过后台/运维路径配置运行态绑定。

## 结论

29/39/59 当前能通，不是因为源码迁移自动把它们绑定到上游，而是因为公网运行态已经有 `account_groups` 绑定。

79/99 应该沿用同一设计：

- 79/99 套餐和 group 由迁移/运行态补齐。
- 79/99 的上游账号绑定由运行态配置完成。
- 推荐通过后端管理路径或安全运维脚本写 `account_groups` 并刷新 `scheduler_outbox`。

不建议把 `cliproxy-local-openai` 或 `accounts.id=1` 写死进通用迁移，因为不同环境的账号名称和 ID 都可能不同。
