# 单用户单生效订阅购买拦截修复结果

时间：2026-07-08 17:37 JST

## 修复内容

- 后端新增错误码 `ACTIVE_SUBSCRIPTION_EXISTS`，message 固定为 `需要先和管理员联系来进行退款`。
- 后端 `PaymentService.validateSubOrder()` 在订阅订单校验阶段检查用户是否已有 active 且未过期订阅；如已有任意 active 订阅，直接拒绝新订阅购买。
- 余额支付订阅路径 `BalancePayOrder()` 已补传 `UserID` 到订阅校验，避免绕过外部支付路径的重复订阅拦截。
- 前端购买页点击订阅卡片、弹窗选择套餐、确认订阅支付前都会检查 active 订阅并弹出同一句提示。
- 前端 `payment.errors.ACTIVE_SUBSCRIPTION_EXISTS` 已加入中文和英文映射；中文为用户指定原句。

## TDD 证据

先写失败测试并确认红灯：

```bash
cd backend
go test -count=1 -tags=unit ./internal/service -run 'TestValidateSubOrderRejectsExistingActiveSubscriptionAcrossGroups|TestBalancePaySubscriptionWithExistingActiveSubscriptionDoesNotCreateOrderOrDeduct'
```

红灯结果：两个测试均因当前代码返回 nil 失败，说明旧逻辑仍允许重复订阅。

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/paymentUx.spec.ts
```

红灯结果：点击订阅卡片没有弹 `payment.errors.ACTIVE_SUBSCRIPTION_EXISTS`，且中文文案映射不存在。

## 通过验证

```bash
cd backend
go test -count=1 -tags=unit ./internal/service -run 'TestValidateSubOrderRejectsExistingActiveSubscriptionAcrossGroups|TestBalancePaySubscriptionWithExistingActiveSubscriptionDoesNotCreateOrderOrDeduct'
```

通过：`ok github.com/Wei-Shaw/sub2api/internal/service 0.843s`

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/paymentUx.spec.ts
```

通过：`2 passed / 41 tests passed`

```bash
cd backend
go test -count=1 -tags=unit ./internal/service
```

通过：`ok github.com/Wei-Shaw/sub2api/internal/service 87.368s`

```bash
cd frontend
pnpm typecheck
```

通过：`vue-tsc --noEmit`

```bash
cd backend
go test -count=1 ./cmd/server
```

通过：`ok github.com/Wei-Shaw/sub2api/cmd/server 0.529s`

```bash
git diff --check
```

通过：无输出。

## 3876129758@qq.com 退款参考

- 29 元订阅 `user_subscriptions.id=76` 开始：`2026-07-05 14:59:28.328526` 东八区。
- 59 元订阅订单 `payment_orders.id=143` 完成：`2026-07-08 15:53:48.749760` 东八区。
- 从 29 元订阅开始到 59 元订阅完成：`3 天 00:54:20`，约 `3.04` 天。
- 从 29 元订阅开始到该旧订阅最后一次 API 使用：`2 天 23:51:08`，约 `2.99` 天。
- 旧订阅累计 `usage_logs=480` 条，内部 USD 用量成本 `67.0562527500`。

## 未执行事项

- 未对公网数据库做退款、软删除旧订阅或状态修正。
- 未构建镜像、未部署公网。
- 未提交 git。
