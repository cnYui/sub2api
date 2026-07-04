# 2799 用户修复为 79 元套餐用户结果

## 目标

- 修复 `2799523972@qq.com` 因直接数据库修改造成的异常状态。
- 使其成为正常可用的 79 元套餐用户。
- 保留当前 GPT 流量包约 15 USD 额度。
- 不影响当前 Sub2API 公网正常使用。

## 执行保护

- 执行前确认公网 health：
  - `127.0.0.1:18084/health` 200
  - `127.0.0.1:8080/health` 200
  - `https://api.aaccx.pw/health` 200
- 执行前备份当前 18084 数据库：
  - `deploy/backups/20260703-113109-sub2api-candidate-before-fix-2799-79-subscription.dump`
  - 文件权限 `600`
  - 已通过运行中 Postgres 容器内 `pg_restore -l` 校验可读
- 未重启容器、未改 nginx、未改 Cloudflare Tunnel、未改全局账号池或全局配置。

## 修复前状态

- 用户 `2799523972@qq.com`：
  - `user_id=31`
  - 用户状态 active
- 流量卡：
  - `credit_id=38`：10 USD 卡，剩余约 `9.995929`
  - `credit_id=45`：5 USD 卡，剩余 `5.000000`
  - 合计约 `14.995929 USD`
- 订阅：
  - 仅有旧 `codex-pool-19-usd` 订阅，已软删除
- API Key：
  - 旧 `Codex_used` 已软删除
  - 直接恢复后发现 key 字段已经是 `__deleted__...` 占位值，不能作为正常 API Key 使用

## 实际修复

- 新增 active 79 元套餐订阅：
  - `user_subscriptions.id=71`
  - `group_id=9`
  - 分组 `codex-pool-69-usd`
  - 日限额 `69 USD`
  - 到期时间 `2026-08-02T10:35:29.314594+08:00`
  - 用量窗口与用量计数初始化为 0
- 将旧占位 Key `id=37` 保持删除态：
  - `status=inactive`
  - `deleted_at` 非空
  - 原因：key 字段已被删除流程匿名化为 `__deleted__...`
- 通过用户 API 正常创建新 Key：
  - `api_keys.id=65`
  - 名称 `Codex_79`
  - `group_id=9`
  - 状态 active
  - key 字符串未写入文档
- 未修改 `user_traffic_credits`，流量包仍为约 `14.995929 USD`。

## 验证

### Health

- 修复后：
  - `127.0.0.1:18084/health` 200
  - `127.0.0.1:8080/health` 200
  - `https://api.aaccx.pw/health` 200

### 用户侧 API

- `/api/v1/subscriptions` 返回 active 订阅：
  - `id=71`
  - `group_id=9`
  - `codex-pool-69-usd`
  - `daily_limit_usd=69`
- `/api/v1/groups/available` 返回 `codex-pool-69-usd`
- `/api/v1/keys` 返回 1 个 active 正常 Key：
  - `id=65`
  - `Codex_79`
  - `group_id=9`
- `/api/v1/payment/checkout-info` 返回：
  - `traffic_credit_summary.total_remaining_usd=14.995929`

### 真实请求

- 使用新 Key 请求 `POST https://api.aaccx.pw/v1/responses`，model=`gpt-5.5`：
  - HTTP 200
  - 新增 `usage_logs.id=38335`
  - `api_key_id=65`
  - `subscription_id=71`
  - `group_id=9`
  - `billing_type=1`（订阅计费）
  - `total_cost=0.004096`
- 请求前后流量卡总余额均为 `14.995929`，未被扣除。
- 订阅 `daily_usage_usd` 从 `0` 增至 `0.004096`。

### 浏览器页面

- 使用 `2799523972@qq.com / 123123` 登录后打开 `/subscriptions`：
  - 显示 `GPT 流量包`，总计 `$0.00 / $15.00`
  - 显示 `codex-pool-69-usd`
  - 显示 79 元订阅池描述
  - 显示到期时间 `2026/08/02`
  - 显示每日 `$0.00 / $69.00`
- `/purchase` 页面中 79 元套餐按钮显示为“续费”。

## 敏感信息

- 本文档未记录完整 API Key、access token、内部 token、SMTP 密码或 HMAC secret。
