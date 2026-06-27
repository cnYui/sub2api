# ZPay Alipay Runtime Config Result

## 已完成

- 已从本地 `main` 创建分支 `codex/zpay-alipay-runtime-config`。
- 已新增可测试脚本 `scripts/configure-zpay-alipay-runtime.mjs`，用于把 ZPay 作为 `easypay` provider instance 写入运行态 PostgreSQL。
- 已新增测试 `scripts/__tests__/configure-zpay-alipay-runtime.test.mjs`，覆盖支付宝-only、微信关闭、退款关闭和输出脱敏。
- 已备份运行态 PostgreSQL：`.tmp-sub2api-before-zpay-alipay-runtime-20260626-134729.dump`，大小约 86MB。
- 已通过隐藏输入写入 ZPay 运行态配置，未在命令输出、源码或文档中记录商户密钥。
- 已验证运行态 DB：
  - `provider_key=easypay`
  - `name=ZPay Alipay`
  - `supported_types=alipay`
  - 初始 `payment_mode=qrcode`，后续已修正为 `payment_mode=popup`
  - `refund_enabled=false`
  - `allow_user_refund=false`
  - `payment_visible_method_alipay_enabled=true`
  - `payment_visible_method_alipay_source=easypay_alipay`
  - `payment_visible_method_wxpay_enabled=false`
- 已验证公共设置接口返回 `payment_enabled=true`。

## 验证命令

```bash
node --test scripts/__tests__/configure-zpay-alipay-runtime.test.mjs
```

结果：3 个测试通过，0 个失败。

```bash
node scripts/configure-zpay-alipay-runtime.mjs --dry-run
```

结果：只输出 provider、支付方式、回调地址等脱敏摘要，不输出 PID/KEY。

```bash
node scripts/configure-zpay-alipay-runtime.mjs --apply
```

结果：`BEGIN / INSERT / COMMIT` 成功。

```bash
/Applications/Docker.app/Contents/Resources/bin/docker exec sub2api-postgres \
  psql -U sub2api -d sub2api -P pager=off -c "<脱敏查询 provider/settings>"
```

结果：支付宝已启用，微信和退款保持关闭。

## 未完成与阻塞

- 应用内浏览器访问 `http://127.0.0.1:5174/purchase` 时没有登录态，被跳转到 `/login?redirect=/purchase`，因此尚未完成 UI 层的支付宝展示确认。
- 宿主机执行 `backend/cmd/jwtgen` 默认连接 `::1:5432`，但当前 PostgreSQL 未映射宿主机端口。
- 将 `jwtgen` 临时编译进 `sub2api` 容器执行时，被运行态迁移 checksum 校验阻塞：`155_seed_codex_subscription_plans_baseline.sql` 的 DB checksum 与当前文件 checksum 不一致。该问题与 ZPay 配置无关，本次未修改历史迁移。
- 尚未创建真实 ZPay 支付订单，尚未完成扫码支付、回调查单和订阅履约的端到端验收。

## 2026-06-26 13:57 修正

- 现象：用户扫描实时生成二维码时报“交易买家不匹配”。
- 证据：最近三笔 ZPay 订单的 `pay_url` 和 `qr_code` 都是 `https://qr.alipay.com/...`，`qr_code_img` 为空。
- 根因：运行态把 ZPay 实例配置成 `payment_mode=qrcode`，后端因此调用 `mapi.php`，并把 ZPay 返回的支付宝原始码交给前端重新生成二维码。该码不适合作为当前 ZPay 第三方收款码通道的买家扫码入口。
- 修正：脚本默认 `paymentMode` 已改为 `popup`，运行态 DB 已将 `ZPay Alipay` 的 `payment_mode` 改为 `popup`。新订单会走 EasyPay/ZPay `submit.php` 托管收银台，让 ZPay 自己展示付款页/付款码。
- 注意：旧订单里的 `qr.alipay.com` 链接不会自动变化，不能继续用于支付；需要新建订单重新测试。

## 后续步骤

- 使用已登录的浏览器会话访问 `/purchase`，确认只显示支付宝、不显示微信、不显示“充值功能暂未开放”。
- 修复或绕开本地 `jwtgen` 的迁移校验阻塞后，用临时 JWT 调 `/api/v1/payment/checkout-info` 和创建订单接口。
- 创建一笔小额或 29 元订阅测试订单，扫码支付后验证：
  - `payment_orders.status=COMPLETED`
  - `payment_trade_no` 非空
  - `paid_at/completed_at` 非空
  - 用户订阅已添加或续期

## 回滚

如需回滚，仅禁用 provider 和支付宝可见方法，不删除历史订单：

```bash
/Applications/Docker.app/Contents/Resources/bin/docker exec sub2api-postgres \
  psql -U sub2api -d sub2api -v ON_ERROR_STOP=1 -c "
BEGIN;
UPDATE payment_provider_instances
   SET enabled=false, updated_at=now()
 WHERE provider_key='easypay'
   AND name='ZPay Alipay';

INSERT INTO settings (key, value, updated_at)
VALUES
  ('payment_visible_method_alipay_enabled', 'false', now()),
  ('payment_visible_method_alipay_source', '', now())
ON CONFLICT (key)
DO UPDATE SET value=EXCLUDED.value, updated_at=now();
COMMIT;"
```
