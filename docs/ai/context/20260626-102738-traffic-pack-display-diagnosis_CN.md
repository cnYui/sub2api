# 2026-06-26 流量卡前端展示排查

## 背景

用户反馈 `2799523972@qq.com` 在公网前端的 `purchase` 或 `subscriptions` 页面没有看到已发放的 10 USD GPT 流量卡。用户截图实际地址为 `https://aaccx.pw/subscriptions`，页面标题为“我的订阅”。

## 排查结论

- 生产库中该账号存在已发放流量卡，不需要重复补发。
- `https://aaccx.pw/subscriptions` 只展示订阅套餐，不展示流量卡余额。
- `https://aaccx.pw/purchase` 使用同一 Chrome 登录态验证，右上角登录用户为 `2799523972@qq.com`，页面正文显示：
  - `GPT 流量包`
  - `当前可用 10.00 刀，最近 2027/6/26 到期`
  - `订阅日额度用完后自动消耗`

## 数据库证据

- 用户：`users.id=31`，`email=2799523972@qq.com`，状态 `active`。
- 流量卡：`user_traffic_credits.id=38`，`pack_id=2`，`initial_usd=10`，`remaining_usd=10`，到期时间 `2027-06-26 08:57:24.31087+08`。
- 订单：`payment_orders.id=38`，`out_trade_no=sub2_gift_gpt10_20260626_u31`，`order_type=traffic_pack`，`payment_type=manual_grant`，`status=COMPLETED`，`amount=0`，`pay_amount=0`。
- 流水：`traffic_credit_ledger.id=38`，`entry_type=purchase`，`amount_usd=10`，`balance_after_usd=10`。

## 前端验证

使用 Chrome 现有登录态打开 `https://aaccx.pw/purchase`，页面可见摘要为：

```json
{
  "url": "https://aaccx.pw/purchase",
  "title": "充值/订阅 - Sub2API",
  "hasEmail": true,
  "hasTrafficPack": true,
  "hasCurrentTen": true,
  "relevant": "GPT 流量包 当前可用 10.00 刀，最近 2027/6/26 到期 订阅日额度用完后自动消耗"
}
```

## 后续说明

用户应从左侧菜单进入“充值/订阅”页面查看流量卡余额。当前“我的订阅”页面只展示订阅计划和每日额度，不展示一次性流量卡余额。
