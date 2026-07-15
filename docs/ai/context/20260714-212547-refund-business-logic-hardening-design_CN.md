# 退款业务逻辑修复设计

时间：2026-07-14 21:25 JST

## 目标

在不迁移旧 `yui.web` 消费记录的前提下，修复当前月度套餐退款链路：

- 支付宝和余额支付都以不含手续费的 `payment_orders.amount` 为退款基数。
- 按北京时间自然日计费，购买当天算第 1 天；进入第 6 天后固定按已用 6 天、剩余 24 天计算。
- 网关退款与订阅撤销分阶段持久化，失败重试不能重复退款。
- 新订单持久化准确的 `user_subscriptions.id`，退款不再按用户和分组猜测当前订阅。
- 余额退款、订阅撤销和订单完成在同一数据库事务内提交。

## 不在范围内

- 不导入旧 SQLite 的 `orders`、`subscription_orders` 或历史退款记录。
- 不把兑换码、赠送、管理员直加伪造成支付订单。
- 不改变流量卡不可自助退款的规则。
- 不调整套餐价格、1% 手续费收取规则、支付渠道配置或已有退款金额。
- 不自动重试当前运行库中的历史 `REFUND_FAILED` 订单。

## 方案比较

### 方案一：只改退款金额公式

改动最小，但无法解决以下已确认风险：网关成功后撤权失败会落为普通 `REFUND_FAILED`，后续重试可能再次调用网关；缺交易号时可能跳过网关却继续撤权；订单仍不能定位准确订阅。

不采用。

### 方案二：增强现有订单退款状态机（采用）

继续以 `payment_orders` 为退款聚合根，增加退款请求幂等标识、网关状态、权益状态、网关退款编号和订阅关联。用户与管理员 API 保持现有路由，服务层按持久化状态决定下一步动作。

该方案直接解决当前风险，改动集中在现有支付服务、Provider 和订单模型中，不引入第二套退款系统。

### 方案三：新增独立退款聚合与异步对账系统

长期边界最清晰，但需要新表、后台任务、Provider 查询接口、管理控制面和对账流程。本轮只有少量同步 ZPay 自动退款，完整系统成本过高。

本轮不采用；若后续开放 Stripe、官方支付宝或微信异步自动退款，再独立设计。

## 数据模型

在 `payment_orders` 增加：

- `subscription_id BIGINT NULL REFERENCES user_subscriptions(id) ON DELETE SET NULL`
- `refund_request_id VARCHAR(128) NULL`
- `refund_gateway_status VARCHAR(20) NOT NULL DEFAULT 'NOT_STARTED'`
- `refund_entitlement_status VARCHAR(20) NOT NULL DEFAULT 'NOT_STARTED'`
- `refund_provider_ref VARCHAR(128) NULL`

网关状态：

- `NOT_STARTED`：尚未请求外部网关。
- `NOT_REQUIRED`：余额支付，不需要外部网关。
- `PENDING`：网关明确返回处理中，不能撤销权益。
- `SUCCEEDED`：网关已确认退款成功，后续重试只能处理权益。
- `FAILED`：网关明确拒绝，确认未退款，可使用同一请求号重试。
- `UNKNOWN`：请求结果无法确认，禁止自动重试，必须人工核验。

权益状态：

- `NOT_STARTED`：尚未撤销。
- `SUCCEEDED`：订阅已撤销或已确认此前撤销完成。
- `FAILED`：资金已退但撤销失败，重试只能撤销权益。

现有订单 `status` 继续作为页面生命周期状态：

- 处理中使用 `REFUNDING`。
- 明确失败或需人工处理使用 `REFUND_FAILED`。
- 最终使用 `PARTIALLY_REFUNDED` 或 `REFUNDED`。

## 当前订单关联迁移

只回填已经存在于当前 `payment_orders` 的订阅订单，不导入旧系统订单：

- 从 `user_subscriptions.notes` 中匹配 `payment order <id>`，要求用户和分组同时一致。
- 唯一匹配时写入 `payment_orders.subscription_id`。
- 无匹配或多匹配时保持 NULL，退款时失败关闭并提示管理员修复关联。
- 新订单履约时直接保存 `AssignOrExtendSubscription()` 返回的订阅 ID，不再依赖 notes 作为财务关联。

## 按天退款计算

统一使用北京时间固定时区 UTC+8，不依赖服务器本地时区：

```text
used_days = 北京时间(now.date - subscription.starts_at.date) + 1
used_days = clamp(used_days, 1, subscription_days)
remaining_days = subscription_days - used_days
refund_amount = round_to_0.1(order.amount * remaining_days / subscription_days)
```

边界：

- 购买当天立即退款：已用 1 天，30 天套餐最多退 29/30。
- 进入第 6 个北京时间自然日：已用 6 天，剩余 24 天。
- 第 30 天：剩余 0 天，不可退款。
- 支付宝和余额支付都不退手续费，统一使用 `order.amount`。

首次发起退款时计算并持久化 `refund_amount`。失败重试复用原金额，不因重试跨日而减少。

## 用户自动退款流程

### 支付宝 / EasyPay

1. 校验订单归属、套餐类型、Provider 资格和准确 `subscription_id`。
2. 对 `COMPLETED` 首次申请生成稳定 `refund_request_id`，计算退款金额，原子锁为 `REFUNDING`。
3. 用稳定请求号调用 Provider；EasyPay 继续用原 `out_trade_no`，官方支付宝、微信和 Stripe 使用稳定幂等键。
4. 网关明确成功后，先持久化 `refund_gateway_status=SUCCEEDED` 和 `refund_provider_ref`。
5. 再撤销准确订阅；成功后写最终退款状态。
6. 网关明确拒绝时写 `FAILED`；用户可复用同一金额和请求号重试。
7. 网络超时、无法解析或未知响应写 `UNKNOWN`，不允许用户自动重试。
8. 网关已成功但撤权失败时写 `refund_entitlement_status=FAILED`；重试只撤权，不再次请求网关。

### 余额支付

余额增加、订阅撤销、订单最终状态和审计日志放进同一 Ent 事务：

- 任一步失败则全部回滚。
- 成功后统一失效余额和订阅缓存。
- `refund_gateway_status=NOT_REQUIRED`，`refund_entitlement_status=SUCCEEDED`。

## Provider 幂等

`payment.RefundRequest` 增加 `RequestID`：

- 官方支付宝：`OutRequestNo=RequestID`。
- 微信支付：`OutRefundNo=RequestID`。
- Stripe：设置 Idempotency-Key。
- Airwallex：使用 `RequestID` 作为 request_id。
- EasyPay API 不支持独立幂等键，仍使用订单 `out_trade_no`；只有明确业务拒绝才允许自动重试，传输结果不确定时进入 `UNKNOWN`。

缺少 `payment_trade_no` 不再被视为退款成功。只要 `out_trade_no` 可用且 Provider 支持，就正常发起；两个标识都为空时直接失败关闭。

## 前端行为

订单响应增加 `refund_retryable`：

- `COMPLETED` 且满足现有退款条件时显示“申请退款”。
- `REFUND_FAILED` 且后端确认是网关明确失败或权益撤销失败时显示“重试退款”。
- `UNKNOWN`、`PENDING` 或不可关联订阅时不显示重试按钮。

前端不自行推断网关状态，退款资格以服务端结果为准。

## 测试

后端 TDD 覆盖：

- 北京时间购买当天、第 6 天、第 30 天和跨午夜。
- 支付宝与余额支付都以 `amount` 为基数。
- 首次失败后跨日重试复用原退款金额。
- 网关成功、撤权失败后重试不再次调用 Provider。
- 网关明确拒绝可重试；未知结果不可自动重试。
- `pending` 不撤权、不标记退款成功。
- 缺 `payment_trade_no` 但有 `out_trade_no` 时仍调用 EasyPay。
- 余额退款事务任一步失败均不增加余额、不撤销订阅、不完成订单。
- 新订单履约写入 `subscription_id`，迁移只回填当前订单。

前端覆盖：

- `refund_retryable=true` 的 `REFUND_FAILED` 显示重试按钮。
- 不可重试失败和处理中状态隐藏按钮。

## 发布约束

- 先迁移 schema，再发布兼容新字段的应用镜像。
- 发布前备份 PostgreSQL 并验证 dump。
- 本轮不自动处理运行库已有失败订单；发布后逐单核验。
- 不修改旧 SQLite 或创建旧订单回填记录。
