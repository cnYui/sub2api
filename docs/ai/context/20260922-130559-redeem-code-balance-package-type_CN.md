# 兑换码新增「余额套餐」类型（balance_package）

- 时间：2026-09-22
- 分支：`worktree-redeem-balance-package`（独立 worktree，基于 `f3cdf99c3`）
- 需求（管理员原话）：「当前兑换码除了直接兑换余额，你也可以再加一种类别，就是能够兑换套餐。用户兑换完之后，可以直接兑换某一个类别的套餐（比如 28 天、分 4 次刷新的那种），兑换完之后就自动绑定上」

## 一、结论先说

新增兑换码类型 `balance_package`：管理员生成时选一个 `balance_package_plans` 档位，
用户兑换后自动绑定该档位的余额套餐，**到账与周刷新逻辑和购买页完全一致**
（复用 `BalancePackageService.creditInitialBalance`，不是另写一套）。

管理员在实现前确认的一个关键决策：

> **用户当前已有有效套餐时，兑换一律拒绝**（不做同档续费）。

原因：续费会把套餐 `payment_order_id` 改绑到这笔零金额的兑换订单
（`renewBalancePackage` 的既有行为，AGENTS.md 也记了「续费会改绑订单」），
用户原来那笔**真实支付**的订单就此不可退款。拒绝是可逆的——事务回滚，
兑换码保持未使用，用户等本期套餐失效后还能再兑。

## 二、数据模型

迁移 `215_redeem_code_balance_package_plan.sql`：

```sql
ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS balance_package_plan_id BIGINT NULL
        REFERENCES balance_package_plans(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_redeem_codes_balance_package_plan_id
    ON redeem_codes (balance_package_plan_id);
```

ent 侧同步（`ent/schema/redeem_code.go` 加字段 + 索引 + `edge.From("balance_package_plan", …)`，
`ent/schema/balance_package_plan.go` 补 `edge.To("redeem_codes", …)`），然后 `go generate ./ent/...`。

> ent 重生在 `.claude/worktrees/` 下的 worktree 里**一次成功**，没有复现
> `ent-regen-file-lock-workaround` 记录的 mmap 锁问题（那条记录说的是仓库主目录）。
> 顺手产生的 `channelmonitor*` 注释漂移已手工还原，保持 diff 聚焦。

`value` 字段对该类型无意义，一律写 0（和 `invitation` 同样处理，见
`redeemTypeIgnoresValue`）——**不要拿它当美元额度展示或统计**。

## 三、发放链路

兑换在 `RedeemService.Redeem` 已有的事务里多了一个分支：

```
Use(兑换码置为已用)  →  BalancePackageService.GrantByRedeemCode(txCtx, tx.Client(), …)
```

`GrantByRedeemCode`（新文件 `internal/service/balance_package_grant_order.go`）：

1. 按 id 取档位，**不加 `for_sale=true` 过滤**——码已经发出去了，下架档位不该让码作废；
   仍然跑 `validateBalancePackagePlan`。
2. 取用户，禁用账号拒绝（`USER_INACTIVE`）。
3. 建一笔零金额、已完成的订单：`payment_type=redeem_code`、
   `order_type=balance_subscription`、`recharge_code=REDEEM-<id>`。
4. `creditInitialBalance` 发首期额度、建 `user_balance_packages` 行。
5. 写审计 `REDEEM_BALANCE_PACKAGE_GRANTED`，operator `user:<id>`。

顺带把后台手动发放（`PaymentService.GrantBalancePackage`）里那段建订单的代码
抽成了共用的 `createBalancePackageGrantOrder` / `loadGrantableUser`，两条路径写出的
订单快照保证一致。原有测试 `TestGrantBalancePackageCreatesAuditablePackageLifecycle` 不变通过。

### 不可退款是「自动」的

`validateRealPaidBalancePackageOrder` 的支付方式白名单只放行
alipay/wxpay/stripe/easypay/airwallex，新的 `redeem_code` 落到 `default` 分支，
直接返回 `REFUND_REQUIRES_REAL_PAYMENT`。这和 AGENTS.md「兑换码订单不可退」的口径一致，
不需要额外加判断。

### 到账邮件

合并 PR #46（到账邮件通知）之后补上的：`GrantByRedeemCode` 把建出来的订单返回给
`RedeemService.Redeem`，事务提交后调 `PurchaseNotifyService.NotifyBalancePackage(order)`，
**复用购买页那封「余额套餐已生效」**（事件 `balance_package_credited`），不新增事件和模板。

- 邮件里的「类型」标成「兑换码兑换」（新增 `purchaseKindRedeem`）。
- 「实付」留空（`—`）：兑换订单是零金额，但码可能是用户在别处花钱买的，
  照搬管理员发放那套写「赠送」是替对方下结论。
- 判定「类型」的那段 switch 抽成了纯函数 `balancePackagePurchaseKind`，方便回归测试。
- 普通余额兑换码仍走 #46 原有的 `NotifyRedeemBalance`，那条分支没动。

⚠️ 通知必须在**事务提交后**发：`sendBalancePackageNotice` 用非事务的 entClient
按 `payment_order_id` 反查 `user_balance_packages`，提交前查不到。

### 已有套餐的闸门

`creditInitialBalance` 里原本只拦 `admin_grant`，现在改成
`admin_grant || redeem_code` 一起拦。注释写明了原因（不改绑真实支付订单）。

## 四、接口与前端

后端：

- `POST /admin/redeem-codes/generate`：`type` 枚举加 `balance_package`，新增
  `balance_package_plan_id`（该类型必填，服务端再校验一次档位）。
- `POST /admin/redeem-codes/create-and-redeem`：同样支持。
  ⚠️ `Value` 原来是 `binding:"required"`，对 float64 来说 0 会被拒，
  套餐码必须允许 0，所以把 tag 去掉、改在 handler 里对**非套餐类型**显式校验非零，
  其它类型的行为和以前完全一样。
- DTO 增加 `balance_package_plan_id` 与 `balance_package_plan`（档位名/价格/周额度/期数），
  用户端 `/redeem`、`/redeem/history` 和管理端列表、余额历史都能直接展示档位名。
- CSV 导出末尾加一列 `balance_package_plan`。

前端：

- 管理端兑换码页：类型下拉、筛选、表格徽章（紫色）与「面值」列展示档位；
  生成弹窗在选了套餐类型时换成档位下拉（数据来自 `GET /admin/payment/balance-packages`，
  只列在售档位），并给出「已有有效套餐会被拒绝」的提示。
- 用户端兑换页：兑换成功卡片显示「余额套餐已开通 - 档位名（每期 $X，共 N 期，有效期 M 天）」，
  历史列表用 gift 图标 + 档位名（`value` 恒为 0，不能显示金额）。
- 管理端「用户余额历史」弹窗同步加了类型筛选项与展示。

## 五、验证

- `go build ./...`、`go vet -tags=unit ./...`、`go vet -tags=integration ./...` 全通过。
- `go test -tags=unit ./...` 全通过（含新增 3 个用例）。
- 新增 `internal/service/redeem_service_balance_package_test.go`（`unit` 标签），
  用 ent/SQLite 真事务跑：
  1. 兑换成功 → 套餐建好、首期到账、订单 `payment_type=redeem_code`、审计存在、退款校验拒绝；
  2. 已有套餐再兑 → `BALANCE_PACKAGE_ACTIVE`，**第二张码回滚后仍是未使用**、余额没变；
  3. 码没绑档位 → `REDEEM_CODE_INVALID`，事务前就拦下。

  第 1 个用例还接了一个探针版 `PurchaseNotifyService`，确认兑换成功真的走到了发信逻辑
  （真发信要 SMTP，这里只验证接线 + 「类型」判定）。
- `purchase_notify_service_test.go` 增加 `TestPurchasePayAmountTextForRedeemCode`。
- `go test -tags=integration -run TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate ./internal/repository/`
  通过（testcontainers 起真 postgres:18，确认迁移 215 能应用且列可空性正确）。
- `golangci-lint run` 改动包 0 issues；前端 `lint:check` + `typecheck` + 相关 vitest 通过。

## 六、遗留 / 注意

- **生产还没跑过这条迁移**，上线时按常规流程走（迁移会随镜像启动自动应用）。
- 已有套餐被拒绝这件事，用户端只能看到 `BALANCE_PACKAGE_ACTIVE` 的服务端消息
  「当前已有有效余额套餐，请先退款后再购买其他套餐」——文案是购买场景的，
  对兑换场景略有偏差；要不要给兑换单独一句文案，待管理员定。
- `create-and-redeem` 也支持套餐码了，外部商城（Sub2ApiPay）若要卖套餐可以直接用；
  但那条路径同样受「已有套餐则拒绝」限制。
- 本次是在独立 worktree 里做的，因为同期另一个会话正在主工作区改
  `redeem_service.go` / `wire.go` / `payment_balance_package_grant.go`（套餐购买后的邮件通知）。
  **2026-09-22 已把 PR #46 合进 main 后 rebase 完成**，实际冲突只有两处：
  `redeem_service.go` 的结构体字段（两边各加一个字段，取并集）和生成文件 `wire_gen.go`
  （直接重生）。`payment_balance_package_grant.go` 反而自动合上了。
  另外 #46 新引入的 `ProvideRedeemService` 要手工补 `*BalancePackageService` 参数，
  只改 `NewRedeemService` 不够——wire 生成的调用走的是那个 Provide 包装。
