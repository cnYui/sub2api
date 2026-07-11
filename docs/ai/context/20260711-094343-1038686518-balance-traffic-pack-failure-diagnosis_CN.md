# 1038686518 余额购买流量包失败诊断

## 结论

- `1038686518@qq.com` 对应运行态 `users.id=48/status=active`。
- 当前已解冻、尚未转余额的返利 `user_affiliates.aff_quota=0`，冻结返利 `aff_frozen_quota=0`，历史累计返利 `aff_history_quota=23.76`；三笔各 `7.92` 元返利已全部转入普通可消费余额，当前 `users.balance=23.76`。
- 当前三个在售流量包实付最高为 `5.05` 元，用户余额充足；失败不是返利冻结、余额不足、active 套餐、商品下架、支付通道或 Redis 缓存导致。
- 2026-07-10 23:01:50、23:01:56、23:03:04 JST 三次 `POST /api/v1/payment/orders/balance-pay` 均到达后端并返回 HTTP 500，实际错误为 `user_traffic_credits_order_id_fkey` 外键违反。
- 根因是余额支付外层 Ent 事务尚未提交 `payment_orders`，流量包仓储却通过全局 `*sql.DB` 另开独立事务写 `user_traffic_credits`；独立事务看不到父订单，外键检查失败。月度套餐仓储会使用外层事务，因此余额购买月度套餐可以成功。
- 三次失败均整体回滚，当前余额未减少，没有失败订单、流量额度、流量流水或支付审计残留。

## 当前资金快照

| 含义 | 字段 | 当前值 |
| --- | --- | ---: |
| 已解冻、尚未转余额的返利 | `user_affiliates.aff_quota` | `0.00000000` |
| 冻结中的返利 | `user_affiliates.aff_frozen_quota` | `0.00000000` |
| 普通可消费余额 | `users.balance` | `23.76000000` |
| 历史累计返利，仅统计 | `user_affiliates.aff_history_quota` | `23.76000000` |

返利转余额流水：

- `user_affiliate_ledger.id=9`：`7.92` 元，余额变为 `7.92`。
- `user_affiliate_ledger.id=11`：`7.92` 元，余额变为 `15.84`。
- `user_affiliate_ledger.id=12`：`7.92` 元，2026-07-10 19:57:30 JST 转入后余额变为 `23.76`，冻结返利归零。

购买代码只校验 `users.balance >= pay_amount`，不区分余额来自支付宝充值、邀请返利转入还是管理员加款。因此该用户返利已经具备正常消费资格。

## 失败证据

三次请求均来自与 `user_id=48/api_key_id=70` 同一连续访问链的 IP `203.190.1.61`：

| JST 时间 | request_id | HTTP | 应用错误 |
| --- | --- | ---: | --- |
| 2026-07-10 23:01:50 | `07db27f0-13e0-4911-8738-d3263c4f3769` | 500 | `user_traffic_credits_order_id_fkey` |
| 2026-07-10 23:01:56 | `0ee19108-804d-4125-9e18-d780e2c3cd7f` | 500 | `user_traffic_credits_order_id_fkey` |
| 2026-07-10 23:03:04 | `be503e4d-414c-4a4c-849d-6464b228e409` | 500 | `user_traffic_credits_order_id_fkey` |

- nginx access 记录购买页发出的三次真实后端请求，均为 Windows Chrome 149，响应体 39 bytes。
- 39 bytes 对应通用错误 `{"code":500,"message":"internal error"}`；nginx error 同窗口无上游错误。
- `ops_system_logs.id=591963/591969/592046` 保存完整数据库错误。
- PostgreSQL DETAIL 中临时订单 ID 为 `158/159/160`；这些 ID 只推进了 sequence，事务回滚后 `payment_orders` 中不存在对应行。
- `payment_orders`、`user_traffic_credits`、`traffic_credit_ledger`、`payment_audit_logs` 对 `158/159/160` 均为 0 行。
- 请求体未保留，无法可靠恢复用户当时选择的具体 `traffic_pack_id`；但三个在售流量包都会经过同一故障路径。

## 根因链路

1. `backend/internal/service/payment_balance_pay.go` 的 `BalancePayOrder()` 开启 Ent 外层事务，在其中扣减余额并创建未提交的 `payment_orders`。
2. 流量包分支调用 `fulfillTrafficPackOrderInTx()`。
3. `backend/internal/repository/traffic_pack_repo.go` 的 `CreditPurchase()` 不读取 Ent 事务上下文，而是通过全局 `*sql.DB` 再次 `BeginTx()`。
4. 新事务插入 `user_traffic_credits.order_id` 时看不到外层事务中的父订单；约束 `user_traffic_credits_order_id_fkey` 为立即检查、不可延迟，因此直接失败。
5. 外层事务 defer rollback，余额扣减、订单创建和流量发放全部撤销。

月度套餐能够成功，是因为 `userSubscriptionRepository.Create()` 通过 `clientFromContext()` 使用 Ent 外层事务，不会跨事务引用未提交订单。

## 测试缺口

- `TestBalancePayTrafficPackDeductsPayAmountAndCreditsPack` 使用总是成功的仓储 stub，只验证调用发生，没有触发真实外键。
- `traffic_pack_repo_test.go` 使用无真实订单外键约束的 SQLite 测试表。
- 现有测试没有覆盖 `PostgreSQL + 真实外键 + 未提交外层订单事务` 的组合，所以该问题从 2026-07-08 余额购买商品上线后一直未被发现。
- 目标 unit 测试当前仍通过：`GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service -run '^TestBalancePayTrafficPackDeductsPayAmountAndCreditsPack$'`。

## 正确修复方向

- 流量包发放必须加入余额支付的同一个数据库事务，不能提前提交订单，也不能用独立连接绕开外层事务。
- 应让流量包仓储支持事务感知的执行器或 Ent client，并补 PostgreSQL 外键集成回归测试。
- 不应通过移除外键、延迟外键或先扣余额后异步补发来规避问题，这些方案会破坏订单、余额与权益发放的原子性。

## 本轮范围

- 仅执行只读数据库、Redis、容器、nginx 日志和源码审计。
- 未修改用户余额、返利、订单、订阅、流量额度、缓存、容器或业务代码。
