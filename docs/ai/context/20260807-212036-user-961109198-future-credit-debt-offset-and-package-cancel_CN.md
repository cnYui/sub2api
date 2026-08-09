# 用户 961109198@qq.com 未到账套餐额度抵销欠费与取消记录

## 授权与范围

管理员授权对用户 `961109198@qq.com`（`users.id=506`）的余额套餐订单 `587` 执行人工结算：将尚未刷新的三期套餐额度优先抵销欠费，取消套餐，不发起现金退款。

## 执行前状态

- 套餐：`user_balance_packages.id=119`
- 每周额度：`128 USD`
- 已到账/总期数：`1/4`
- 未到账额度：`3 × 128 = 384 USD`
- 普通余额：`-430.90361296 USD`
- 套餐当前周剩余：`0 USD`
- 订单状态：`COMPLETED`，退款金额 `0 CNY`

## 执行结果

- 以 `384 USD` 抵销欠费后，普通余额为 `-46.90361296 USD`。
- 套餐状态改为 `cancelled`，`remaining_usd=0`，清空 `next_credit_at`；已到账次数保留为 `1/4`。
- 支付订单保持 `COMPLETED`，不调用支付网关、不产生现金退款。
- 写入 `balance_debt_ledger` 还款流水，来源为 `balance_package_future_credit_debt_offset`。
- 写入两条支付审计：`BALANCE_PACKAGE_FUTURE_CREDIT_DEBT_OFFSET` 与 `BALANCE_PACKAGE_CANCELLED`。
- 已清除该用户余额缓存；套餐查询不再返回有效套餐。
