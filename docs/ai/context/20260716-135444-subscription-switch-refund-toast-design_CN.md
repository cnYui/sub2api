# 跨套餐购买自助退款提示文案设计

## 背景

用户已有有效订阅并点击不同 `group_id` 的套餐时，购买页会在打开支付方式前拦截请求。当前提示仍要求用户联系管理员退款，但订阅退款已经支持在“我的订单”自助按比例处理，旧文案会误导用户。

## 决策

跨套餐购买提示统一改为：

`仅可续费当前套餐；购买新套餐前，请在“我的订单”按比例退款。`

英文对应为：

`You can only renew your current plan. Before buying a new plan, request a prorated refund in My Orders.`

## 范围

- 更新 `payment.errors.ACTIVE_SUBSCRIPTION_SWITCH_REQUIRES_REFUND`，这是购买页跨套餐预检实际使用的键。
- 同步更新旧兼容键 `payment.errors.ACTIVE_SUBSCRIPTION_EXISTS`，防止后端或旧路径返回该错误码时重新显示过时指引。
- 保留当前同 `group_id` 续费、跨套餐拦截、自动退款、按比例计算、订单状态机和后端校验。
- 不新增页面跳转、按钮、退款流程或接口调用。

## 验证

- 保留购买页测试，确认跨套餐仍不会打开支付方式或创建订单。
- 扩展现有语言资源测试，精确验证中英文的两个错误码均使用新的自助退款提示。
