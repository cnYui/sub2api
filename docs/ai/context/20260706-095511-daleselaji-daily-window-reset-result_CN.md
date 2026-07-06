# daleselaji@gmail.com 日额度 403 排查与运行态修复

## 用户与报错

- 用户邮箱：`daleselaji@gmail.com`
- 用户 `id=60`，状态 `active`
- 用户反馈报错：
  - `unexpected status 403 Forbidden: daily usage limit exceeded`
  - URL：`https://aaccx.pw/responses`
  - request id：`be711a86-2855-48ef-b3a4-b3355ad9901e`

## 排查结果

- 用户 active Key：
  - `api_keys.id=81`，`name=111`，`group_id=NULL`，脱敏 Key：`sk-47422...bd90`
  - `api_keys.id=76`，`name=114514`，`group_id=NULL`，脱敏 Key：`sk-efe2a...fddd`
- active 订阅：
  - `user_subscriptions.id=77`
  - `group_id=4`
  - 分组：`codex-pool-49-usd`
  - `daily_limit_usd=49.00000000`
  - 排查时 `daily_usage_usd=49.0803109000`
- 用户当前无可用 GPT/OpenAI 流量卡，因此套餐超限后没有流量包兜底。
- 日志确认 request id `be711a86-2855-48ef-b3a4-b3355ad9901e` 在 Sub2API 网关计费资格检查阶段被拒绝：
  - `user_id=60`
  - `api_key_id=76`
  - `group_id=4`
  - `model=gpt-5.5`
  - `path=/responses`
  - `error=DAILY_LIMIT_EXCEEDED`
  - HTTP `403`

## 根因

- `usage_logs` 按自然日聚合显示：
  - 2026-07-05：`321` 次成功用量，合计 `49.0803109000` USD
  - 2026-07-06：`0` 次成功用量
- 订阅记录里的窗口字段为：
  - `daily_window_start=2026-07-05 00:00:00+08`
  - 按代码应在 `2026-07-06 00:00:00+08` 后重置
- 排查时 DB 时间已是 `2026-07-06 08:53+08`，但 `daily_usage_usd` 仍未归零。
- 代码层观察：
  - `SubscriptionService.CheckAndResetWindows()` 具备日窗口重置逻辑。
  - API 网关计费资格检查走 `BillingCacheService.CheckBillingEligibility()` -> `GetSubscriptionStatus()`，只读缓存/DB 中的 `daily_usage_usd`，没有在该入口主动执行窗口维护。
- 因此这是“订阅日窗口过期后，API 请求入口未触发窗口重置”的运行态问题；用户说“今天额度还是满的”从自然日用量角度成立。

## 运行态修复

- 修复前已备份公网候选库：
  - `deploy/backups/20260706-095428-sub2api-candidate-before-reset-daleselaji-daily-window.dump`
- 执行等价于现有重置逻辑的最小 DB 修复：
  - `user_subscriptions.id=77`
  - `daily_usage_usd=0`
  - `daily_window_start=2026-07-06 00:00:00+08`
  - 保留周/月累计不变
- 删除 Redis billing 订阅缓存：
  - `DEL billing:sub:60:4`
  - 返回 `0`，表示当时没有该缓存 Key。

## 验证

- 使用用户报错对应 Key `api_keys.id=76` 请求：
  - URL：`https://aaccx.pw/responses`
  - 模型：`gpt-5.5`
- HTTP 返回：`200`
- 请求前 `daily_usage_usd=0.0000000000`
- 请求后 `daily_usage_usd=0.0039960000`
- 最新用量：
  - `usage_logs.id=50696`
  - `api_key_id=76`
  - `user_id=60`
  - `group_id=4`
  - `subscription_id=77`
  - `billing_type=1`
  - `total_cost=0.0039960000`

## 后续建议

- 需要补一个源码修复：API 网关计费资格检查前，或 `BillingCacheService.GetSubscriptionStatus()` 从 DB miss/读取时，应识别过期的 `daily_window_start` 并执行日窗口重置/缓存失效，避免其他用户跨日后继续被昨天的超限值拦截。
- 也需要检查订阅进度页是否展示自然日用量或订阅窗口用量，避免用户看到“今天没用”但 API 按旧窗口拒绝。
