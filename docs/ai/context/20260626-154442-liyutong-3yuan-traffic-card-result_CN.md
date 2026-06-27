# liyutong2883@gmail.com 3 元流量卡处理结果

## 结论

- `liyutong2883@gmail.com` 对应运行态用户 ID 为 `12`，状态为 `active`。
- 3 元流量卡对应现有 `traffic_packs.code = gpt_traffic_10usd_3cny`，即 10 USD OpenAI/GPT 流量，有效期 365 天。
- 2026-06-26 14:35:19 +08，该用户通过支付宝订单 `payment_orders.id = 53` 已自动发放一张 10 USD 流量卡，`user_traffic_credits.id = 46`。
- 处理过程中曾手工新增一张 `payment_orders.id = 55 / user_traffic_credits.id = 47 / traffic_credit_ledger.id = 100`，复核后确认这会让总额变为 30 USD，不符合“合集 20 USD”，已在同日删除撤回。

## 当前保留记录

当前用户可用 OpenAI/GPT 流量卡共 2 张，合计 20 USD：

| credit_id | order_id | 来源 | 初始额度 | 剩余额度 | 到期时间 |
| --- | --- | --- | --- | --- | --- |
| 46 | 53 | 支付宝自动履约 | 10 USD | 10 USD | 2027-06-26 14:35:19 +08 |
| 7 | 7 | 早前手工发放 | 10 USD | 10 USD | 2027-06-26 08:57:24 +08 |

## 验证

- `available_openai_traffic_usd = 20.0000000000`
- `available_cards = 2`
- `traffic_credit_ledger` 中该用户只剩两条 `purchase` 流水：`id = 99` 和 `id = 7`

## 备份

- 修改前已备份生产库到 `.tmp-sub2api-before-liyutong-3yuan-traffic-card-20260626-154300.dump`。

## 注意

- 本次未修改余额、订阅、分组、支付 provider 或用户 API Key。
- 支付宝购买订单已确认自动发放，不需要再为同一笔支付宝订单手工补发。
