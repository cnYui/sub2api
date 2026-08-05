# 余额套餐生命周期规则与 `/subscriptions` 修复

## 需求口径

- `/purchase` 的余额套餐按套餐档位（如 ¥29、¥39、¥49、¥299）归属用户。
- 每个套餐周期固定 28 天，每 7 天到账一次，共 4 次；页面进度分母必须是 4。
- `/subscriptions` 只展示当前用户未退款、未过期的最新有效余额套餐；退款和失效记录保留在数据库中用于审计，但不进入用户展示接口。
- 用户同时只能有一个有效余额套餐。同档购买允许续费，续费只在当前到期日上增加 28 天，不增加刷新次数、不重置 `credited_count` 和刷新计划；异档购买必须先退款。

## 实现

- `BalancePackageService.ListUserPackages` 在服务端过滤 `active`/`completed` 且 `expires_at > now()` 的记录，并限制返回最新一条。
- 创建订单时先做普通校验，事务内再锁定用户行并复核当前有效套餐；异档返回 `BALANCE_PACKAGE_ACTIVE`。履约回调使用同一锁和同一规则，同档订单更新原套餐到期日并写入续费审计。
- 199 号迁移将历史订单快照和用户套餐生命周期统一到 28 天、7 天、4 次，并将同一用户较旧的非退款记录标为 `expired`；未来 18080 迁移脚本也固定使用该标准。
- 购买页将服务端错误码映射为中英文“已有有效套餐，请先退款后购买其他套餐”提示。

## 验证

- 18082 生产数据库：套餐总数 114，当前有效套餐 62；当前有效记录中 `refresh_count`、`refresh_interval_days`、`expires_at - starts_at` 异常均为 0；重复有效用户为 0。
- `schema_migrations` 已记录 `199_normalize_balance_package_lifecycle.sql`。
- `go test ./internal/service -run 'BalancePackage|PaymentBalancePackage|PaymentOrder' -count=1` 通过。
- `go test ./internal/handler ./internal/server/routes -count=1` 通过。
- `pnpm typecheck` 通过；`pnpm vitest run src/api/__tests__/payment.spec.ts` 的 3 个测试通过。

## 约束与后续

续费会将 `user_balance_packages.payment_order_id` 更新为最新订单，以便当前套餐退款可以撤销该套餐；旧续费订单仍保留，但没有独立的套餐关联。若未来要求每一笔续费都能独立报价和退款，应新增订单到套餐的映射表或关联字段，并保持审计链路。
