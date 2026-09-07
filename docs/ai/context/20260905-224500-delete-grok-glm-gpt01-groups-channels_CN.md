# 删除 Grok / GLM / GPT0.1 三个分组与渠道

- 时间：2026-09-05 22:40 ~ 22:50（+09）
- 环境：生产 `https://aaccx.pw`
- 操作方式：浏览器既有管理员会话调管理 API（未输入任何凭证），未直连数据库
- 管理员指令：「不仅停用 Grok / GLM / GPT0.1倍率 的渠道，而是直接删除这三个渠道，之后也不再提供这三个分组」

## 删除对象

| 类型 | id | 名称 | DELETE |
| --- | --- | --- | --- |
| 分组 | 3 | Grok0.9倍率(优质) | 200 |
| 分组 | 6 | GLM0.6倍率 | 200 |
| 分组 | 11 | Gpt0.1倍率(优惠) | 200 |
| 渠道（账号） | 1 | Grok模型官方0.6倍价格 | 200 |
| 渠道（账号） | 4 | GLM模型官方0.2折价格 | 200 |
| 渠道（账号） | 1130 | GPT模型官方0.1倍低价 | 200 |

接口：`DELETE /api/v1/admin/accounts/{id}` 与 `DELETE /api/v1/admin/groups/{id}`。先删账号后删分组（顺序无所谓，两侧都清 `account_groups` 关联行）。

## 关键事实：「删除」是**软删除**，计费历史完整保留

改动前已从源码确认（这决定了此操作可放心执行）：

- **分组删除** `groupRepo.DeleteCascade`（`group_repo.go:762`）在一个事务里：
  ① 订阅型分组才软删 `user_subscriptions`（这三个是 `standard`，跳过）；
  ② 硬删 `user_allowed_groups`（专属授权关联，非专属无行）；
  ③ 硬删 `account_groups`（账号↔分组绑定行）；
  ④ 软删 `composite_model_routes`；
  ⑤ **分组本身软删**（ent `SoftDeleteMixin` → 置 `deleted_at`，行不消失）。
- **账号删除** `accountRepo.Delete`（`account_repo.go:1000`）：硬删 `account_groups`、`scheduled_test_plans` 关联行，级联软删 spark 影子（这三个是 apikey/`global`，无影子），**账号本身软删**（同 `SoftDeleteMixin`）。
- **两者都完全不碰 `usage_logs`**，`usage_logs.account_id/group_id` 仍指向带 `deleted_at` 的行，历史可 JOIN 可查。删后实测 `glm-5.1` 的 2 条历史日志仍可正常返回。
- 因为是软删、行还在，`usage_logs` 也没有硬外键会挡删或级联清空。

## 影响：约 18 个用户 API Key 被孤立（删除前已告知）

删除前统计三个分组上仍绑定的 API Key：

| 分组 | 绑定 Key | 活跃 | 关联用户（部分） |
| --- | --- | --- | --- |
| Grok #3 | 3 | 3 | 483, 528 |
| GLM #6 | 3 | 2 | 448, 505, 528 |
| GPT0.1 #11 | 12 | 12 | 636, 600, 598, 607, 495, 604 … |

- 这三个分组删除前**都已是停用状态**（Grok/GPT0.1 于 2026-09-05 早些下架，GLM 同日 22:35 下架），
  停用分组的绑定 Key 本就已不可用，删除**不新增破坏**，只是把「不可用」变成永久。
- **未删除任何用户 API Key**——那是用户自有资产。删组后这些 Key 变惰性（请求期解析不到分组 → 拒绝），
  用户可自行改绑到其它分组或删除。
- 这三个分组的定价源码（`fallbackPrices["glm-5.x"]` 等）未删，无实际影响（无账号、无分组引用）。

## 验证（删后实测）

- `GET /admin/groups/{3,6,11}` → 全部 `404`；分组列表不再含这三个。
- `GET /admin/accounts/{1,4,1130}` → 全部 `404`；账号列表不再含这三个。
- 模型广场 `/api/v1/model-plaza`：无 GLM / Grok / GPT0.1，残留检查干净。
- 用户可选分组 `/groups/available`：精确匹配 `Gpt0.1倍率(优惠)` / `GLM*` / `Grok*` 均无。
- 计费历史：`glm-5.1` 历史 `usage_logs` 仍可查（2 条），证明软删未动历史。

## 回滚

软删可逆但**无 UI**，需直连数据库把对应 `groups`/`accounts` 行的 `deleted_at` 置回 `NULL`
并恢复 `account_groups` 绑定（该关联是**硬删**，需重新插入 `(account_id, group_id)`）。
`user_allowed_groups` 同为硬删。若确需回滚，按 VPS runbook 走 SERIALIZABLE 事务。

## 未改动

`BILLING_FINAL_MULTIPLIER=18`、其余分组与账号（含刚开长上下文计费的 6 个 GPT 账号）、
用户余额、订单、退款、以及全部 `usage_logs` 历史。
