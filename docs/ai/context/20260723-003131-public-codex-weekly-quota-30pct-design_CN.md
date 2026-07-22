# 公共 Codex 周额度提升 30% 设计

## 背景

用户希望在价格不变的前提下，把当前公开 Codex 订阅档位的周额度整体提升约 30%，使用整数额度，并以 `58 -> 76 USD` 这种形式展示变化。本次需求已经确认：

- 只覆盖 7 个公开 Codex 订阅档位。
- `codex-pool-local-unlimited` 和其他分组不变。
- 套餐价格保持 `29 / 39 / 59 / 79 / 99 / 149 / 199 CNY` 不变。
- 当前用户今天、本周、历史已用额度记录全部保留，只提高上限。
- 公网数据不能丢失，发布前必须备份并确认备份可读。
- 购买页、订阅页、用量页、Dashboard、管理端只展示新值，不做升级弹窗，不展示旧值。
- 未完成发放的订阅订单也更新快照，后续支付成功按新额度发放。
- 自动退款按新 28 天总额度计算；历史已完成退款不回写。

## 额度表

| 价格 | 分组 | 周额度变化 | 28 天总额度 |
|---:|---|---:|---:|
| 29 CNY | `codex-pool-19-usd` | 58 -> 76 USD | 304 USD |
| 39 CNY | `codex-pool-29-usd` | 78 -> 102 USD | 408 USD |
| 59 CNY | `codex-pool-49-usd` | 118 -> 154 USD | 616 USD |
| 79 CNY | `codex-pool-69-usd` | 158 -> 206 USD | 824 USD |
| 99 CNY | `codex-pool-89-usd` | 198 -> 258 USD | 1032 USD |
| 149 CNY | `codex-pool-135-usd` | 299 -> 389 USD | 1556 USD |
| 199 CNY | `codex-pool-179-usd` | 400 -> 520 USD | 2080 USD |

## 必须修改的系统事实

后端固定映射：

- `backend/internal/service/group.go` 的公开 Codex 周额度常量改为新值。
- 所有通过 `PublicCodexSubscriptionWeeklyLimitUSD`、`EffectiveWeeklyLimitUSD`、`NormalizePublicCodexSubscriptionQuota` 派生的接口、下单快照和管理端校准都自然读取新值。

数据库前向迁移：

- 新增 `176_increase_public_codex_weekly_quota_amounts.sql`，不修改历史迁移 `175`。
- 更新 `groups`：7 个分组的 `weekly_limit_usd`、`default_validity_days`、描述文案；`daily_limit_usd` 和 `monthly_limit_usd` 保持 `NULL`。
- 更新 `subscription_plans`：7 个套餐的名称、商品名、28 天有效期、说明、features；价格不变。
- 更新有效权益 `subscription_entitlement_periods`：只更新 `status='active'` 且 `expires_at > NOW()` 的公开 Codex 权益段，把 `weekly_limit_usd` 和 `period_total_quota_usd` 提到新值；不动 `weekly_usage_usd`、窗口起点、锚点、用量事实。
- 更新未完成发放订单 `payment_orders.subscription_snapshot`：仅覆盖 `order_type='subscription'`、对应 7 个分组、未支付且未过期的 `PENDING` 订单，以及已支付但尚未履约完成的 `PAID`、`RECHARGING`、`paid_at IS NOT NULL` 的可重试 `FAILED` 订单；快照会补齐 `version` 和 `plan_id`，让后续支付成功或重试发放时按新额度履约。

## 不修改的系统事实

- 不回写已完成退款订单的 `refund_basis`。
- 不清理、不重算 `usage_logs`、`usage_facts`、`weekly_usage_usd`、`daily_usage_usd`。
- 不改变订阅开始时间、过期时间、周窗口起点、`weekly_anchor_at`。
- 不改变购买金额、实付金额、支付流水和审计日志。
- 不调整流量卡、余额和非公开订阅分组。

## 前端展示设计

前端继续复用 `frontend/src/utils/subscriptionQuota.ts` 作为公开 Codex 映射入口：

- 购买页：展示新周额度和新 28 天总额度。
- 订阅页：有效订阅展示新每周上限；当后端返回 `effective_weekly_limit_usd` 时优先使用后端值。
- Key 用量页和 Dashboard：按新周额度显示剩余与进度。
- 管理端分组、套餐编辑、订单详情和退款弹窗：展示新周额度和新 28 天总额度。
- 首页公开文案同步新值，避免入口信息和购买页不一致。

## 发布策略

发布允许短暂维护：

1. 开启维护，暂停模型请求结算、订阅购买、退款写入。
2. 对 PostgreSQL 做完整备份，使用单独恢复/读取命令验证备份可读。
3. 部署包含迁移 `176` 的后端版本，执行迁移。
4. 清理受影响的计费和套餐缓存。
5. 逐档对账：`groups`、`subscription_plans`、有效权益、未发放订单快照。
6. 验证购买页、订阅页、用量页、Dashboard、管理端展示。
7. 解除维护。

公网恢复前如果发现问题，可以回滚镜像并从备份恢复数据库。公网恢复后不直接恢复旧库，避免覆盖恢复后的新增订单、用量和退款；若必须降级，只做前向修正迁移。

## 验收条件

- 7 个公开 Codex 分组均返回新周额度。
- 7 个套餐均展示新周额度和新 28 天总额度，价格不变。
- 当前有效订阅的周用量保留，上限提高。
- 未发放订单支付成功后发放新额度。
- 退款 quote 使用新 28 天总额度计算。
- 相关后端单测、前端单测、类型检查、构建通过。
