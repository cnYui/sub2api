# 余额购买流量包事务修复实施计划

> **面向执行代理：** REQUIRED SUB-SKILL: 使用 `subagent-driven-development`（推荐）或 `executing-plans` 逐项执行；每一步使用复选框追踪。

**目标：** 修复余额购买流量包时真实仓储另开事务导致订单外键失败的问题，并用 PostgreSQL 集成测试锁定事务传播语义。

**架构：** `CreditPurchase()` 在已有 Ent 事务时复用 `tx.Client()`，否则保留当前本地 SQL 事务。余额扣减、订单、流量额度和购买流水继续保持单事务原子性，不改变 service 接口或数据库 schema。

**技术栈：** Go、Ent、`database/sql`、PostgreSQL、testcontainers、Testify。

---

## 文件结构

- 新建 `backend/internal/repository/traffic_pack_repo_integration_test.go`：覆盖未提交父订单、事务复用和外层回滚。
- 修改 `backend/internal/repository/traffic_pack_repo.go`：让 `CreditPurchase()` 复用已有 Ent 事务。
- 新建实施结果文档并更新 `AGENTS.md`：记录验证结果和未部署状态。

### Task 1：新增失败的 PostgreSQL 回归测试

**文件：**

- 新建：`backend/internal/repository/traffic_pack_repo_integration_test.go`

- [ ] **Step 1：写入失败测试**

测试应创建已提交用户，开启 Ent 外层事务，在事务内创建 `payment_orders`，再调用真实仓储：

```go
func TestTrafficPackRepository_CreditPurchase_ReusesOuterTransaction(t *testing.T) {
    ctx := context.Background()
    user := mustCreateUser(t, integrationEntClient, &service.User{
        Email:       fmt.Sprintf("traffic-pack-tx-%d@example.com", time.Now().UnixNano()),
        PasswordHash: "hash",
        Role:         service.RoleUser,
        Status:       service.StatusActive,
        Concurrency:  5,
    })

    outerTx, err := integrationEntClient.Tx(ctx)
    require.NoError(t, err)
    t.Cleanup(func() { _ = outerTx.Rollback() })
    txCtx := dbent.NewTxContext(ctx, outerTx)

    now := time.Now()
    order, err := outerTx.Client().PaymentOrder.Create().
        SetUserID(user.ID).
        SetUserEmail(user.Email).
        SetUserName(user.Username).
        SetAmount(3).
        SetPayAmount(3.03).
        SetFeeRate(1).
        SetRechargeCode(fmt.Sprintf("PAY-BALANCE-TX-%d", now.UnixNano())).
        SetOutTradeNo(fmt.Sprintf("sub2_tx_%d", now.UnixNano())).
        SetPaymentType(payment.TypeBalance).
        SetPaymentTradeNo("balance").
        SetOrderType(payment.OrderTypeTrafficPack).
        SetStatus(service.OrderStatusRecharging).
        SetExpiresAt(now).
        SetPaidAt(now).
        SetClientIP("127.0.0.1").
        SetSrcHost("test.local").
        Save(txCtx)
    require.NoError(t, err)

    repo := NewTrafficPackRepository(integrationDB)
    err = repo.CreditPurchase(txCtx, service.CreditTrafficPackInput{
        UserID: user.ID, OrderID: order.ID, PackID: 2,
        CreditUSD: 10, ValidityDays: 365, CreditedAt: now,
    })
    require.NoError(t, err)

    // 事务内可见额度和流水；回滚后全局均不可见。
}
```

- [ ] **Step 2：运行测试确认 RED**

运行：

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run '^TestTrafficPackRepository_CreditPurchase_ReusesOuterTransaction$'
```

预期：FAIL，错误包含 `user_traffic_credits_order_id_fkey`，证明测试复现生产根因。

### Task 2：让购买入账复用外层事务

**文件：**

- 修改：`backend/internal/repository/traffic_pack_repo.go`

- [ ] **Step 1：实现最小事务辅助函数**

增加 Ent import，并让辅助函数在已有事务时不自行提交：

```go
func (r *trafficPackRepository) withCreditPurchaseTx(
    ctx context.Context,
    fn func(context.Context, sqlExecutor) error,
) error {
    if tx := dbent.TxFromContext(ctx); tx != nil {
        return fn(ctx, tx.Client())
    }

    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer func() { _ = tx.Rollback() }()

    if err := fn(ctx, tx); err != nil {
        return err
    }
    return tx.Commit()
}
```

- [ ] **Step 2：改写 `CreditPurchase()`**

将额度和流水写入放进 `withCreditPurchaseTx()`，并使用 `scanSingleRow()` 代替 `QueryRowContext()`：

```go
return r.withCreditPurchaseTx(ctx, func(txCtx context.Context, exec sqlExecutor) error {
    var creditID int64
    err := scanSingleRow(txCtx, exec, insertCreditSQL, []any{
        input.UserID, input.OrderID, input.PackID,
        service.TrafficPackPlatformOpenAI, creditUSD,
        input.CreditedAt, expiresAt,
    }, &creditID)
    if errors.Is(err, sql.ErrNoRows) {
        return nil
    }
    if err != nil {
        return err
    }
    _, err = exec.ExecContext(txCtx, insertLedgerSQL,
        input.UserID, creditID, input.OrderID,
        service.TrafficCreditLedgerTypePurchase, creditUSD, input.CreditedAt)
    return err
})
```

保持 SQL、金额精度、幂等约束和 `Deduct()` 不变。

- [ ] **Step 3：运行目标集成测试确认 GREEN**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run '^TestTrafficPackRepository_CreditPurchase_ReusesOuterTransaction$'
```

预期：PASS。

### Task 3：验证相关行为没有回归

**文件：**

- 验证：`backend/internal/repository/traffic_pack_repo_test.go`
- 验证：`backend/internal/service/payment_balance_pay_test.go`

- [ ] **Step 1：运行流量包仓储 unit 测试**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/repository -run '^TestTrafficPackRepository_'
```

预期：PASS，独立事务、幂等和扣减顺序保持不变。

- [ ] **Step 2：运行余额支付 service 测试**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run '^TestBalancePay'
```

预期：PASS。

- [ ] **Step 3：运行相关包测试**

```bash
cd backend
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/repository ./internal/service
```

预期：PASS。

- [ ] **Step 4：运行格式与静态差异检查**

```bash
gofmt -w backend/internal/repository/traffic_pack_repo.go backend/internal/repository/traffic_pack_repo_integration_test.go
git diff --check
```

预期：无输出，退出码 0。

### Task 4：记录结果

**文件：**

- 新建：`docs/ai/context/20260711-095338-balance-traffic-pack-transaction-fix-result_CN.md`
- 修改：`AGENTS.md`

- [ ] **Step 1：记录 RED/GREEN 证据、修改范围和测试命令**
- [ ] **Step 2：明确未构建镜像、未部署 18084、未修改运行态数据**
- [ ] **Step 3：复核计划无占位符、设计约束均有对应测试**

本计划不包含提交、推送或部署步骤；用户当前仅要求创建分支并完成本地修复。
