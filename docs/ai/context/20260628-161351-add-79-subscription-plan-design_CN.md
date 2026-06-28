# 79 元订阅套餐上架设计

## 背景

用户确认新增 79.79 元套餐，日限额为 69 USD，对应新分组 `codex-pool-69-usd`。

现有购买链路已经具备：

- `/purchase` 通过 `/api/v1/payment/checkout-info` 获取 `subscription_plans` 中 `for_sale=true` 的套餐。
- 用户选择套餐后复用现有支付下单逻辑，订单类型为 `subscription`，请求携带 `plan_id`。
- 支付完成后由现有履约逻辑把套餐绑定到用户账户。
- `/admin/orders/plans` 通过 `/api/v1/admin/payment/plans` 读取同一张 `subscription_plans` 表。

因此本次不应该在前端硬编码套餐卡片，也不应该复制支付或履约逻辑。

## 方案

采用方案 A：新增幂等迁移 `156_seed_codex_79_subscription_plan.sql`。

迁移职责：

- 新增或更新 `groups.name='codex-pool-69-usd'` 的订阅型 OpenAI 分组。
- 设置 `daily_limit_usd=69`、`default_validity_days=30`、`allow_image_generation=true`。
- 新增或更新 `subscription_plans` 中的 `79 元订阅池`。
- 设置价格 `79.79`、有效期 30 天、`for_sale=true`、排序 `79`。
- 不插入或更新 `account_groups`，避免误绑定上游账号。

## 取舍

- 后端 seed 是唯一事实源，admin 和用户购买页自动复用同一数据。
- 前端只补测试 fixture，证明 5 个套餐时仍复用现有卡片和下单链路。
- 不修改 155 历史 baseline，避免已应用迁移 checksum 风险；新增 156 更安全。
- 新增迁移会让新环境自动拥有 79 套餐；已运行的 18084 环境需要部署新镜像或手动应用迁移后才会出现。

## 验收

- 迁移文件包含 `codex-pool-69-usd`、`79 元订阅池`、`79.79`、`daily_limit_usd = 69`。
- 迁移文件不包含 `INSERT INTO account_groups` 或 `UPDATE account_groups`。
- `/purchase` 测试覆盖 5 个套餐，并验证 79 套餐创建订阅订单时携带 `plan_id`、`amount=79.79`、`payment_type=alipay`。
- 后端迁移回归测试通过。
