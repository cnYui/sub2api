# Dashboard 套餐权益周期 Task 3 结果

## 背景

本轮继续执行 `docs/ai/context/20260716-155243-dashboard-subscription-quota-realtime-implementation-plan_CN.md` 的 Task 3，只接入权益周期发放来源与撤权边界，不实现 Dashboard quota 读模型、前端展示或轮询。

## 已完成

- 套餐支付、余额支付、混合支付履约统一使用 `payment_order:<payment_orders.id>` 作为权益周期 source；同一支付订单重放不会重复续期。
- 正天数订阅兑换码使用 `redeem_code:<redeem_codes.id>` source；兑换码事务复用同一 Ent Tx。
- 注册默认套餐和管理后台创建用户默认套餐使用 `signup_default:<user_id>:<group_id>` source。
- OAuth 首次绑定默认套餐读取 `user_provider_default_grants.id`，使用 `provider_default:<grant_id>:<group_id>` source，避免 provider 默认发放重复。
- 管理员首次手动分配新订阅时写 `admin_assignment:<user_id>:<group_id>:<validity_days>:<notes_sha256>` period；复用已有订阅不伪造 period，语义冲突仍失败。
- 管理员正向延长订阅追加 `admin_adjustment:<subscription_id>:<new_expires_at>` period，从旧 `expires_at` 开始接续；过期订阅正向恢复时从当前时间开始。
- 管理员负向调整撤销该订阅尚未过期的 active period，reason 为 `admin_adjustment_negative`，不截短历史行。
- 负天数订阅兑换码撤销该订阅尚未过期的 active period，reason 为 `redeem_negative_adjustment`，不截短历史行。
- 退款撤权只按当前订单的 `payment_order:<order_id>` source 撤销 period，reason 为 `payment_refund`，保留共享订阅与期限变化人工审核保护。

## 关键取舍

- 无 source 的旧 `AssignSubscription` / `AssignOrExtendSubscription` 调用仍保持旧行为，不自动创建 period，避免把历史或未定义来源伪装为精确权益周期。
- 后台负向调整与负天数兑换码会破坏精确周期边界，因此只撤销未过期精确周期，后续 Dashboard 读模型应降级为 legacy 口径。
- 余额支付与混合支付没有单独 source 类型，因为它们本质上都是同一商品订单履约；source 稳定锚定 `payment_orders.id`。

## 验证

已通过：

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'Test(ConfirmPaymentCompletesSubscriptionWhenAmountMatches|BalancePaySubscriptionRenewsSameActiveSubscription|BalancePaySubscriptionDeductsPayAmountAndCompletesOrder|RequestRefund|AssignSubscription|RevokeSubscription|AuthService.*Subscription|Redeem.*Entitlement|RedeemNegativeSubscriptionRevokesUnexpiredEntitlementPeriods|ExtendSubscription_(PositiveAdjustmentAppendsAdminAdjustmentEntitlementPeriod|NegativeAdjustmentRevokesUnexpiredEntitlementPeriods)|BulkAssignSubscriptionCreatedReusedAndConflict|AdminService_CreateUser_AssignsDefaultSubscriptions)'
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service
GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/server
git diff --check
```

## 未做

- 未实现 Dashboard quota read model、`/usage/dashboard/quota`、前端展示或 15 秒轮询。
- 未改生产数据库、Redis、Nginx、容器或运行态。
- 未部署、未调用真实支付或真实退款。
