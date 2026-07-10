# 管理员撤销订阅后二次购买修复设计与计划

## 背景

管理员在 Subscriptions 页面撤销用户套餐后，用户仍可能无法二次购买。当前业务约束是“一个用户当前只能存在一个套餐”，但管理员撤销后应解除这个约束。

## 根因

1. 后端管理员撤销入口 `DELETE /api/v1/admin/subscriptions/:id` 调用 `SubscriptionService.RevokeSubscription()`。
2. 当前实现只调用 `userSubRepo.Delete()`；该仓库走 Ent `SoftDeleteMixin`，实际只写入 `deleted_at=now()`，不会把 `status` 从 `active` 改为 `expired`。
3. 数据库因此会留下 `deleted_at IS NOT NULL AND status='active' AND expires_at > now()` 的记录。Ent 普通查询默认会过滤软删除，但数据库事实语义不完整，且任何绕过软删除或直接 SQL 的活跃判断都可能误判。
4. 用户购买页还有 60 秒活跃订阅缓存；管理员在另一个页面撤销后，用户页可能仍拿旧 `activeSubscriptions` 本地拦截，不发起新购买请求。

## 目标行为

- 管理员撤销订阅必须真实写库：
  - `status='expired'`
  - `deleted_at=now()`
  - `updated_at=now()`
- 撤销后用户的 active subscription 查询和购买保护都不应命中旧套餐。
- 用户购买页在执行本地“已有套餐”拦截前，强制刷新一次活跃订阅，避免旧缓存误拦。

## 方案

推荐方案：后端撤销语义修正 + 前端刷新拦截。

- 后端在 `RevokeSubscription()` 中先调用 `UpdateStatus(..., expired)`，再调用 `Delete()` 软删除。
- 继续保留既有缓存失效逻辑。
- 前端把购买页 `showActiveSubscriptionPurchaseBlocked()` 改为异步刷新检查：
  - 先 `fetchActiveSubscriptions(true)`。
  - 刷新失败时不因网络错误直接放行 stale active；仍按当前 store 状态兜底，由后端最终校验。
  - 刷新后如果仍有 active，再显示 `ACTIVE_SUBSCRIPTION_EXISTS`。
- 选择套餐、续费弹窗选择套餐、确认订阅前都走同一异步检查。

## TDD 计划

1. 后端服务测试：构造 fake repo，调用 `RevokeSubscription()`，断言先写 `UpdateStatus(expired)` 再 `Delete()`。
2. 前端测试：构造 stale active subscription，`fetchActiveSubscriptions(true)` 清空缓存；点击订阅卡后应允许打开支付确认，不显示拦截错误。
3. 实现后运行目标测试，再运行相关包测试与 `git diff --check`。

## 不做

- 不改退款订单状态。
- 不删除 usage。
- 不改变“一用户只能存在一个当前套餐”的业务约束。
- 不改套餐购买后赋权逻辑。
