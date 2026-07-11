# 余额购买流量包事务修复设计

## 背景

余额购买商品由 `PaymentService.BalancePayOrder()` 在一个 Ent 事务内完成余额扣减、订单创建和权益发放。订阅权益仓储会从 `context` 复用该事务；流量包仓储 `CreditPurchase()` 则固定通过全局 `*sql.DB` 再开事务，导致 `user_traffic_credits.order_id` 引用尚未提交的 `payment_orders` 时触发外键错误。

## 必须满足的约束

- 余额扣减、订单创建、流量额度和流量流水必须同成同败。
- 支付宝等已提交订单的普通流量包发货仍要自行开启事务，保持流量额度与流水原子写入。
- 现有 `TrafficPackRepository` 服务接口和依赖注入尽量不变。
- 不移除或延迟外键，不提前提交余额订单，不引入异步补偿。

## 方案比较

### 方案 A：`CreditPurchase()` 感知 Ent 事务上下文，推荐

- 若 `dbent.TxFromContext(ctx)` 存在，直接使用 `tx.Client()` 执行流量额度和流水 SQL，由外层提交或回滚。
- 若不存在外层事务，继续通过当前 `*sql.DB` 开启本地事务并自行提交。
- 将 `QueryRowContext()` 改为项目已有的 `scanSingleRow()`，使 `*sql.Tx` 和 Ent transaction client 都能作为统一执行器。

优点：改动集中在流量包仓储，不改变 service 接口、构造函数和 Wire；同时保留两种调用场景的原子性。

### 方案 B：把流量包仓储整体改为 Ent client

- 构造函数改为接收 `*ent.Client`，全部读写改用 transaction-aware client。

优点是事务模型统一；缺点是会扩大到列表、汇总、扣减和依赖注入，超出本次根因修复范围。

### 方案 C：先提交订单，再独立发放流量

该方案会产生余额已扣、订单已存在但权益未发放的中间态，需要补偿任务和重试协议，破坏当前同步原子语义，不采用。

## 最终设计

采用方案 A。在 `trafficPackRepository` 中新增一个仅供购买入账使用的事务执行辅助函数：

1. 从 `context` 检查是否已有 Ent 事务。
2. 已有事务时，把 `tx.Client()` 作为 `sqlExecutor` 传给写入函数，不提交、不回滚。
3. 没有事务时，从 `r.db` 开启 `*sql.Tx`，写入成功后提交，失败时回滚。
4. `CreditPurchase()` 的幂等 `ON CONFLICT (order_id) DO NOTHING`、额度金额和流水语义保持不变。

`Deduct()` 不在本次修改范围内，因为当前生产故障只发生在购买入账引用未提交订单的路径；不扩大行为变更。

## 测试设计

新增 PostgreSQL integration 回归测试：

1. 创建一个已提交用户，使用迁移内置流量包。
2. 开启外层 Ent 事务，在事务内创建尚未提交的 `payment_orders`。
3. 用带有 `dbent.NewTxContext()` 的 context 调真实 `trafficPackRepository.CreditPurchase()`。
4. 修复前应因 `user_traffic_credits_order_id_fkey` 失败。
5. 修复后应能在外层事务内看到一条流量额度和一条购买流水。
6. 回滚外层事务后，订单、额度和流水在全局连接中均不存在，证明仓储没有独立提交。

保留并运行现有 SQLite 仓储单测和 service 余额支付单测，验证独立事务、幂等和上层调用行为没有回归。

## 非目标

- 不修改前端支付交互或错误文案。
- 不修改数据库 schema、外键或 migration。
- 不补写昨天失败的临时订单，也不操作用户余额和流量额度。
- 不构建或发布公网 18084，除非用户另行要求。
