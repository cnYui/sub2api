# 套餐 F：198 元 / 每日 179 USD 订阅设计

## 背景

用户确认新增一个新的价格额度套餐 F：人民币 198 元，每日额度 179 USD。当前购买页由后端 `/payment/checkout-info` 返回的 `subscription_plans` 驱动，前端按套餐顺序自动显示“阅读订阅套餐A/B/C...”。现有在售订阅档位为 29/39/59/79/99 元，对应每日 19/29/49/69/89 USD。

## 目标

新增第六档订阅：

- 商品名：`198 元订阅池`
- 分组名：`codex-pool-179-usd`
- 基础价格：`198.00` 元人民币
- 每日额度：`179 USD`
- 有效期：30 天
- 刷新节奏：每日 24 点刷新
- 排序：`198`
- 前端展示：按现有索引规则自动成为“阅读订阅套餐F”

## 方案

采用最小数据驱动方案：新增一条幂等 migration，创建或更新 `groups` 与 `subscription_plans`，不修改支付履约链路。

- `groups` 新增或更新 `codex-pool-179-usd`，类型为 `openai / subscription`。
- `daily_limit_usd=179`，`weekly_limit_usd=NULL`，`monthly_limit_usd=NULL`。
- 图片价格、模型配置、消息派发、隐私/OAuth 要求等字段沿用现有 79/99 套餐模式，从 `codex-pool-49-usd` 读取模板值并设置默认兜底。
- `subscription_plans` 新增或更新 `198 元订阅池`，`price=198.00`，`for_sale=true`，`sort_order=198`。
- migration 不写入 `account_groups`，避免把公网当前的 `accounts.id=1` 或账号名称硬编码到通用源码。

## 前端影响

前端不需要硬编码 F 套餐。购买页会从 checkout API 获取新计划，并通过 `planTitleSuffix(index)` 生成“阅读订阅套餐F”。需要补回归测试覆盖：

- 六个订阅档位与余额充值卡、流量卡共同显示时，购买卡片数量正确。
- 第六档显示为“阅读订阅套餐F”，日限额为 `179刀`，基础价为 `¥198`。
- 选择 F 档后，下单 payload 使用 `plan_id`、`amount=198`、`order_type=subscription`。

## 后端影响

后端已有 `ListPlansForSale`、`validateSubOrder`、支付订单与订阅履约链路，均基于 `subscription_plans` 和 `group_id` 工作。新增套餐只需要补迁移与迁移回归测试：

- 新增 `161_seed_codex_198_subscription_plan.sql`。
- 测试断言迁移包含 `codex-pool-179-usd`、`198 元订阅池`、`daily_limit_usd = 179`、`price = 198.00`。
- 测试断言迁移不包含 `INSERT INTO account_groups` 或 `UPDATE account_groups`。

## 发布与运行态验收

历史上 79/99 新套餐曾因运行态未绑定上游账号而出现 `no available accounts`。因此上线 F 套餐时必须单独做运行态绑定验收：

- 发布新镜像，让公网 DB 自动应用 migration。
- 查询 `codex-pool-179-usd` 与 `198 元订阅池` 存在且在售。
- 将公网唯一 OpenAI 上游入口 `cliproxy-local-openai` 绑定到新 group，推荐通过后台/运维路径写 `account_groups`，并刷新调度快照。
- 用 active 自动 Key 或测试用户购买/分配 F 套餐后真实请求 `/v1/responses`，确认落库 `usage_logs.group_id` 指向新 group。

## 不做事项

- 不改订阅购买防重复逻辑。
- 不修改余额支付、支付宝、返利、流量卡逻辑。
- 不在源码 migration 中绑定公网账号。
- 不修改 nginx、Cloudflare Tunnel、CLIProxyAPI。

## 自检

- 无 TBD/TODO。
- 价格口径明确为基础价 `198.00`，手续费仍由现有前端和支付逻辑按运行态 `recharge_fee_rate` 计算。
- “代码上架”和“运行态上游绑定”职责分离，符合现有 79/99 套餐设计。
