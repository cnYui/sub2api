# 20260722 公共 Codex 周额度下调实施计划

## 背景

公共 Codex 订阅已经切换为 28 天有效期、每 7 天按订阅锚点刷新周额度。本轮只调整固定周额度金额，不改变 28 天周期、滚动周窗口、尾窗比例折算、订单快照和额度耗尽 429/暂停语义。

## 新额度

| 分组 | 周额度 USD | 28 天总额度 USD |
|---|---:|---:|
| codex-pool-19-usd | 58 | 232 |
| codex-pool-29-usd | 78 | 312 |
| codex-pool-49-usd | 118 | 472 |
| codex-pool-69-usd | 158 | 632 |
| codex-pool-89-usd | 198 | 792 |
| codex-pool-135-usd | 299 | 1196 |
| codex-pool-179-usd | 400 | 1600 |

## 实施范围

1. 前端公共 Codex 套餐、购买页、订阅页、首页和管理端展示统一读取新周额度。
2. 后端公共 Codex 分组归一化、套餐 API、订单快照、cutover 工具和测试统一使用新额度。
3. 本地数据库执行前备份并验证备份可读，再更新 groups、subscription_plans 和当前权益段周额度/周期总额度；不自动改历史已完成订单金额。
4. 额度耗尽自动暂停沿用现有周窗口限额判断：用量达到当前有效周额度后返回 429，并由现有链路阻止继续消费。

## 验证

- 前端：`pnpm test:run`、`pnpm typecheck`、`pnpm lint:check`、`pnpm build`。
- 后端：覆盖受影响的 Go 测试，至少包括 payment config、fulfillment、subscription quota 与 migration/cutover 相关测试。
- 本地运行态：更新数据库后重启 `sub2api-dev`，验证 `/health` 和本地套餐/订阅接口展示新额度。
