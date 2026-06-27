# 当前三个套餐兑换码行为核查

## 当前实例套餐

当前 Postgres 中有三个在售订阅套餐：

| 套餐 | subscription_plans.group_id | 分组 | 分组日额度 |
| --- | ---: | --- | ---: |
| 29 元订阅池 | 2 | `codex-pool` | 19 USD / day |
| 39 元订阅池 | 3 | `codex-pool-29-usd` | 29 USD / day |
| 59 元订阅池 | 4 | `codex-pool-49-usd` | 49 USD / day |

另有自用分组 `codex-pool-local-unlimited`，不是在售套餐。

## 兑换码能力

- 管理员可在 `/admin/redeem` 生成 `subscription` 类型兑换码。
- 生成订阅兑换码时必须选择订阅类型分组 `group_id`，并设置 `validity_days`。
- 用户在 `/redeem` 输入兑换码。
- 用户兑换 `subscription` 类型兑换码后，后端调用 `AssignOrExtendSubscription()`：
  - 没有该分组订阅：创建用户订阅。
  - 已有未过期订阅：从当前到期时间继续累加天数。
  - 已过期订阅：从当前时间重新计算有效期并激活。
- 兑换后会失效用户鉴权缓存和该用户该分组的订阅计费缓存。

## 生效边界

- 兑换码开通的是“用户对某个订阅分组的有效订阅”，不是直接充值余额。
- API 请求实际按 API Key 绑定的 `group_id` 走对应分组额度。
- 用户只有持有某订阅分组的有效订阅，才能创建或切换 API Key 到该分组。
- 当前实例已有 API Key 绑定情况：
  - `codex-pool`：16 个 key。
  - `codex-pool-29-usd`：0 个 key。
  - `codex-pool-49-usd`：0 个 key。
  - `codex-pool-local-unlimited`：1 个 key。

## 结论

可以为当前三个套餐分别生成兑换码。用户兑换后，对应套餐的订阅有效期和分组额度会自动生效；但如果用户已有 API Key 仍绑定旧分组，需要用户新建或切换到兑换后拥有订阅的分组，API 请求才会按新套餐额度计费/限额。
