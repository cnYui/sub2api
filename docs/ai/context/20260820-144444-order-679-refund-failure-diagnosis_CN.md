# 2026-08-20 订单 679 退款失败诊断

## 结论

订单 `679` 当前仍无法退款的直接原因是 ZPay 商户侧余额不足，不是本地退款报价、支付实例绑定或网络代理问题。2026-08-20 13:07 和 13:09 的两次退款请求均已从生产应用到达 `https://zpayz.cn/api.php?act=refund`，ZPay 返回 HTTP 200，但业务响应为 `卖家余额不足`。后端因此把订单保持为 `REFUND_FAILED`。

只有在 ZPay 商户账户补足可退款余额后，才有条件重试该订单；当前重试不会绕过商户资金限制。未执行第三次真实退款，也未修改订单、余额或套餐权益。

## 订单与支付实例证据

- 订单：`payment_orders.id=679`，用户 ID `628`，类型 `balance_subscription`，支付方式 `alipay`。
- 标价 `29.00`，实付 `29.29`，当前退款报价/请求金额 `28.15`。
- 原始支付单号：`payment_trade_no=2026081923001437861403903942`；商户订单号：`out_trade_no=sub2_20260819AbOrauoY`。
- 顶层 `provider_instance_id=1`，快照也保存 `provider_instance_id=1`、`provider_key=easypay`、`merchant_id=2026061815273954`，绑定可验证且没有路由到其他商户。
- 实例 1 为 `ZPay Alipay`，`refund_enabled=true`、`allow_user_refund=true`，配置 API 域名为 `https://zpayz.cn`。

## 生产日志证据

两次请求的关键日志均为：

```text
easypay refund failed (HTTP 200): 卖家余额不足
```

对应时间为 `2026-08-20 13:07:43 +08` 和 `2026-08-20 13:09:44 +08`。请求路径分别记录为 `POST /api/v1/payment/orders/679/refund-request`，状态码 `500` 是应用把网关业务失败映射为 `REFUND_FAILED` 后返回的结果。

当前应用容器环境中没有 `HTTP_PROXY`、`HTTPS_PROXY` 或 `ALL_PROXY`，仅有包含 `zpayz.cn` 的 `NO_PROXY/no_proxy`。因此 2026-08-19 修复的直连路径和生产配置已经生效；本次失败不是此前 `host.docker.internal:7897` 代理拒绝连接。

## 当前本地状态

- 订单状态：`REFUND_FAILED`。
- `failed_reason`：`easypay refund failed (HTTP 200): 卖家余额不足`。
- 退款失败审计已记录 1 条，订单 679 的历史订单创建、支付、首期到账和失败审计均存在。
- 余额套餐 `user_balance_packages.id=163` 已被管理员在 `2026-08-20 13:21:38 +08` 手动取消：状态 `cancelled`、剩余额度清零、后续到账停止；该操作没有执行网关退款，用户余额保持不变。这不会补足 ZPay 商户退款资金，也不会把订单变成已退款。

## 发现的次要缺陷

数据库迁移 `131_affiliate_rebate_hardening.sql` 为 `payment_audit_logs(order_id, action)` 建立唯一索引。退款失败处理仍使用普通 `Create()` 写入 `REFUND_FAILED`，所以同一订单第二次失败时会额外出现：

```text
duplicate key value violates unique constraint "idx_payment_audit_logs_order_action_uniq"
``` 

这不会改变 ZPay 已返回“卖家余额不足”的事实，但会导致新的失败审计详情无法写入，并在日志中产生误导性噪声。后续代码修复应让退款失败审计幂等（例如按订单和动作冲突时更新详情或安全忽略），同时保留最后一次网关错误。此次按用户请求仅做诊断，未修改代码。

## 后续动作边界

1. 先由支付商户确认并补足 ZPay 卖家余额。
2. 资金恢复后再逐笔重试 `REFUND_FAILED` 订单，先观察 ZPay 响应；不能把“卖家余额不足”订单当作代理故障重试。
3. 单独修复 `REFUND_FAILED` 审计写入的唯一索引冲突，并增加重复失败回归测试。
4. 本次未自动退款、未伪造成功状态、未恢复订单 679 已取消的套餐权益。

