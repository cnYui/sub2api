# 支付宝与余额组合支付实施计划

> **供执行 Agent 使用：** 必须逐项采用 TDD。每一项先写失败测试，确认其因目标能力缺失而失败，再写最小实现并复跑；不跳过红灯验证。

**目标：** 让套餐和流量包以一笔真实商品订单完成“可用余额冻结 + 支付宝精确差额”支付，并在取消、超时、迟到回调和退款中保持资金与权益幂等一致。

**架构：** `payment_orders` 保存订单资金拆分与外部支付解析状态；新增 `payment_balance_holds` 保存余额占用生命周期。订单创建事务原子扣减可用余额并创建占用；支付宝初始化由订单状态与短租约串行化。`PAID/UNPAID/UNKNOWN` 决定是否履约或释放，`UNKNOWN` 只延长到订单到期后的 5 分钟，迟到成功款补入余额而不恢复旧权益。

**技术栈：** Go、Ent/PostgreSQL、shopspring/decimal、Gin、Vitest/Vue 3、现有 EasyPay/支付宝 Provider 与支付审计。

---

## 范围与不变量

- 仅支持 `payment_type=alipay` 的余额组合支付；余额充值、微信、Stripe、Airwallex 继续原行为。
- `amount` 为商品不含手续费本金，`pay_amount` 为含手续费商品总价，且 `pay_amount = balance_amount + gateway_amount`。
- Webhook/主动查询的成功金额必须等于 `gateway_amount`；支付宝退款金额必须是首次退款持久化的 `refund_gateway_amount`。
- `users.balance` 只代表可用余额。占用记录只能从 `RESERVED` 变为 `CAPTURED` 或 `RELEASED`。
- 余额已释放的订单即使收到迟到付款，也不重新扣余额、不发旧商品，而是把 `gateway_amount` 唯一地补入余额并变为 `COMPENSATED`。
- 订单到期为 30 分钟；只有查询结果 `UNKNOWN` 才最多确认至 `expires_at + 5m`。

### Task 1：固定金额计算与订单资金模型

**文件：**

- 新增：`backend/internal/service/payment_hybrid_amounts.go`
- 新增：`backend/internal/service/payment_hybrid_amounts_test.go`
- 修改：`backend/internal/service/payment_service.go`
- 修改：`backend/internal/payment/types.go`

- [ ] **Step 1：先写金额拆分失败测试**

```go
func TestCalculateHybridFunding_UsesBalanceBeforeAlipay(t *testing.T) {
    got, err := calculateHybridFunding(decimal.RequireFromString("79.00"), decimal.RequireFromString("0.79"), decimal.RequireFromString("6.32"))
    require.NoError(t, err)
    require.Equal(t, "79.79", got.PayAmount.StringFixed(2))
    require.Equal(t, "6.32", got.BalanceAmount.StringFixed(2))
    require.Equal(t, "73.47", got.GatewayAmount.StringFixed(2))
    require.Equal(t, "6.32", got.BalancePrincipal.StringFixed(2))
    require.Equal(t, "72.68", got.GatewayPrincipal.StringFixed(2))
}

func TestCalculateHybridFunding_RejectsNonCentClientExpectation(t *testing.T) {
    _, err := validateHybridCheckoutExpectation("79.79", "6.321", decimal.RequireFromString("79.79"), decimal.RequireFromString("6.32"))
    require.ErrorIs(t, err, errCheckoutChanged)
}
```

- [ ] **Step 2：运行测试，确认因函数不存在而失败**

运行：`GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestCalculateHybridFunding|TestCalculateHybridFunding_Rejects'`

预期：编译失败，提示 `calculateHybridFunding` 与 `validateHybridCheckoutExpectation` 未定义。

- [ ] **Step 3：以 decimal 实现最小资金拆分**

```go
type hybridFunding struct {
    PayAmount, BalanceAmount, GatewayAmount decimal.Decimal
    BalancePrincipal, GatewayPrincipal decimal.Decimal
}

func calculateHybridFunding(principal, fee, available decimal.Decimal) (hybridFunding, error) {
    pay := principal.Add(fee).Round(2)
    balance := decimal.Min(available.Round(2), pay)
    if !balance.GreaterThan(decimal.Zero) || !balance.LessThan(pay) {
        return hybridFunding{}, errNotHybridFunding
    }
    return hybridFunding{
        PayAmount: pay, BalanceAmount: balance, GatewayAmount: pay.Sub(balance),
        BalancePrincipal: decimal.Min(balance, principal),
        GatewayPrincipal: principal.Sub(decimal.Min(balance, principal)),
    }, nil
}
```

将 `CreateOrderRequest` 扩展为 `UseBalance`、`ExpectedPayAmount`、`ExpectedBalanceAmount`；只接受字符串形式的客户前置条件。后端从商品价格、手续费和当前余额重新计算，任何差异返回 `CHECKOUT_CHANGED`，不静默增加支付宝金额。

- [ ] **Step 4：运行金额测试并确认通过**

运行：`GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestCalculateHybridFunding|TestCalculateHybridFunding_Rejects'`

预期：PASS。

### Task 2：持久化资金拆分与余额占用

**文件：**

- 修改：`backend/ent/schema/payment_order.go`
- 新增：`backend/ent/schema/payment_balance_hold.go`
- 修改：`backend/ent/generate.go`
- 重新生成：`backend/ent/*.go`
- 新增：`backend/migrations/163_alipay_balance_hybrid_payment.sql`
- 修改：`backend/internal/repository/migrations_schema_integration_test.go`
- 新增：`backend/internal/service/payment_balance_hold_test.go`

- [ ] **Step 1：先写 schema 和并发占用失败测试**

```go
func TestReserveBalanceForHybridOrder_OnlyOneConcurrentReservationSucceeds(t *testing.T) {
    // 用户余额 10.00，同时创建两笔各冻结 8.00 的订单。
    // 断言仅一笔创建 RESERVED hold，用户最终可用余额为 2.00。
}

func TestPaymentBalanceHold_TransitionsOnlyFromReserved(t *testing.T) {
    // CAPTURED 与 RELEASED 互斥；第二次转换影响行数必须为 0。
}
```

- [ ] **Step 2：运行测试，确认新实体和字段缺失**

运行：`GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestReserveBalanceForHybridOrder|TestPaymentBalanceHold'`

预期：编译失败，提示 `PaymentBalanceHold`、资金字段或 reservation helper 不存在。

- [ ] **Step 3：扩展 Ent schema、生成代码并添加兼容迁移**

订单字段新增：`funding_mode`、`balance_amount`、`gateway_amount`、`provider_init_status`、`provider_init_attempted_at`、`provider_init_lease_until`、`payment_resolution_status`、`payment_resolution_deadline`、`cancel_requested_at`、`compensation_amount`、`compensated_at`、`refund_balance_amount`、`refund_gateway_amount`、`refund_balance_status`。`payment_balance_holds` 以 `order_id UNIQUE` 约束唯一占用，含 `user_id`、`amount`、`status`、`expires_at`、capture/release 时间与原因。

迁移必须：

```sql
UPDATE payment_orders
SET funding_mode = CASE WHEN payment_type = 'balance' THEN 'balance' ELSE 'gateway' END,
    balance_amount = CASE WHEN payment_type = 'balance' THEN pay_amount ELSE 0 END,
    gateway_amount = CASE WHEN payment_type = 'balance' THEN 0 ELSE pay_amount END
WHERE funding_mode IS NULL OR funding_mode = '';
```

历史订单不得创建 hold；现有 `PENDING` 订单继续按纯网关处理。运行 `cd backend && go generate ./ent` 后检查生成文件和 migration schema integration 覆盖新列、索引与唯一约束。

- [ ] **Step 4：运行 schema、占用和迁移测试**

运行：`GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestReserveBalanceForHybridOrder|TestPaymentBalanceHold'`

运行：`GOMAXPROCS=2 go test -p=1 -count=1 ./internal/repository -run TestMigrationsSchema`

预期：PASS；第二条需要本地 PostgreSQL 测试环境。

### Task 3：创建订单事务和 Provider 初始化租约

**文件：**

- 修改：`backend/internal/service/payment_order.go`
- 新增：`backend/internal/service/payment_hybrid_order.go`
- 新增：`backend/internal/service/payment_hybrid_order_test.go`
- 修改：`backend/internal/service/payment_order_provider_snapshot.go`
- 修改：`backend/internal/service/payment_service.go`

- [ ] **Step 1：先写创建订单和租约抢占失败测试**

```go
func TestCreateHybridOrder_ReservesBalanceAndSendsOnlyGatewayDifference(t *testing.T) {
    // 用 6.32 元余额购买 79.00 元、1% 手续费的套餐。
    // 断言订单为 mixed，hold=RESERVED 6.32，Provider 请求金额为 "73.47"。
}

func TestClaimProviderInitialization_AllowsOnlyOneCaller(t *testing.T) {
    // 两个 goroutine 竞争同一订单；仅一个拿到 CREATING 租约。
}

func TestCreateHybridOrder_CheckoutChangedDoesNotReserveBalance(t *testing.T) {
    // 前端预期余额与数据库余额不一致时返回 CHECKOUT_CHANGED，余额和 hold 均不变。
}
```

- [ ] **Step 2：运行测试，确认混合订单尚未创建**

运行：`GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestCreateHybridOrder|TestClaimProviderInitialization'`

预期：FAIL，当前创建订单只保存 `pay_amount` 且 Provider 收整单金额。

- [ ] **Step 3：实现事务创建与外部初始化状态机**

在 `createOrderInTx` 中，只有 `UseBalance=true` 且余额不足覆盖总价时创建 `mixed`：使用 `tx.User.Update().Where(user.IDEQ(req.UserID), user.BalanceGTE(balanceAmount)).AddBalance(-balanceAmount)`；影响行数不是 1 时返回 `CHECKOUT_CHANGED`。同一事务创建订单、`RESERVED` hold（`expires_at + 5m`）和 `ORDER_CREATED`/`BALANCE_RESERVED` 审计。

调用 Provider 前以条件更新抢占 `NOT_STARTED|可重试状态 -> CREATING` 并设置短租约。抢占者使用同一个 `out_trade_no` 和 `gateway_amount`；成功写 `CREATED/PENDING`，明确未创建失败时在事务中 `RELEASED`，网络未知写 `UNKNOWN` 并不释放。租约过期的后台接管者必须先查询 Provider，再决定是否用同一订单号续建。

- [ ] **Step 4：运行订单创建与既有订单测试**

运行：`GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestCreateHybridOrder|TestClaimProviderInitialization|TestCreateOrderInTx|TestBuildPaymentOrderProviderSnapshot'`

预期：PASS。

### Task 4：三态查询、取消与 30+5 分钟过期扫描

**文件：**

- 修改：`backend/internal/service/payment_order_lifecycle.go`
- 修改：`backend/internal/service/payment_order_expiry_service.go`
- 修改：`backend/internal/service/payment_resume_lookup.go`
- 修改：`backend/internal/service/payment_order_lifecycle_test.go`
- 新增：`backend/internal/service/payment_hybrid_lifecycle_test.go`

- [ ] **Step 1：先写 PAID/UNPAID/UNKNOWN 失败测试**

```go
func TestResolveHybridPayment_QueryFailureStaysUnknownAndKeepsHold(t *testing.T) {
    // 到期时 Provider 超时，订单仍 PENDING，resolution=UNKNOWN，hold 不释放。
}

func TestResolveHybridPayment_ExplicitUnpaidReleasesHold(t *testing.T) {
    // Provider 明确关闭/失败，订单 EXPIRED，余额和 hold 同事务恢复为 RELEASED。
}

func TestCancelHybridPayment_UnknownReturnsConfirmationPending(t *testing.T) {
    // 取消查询失败只记录 cancel_requested_at，不能把订单直接取消。
}

func TestResolveHybridPayment_ReleasesUnknownAfterFiveMinuteDeadline(t *testing.T) {
    // expires_at+5m 后仍未知，本地终止并释放；后续付款走补偿。
}
```

- [ ] **Step 2：运行测试，确认当前空字符串语义导致错误释放**

运行：`GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestResolveHybridPayment|TestCancelHybridPayment'`

预期：FAIL；当前 `checkPaidWithOptions` 用空字符串同时表示未支付与查询失败，`cancelCore` 仍会取消订单。

- [ ] **Step 3：引入显式解析枚举并收敛恢复窗口**

```go
type paymentResolution string
const (
    paymentResolutionPaid paymentResolution = "PAID"
    paymentResolutionUnpaid paymentResolution = "UNPAID"
    paymentResolutionUnknown paymentResolution = "UNKNOWN"
)
```

将 Provider 查询、关闭和金额异常映射为三态；只有 `UNPAID` 调用 release。过期扫描每 60 秒查询混合 `PENDING` 订单，超过 30 分钟的 `UNKNOWN` 写入固定 deadline，截止后才释放。提前取消 deadline 为 `min(cancel_requested_at+5m, expires_at+5m)`。普通网关订单保留兼容行为，但 `CANCELLED/EXPIRED` 被 Webhook 恢复的条件必须收敛到该订单的 resolution deadline。

- [ ] **Step 4：运行生命周期全量测试**

运行：`GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestResolveHybridPayment|TestCancelHybridPayment|TestVerifyOrderByOutTradeNo|TestCancelOrder|TestExpire'`

预期：PASS。

### Task 5：Webhook 履约、迟到付款补偿与返利基数

**文件：**

- 修改：`backend/internal/service/payment_webhook_provider.go`
- 修改：`backend/internal/service/payment_fulfillment.go`
- 修改：`backend/internal/service/payment_stats.go`
- 修改：`backend/internal/service/payment_fulfillment_test.go`
- 新增：`backend/internal/service/payment_hybrid_fulfillment_test.go`
- 修改：`backend/internal/handler/payment_webhook_handler_test.go`

- [ ] **Step 1：先写成功、金额不符和迟到回调失败测试**

```go
func TestHybridWebhook_CapturesHoldAndFulfillsExactlyOnce(t *testing.T) {
    // 73.47 的支付宝通知匹配 gateway_amount，RESERVED -> CAPTURED，权益只创建一次。
}

func TestHybridWebhook_RejectsPayAmountInsteadOfGatewayAmount(t *testing.T) {
    // 对 79.79 的通知拒绝履约，不更新余额占用。
}

func TestHybridWebhook_AfterReleasedHoldCreditsGatewayAmountOnce(t *testing.T) {
    // RELEASED 后迟到成功仅余额 +73.47，订单 COMPENSATED，不创建订阅/流量包。
}
```

- [ ] **Step 2：运行测试，确认当前代码把通知金额覆写 pay_amount**

运行：`GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestHybridWebhook|TestToPaid|TestFulfill'`

预期：FAIL；当前 `toPaid` 调用 `SetPayAmount(paid)`，没有 hold 生命周期或 `COMPENSATED`。

- [ ] **Step 3：实现原子履约与补偿**

把 `toPaid` 改为校验/保存 `gateway_amount`，绝不覆盖 `pay_amount`。正常成功处理使用同一 Ent 事务完成：resolution=`PAID`、hold `RESERVED -> CAPTURED`、订单 `PAID/RECHARGING/COMPLETED`、套餐或流量包权益、准确关联 ID 和审计。对已 `RELEASED` hold 的晚到成功，在独立事务 `users.balance += gateway_amount`、写确定性补偿审计、订单 `COMPENSATED`；重复 Webhook 必须只读返回既有结果。组合订单返利基数改用 `gateway_principal`，纯订单不变。

- [ ] **Step 4：运行履约和 Webhook 回归**

运行：`GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service ./internal/handler -run 'TestHybridWebhook|TestFulfill|TestToPaid|TestHandlePaymentNotification'`

预期：PASS。

### Task 6：组合支付退款原路拆分

**文件：**

- 修改：`backend/internal/service/payment_refund.go`
- 修改：`backend/internal/service/payment_refund_state.go`
- 修改：`backend/internal/service/payment_refund_test.go`
- 新增：`backend/internal/service/payment_hybrid_refund_test.go`

- [ ] **Step 1：先写退款固定拆分失败测试**

```go
func TestHybridRefund_PersistsSplitAndDoesNotRefundFee(t *testing.T) {
    // 79 元本金，余额本金 6.32，按既有日期规则退款 63.2：
    // 首次固定 balance/gateway 拆分，余额部分回站内余额，网关只收 refund_gateway_amount。
}

func TestHybridRefund_GatewaySuccessThenLocalRetryDoesNotRefundGatewayTwice(t *testing.T) {
    // 网关成功但本地余额/撤权事务失败后，重试只完成本地收尾。
}
```

- [ ] **Step 2：运行测试，确认当前退款只支持单一资金来源**

运行：`GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestHybridRefund'`

预期：FAIL，订单没有固定的 `refund_balance_amount` 与 `refund_gateway_amount`。

- [ ] **Step 3：实现固定拆分和幂等收尾**

首次退款把 `refund_amount` 依余额本金比例拆成两项，舍入余数归支付宝部分并校验均不超过各自本金。先持久化两项、稳定请求号和审计；`refund_gateway_amount>0` 时走现有 Provider 退款并把明确成功落库。随后在同一事务完成余额加回、权益撤销、`refund_balance_status=SUCCEEDED` 和订单最终状态。网关 `UNKNOWN/PENDING` 时不动余额和权益；网关已成功的重试不得再次调用 Provider。

- [ ] **Step 4：运行退款完整回归**

运行：`GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run 'TestHybridRefund|Test.*Refund'`

预期：PASS。

### Task 7：API 契约、购买页与支付状态展示

**文件：**

- 修改：`backend/internal/handler/payment_handler.go`
- 修改：`backend/internal/handler/payment_handler_refund_test.go`
- 修改：`frontend/src/types/payment.ts`
- 修改：`frontend/src/components/payment/paymentFlow.ts`
- 修改：`frontend/src/views/user/PaymentView.vue`
- 修改：`frontend/src/components/payment/PaymentStatusPanel.vue`
- 修改：`frontend/src/views/user/__tests__/PaymentView.spec.ts`
- 修改：`frontend/src/components/payment/__tests__/paymentFlow.spec.ts`
- 修改：`frontend/src/components/payment/__tests__/PaymentStatusPanel.spec.ts`
- 修改：`frontend/src/i18n/locales/zh.ts`
- 修改：`frontend/src/i18n/locales/en.ts`

- [ ] **Step 1：先写前端失败测试**

```ts
it('余额不足时创建混合套餐订单而不是进入余额充值', async () => {
  authState.userBalance = 6.32
  createOrder.mockResolvedValue({ order_id: 41, pay_amount: 79.79, balance_amount: 6.32, gateway_amount: 73.47 })
  await wrapper.get('[data-testid="subscription-plan-7"]').trigger('click')
  expect(createOrder).toHaveBeenCalledWith(expect.objectContaining({ use_balance: true, expected_balance_amount: '6.32' }))
  expect(balancePayOrder).not.toHaveBeenCalled()
})

it('在 UNKNOWN 确认期展示确认中而非最终过期', async () => {
  // status=PENDING, payment_resolution_status=UNKNOWN, deadline 尚未到期。
})
```

- [ ] **Step 2：运行前端测试，确认当前逻辑仍跳转充值**

运行：`pnpm test:run src/views/user/__tests__/PaymentView.spec.ts src/components/payment/__tests__/paymentFlow.spec.ts src/components/payment/__tests__/PaymentStatusPanel.spec.ts`

预期：FAIL；当前 `openRechargeConfirm(shortage)` 清空原商品选择并创建独立充值订单。

- [ ] **Step 3：实现混合结账与状态反馈**

`CreateOrderRequest` 和 `PaymentOrder` 接收资金拆分、funding mode、resolution/deadline、compensation 字段。购买页在 `0 < balance < pay_amount` 时显示“余额 + 支付宝”及本金/手续费/余额/支付宝差额摘要，调用商品 `createOrder` 传 `use_balance` 和显示时的金额前置条件；不再为套餐/流量包调用 `openRechargeConfirm`。余额足额继续现有纯余额路由，余额为零或用户选纯支付宝继续外部整单路由。

`PaymentStatusPanel` 在 00:00 后刷新服务端状态；`PENDING+UNKNOWN` 且未到 resolution deadline 显示“正在确认支付结果”并低频轮询；`COMPENSATED` 显示“支付已转入余额”，不显示购买成功。取消确认中禁止重复取消。

- [ ] **Step 4：运行前端回归和类型检查**

运行：`pnpm test:run src/views/user/__tests__/PaymentView.spec.ts src/components/payment/__tests__/paymentFlow.spec.ts src/components/payment/__tests__/PaymentStatusPanel.spec.ts src/api/__tests__/payment.spec.ts`

运行：`pnpm typecheck`

预期：PASS。

### Task 8：端到端回归、文档与提交前检查

**文件：**

- 新增：`docs/ai/context/YYYYMMDD-HHMMSS-alipay-balance-hybrid-payment-result_CN.md`
- 修改：`AGENTS.md`

- [ ] **Step 1：运行后端支付全量测试和 migration integration**

运行：`cd backend && GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service ./internal/payment/provider ./internal/handler`

运行：`cd backend && GOMAXPROCS=2 go test -p=1 -count=1 ./internal/repository -run TestMigrationsSchema`

预期：PASS。

- [ ] **Step 2：运行前端完整验证和 embed 编译**

运行：`cd frontend && pnpm test:run && pnpm typecheck && pnpm build`

运行：`cd backend && GOMAXPROCS=2 go test -p=1 -count=1 -tags=embed ./internal/web && go test -p=1 -count=1 ./cmd/server`

预期：PASS。

- [ ] **Step 3：审查变更、记录结果并提交**

运行：`git diff --check`

确认迁移只回填历史资金摘要、不创建历史 hold；确认没有调用真实支付宝、没有修改生产数据库或容器。新增结果文档和 `AGENTS.md` 记忆，记录测试命令、结果、分支和未部署状态。仅暂存本分支本功能文件后执行 `git add AGENTS.md backend frontend docs/ai/context`，再执行 `git commit -m "feat: add alipay balance hybrid payment"`。
