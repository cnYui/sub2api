# ZPay 1% 手续费运行态配置计划

## 背景

- ZPay 每笔支付会从商户账户扣 1% 手续费。
- 如果用户仍按套餐原价支付，商户 ZPay 余额不足时会出现“ZPAY账户余额不足，无法发起支付，请提前充值余额，抵扣手续费使用。”
- 当前系统已有统一手续费配置 `RECHARGE_FEE_RATE`：
  - 后端创建订单时用 `cfg.RechargeFeeRate` 计算 `payment_orders.pay_amount`。
  - 前端 `/purchase` 使用 `checkout.recharge_fee_rate` 展示“手续费”和“实付金额”。
  - 订单详情页也会展示 `fee_rate` 和 `pay_amount`。

## 决策

- 不修改套餐/流量包的基础价格字段。
- 将运行态 `RECHARGE_FEE_RATE` 设置为 `1`。
- 结果表现：
  - 29 元订阅池基础价仍是 29 元。
  - 前端确认页显示手续费 `1%` 和实付 `¥29.29`。
  - 后端订单 `amount=29.00`、`fee_rate=1`、`pay_amount=29.29`。
  - ZPay 收银台金额使用 `pay_amount`。

## 执行步骤

1. 备份当前配置值：
   - 查询 `settings.RECHARGE_FEE_RATE`。
2. 写入运行态配置：
   - `INSERT INTO settings (key,value,updated_at) VALUES ('RECHARGE_FEE_RATE','1',now()) ON CONFLICT ...`
3. 验证：
   - 查询 settings 确认为 `1`。
   - 调 `/api/v1/payment/checkout-info` 或刷新 `/purchase`，确认 `recharge_fee_rate=1`。
   - 新建测试订单时确认 `pay_amount = amount * 1.01`，小数按支付币种最小单位向上取整。

## 回滚

如需取消手续费：

```sql
UPDATE settings SET value='0', updated_at=now() WHERE key='RECHARGE_FEE_RATE';
```
