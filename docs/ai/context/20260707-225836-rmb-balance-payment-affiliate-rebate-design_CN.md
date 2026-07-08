# 人民币余额支付与邀请返利重构设计

## 背景

本地 `main` 已有真实站内余额：支付宝/EasyPay 回调可完成 `order_type=balance` 余额充值订单，并把金额加入 `users.balance`。当前缺口是：用户不能用站内余额购买套餐或 GPT 流量包；`/purchase` 只走外部支付订单。

本设计采用方案 A：新增余额支付专用后端路径，复用现有订单表，不把余额支付伪装成第三方支付回调。

## 已确认业务规则

- `users.balance` 是人民币余额，前端和订单展示使用人民币符号 `¥`。
- 商品价格均按人民币定价扣款；GPT 流量包里的 `credit_usd` 只是 API 计费用美元额度，不是余额币种。
- 用户侧支付方式只保留“支付宝”和“余额”；微信、Stripe、Airwallex 从用户购买/充值页面隐藏，后台 provider 配置和历史订单保留。
- 支付宝直购套餐/流量包保留。
- 余额支付购买套餐/流量包保留。
- 两种支付方式不能混合支付。余额不足时不创建商品订单，直接引导用户充值。
- 余额充值只允许支付宝支付，不显示余额支付，也不显示微信、Stripe、Airwallex。
- 余额充值金额必须是 1-100 的整数，默认 1 元；非法输入显示错误并禁用付款按钮，不自动纠正用户输入。
- 余额充值不收手续费：`amount=pay_amount=充值金额`，`fee_rate=0`。
- 产品购买收手续费：`amount=商品人民币基础价`，`pay_amount=含手续费应付价`，余额支付也扣 `pay_amount`。
- 余额支付购买套餐/流量包必须生成订单记录，用户 `/orders` 页面和后台订单页都能完整展示。
- 余额支付订单不开放新的用户自助退款能力；已有支付宝余额充值退款流程不在本设计中改变。

## 邀请返利规则

- 全局返利比例改为 8%。
- 返利冻结期为 24 小时。
- 返利有效期为被邀请用户注册后 365 天。
- 单个被邀请用户累计返利上限为 ¥100。
- 支付宝余额充值产生返利，返利基数为充值本金 `amount`。
- 支付宝直购月度套餐产生返利，返利基数为套餐人民币基础价 `amount`，例如 2/3/5/29/39/59/79/99，而不是 29.29/39.39 这类含手续费金额。
- 支付宝直购 GPT 流量包产生返利，返利基数为流量包人民币售价 `amount`。
- 余额支付购买套餐/流量包不产生返利。
- 返利判断不能只看 `order_type`，必须同时看 `payment_type`。只有 `payment_type=alipay` 且订单已真实完成时才产生返利。
- 用户专属返利比例 `aff_rebate_rate_percent` 继续保留；没有专属比例时使用全局 8%。

## 前端设计

### `/purchase` 商品列表

保留当前定制化卡片布局，不大改视觉样式。

商品列表第一张固定为“余额充值”卡片，后续原有套餐卡、流量包卡按原顺序后移。余额充值卡片复用 `PurchaseProductCard` 的整体样式，只替换文案：

- 标题：余额充值
- 价格：¥1 起
- 按钮：立即充值
- 详情：用途=购买套餐/流量包，到账=实时到账，手续费=无

点击“立即充值”进入余额充值确认视图，复用现有套餐/流量包确认页结构：

- 顶部卡片展示“余额充值”
- 卡片内提供金额输入框
- 输入范围：1-100 元整数
- 默认值：1
- 支付方式区域只显示支付宝
- 金额明细只展示充值金额和实付金额，二者相等；不展示产品购买手续费
- 主按钮文案：确认支付 ¥N
- 返回按钮回到商品卡片列表

余额不足时，直接切到余额充值确认视图：

- 默认充值金额为 `ceil(缺口金额)`
- 默认金额最小 1，最大 100
- 如果缺口超过 100，提示“单次最多充值 100 元，请分多次充值”

### 产品确认页

套餐和流量包确认页支付方式显示两项：

- 支付宝
- 余额

余额支付和支付宝支付不能合并。选择余额支付时：

- 前端可先用当前用户余额做预检查
- 余额不足时不请求创建商品订单，直接进入余额充值确认视图
- 余额足够时调用余额支付专用后端接口

选择支付宝支付时继续走现有 `POST /api/v1/payment/orders` 创建外部支付订单。

### 订单页面

用户 `/orders` 页面继续复用现有 `OrderTable`。

需要补充展示口径：

- `payment_type=balance` 显示为“余额”
- `order_type=traffic_pack` 显示为“流量包”
- `order_type=balance` 显示为“余额充值”
- 金额符号使用 `currency=CNY` 或默认人民币，不再把余额入账金额显示为 `$`
- 余额支付订单状态直接展示 `COMPLETED`

后台订单筛选需要补上 `traffic_pack` 订单类型；现状只列了 `balance` 和 `subscription`。

## 后端设计

### 订单类型白名单

修复现有 `order_type` 防护不足：

- 空 `order_type` 为兼容历史请求可继续默认 `balance`
- 非空时只能是 `balance`、`subscription`、`traffic_pack`
- 非法值返回 `INVALID_ORDER_TYPE`
- 履约分发不能再让未知 `order_type` 落到余额充值履约

### 支付方式

新增内部支付方式：

```text
payment_type=balance
```

它只用于站内余额支付商品，不调用第三方支付 provider，不进入 webhook 流程。

用户侧外部支付入口只允许支付宝。后台 provider 配置、webhook、历史订单不删除。

### 余额充值订单

余额充值仍使用现有外部支付订单链路：

```text
order_type=balance
payment_type=alipay
amount=充值金额
pay_amount=充值金额
fee_rate=0
```

支付宝回调成功后继续走现有余额履约，把 `amount` 加入 `users.balance`。

需要调整现有金额计算：`order_type=balance` 永远不套产品手续费；旧的 `recharge_fee_rate` 只用于产品购买。

### 余额购买商品接口

新增余额支付专用用户接口，建议放在现有订单路由下：

```text
POST /api/v1/payment/orders/balance-pay
```

请求体：

```json
{
  "order_type": "subscription",
  "plan_id": 1
}
```

或：

```json
{
  "order_type": "traffic_pack",
  "traffic_pack_id": 3
}
```

不接受 `order_type=balance`，因为余额不能给自己充值。

服务端事务内完成：

1. 校验用户 active。
2. 校验商品可售。
3. 计算 `amount=商品基础价`。
4. 计算 `pay_amount=商品基础价 + 产品手续费`，与支付宝直购一致。
5. 条件扣减 `users.balance >= pay_amount`。
6. 创建 `payment_orders`：
   - `payment_type=balance`
   - `order_type=subscription` 或 `traffic_pack`
   - `amount=商品基础价`
   - `pay_amount=含手续费金额`
   - `fee_rate=产品手续费率`
   - `status=COMPLETED`
   - `paid_at/completed_at=now`
   - `out_trade_no` 和 `recharge_code` 仍按现有订单格式生成，方便排查
7. 发放套餐或流量包。
8. 写支付审计日志。
9. 清理用户余额缓存，并刷新用户信息。

余额不足返回：

```text
BALANCE_INSUFFICIENT
```

余额不足时不扣款、不发货、不生成完成订单。

### 履约复用

余额支付商品不能调用现有 `executeFulfillment` 的外部支付状态流。建议抽出内部发货能力：

- `fulfillSubscriptionOrderInTx`
- `fulfillTrafficPackOrderInTx`

外部支付回调和余额支付都复用发货逻辑，但只有外部支付宝订单进入邀请返利逻辑。

### 邀请返利设置

需要把全局默认与运行态 settings 调整为：

```text
affiliate_rebate_rate=8
affiliate_rebate_freeze_hours=24
affiliate_rebate_duration_days=365
affiliate_rebate_per_invitee_cap=100
```

建议迁移或部署步骤对这些 key 做显式 upsert，保证当前运行库生效。

### 余额币种口径

当前代码和文案仍有“余额为 USD”或 `$` 的历史残留，需要改为人民币：

- 用户余额展示
- 余额充值/兑换提示
- 邀请返利金额
- 订单列表里的入账金额
- 管理端用户余额列和余额历史
- 低余额提醒文案
- `balance_recharge_multiplier` 相关文案需要废弃或固定为 1，不再表达“1 CNY = X USD 余额”

## 错误处理

- 非法 `order_type`：`INVALID_ORDER_TYPE`
- 非法充值金额：`INVALID_AMOUNT`
- 余额不足：`BALANCE_INSUFFICIENT`
- 商品不可售：沿用 `PLAN_NOT_AVAILABLE` / `TRAFFIC_PACK_NOT_AVAILABLE`
- 余额支付中任一步失败：事务回滚，不扣款、不发货、不生成完成订单
- 前端收到 `BALANCE_INSUFFICIENT`：直接打开余额充值确认页并填入缺口金额

## 测试范围

后端需要 TDD 覆盖：

- 非空非法 `order_type` 被拒绝，不会落到余额履约。
- `order_type` 为空仍默认余额充值。
- 余额充值订单 `fee_rate=0`、`amount=pay_amount`。
- 支付宝套餐订单返利基数为 `amount`，不是 `pay_amount`。
- 支付宝流量包订单产生返利。
- 余额支付套餐订单不产生返利。
- 余额支付流量包订单不产生返利。
- 余额不足返回 `BALANCE_INSUFFICIENT`，且不扣款、不发货、不生成完成订单。
- 余额足够时扣 `pay_amount`，生成 `payment_type=balance` 完成订单，并发放对应权益。
- 单个被邀请用户累计返利上限 ¥100、冻结 24 小时、有效期 365 天生效。

前端需要覆盖：

- `/purchase` 第一张卡是余额充值卡，其它商品后移。
- 余额充值确认页默认金额为 1。
- 输入 0、小数、101、空值时按钮禁用并展示错误。
- 输入 1-100 整数时只显示支付宝，创建 `order_type=balance` 订单。
- 产品确认页只显示支付宝和余额。
- 余额不足时打开充值确认页，金额为缺口向上取整。
- 订单表正确显示 `payment_type=balance` 与人民币金额。
- 用户侧不展示微信、Stripe、Airwallex。

## 非目标

- 不删除后台 provider 配置。
- 不删除历史微信、Stripe、Airwallex 订单和 webhook。
- 不新增余额支付订单的用户自助退款。
- 不实现混合支付。
- 不改变 API 用量计费美元额度逻辑。

## 自检

- 本设计没有要求用美元余额购买人民币商品；余额统一为人民币。
- 本设计没有让返利余额再次产生返利；余额支付商品不返利。
- 本设计没有用 `pay_amount` 作为产品返利基数；返利按商品基础价或充值本金 `amount`。
- 本设计包含此前发现的两个小修复：`order_type` 白名单、后台订单筛选补 `traffic_pack`。
