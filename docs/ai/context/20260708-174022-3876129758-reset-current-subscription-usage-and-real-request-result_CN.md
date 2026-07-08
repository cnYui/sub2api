# 3876129758 当前订阅额度清零与真实请求结果

时间：2026-07-08 17:44 JST

## 目标

- 先让 `3876129758@qq.com` 当前 59 元套餐能继续使用。
- 清零当前 59 元套餐订阅的已用额度。
- 用该用户当前 active 自动 Key 真实请求模型，确认请求能走当前 59 元套餐扣费。

## 执行前保护

- 写库前已备份公网候选库：
  - `deploy/backups/20260708-174022-sub2api-candidate-before-3876129758-sub90-usage-reset.dump`
- 备份文件权限为 `600`，大小约 38 MB。
- 已在 Postgres 容器内用 `pg_restore -l` 解析备份，输出 941 行对象清单，说明 dump 可读。

## 用户与订阅状态

- 用户：`3876129758@qq.com`
- 用户 id：`56`
- 当前 active API Key：
  - `api_keys.id=75`
  - `group_id=NULL`
  - Key 掩码：`sk-c7b2c6c...5607`
  - 这是自动 Key，不写死分组。
- 旧 29 元套餐：
  - `user_subscriptions.id=76`
  - `group_id=2 / codex-pool-19-usd`
  - 状态仍为 `active`
  - 当前未处理，等待退款后再软删除或撤销。
- 当前 59 元套餐：
  - `user_subscriptions.id=90`
  - `group_id=4 / codex-pool-49-usd`
  - 状态为 `active`

## 清零操作

已清零当前 59 元套餐 `user_subscriptions.id=90` 的用量字段：

- `daily_usage_usd=0`
- `weekly_usage_usd=0`
- `monthly_usage_usd=0`

旧 29 元套餐 `user_subscriptions.id=76` 未清零、未软删除、未撤销。

## 真实模型请求

使用该用户 active 自动 Key 请求公网正式入口：

- URL：`https://api.aaccx.pw/v1/responses`
- 模型：`gpt-5.5`
- HTTP 状态：`200`
- response id：`resp_02148cdc4c72abb8016a4e0d7114e08191aaee5fa4f784dc81`
- 模型回复：`收到：额度清零后的真实请求测试 OK。`

请求后新增用量：

- `usage_logs.id=66379`
- `request_id=client:07dc340b-8516-4f0d-92e0-0039b8db8214`
- `subscription_id=90`
- `group_id=4 / codex-pool-49-usd`
- `total_cost=0.0049760000`

这证明自动 Key 当前真实扣在 59 元套餐 `id=90` 上，而不是旧 29 元套餐或流量卡。

## 请求后状态

当前 59 元套餐 `id=90` 请求后用量：

- `daily_usage_usd=0.0049760000`
- `weekly_usage_usd=0.0049760000`
- `monthly_usage_usd=0.0049760000`

该测试请求后，`traffic_credit_ledger` 未产生新的 deduction，说明没有扣流量卡。

## 健康检查

- `http://127.0.0.1:18084/health` 返回 `200`
- `http://127.0.0.1:8080/health` 返回 `200`

## 退款参考

旧 29 元套餐使用时长已在重复订阅诊断与购买拦截结果中核算：

- 从 29 元订阅开始到 59 元订阅完成：约 `3.04` 天。
- 从 29 元订阅开始到旧订阅最后一次 API 使用：约 `2.99` 天。
- 旧订阅累计 `usage_logs=480` 条，内部 USD 用量成本 `67.0562527500`。

## 未执行事项

- 未对旧 29 元套餐做退款、软删除或撤销。
- 未构建镜像、未部署公网代码修复。
- 未提交 git。
