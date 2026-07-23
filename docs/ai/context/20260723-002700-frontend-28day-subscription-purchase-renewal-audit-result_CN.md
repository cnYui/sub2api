# 前端 28 天订阅购买与续费只读审计结果

## 结论

- 新用户购买公开 Codex 订阅时，前端只提交 `plan_id`；后端锁定套餐并写入不可变订单快照。公开 Codex 快照强制为 `28` 天、`7` 天周窗口和四周总额度。
- 支付履约时，新订阅从履约时刻起算，到期时间为起点加 28 个日历日；同一套餐在未到期时提前续费，从原到期时间再加 28 天，不覆盖剩余有效期。
- 对已经以 28 天且到期点在 7 天窗口边界上的订阅，续费后有效期和周额度均正常连续：28 天正好是 4 个完整周窗口。
- 不能把“所有历史用户提前续费后的周额度也完全正确”作为结论。当前运行库中仍有大量旧权益段不是 28 天，且到期点未对齐 7 天窗口；它们提前续费虽然会正确增加 28 天有效期，但周窗口继续沿用旧锚点，首周会跨越旧、新权益段，末周成为短尾，导致旧周期未用额度可能带入新周期，或新周期额度被旧周期用量抵扣。

## 当前运行库只读核验

- 对象：`sub2api-postgres-dev / sub2api`，公网本地入口实际指向 `sub2api-dev:18080`。
- 7 个在售公开 Codex 套餐均为 `validity_days=28`，周额度分别为 58、78、118、158、198、299、400 USD。
- 已完成且订单快照为周额度的订单有 5 笔；5 笔均存在支付订单来源的权益段，`period_days=28` 且 `expires_at = starts_at + 28 days`，无缺失。
- 65 个当前有效的公开 Codex 订阅均已关联当前权益段，但只有 6 个权益段为 28 天；其余包括 51 个 30 天权益段以及 8、60、61、120、150、210 天权益段。
- 65 个有效订阅中，11 个到期点与周锚点对齐，54 个有 1 至 6 天偏移。当前库尚不存在可用于验证“新周额度套餐提前续费”的已完成连续支付权益段样本。

## 代码路径

- 前端 `frontend/src/views/user/PaymentView.vue` 展示 28 天与每周额度，创建订单只发送套餐 ID。
- `backend/internal/service/subscription_order_snapshot.go` 为公开 Codex 订单固定 28 天、周窗口 7 天和四周总额度。
- `backend/internal/service/subscription_entitlement_period.go` 在新购时使用当前时间起算；未到期续费使用原订阅到期时间起算，并创建独立权益段。
- `backend/internal/service/subscription_weekly_window.go` 以 `weekly_anchor_at` 推进周窗口。提前续费的权益段未在此路径更新锚点；当旧到期点不是 7 的倍数时，窗口会跨权益段。

## 验证

- 后端：`go test ./internal/service -run 'Test(ConfirmPaymentFulfillsPublicCodexSubscriptionFromOrderSnapshotWhenGroupDisabled|CalculateSubscriptionWeeklyWindow_NewPurchaseHasFourFullWindows|AssignSubscription_WithEntitlementSource_ContinuesFromExistingExpiry)$'` 通过。
- 前端：`pnpm test:run src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/SubscriptionsView.spec.ts src/utils/__tests__/subscriptionQuota.spec.ts` 通过，3 个文件、49 个测试。

## 后续建议

- 修复前应避免把历史未对齐订阅的提前续费当作已验证正常路径。
- 修复应使周窗口以每个权益段的 `starts_at` 为锚点，或在兼容迁移中把历史权益段切到 7 天边界；同时补充“30 天历史订阅提前续费为 28 天”的端到端测试，验证四个完整周额度、权益段之间不传递用量。
