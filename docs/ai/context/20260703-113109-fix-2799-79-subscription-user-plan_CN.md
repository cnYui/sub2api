# 修复 2799 用户为可用 79 元套餐用户计划

## 背景

- 用户说明：`2799523972@qq.com` 曾被直接数据库修改，当前状态异常，需要修复为正常可用的 79 元套餐用户，并保留现有 GPT 流量包 15 USD 额度。
- 当前公网入口实际指向 `127.0.0.1:18084`，数据库为 `sub2api-candidate-postgres`。

## 当前只读核对

- 用户：`2799523972@qq.com`，`user_id=31`，状态 `active`。
- 现有 GPT 流量包：
  - `credit_id=38`：10 USD 卡剩余约 `9.995929`
  - `credit_id=45`：5 USD 卡剩余 `5.000000`
  - 合计约 `14.995929 USD`
- 当前订阅：
  - 仅有一条旧 `codex-pool-19-usd` 订阅，已软删除。
- 当前 API Key：
  - 仅有一条 `Codex_used`，已软删除，原绑定 `codex-pool-19-usd`。
- 79 元套餐：
  - `subscription_plans.id=7`
  - `group_id=9`
  - 分组 `codex-pool-69-usd`
  - 日限额 `69 USD`
  - 有效期 `30` 天
  - 上游账号 `cliproxy-local-openai` 已绑定该分组。

## 修复目标

1. 保持用户 active。
2. 保留现有 `user_traffic_credits`，不改余额、不改过期时间、不补发/删除流量卡。
3. 为用户建立一条未删除、active 的 79 元套餐订阅：
   - `group_id=9`
   - `starts_at=now()`
   - `expires_at=now()+30 days`
   - 用量窗口和用量计数清零。
4. 恢复用户原软删除 API Key `Codex_used` 并改绑 `group_id=9`：
   - `deleted_at=NULL`
   - `status='active'`
   - `quota=0`、`quota_used=0`
   - `expires_at=NULL`
   - 保留原 Key 字符串，不在文档或回复中输出。
5. 验证：
   - `/api/v1/subscriptions` 返回 79 元分组订阅。
   - `/api/v1/groups/available` 返回 `codex-pool-69-usd`。
   - `/api/v1/keys` 返回 active Key，绑定 `group_id=9`。
   - `/api/v1/payment/checkout-info` 仍返回约 `14.995929 USD` 流量包汇总。
   - 用该 Key 对 `/v1/responses` 发起真实小请求返回 200；由于订阅有效且未超限，计费应走订阅，不应扣流量卡。

## 执行保护

- 执行前备份 `sub2api-candidate-postgres`。
- 全部 DB 修改放在单个事务中，先锁定目标用户行。
- 不输出完整 API Key、access token、内部 token、SMTP 密码或 HMAC secret。
