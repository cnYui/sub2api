# 支付宝与余额组合支付设计自审修订

时间：2026-07-15 09:46 JST

## 关联文档

主设计：`docs/ai/context/20260715-093416-alipay-balance-hybrid-payment-design_CN.md`

本文件是主设计的自审修订。未明确修改的部分继续以主设计为准；若两者表述冲突，以本文件为准。

## 自审结论

设计范围可由一份实施计划完成，没有 `TBD`、`TODO` 或待定业务决策。单商品订单、余额占用、支付宝精确差额、30 分钟有效期、最多 5 分钟 `UNKNOWN` 确认窗口、迟到付款补偿、返利和按资金来源拆分退款的总体方案保持不变。

需要收紧以下两项技术约束。

## 支付与退款金额字段分离

主设计“支付宝验签、主动查询和退款只比较或使用 `gateway_amount`”的表述不够准确，修订为：

- 支付创建、Webhook 验签后的金额核对和主动支付查询必须使用订单的 `gateway_amount`。
- 支付宝退款请求必须使用首次退款申请时持久化的 `refund_gateway_amount`。
- `pay_amount` 只表示整笔商品订单含手续费总应付，不能被支付宝实付差额覆盖。
- `refund_gateway_amount` 不能在重试时由当前余额、当前套餐或当前日期重新计算。

对应不变量为：

```text
pay_amount = balance_amount + gateway_amount
refund_amount = refund_balance_amount + refund_gateway_amount
```

## Provider 初始化互斥

仅记录 `provider_init_attempted_at` 不能避免前台请求线程和后台扫描器同时调用 Provider。实现时必须增加显式初始化抢占状态和可恢复租约：

- `provider_init_status` 增加 `CREATING`。
- `payment_orders` 增加 `provider_init_lease_until TIMESTAMPTZ NULL`。
- 调用 Provider 前，通过数据库条件更新把 `NOT_STARTED` 或已确认可重试的状态原子地转为 `CREATING`，同时写入短租约。
- 只有成功抢占者可以调用 Provider；其他请求读取已有订单状态，不得并行创建。
- 创建成功后转 `CREATED` 并清除租约。
- 明确未创建时转 `FAILED` 并释放余额；结果不确定时转 `UNKNOWN`，先按同一 `out_trade_no` 查询，不能直接再次创建。
- 进程在 `CREATING` 中崩溃时，后台只在租约过期后接管；接管后先查询 Provider，确认不存在才使用原 `out_trade_no` 续接创建。

稳定 `out_trade_no` 是外部幂等键，数据库抢占和租约是本地并发边界，两者必须同时存在。

## 最终自审检查

- 范围：只支持支付宝与余额，不扩展其他外部渠道。
- 金额：商品本金、手续费、余额出资、支付宝出资和退款拆分定义无冲突。
- 状态：`PAID` 发放并捕获余额，`UNPAID` 释放，`UNKNOWN` 在截止前不发放也不释放。
- 超时：正常等待 30 分钟，只有支付结果无法确认时最多延长 5 分钟。
- 迟到付款：余额释放后不再扣款、不发原商品，支付宝实付全额补入站内余额。
- 并发：余额冻结由 PostgreSQL 事务保证，Provider 初始化由数据库抢占、租约和稳定外部订单号共同保证。
- 退款：手续费不退，余额本金退余额，支付宝本金按 `refund_gateway_amount` 原路退回。
- 依赖：必须先集成已完成的退款状态机修复，再实施组合支付。

本轮仅完成设计和自审，没有修改支付业务代码、数据库 schema 或运行态。
