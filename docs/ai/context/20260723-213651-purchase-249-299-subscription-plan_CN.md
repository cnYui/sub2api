# /purchase 新增 249/299 元订阅套餐计划

## 背景

用户要求在当前项目 `/purchase` 页面新增两个订阅套餐：

- 249 元 + 1% 手续费
- 299 元 + 1% 手续费

现有购买页套餐来自后端 `/api/v1/payment/checkout-info`，基础价写在 `subscription_plans.price`；1% 手续费由现有 `checkout.recharge_fee_rate` 统一计算，不应把基础价写成含手续费价格。

## 周限额推导

当前公共 Codex 订阅周额度：

- 29 元：76 USD/周
- 199 元：520 USD/周

按 29 元到 199 元端点线性外推：

```text
slope = (520 - 76) / (199 - 29) = 2.6117647059
limit(price) = 76 + (price - 29) * slope
```

结果：

- 249 元：650.588...，按当前整数 USD 展示/配置口径取 651 USD/周
- 299 元：781.176...，按当前整数 USD 展示/配置口径取 781 USD/周

## 实现范围

- 新增数据库 migration seed 两个分组和订阅计划：
  - `codex-pool-651-usd` / `249 元订阅池` / `price=249.00` / `weekly_limit_usd=651`
  - `codex-pool-781-usd` / `299 元订阅池` / `price=299.00` / `weekly_limit_usd=781`
- 后端公共 Codex 周额度白名单加入两档，保证下单快照、管理端规范化和 API 展示一致。
- 前端 `subscriptionQuota` 映射加入两档，保证 `/purchase` 在旧 payload 或缺少字段时仍能展示周额度。
- 补测试：migration 内容、后端额度映射、前端映射与 `/purchase` 套餐卡展示。

## 不做

- 不修改已存在历史订单、已激活权益段或 usage 事实。
- 不修改现有 1% 手续费算法。
- 不触碰公网运行态数据库、容器或 Nginx。
