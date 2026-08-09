# 用户 853436957@qq.com 提前刷新套餐额度抵销欠费记录

管理员授权对用户 `853436957@qq.com`（`users.id=490`）提前发放下一周余额套餐额度，并按正常周刷新规则优先抵销欠费。

## 执行前

- 余额套餐：`user_balance_packages.id=15`，订单 `548`
- 套餐状态：`active`，第 `1/4` 期
- 每周额度：`102 USD`，当前窗口剩余：`0 USD`
- 原定下次刷新：2026-08-10 09:37:42 +08
- 普通余额：`-64.75364343 USD`
- 订单：`COMPLETED`，退款金额 `0 CNY`

## 执行结果

- 在串行化事务中将套餐提前刷新至第 `2/4` 期。
- 本周 `102 USD` 额度先抵销 `64.75364343 USD` 欠费，余额和本周套餐剩余均为 `37.24635657 USD`。
- 下次刷新顺延一周至 2026-08-17 09:37:42 +08；套餐保持 `active`，订单保持 `COMPLETED`，未退款、未取消。
- 写入 `balance_debt_ledger` 还款流水，来源 `balance_package_forced_early_credit_debt_offset`、引用 `package:15:credit:2`。
- 写入订单 `548` 的 `BALANCE_PACKAGE_FORCED_EARLY_CREDIT_2` 审计；已清除该用户余额缓存键（键不存在时返回 0）。
