# 购买页 1% 手续费展示与订单校验

时间：2026-08-07 13:10:00（Asia/Tokyo）

## 需求口径

- 余额套餐和流量卡均按 `RECHARGE_FEE_RATE=1` 收取手续费。
- 手续费只增加用户实际支付金额；套餐到账额度和流量卡额度仍按商品标价履约。

## 实现

- 新增 `frontend/src/components/payment/paymentAmount.ts`，将 CNY 手续费统一按分向上取整。
- `PurchaseShopView.vue` 的余额套餐和流量卡目录卡改为展示实付价，并列出标价与手续费；确认页改用同一金额工具函数。
- 服务端原有 `PaymentService.CreateOrder` 已使用商品服务端价格和 `RECHARGE_FEE_RATE` 计算 `pay_amount`。新增回归测试，锁定余额套餐 ¥29 实付 ¥29.29、流量卡 ¥2 实付 ¥2.02。
- 前端继续只提交商品标价，禁止信任前端实付金额；订单 `amount` 保留商品标价，`pay_amount` 保留含手续费金额。

## 验证

- `pnpm vitest run src/components/payment/__tests__/paymentAmount.spec.ts`：3/3 通过。
- `go test ./internal/service -run 'TestCalculateCreateOrderPayAmountFor(BalancePackage|TrafficPack|Subscription)'`：通过。
- `pnpm typecheck`：通过。
- `pnpm build`：通过；仅有既有的 Browserslist、分包和大 chunk 警告。
- `pnpm test:run` 未全绿，且问题与本次改动无关：`HomeView.compact.spec.ts` 断言过期站点名；GroupsView 测试 mock 缺少 `adminAPI.groups.getLiveCapability`，产生未处理拒绝。购买相关 `PaymentView.spec.ts` 和新增金额工具测试均通过。
