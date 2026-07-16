# 支付宝与余额组合支付设计

时间：2026-07-15 09:34 JST

## 背景

当前 `/purchase` 在用户选择余额但余额不足时，会把原套餐或流量包选择清空，转为一笔独立的支付宝余额充值订单。充值成功后只刷新用户余额，不会继续创建商品订单。该流程看起来像“余额 + 支付宝”，实际是两笔互不关联的订单，已经导致用户支付宝付款成功但套餐没有自动发放。

本设计把组合支付改为一笔真实商品订单：订单同时记录余额出资和支付宝出资，余额在下单时冻结，支付宝只支付精确差额，支付成功后再统一完成资金确认和权益发放。

## 目标

- 套餐和流量包都支持余额与支付宝组合支付。
- 用户只看到一笔商品订单，不再先充值再购买。
- 支付宝只收精确差额，不对差额向上取整。
- 余额在等待支付期间被可靠占用，不能被并发订单重复使用。
- 支付成功、取消、过期、迟到回调和退款均有可证明的资金状态。
- 复用现有 30 分钟订单有效期和 5 分钟支付状态不确定窗口。
- 与现有退款状态机、单 active 套餐约束、支付审计和缓存失效规则兼容。

## 不在范围内

- 不支持余额与微信、Stripe、Airwallex 等其他渠道组合支付。
- 不改变独立余额充值功能。
- 不允许客户端自行决定最终扣款金额。
- 不增加余额使用比例滑块；组合支付默认使用当前全部可用余额，最多使用到本单应付金额。
- 不回填历史订单的余额占用记录。
- 不在本设计阶段修改运行态、数据库或业务代码。

## 核心不变量

1. 一笔商品购买只能有一个 `payment_orders` 聚合根。
2. `pay_amount = balance_amount + gateway_amount` 必须始终成立。
3. `amount` 始终表示不含手续费的商品本金；`pay_amount` 始终表示含手续费总应付。
4. 支付宝验签、主动查询和退款只比较或使用 `gateway_amount`，不能再把支付宝实付写回覆盖 `pay_amount`。
5. 一个订单最多存在一条余额占用记录。
6. 余额占用只能单向转移：`RESERVED -> CAPTURED` 或 `RESERVED -> RELEASED`。
7. 余额已释放后不能因迟到回调再次扣除。
8. 未确认支付宝结果时不能发放套餐，也不能释放余额。
9. 所有重复请求、重复回调和后台重试都必须幂等。

## 方案选择

### 方案一：单商品订单 + 余额冻结 + 支付宝差额（采用）

商品订单保存完整资金拆分，余额通过独立占用记录保证并发安全。该方案语义真实，订单、退款和审计均可解释。

### 方案二：支付宝充值后自动余额购买

仍然是两笔订单，会继续存在充值成功但购买失败、商品变价、用户出现新套餐、重复点击和退款割裂问题。不采用。

### 方案三：支付宝先付全款，再返还或扣减余额

资金路径与页面展示不一致，增加无意义的外部收款和退款，不采用。

## 用户交互

商品确认页保留以下支付选择：

- `支付宝`：支付宝支付全部 `pay_amount`，不使用余额。
- `余额`：仅在余额足够覆盖全部 `pay_amount` 时可用。
- `余额 + 支付宝`：仅在 `0 < balance < pay_amount` 时显示。

组合支付摘要示例：

```text
商品金额        ¥79.00
手续费          ¥0.79
余额抵扣        -¥6.32
支付宝支付      ¥73.47
```

页面必须明确说明余额在支付等待期间会被占用。支付宝页面仍沿用当前新窗口或二维码交互，倒计时继续显示 30 分钟。

前端不再调用 `openRechargeConfirm(shortage)`，也不再清空原商品选择。组合支付成功后与普通支付宝商品订单一样刷新用户和权益状态。

## 金额计算

全部金额使用十进制定点计算并最终保留到人民币分，禁止使用二进制浮点直接做财务比较。

```text
product_amount = 商品本金
fee_amount = round(product_amount * fee_rate / 100, 2)
pay_amount = product_amount + fee_amount
balance_amount = min(round(current_balance, 2), pay_amount)
gateway_amount = pay_amount - balance_amount
```

组合支付成立条件：

```text
0 < balance_amount < pay_amount
gateway_amount >= 支付 Provider 最小支付金额
```

若支付宝差额低于 Provider 最小支付金额，前端不展示组合支付，用户只能选择纯余额或纯支付宝。不能为了达到最低金额而多收支付宝，再把差额回充余额。

### 本金与手续费的资金归属

余额优先抵扣商品本金，抵完本金后才抵扣手续费：

```text
balance_principal = min(balance_amount, product_amount)
gateway_principal = product_amount - balance_principal
balance_fee = balance_amount - balance_principal
gateway_fee = fee_amount - balance_fee
```

该拆分用于退款和邀请返利，防止用极少支付宝金额触发整单返利。

## 数据模型

### payment_orders 新增字段

- `funding_mode VARCHAR(20) NOT NULL DEFAULT 'gateway'`
  - `gateway`：纯外部支付。
  - `balance`：纯余额支付。
  - `mixed`：余额与支付宝组合支付。
- `balance_amount NUMERIC(20,2) NOT NULL DEFAULT 0`
- `gateway_amount NUMERIC(20,2) NOT NULL DEFAULT 0`
- `provider_init_status VARCHAR(20) NOT NULL DEFAULT 'NOT_REQUIRED'`
  - `NOT_REQUIRED`、`NOT_STARTED`、`CREATED`、`FAILED`、`UNKNOWN`。
- `provider_init_attempted_at TIMESTAMPTZ NULL`
- `payment_resolution_status VARCHAR(20) NOT NULL DEFAULT 'NOT_REQUIRED'`
  - `NOT_REQUIRED`、`PENDING`、`PAID`、`UNPAID`、`UNKNOWN`。
- `payment_resolution_deadline TIMESTAMPTZ NULL`
- `cancel_requested_at TIMESTAMPTZ NULL`
- `compensation_amount NUMERIC(20,2) NOT NULL DEFAULT 0`
- `compensated_at TIMESTAMPTZ NULL`
- `refund_balance_amount NUMERIC(20,2) NOT NULL DEFAULT 0`
- `refund_gateway_amount NUMERIC(20,2) NOT NULL DEFAULT 0`
- `refund_balance_status VARCHAR(20) NOT NULL DEFAULT 'NOT_REQUIRED'`

`payment_orders.status` 继续表示用户可见生命周期。新增终态：

- `COMPENSATED`：订单未发放商品，迟到支付宝实付已补入站内余额。

新建纯支付宝和混合订单设置 `provider_init_status=NOT_STARTED`；Provider 创建成功后再设置 `CREATED` 和 `payment_resolution_status=PENDING`。纯余额订单两个状态均为 `NOT_REQUIRED`。混合订单的 `payment_type` 仍为 `alipay`，Provider 路由不引入虚构的 `hybrid` 支付渠道，页面和订单列表通过 `funding_mode=mixed` 展示组合支付。

### payment_balance_holds 新表

- `id BIGSERIAL PRIMARY KEY`
- `order_id BIGINT NOT NULL UNIQUE REFERENCES payment_orders(id) ON DELETE RESTRICT`
- `user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT`
- `amount NUMERIC(20,2) NOT NULL CHECK (amount > 0)`
- `status VARCHAR(20) NOT NULL`
  - `RESERVED`、`CAPTURED`、`RELEASED`
- `expires_at TIMESTAMPTZ NOT NULL`
- `captured_at TIMESTAMPTZ NULL`
- `released_at TIMESTAMPTZ NULL`
- `release_reason VARCHAR(50) NULL`
- `created_at/updated_at TIMESTAMPTZ NOT NULL`

余额占用不是第二笔支付订单。创建 `RESERVED` 时同时从 `users.balance` 扣除，因此 `users.balance` 始终表示可用余额；`CAPTURED` 只改变占用状态，`RELEASED` 才把金额加回用户余额。

## API 设计

现有 `POST /api/v1/payment/orders` 增加组合支付参数，不新增“充值后续接”接口：

```json
{
  "order_type": "subscription",
  "plan_id": 7,
  "payment_type": "alipay",
  "use_balance": true,
  "expected_pay_amount": "79.79",
  "expected_balance_amount": "6.32"
}
```

服务端必须重新计算商品金额、手续费和当前可用余额。客户端金额只作为并发变化前置条件：

- 商品价格、手续费或余额变化时返回 `CHECKOUT_CHANGED`。
- 前端刷新确认页，由用户重新确认。
- 服务端不得在余额减少后静默提高支付宝金额。

订单创建响应增加：

```json
{
  "funding_mode": "mixed",
  "amount": 79.00,
  "pay_amount": 79.79,
  "balance_amount": 6.32,
  "gateway_amount": 73.47
}
```

纯余额支付继续保留现有 `/payment/orders/balance-pay` 路由，但服务层应与组合支付复用商品解析、金额计算、履约和审计能力，避免两套实现分叉。

## 创建订单流程

### 第一阶段：数据库事务

1. 校验用户 active、商品在售、单 active 套餐约束和待支付订单数量。
2. 重新计算 `amount/pay_amount` 和资金拆分。
3. 使用条件更新扣减余额：`users.balance >= balance_amount`。
4. 创建唯一 `payment_orders`，状态 `PENDING`，`expires_at=now+30min`。
5. 创建唯一 `payment_balance_holds`，状态 `RESERVED`，到期时间为订单 `expires_at+5min`。
6. 写 `ORDER_CREATED` 和 `BALANCE_RESERVED` 审计。
7. 提交事务并失效余额、鉴权缓存。

两个并发订单不能同时使用同一余额。条件更新和数据库事务是唯一正确性边界，Redis 只负责缓存，不能承担资金锁。

### 第二阶段：创建支付宝订单

使用稳定 `out_trade_no` 和 `gateway_amount` 请求支付宝 Provider。

- 调用前写 `provider_init_attempted_at`。
- 明确创建成功：保存支付 URL/二维码，设置 `provider_init_status=CREATED`、`payment_resolution_status=PENDING`。
- 明确创建失败且确认上游未创建订单：设置 `provider_init_status=FAILED`，事务内将订单标记 `FAILED`，余额占用转 `RELEASED`。
- 网络超时或结果无法确认：保持订单 `PENDING`，设置 `provider_init_status=UNKNOWN`、`payment_resolution_status=UNKNOWN`，由后台按 `out_trade_no` 查询；不能立即释放余额。

Provider 创建重试必须使用相同 `out_trade_no`，不得生成第二笔外部订单。

后台每 60 秒同时处理初始化异常：

- `NOT_STARTED`：说明进程可能在数据库提交后、调用 Provider 前退出，使用原 `out_trade_no` 发起创建。
- `UNKNOWN`：先查询 Provider；确认订单不存在后才使用原 `out_trade_no` 重试创建。
- `CREATED`：只进入正常支付状态查询，不重复创建。

## 支付状态三态

三态用于回答“是否可以释放余额”，不是直接复制 Provider 字符串。

### PAID

- 收到验签成功的 Webhook，且订单号、商户身份、币种和 `gateway_amount` 全部匹配。
- 或主动查询明确返回已支付，金额和交易号有效。

处理：占用转 `CAPTURED`，进入商品履约。

### UNPAID

- 官方支付宝明确返回交易关闭或失败。
- 可取消 Provider 已成功关闭外部订单，并复查未支付。
- EasyPay/ZPay 在本地过期后持续返回 pending，经过 5 分钟确认窗口仍未支付，按本地业务截止规则终止。

处理：订单取消或过期，占用转 `RELEASED`。

### UNKNOWN

- DNS、连接、TLS、超时或上游 5xx。
- JSON 解析失败、缺少关键字段、金额无效或未知状态。
- 30 分钟到期时 Provider 仍返回 pending，支付可能在途。
- 用户取消时查询或关闭外部订单的结果无法确认。

处理：不发商品、不释放余额，在 5 分钟窗口内每 60 秒重试。

当前 `checkPaidWithOptions()` 和 `cancelCore()` 必须重构为显式三态，不能继续用空字符串同时表达未支付和查询失败。

## Webhook 与履约

Webhook 处理必须锁定订单并验证：

- Provider 实例和商户身份与订单快照一致。
- `out_trade_no` 唯一匹配。
- 支付金额等于 `gateway_amount`，不是 `pay_amount`。
- 订单资金拆分满足不变量。

首次成功处理在同一数据库事务中：

1. `payment_resolution_status -> PAID`
2. 余额占用 `RESERVED -> CAPTURED`
3. 订单 `PENDING -> PAID -> RECHARGING`
4. 发放或延长准确的套餐/流量包权益
5. 保存准确 `subscription_id` 或流量包购买关联
6. 订单转 `COMPLETED`
7. 写资金与履约审计

事务失败则全部回滚。重复 Webhook、主动查询和后台重试只返回已有结果，不重复发放。

`toPaid()` 不得再执行 `SetPayAmount(paid)`；支付宝实付只更新或验证 `gateway_amount`。

## 30 分钟超时与5分钟确认窗口

前端继续按后端 `expires_at` 显示 30 分钟倒计时。倒计时归零只改变页面状态，真实订单由后端任务处理。

前端到 `00:00` 后不能立即展示最终“订单已过期”并停止所有查询，而应先请求后端状态：

- 后端已 `COMPLETED/EXPIRED/CANCELLED`：展示对应终态。
- 后端仍在 `PENDING + UNKNOWN`：展示“正在确认支付结果”，继续低频轮询到 `payment_resolution_deadline`。
- 最终截止后仍未支付：展示过期和余额已退回。

后台每 60 秒扫描：

- `PAID`：立即完成履约。
- `UNPAID`：订单转 `EXPIRED`，占用立即释放。
- `UNKNOWN`：保持占用，设置或复用 `payment_resolution_deadline=expires_at+5min`。

30-35 分钟期间继续查询。明确未支付时立即释放，不必等满 5 分钟。超过截止时间仍无法确认时，订单转 `EXPIRED` 并释放余额。

余额正常占用 30 分钟，只有支付状态无法确认时最长约 35 分钟，不使用 24 小时宽限。

## 用户主动取消

取消操作不能先改本地状态：

- 查询为 `PAID`：拒绝取消并继续履约。
- 查询为 `UNPAID` 且上游关闭成功或无需关闭：订单转 `CANCELLED`，释放余额。
- 查询或关闭结果为 `UNKNOWN`：写 `cancel_requested_at`，页面显示“取消确认中”，最多重试 5 分钟。

提前取消的确认截止为 `min(cancel_requested_at+5min, expires_at+5min)`。`payment_balance_holds.expires_at` 是最长保护时间，不妨碍后台在更早的取消确认截止后释放余额。

现有 `CANCELLED` 可无限期被 Webhook 恢复的行为必须收敛到同一 `payment_resolution_deadline`。

## 迟到付款补偿

若余额仍为 `RESERVED` 且在确认窗口内收到成功回调，正常完成商品订单。

若余额已经 `RELEASED` 后才收到真实成功回调：

1. 不重新扣除用户余额。
2. 不发放原商品，避免用户已购买其他套餐或价格条件已变化。
3. 将支付宝实际支付的 `gateway_amount` 作为人民币余额补入用户账户。
4. 创建唯一、幂等的余额补偿记录并关联原订单。
5. 订单转 `COMPENSATED`，记录 `PAYMENT_AFTER_HOLD_RELEASED` 和 `PAYMENT_COMPENSATED`。

补偿只发生一次；重复回调直接返回成功。补偿属于支付异常返还，不是新的充值或商品支付，不产生邀请返利，也不增加商品购买次数。

## 邀请返利

组合支付不能按整单 `amount` 计算支付宝邀请返利，否则用户可以用大量余额加极少支付宝触发整单返利。

混合订单返利基数使用：

```text
affiliate_rebate_base = gateway_principal
```

余额本金和全部手续费都不进入返利基数。纯支付宝和纯余额订单保持现有规则。

## 退款分摊

组合支付继续复用已增强的退款状态机：退款基数为不含手续费的 `order.amount`，按北京时间自然日计算，首次申请后持久化，重试不重新计算。

### 首次申请时固定拆分

```text
refund_amount = 当前规则计算出的可退商品本金
refund_balance_amount = round(refund_amount * balance_principal / amount, 2)
refund_gateway_amount = refund_amount - refund_balance_amount
```

拆分使用十进制定点数，并把舍入余数统一放入 `refund_gateway_amount`，保证两部分之和严格等于已持久化的 `refund_amount`，且任何一部分不超过对应本金。手续费不退。余额本金原路退回站内余额，支付宝本金原路退回支付宝。

### 执行顺序

1. 持久化总退款金额、余额退款金额、支付宝退款金额和稳定退款请求号。
2. `refund_gateway_amount > 0` 时先调用支付宝退款。
3. 支付宝明确成功后持久化 `refund_gateway_status=SUCCEEDED`。
4. 在单个数据库事务内增加 `refund_balance_amount`、撤销准确权益、更新余额退款状态和订单最终状态。
5. 若支付宝成功但本地事务失败，重试只能继续本地余额与权益收尾，不能再次请求支付宝。
6. 支付宝结果 `UNKNOWN/PENDING` 时不退站内余额、不撤销权益，进入现有人工核验流程。

新增 `refund_balance_status`：

- `NOT_REQUIRED`
- `NOT_STARTED`
- `SUCCEEDED`
- `FAILED`

余额回退、权益撤销和最终订单状态必须同事务提交，避免部分完成。

## 审计与幂等

新增审计动作：

- `BALANCE_RESERVED`
- `BALANCE_CAPTURED`
- `BALANCE_RELEASED`
- `PAYMENT_STATUS_UNKNOWN`
- `PAYMENT_AFTER_HOLD_RELEASED`
- `PAYMENT_COMPENSATED`
- `MIXED_REFUND_SPLIT`

幂等约束：

- `payment_orders.out_trade_no` 唯一。
- `payment_balance_holds.order_id` 唯一。
- 余额补偿 code 使用订单 ID 派生的确定性唯一值。
- Webhook 审计保持订单 + action 唯一。
- 退款继续复用稳定 `refund_request_id`。

## 缓存与通知

以下场景必须失效余额缓存和用户所有 API Key 的鉴权快照：

- 余额冻结。
- 余额释放。
- 迟到付款补偿。
- 混合退款余额回退。

套餐和流量包履约继续失效对应 billing cache。支付成功邮件和订单页增加余额/支付宝拆分字段；补偿终态发送“支付已转入余额”通知，不发送“套餐购买成功”。

## 迁移与兼容

新字段迁移应排在退款状态机迁移之后。

历史订单只回填资金摘要，不创建占用记录：

- `payment_type=balance`：`funding_mode=balance`、`balance_amount=pay_amount`、`gateway_amount=0`。
- 其他历史订单：`funding_mode=gateway`、`balance_amount=0`、`gateway_amount=pay_amount`。

只有新建 `mixed` 订单创建 `payment_balance_holds`。发布时已有的 PENDING 老订单继续按纯外部支付处理，避免迁移期间错误扣余额。

应用代码必须同时兼容迁移前历史订单和迁移后新订单。旧订单金额校验继续回退到 `pay_amount`；新混合订单只校验 `gateway_amount`。

## 测试策略

### 后端单元测试

- 金额拆分和人民币分舍入。
- 余额优先抵扣本金。
- 组合支付最低支付宝金额边界。
- `PAID/UNPAID/UNKNOWN` 映射。
- EasyPay pending 在30分钟时进入 UNKNOWN，35分钟后终止。
- Webhook 金额必须匹配 `gateway_amount`。
- 混合返利只按 `gateway_principal`。
- 混合退款稳定分摊和重试。

### PostgreSQL 集成测试

- 两个并发订单只能有一个成功冻结余额。
- 创建订单事务失败不扣余额、不留占用。
- 余额占用只能 capture 或 release 一次。
- 回调事务失败不发权益、不捕获余额。
- 重复回调只发一次权益。
- 取消、过期和 UNKNOWN 重试不会提前释放。
- 迟到回调只补偿一次。
- 支付宝退款成功后本地失败，重试不重复请求网关。

### 前端测试

- 余额为0、部分余额、足额余额时展示正确支付选项。
- 拆分金额和手续费显示正确。
- `CHECKOUT_CHANGED` 强制刷新并重新确认。
- 30分钟倒计时、取消确认中、补偿终态显示正确。
- 组合支付不进入余额充值页面。

### 回归验证

- 纯支付宝套餐/流量包。
- 纯余额套餐/流量包。
- 独立余额充值。
- 用户主动取消和系统过期。
- 支付回调恢复、退款和邀请返利。
- 前端 typecheck、build、embed 及后端完整 payment/service/repository 测试。

## 发布顺序

1. 合并并验证退款状态机修复，确保 `subscription_id` 和稳定退款状态已存在。
2. 备份 PostgreSQL 并验证 dump。
3. 发布只增加兼容字段和新表的迁移。
4. 发布兼容新旧订单的后端。
5. 发布前端组合支付入口。
6. 先用测试账号验证部分余额 + 小额支付宝组合支付。
7. 验证支付成功、主动取消、自然过期、UNKNOWN 和退款。
8. 观察订单、余额占用、补偿和支付错误日志后再全量开放。

## 验收标准

- 用户使用部分余额时，支付宝只显示并收取精确差额。
- 支付成功后只生成一笔商品订单和一份权益。
- 取消或明确未支付过期后，余额自动恢复。
- 支付状态不确定时余额最多占用约35分钟。
- 余额释放后的迟到付款不会再次扣余额，支付宝实付会转为站内余额。
- 重复回调、重复取消和后台重试不产生重复资金或权益。
- 退款分别原路退回余额和支付宝，手续费不退。
- 纯支付宝、纯余额和独立充值行为不回归。
