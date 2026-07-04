# 18084 当前用户补发 10 USD GPT 流量卡计划

## 背景

- 2026-07-02 的全员 10 USD 流量卡发放写入了 `sub2api-postgres` / 18080。
- 当前公网实际由 nginx 反代到 `127.0.0.1:18084`，读取 `sub2api-candidate-postgres`。
- 用户要求：给当前 18084 端口的用户重新发放一张 10 USD 流量卡，发放后登录 `2799523972@qq.com` 页面刷新确认是否出现。

## 当前只读确认

- 当前操作对象：`sub2api-candidate-postgres`。
- 当前 18084 未删除用户数：52。
- 10 USD OpenAI/GPT 流量包商品：
  - `traffic_packs.id=2`
  - `code=gpt_traffic_10usd_3cny`
  - `credit_usd=10`
  - `validity_days=365`
- 当前 18084 已有可用 OpenAI 流量卡：45 张，剩余合计约 `363.96891185 USD`。

## 执行方案

- 批次号：`grant-20260703-10usd-18084-current-users`。
- 执行前备份 `sub2api-candidate-postgres`。
- 单事务内执行：
  1. 锁定当前未删除用户集合。
  2. 为每个未删除用户创建一条系统订单：
     - `order_type='traffic_pack'`
     - `payment_type='manual_grant'`
     - `status='COMPLETED'`
     - `amount=0`
     - `pay_amount=0`
     - `out_trade_no='grant-20260703-10usd-18084-current-users-' || user_id`
  3. 为每条订单插入一张 `user_traffic_credits`：
     - `pack_id=2`
     - `platform='openai'`
     - `initial_usd=10`
     - `remaining_usd=10`
     - `expires_at=credited_at + 365 days`
  4. 为每张卡插入一条 `traffic_credit_ledger` purchase 流水。
- 使用 `out_trade_no` 唯一约束保证幂等；如果批次已存在则不会重复发放。

## 验证

- 数据库批次校验：
  - 新增/存在 52 条订单。
  - 新增/存在 52 张 10 USD OpenAI 卡。
  - 新增/存在 52 条 purchase 流水。
- health 校验：
  - `127.0.0.1:18084/health`
  - `127.0.0.1:8080/health`
  - `https://api.aaccx.pw/health`
- 用户侧验证：
  - 登录 `2799523972@qq.com`
  - `/api/v1/payment/checkout-info` 的 `traffic_credit_summary.total_remaining_usd` 应增加 10 USD
  - 浏览器 `/subscriptions` 应显示 GPT 流量包汇总增加后的余额

## 边界

- 不重启容器。
- 不改 nginx、Cloudflare Tunnel、Redis、账号池和全局配置。
- 不记录完整 API Key、access token、内部 token、SMTP 密码或 HMAC secret。
