# 3876129758 删除旧 29 元订阅并验证 59 元套餐请求结果

时间：2026-07-08 18:00 JST

## 目标

- 删除 `3876129758@qq.com` 仍处于 active 的旧 29 元套餐。
- 只保留当前 59 元套餐 active。
- 不构建镜像、不替换容器、不重启 18084 相关服务。
- 用该用户当前 active 自动 Key 再真实请求模型，确认能正常使用。

## 执行前保护

- 写库前已备份公网候选库：
  - `deploy/backups/20260708-175300-sub2api-candidate-before-3876129758-delete-old-29-subscription.dump`
- 备份文件权限为 `600`，大小约 38 MB。
- 已在 Postgres 容器内用 `pg_restore -l` 解析备份，输出 941 行对象清单，说明 dump 可读。

## 数据修改

旧 29 元套餐：

- `user_subscriptions.id=76`
- `group_id=2 / codex-pool-19-usd`
- 修改前：`status=active`、`deleted_at=NULL`
- 修改后：`status=expired`、`deleted_at=2026-07-08 16:58:50.722897` 东八区
- 保留原用量：
  - `daily_usage_usd=15.8351032500`
  - `weekly_usage_usd=28.7947342500`
  - `monthly_usage_usd=67.0562527500`

当前 59 元套餐：

- `user_subscriptions.id=90`
- `group_id=4 / codex-pool-49-usd`
- 保持 `status=active`、`deleted_at=NULL`

执行后该用户未删除 active 订阅数为 `1`，active 订阅 id 只剩 `{90}`。

## 缓存处理

- 已尝试删除该用户相关 Redis key：
  - `billing:sub:56:2`
  - `billing:sub:56:4`
  - `apikey:rate:75`
- Redis 返回删除数量为 `0`，说明当时没有这些缓存 key 或已过期。

## 真实请求验证

使用该用户 active 自动 Key：

- `api_keys.id=75`
- `group_id=NULL`
- Key 掩码：`sk-c7b2c6c...5607`

真实请求公网正式入口：

- URL：`https://api.aaccx.pw/v1/responses`
- 模型：`gpt-5.5`
- HTTP 状态：`200`
- response id：`resp_07875abce593311b016a4e116c3a888191bc52f22025b7f13a`
- 模型回复：`OK`

新增用量：

- `usage_logs.id=66563`
- `request_id=client:7182dfcf-7a7f-4ea4-9114-0ab1a7db3c12`
- `subscription_id=90`
- `group_id=4 / codex-pool-49-usd`
- `total_cost=0.0044060000`

这证明旧 29 元订阅删除后，自动 Key 仍正常请求模型，并且扣在 59 元套餐 `id=90` 上。

## 请求后状态

当前 59 元套餐 `id=90` 用量：

- `daily_usage_usd=0.0093820000`
- `weekly_usage_usd=0.0093820000`
- `monthly_usage_usd=0.0093820000`

旧订阅删除后的测试窗口内，`traffic_credit_ledger` 新增 deduction 数为 `0`，说明没有扣流量卡。

## 健康与容器

- `http://127.0.0.1:18084/health` 返回 `200`
- `http://127.0.0.1:8080/health` 返回 `200`
- 本轮未构建镜像、未替换应用容器、未重启 Postgres/Redis/nginx。
- 当前应用镜像仍为 `sub2api-candidate:20260708-115902-83cf82584-east8-subscription-refresh`。

## 未执行事项

- 本地代码修复仍未发布到公网 18084。
- 未提交 git。
