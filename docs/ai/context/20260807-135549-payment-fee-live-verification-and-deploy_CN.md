# 购买页手续费实测与 18082 发布

时间：2026-08-07 13:55:49（Asia/Tokyo）

## 结论

- `RECHARGE_FEE_RATE` 已在运行数据库设置为 `1`，表示 1%。
- 用户实际支付金额使用订单 `pay_amount`，按人民币分向上取整；商品标价 `amount`、余额套餐到账额度和流量卡额度不变。
- 此前运行库缺少该设置，服务端等效按 0% 处理。这是此前购买页面未实际收取手续费的根因；已补写设置并回读确认。

## 服务端实测

使用临时无余额测试用户调用运行中 `18082` 的 `POST /api/v1/payment/orders`。支付方式为已启用的 EasyPay `alipay`，仅创建待支付订单，不扫码、不调用支付回调。

| 订单 | 商品标价 amount | 实付 pay_amount | fee_rate | 网关结果 |
| --- | ---: | ---: | ---: | --- |
| `605`，`balance_subscription`，余额套餐 `balance-29` | ¥29.00 | ¥29.29 | 1.0000 | `order_created` |
| `606`，`traffic_pack`，流量卡 `gpt_traffic_5usd_2cny` | ¥2.00 | ¥2.02 | 1.0000 | `order_created` |

- 两笔订单的 `provider_key=easypay`、`provider_instance_id=1`，说明网关创建请求使用了含手续费金额。
- 已通过正式取消接口将订单 `605`、`606` 置为 `CANCELLED`；二者 `paid_at`、`completed_at` 均为空，未发放余额套餐或流量卡额度。
- 测试账户 `id=601` 已软删除，仅保留订单金额和取消状态审计。

因此，用户完成实际付款时会支付标价加 1% 手续费；本次验证只到网关待支付创建，不包含真实扫码扣款。

## 代码与发布

- 手续费展示改动已合入本地 `main`：`2a259350c fix: 展示套餐与流量卡支付手续费`。
- 前端展示和服务端订单金额回归测试已在发布前通过；全量前端测试中仍有既有的首页站点名断言和 GroupsView mock 失败，和本次改动无关。
- 使用以下命令仅重建应用容器：

```powershell
docker compose -f deploy/docker-compose.dev.yml -f deploy/docker-compose.18082.yml up -d --build --force-recreate --no-deps sub2api
```

- 新应用容器 `sub2api-official-18082` 已启动并处于 `healthy`；PostgreSQL、Redis 和数据卷未重建。
- 健康检查均返回 HTTP 200：`http://127.0.0.1:18082/health`、`http://127.0.0.1:8080/health`、`https://aaccx.pw/health`、`https://www.aaccx.pw/health`、`https://api.aaccx.pw/health`。
- `BILLING_FINAL_MULTIPLIER=18` 保持原值，未随本次支付手续费发布改动。
