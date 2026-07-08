# 149/199 元订阅套餐替代 198 元本地方案设计

## 背景

上一轮本地未提交、未部署地新增了 `198 元订阅池 / codex-pool-179-usd / 每日 179 USD`。用户随后确认调整为：

- 价格修改为 `199 元 / 179刀`
- 再新增 `149 元 / 135刀`

因为上一轮 198 元方案尚未提交、未构建、未部署、公网 DB 未应用，所以本轮不需要新增修正迁移，也不应让新环境先 seed 198 再改 199。

## 目标

最终本地代码只表达两个新增套餐：

| 前端档位 | 商品名 | group | 基础价 | 日额度 | 排序 |
| --- | --- | --- | --- | --- | --- |
| F | `149 元订阅池` | `codex-pool-135-usd` | `149.00` | `135 USD` | `149` |
| G | `199 元订阅池` | `codex-pool-179-usd` | `199.00` | `179 USD` | `199` |

完整购买页排序为：29、39、59、79、99、149、199。

## 方案

采用替换未发布迁移的方案：

- 删除未提交的 `backend/migrations/161_seed_codex_198_subscription_plan.sql`。
- 新增 `backend/migrations/161_seed_codex_149_199_subscription_plans.sql`。
- 在同一个 migration 中循环 seed 两档，避免复制大段 SQL。
- 两档都只写 `groups` 与 `subscription_plans`，不写 `account_groups`。
- 上游账号绑定仍作为公网发布后的运行态步骤：把 `cliproxy-local-openai` 分别绑定到 `codex-pool-135-usd` 与 `codex-pool-179-usd`。

## 前端影响

前端购买页本身是数据驱动的，不需要改生产组件。测试 fixture 改为七个订阅档位：

- `149 元订阅池` 显示为“阅读订阅套餐F”，下单 `amount=149/plan_id=6`。
- `199 元订阅池` 显示为“阅读订阅套餐G”，下单 `amount=199/plan_id=7`。

## 后端影响

后端支付、订阅履约、余额支付、返利逻辑不变。只更新迁移与迁移回归测试：

- 测试断言 migration 包含两档的 group、商品名、日额度、价格。
- 测试断言 migration 不包含 `INSERT INTO account_groups` 或 `UPDATE account_groups`。

## 不做事项

- 不构建镜像。
- 不部署公网 18084。
- 不写公网 DB。
- 不改 nginx、Redis、Cloudflare Tunnel、CLIProxyAPI。
- 不删除历史上下文文档；新增文档说明 198 元本地方案已被替代。

## 自检

- 无 TBD/TODO。
- 价格口径明确为基础价，手续费继续由现有运行态配置计算。
- 迁移命名与内容只保留最终想要的新档位，不暴露未发布中间态。
