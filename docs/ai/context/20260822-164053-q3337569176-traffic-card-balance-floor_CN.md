# q3337569176@163.com 流量卡切换前余额保底处理

## 根因

生产用户 `users.id=529`（`q3337569176@163.com`）的普通余额为 `-2.93420205 USD`，当前唯一 API Key `id=147` 绑定标准 OpenAI 分组 `groups.id=13`。用户在 2026-08-22 15:30 左右连续收到 HTTP 403，但没有进入上游或流量卡扣费。

当前源码 `backend/internal/server/middleware/api_key_auth.go:196-199` 仍保留旧逻辑：只要普通余额小于 `0`，就在流量卡资格判断前直接返回 `INSUFFICIENT_BALANCE`。8 月 5 日的流量卡自动切换修复只补充了后面的 `CanUseTrafficPackCredit` 判断，8 月 8 日新增的负余额硬拦截没有同步移除，因此负余额用户即使有流量卡也无法到达切换逻辑。

## 生产数据快照

- 余额套餐：`user_balance_packages.id=32`、`91` 均已过期且剩余 `0`。
- 流量卡：`user_traffic_credits.id=384` 剩余 `1.0019652200 USD`；`id=438`、`439`、`440` 各剩余 `30 USD`，均未过期且平台为 `all`。
- 有效流量卡合计：`91.0019652200 USD`。
- 调整前没有 `traffic_credit_debt_ledger` 或 `balance_debt_ledger` 欠费流水。
- 调整前最新流量卡扣费发生于 `2026-08-22 14:37:21`，新卡购买后没有扣费流水，证明用户未能进入流量卡结算。

## 本次按管理员要求执行

- 在 PostgreSQL 事务中锁定用户 `529`，将 `users.balance` 从 `-2.93420205` 设置为 `0.01000000 USD`。
- 未修改余额套餐、流量卡额度、订单、历史用量、累计充值金额或任何欠费账本。
- 新增 `payment_audit_logs.id=2030`：`action=BALANCE_MANUAL_FLOOR`，操作者 `admin:authorized_manual_balance_floor`，记录变更前后余额和未修改权益。
- 清理了 API Key 认证缓存 `apikey:auth:<user-key-hash>` 和余额缓存 `billing:balance:529`；进程内认证缓存等待短 TTL 自然过期。

## 当前结果与后续验证

调整后用户余额已核验为 `0.01000000 USD`，有效流量卡合计仍为 `91.0019652200 USD`。调整后的等待窗口内没有用户新请求，因此尚未产生新的流量卡扣费流水；用户下一次请求需观察 `traffic_credit_ledger.entry_type='deduction'` 和对应 `usage_logs.billing_type` 是否落账。

本次余额保底是临时数据处理，源码中的负余额硬拦截仍会影响其他负余额用户；应另行修复为让流量卡资格判断先于负余额拒绝，并增加“负余额 + 有效全渠道流量卡”的认证回归测试。
