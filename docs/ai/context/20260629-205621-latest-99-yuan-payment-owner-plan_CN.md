# 最近一笔 99 元账单付款人只读排查计划

## 目标

- 找出公网候选库中最近一笔 99 元附近的已支付/已完成账单是谁付的钱。
- 区分订单基础金额 `amount=99` 与含 1% 手续费的实付金额 `pay_amount≈99.99`。

## 范围

- 只读查询 `sub2api-candidate-postgres`。
- 不输出完整支付流水号、内部 token、完整 API Key 或密钥。
- 不修改公网应用、DB、Redis、nginx 或容器运行态。

## 查询口径

1. 查询 `payment_orders` 中 `status IN ('PAID','COMPLETED')` 且 `amount=99` 或 `pay_amount` 在 99 元附近的订单，按 `paid_at/completed_at/created_at` 倒序取最近记录。
2. Join `users`、`subscription_plans`、`groups`、`traffic_packs`，确认订单类型、套餐/流量包、付款用户邮箱。
3. 对比最近若干笔已支付订单，避免误把 99.99 实付价、99 基础价或非完成订单搞混。
4. 保存结果上下文。
