# xinlise@gmail.com 退款天数只读审计计划

## 背景

用户要求核算 `xinlise@gmail.com` 实际已使用几天、剩余多少天；该用户今天额度已用满，管理员准备退款。

## 口径

- 只读查询运行态 `sub2api-candidate-postgres`。
- 以用户当前未删除 active 订阅为主，核对 `starts_at`、`expires_at`、`daily_limit_usd`、`daily_usage_usd` 与支付订单。
- 用北京时间/运行态业务口径核算完整剩余天数；退款金额参考当前自动退款规则：月度套餐按剩余完整天数比例退款，支付宝只退基础价 `amount`，不退 1% 手续费。
- 对照 usage_logs 聚合，确认今天额度是否确实用满，以及历史实际活跃天数。

## 不做

- 不执行退款。
- 不修改用户、订阅、订单、usage 或 Redis。
- 不重启容器、不发布代码。
