# 29 元套餐分组重命名结果

## 背景

兑换码生成弹窗的订阅分组下拉中，29 元套餐分组显示为 `codex-pool`，但其真实日额度为 `daily_limit_usd=19`。为和 `codex-pool-29-usd`、`codex-pool-49-usd` 的命名保持一致，需要改为能表达日额度的名称。

## 执行

- 将 `groups.id=2` 从 `codex-pool` 更名为 `codex-pool-19-usd`。
- 将该分组描述改为 `yui.web 29 元订阅池迁移：每日 19 USD`。
- 同步更新 `scripts/migrate-yuiweb-legacy-api-keys.mjs` 中的计划映射和默认兜底分组名。
- 更新 `AGENTS.md` 当前协作记忆。

## 验证

当前分组：

| group id | name | daily_limit_usd |
| ---: | --- | ---: |
| 2 | `codex-pool-19-usd` | 19 |
| 3 | `codex-pool-29-usd` | 29 |
| 4 | `codex-pool-49-usd` | 49 |
| 5 | `codex-pool-local-unlimited` | NULL |

当前套餐绑定：

| 套餐 | group id | group name |
| --- | ---: | --- |
| 29 元订阅池 | 2 | `codex-pool-19-usd` |
| 39 元订阅池 | 3 | `codex-pool-29-usd` |
| 59 元订阅池 | 4 | `codex-pool-49-usd` |

当前绑定数量：

- `codex-pool-19-usd`：16 个 API Key，16 条订阅。
- `codex-pool-29-usd`：0 个 API Key，0 条订阅。
- `codex-pool-49-usd`：0 个 API Key，0 条订阅。
- `codex-pool-local-unlimited`：1 个 API Key，1 条订阅。

## 影响

只修改显示名和本地迁移脚本映射，不修改：

- group id。
- 套餐价格。
- 日额度。
- 现有 API Key 绑定。
- 现有用户订阅。
- 上游账号绑定。
