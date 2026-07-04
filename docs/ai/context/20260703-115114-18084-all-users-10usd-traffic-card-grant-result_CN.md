# 18084 当前用户补发 10 USD GPT 流量卡结果

## 目标

- 给当前 18084 / `sub2api-candidate-postgres` 的所有未删除用户重新发放一张 10 USD OpenAI/GPT 流量卡。
- 发放后验证 `2799523972@qq.com` 前端页面是否显示。
- 不影响当前 Sub2API 公网正常使用。

## 执行保护

- 执行前 health：
  - `127.0.0.1:18084/health` 200
  - `127.0.0.1:8080/health` 200
  - `https://api.aaccx.pw/health` 200
- 执行前备份：
  - `deploy/backups/20260703-114705-sub2api-candidate-before-18084-10usd-grant.dump`
  - 文件权限 `600`
  - 已用运行中的 Postgres 容器内 `pg_restore -l` 校验可读
- 未重启容器、未改 nginx、未改 Cloudflare Tunnel、未改 Redis、账号池或全局配置。

## 执行内容

- 操作对象：`sub2api-candidate-postgres`
- 批次号：`grant-20260703-10usd-18084-current-users`
- 发放对象：当前 18084 未删除用户 52 个
- 流量包商品：
  - `traffic_packs.id=2`
  - `gpt_traffic_10usd_3cny`
  - `platform=openai`
  - `credit_usd=10`
  - `validity_days=365`

## 数据结果

- 本批次系统订单：52 条
- 本批次 10 USD OpenAI 流量卡：52 张
- 本批次 purchase 流水：52 条
- 本批次初始额度合计：`520.0000000000 USD`
- 本批次剩余额度合计：`520.0000000000 USD`

## 过程备注

- 第一次事务插入了 52 条 `payment_orders`，但同一 SQL 中后续 CTE 没读到刚插入的订单，原因是 PostgreSQL 数据修改 CTE 的快照可见性规则；该事务未插入流量卡或流水。
- 随后用同一批次号分两步幂等补齐：
  - 先从已落库订单插入 52 张 `user_traffic_credits`
  - 再从已落库流量卡插入 52 条 `traffic_credit_ledger` purchase 流水
- 最终批次校验完整，未重复发放。

## 验证

### Health

- 执行后：
  - `127.0.0.1:18084/health` 200
  - `127.0.0.1:8080/health` 200
  - `https://api.aaccx.pw/health` 200

### 用户数据

- `1038686518@qq.com`：
  - 当前可用 OpenAI 流量卡：1 张
  - 当前可用余额：`10.0000000000 USD`
- `2799523972@qq.com`：
  - 当前可用 OpenAI 流量卡：3 张
  - 当前可用余额：`24.9959290000 USD`

### 用户侧 API

- `2799523972@qq.com` 登录后 `/api/v1/payment/checkout-info` 返回：
  - `traffic_credit_summary.total_remaining_usd=24.995929`
  - `next_expiring_usd=9.995929`
  - `next_expires_at=2027-06-26T08:57:24.31087+08:00`
- `/api/v1/subscriptions` 仍返回 79 元订阅：
  - `subscription_id=71`
  - `group_id=9`
  - `codex-pool-69-usd`
  - `daily_limit_usd=69`

### 浏览器页面

- 使用已登录的 `2799523972@qq.com` 浏览器页打开 `/subscriptions`。
- 页面刷新后显示：
  - `GPT 流量包`
  - `总计 $0.00 / $25.00`
  - `codex-pool-69-usd`
  - 每日 `$0.00 / $69.00`

## 回滚提示

- 如需撤回本批次，可按 `out_trade_no LIKE 'grant-20260703-10usd-18084-current-users-%'` 定位：
  1. 删除对应 `traffic_credit_ledger`
  2. 删除对应 `user_traffic_credits`
  3. 删除对应 `payment_orders`
- 若已有用户消耗本批次额度，回滚前需要先单独核算已消费额度。

## 敏感信息

- 本文档未记录完整 API Key、access token、内部 token、SMTP 密码或 HMAC secret。
