# 自动套餐退款实施计划

时间：2026-07-09 10:04 JST

## 必做任务

1. 后端服务层 TDD：
   - 新增自动退款金额计算测试。
   - 覆盖支付宝套餐：`amount=29`、`pay_amount=29.29`、还剩 25 天，网关退款金额应为 `24.2`，不退手续费。
   - 覆盖余额套餐：`amount=29`、`pay_amount=29.29`、还剩 25 天，余额退回金额应为 `24.4`，退手续费。
   - 覆盖还剩 `24.2` 天按 `24` 天算。
   - 覆盖流量卡退款拒绝。
2. 后端实现：
   - 增加套餐退款金额计算 helper：按剩余完整天数和一位小数四舍五入。
   - 改 `RequestRefund()`：从“写入 REFUND_REQUESTED”改为“自动执行退款”。
   - 保留管理员退款入口，但让默认金额规则符合新口径。
   - 新增余额退款路径：余额支付的套餐订单退回余额，不调用网关。
   - 支付宝退款路径复用 `gwRefund()`，但网关金额使用新计算结果。
3. 运行态配置：
   - 更新 `sub2api-candidate-postgres` 中 `ZPay Alipay` provider：
     - `enabled=true`
     - `supported_types=alipay`
     - `payment_mode=popup`
     - `refund_enabled=true`
     - `allow_user_refund=true`
     - `apiBase=https://zpayz.cn`
   - PID/KEY 写入加密 config，文档和输出只记录“已写入脱敏配置”。
4. 验证：
   - 目标 unit 测试。
   - 相关 payment service unit 测试。
   - `git diff --check`。
   - 只读查询运行态 provider 开关。

## 风险控制

- 不对 `traffic_pack` 做退款。
- 不对非支付宝、非余额的套餐订单开放用户自动退款。
- 网关失败不取消套餐。
- 余额退款撤销套餐失败时尝试扣回余额，并写审计日志。
- 不构造真实退款测试，除非用户明确指定某个订单执行真实退款。
