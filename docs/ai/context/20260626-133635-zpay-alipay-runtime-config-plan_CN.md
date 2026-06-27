# ZPay Alipay Runtime Config Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给当前运行态 Sub2API 添加 ZPay/EasyPay 支付宝-only 支付配置，让用户可在 `/purchase` 选择套餐、生成动态支付宝支付订单，并在支付成功后自动完成订阅履约。

**Architecture:** 不改业务架构，不新增独立 `zpay` provider。ZPay 作为 `easypay` provider instance 写入运行态 PostgreSQL，前台只启用 `alipay -> easypay_alipay`，微信支付关闭，退款关闭；当前 ZPay 第三方收款码通道使用 `popup/submit.php` 托管收银台，不直接渲染 `mapi.php` 返回的 `qr.alipay.com` 原始码。

**Tech Stack:** PostgreSQL、Docker Desktop bundled CLI、Sub2API Go backend、Vue `/purchase` 前端、EasyPay/ZPay MD5 签名协议。

---

## Context And Constraints

- 当前服务容器：
  - Sub2API 后端：`sub2api`，本机端口 `127.0.0.1:18080 -> 8080`
  - PostgreSQL：`sub2api-postgres`
  - 本地 Vite 前端：`http://127.0.0.1:5174`
- 支付网关：`https://zpayz.cn/`
- 支付方式：只启用支付宝 `alipay`。
- 微信支付：保持关闭。
- 退款：`refund_enabled=false`，`allow_user_refund=false`。
- ZPay 商户号和密钥只允许通过执行时环境变量或隐藏输入进入运行态数据库，不允许写入本计划、源码、提交、日志或长期文档。
- 当前 provider config 是明文 JSON 存储，`decryptConfig` 兼容旧密文；运行态直接写入 JSON 可被服务读取。
- 直接写 DB 后，新订单选择 provider 时会查 DB；webhook 对新订单会通过订单绑定的 `provider_instance_id` 回查实例创建 provider。因此本次配置不要求重启后端，但计划保留重启/刷新作为异常恢复手段。

## Demo And Protocol Confirmation

三个 SDK demo 结论：

- `python/pay.txt`：构造 `submit.php` 跳转支付，签名为按字段顺序拼接后追加 KEY 做 MD5。
- `node/pay.txt`：对参数名排序，排除空值、`sign`、`sign_type`，拼接 `a=b&c=d` 后追加 KEY，MD5 小写。
- `java/pay.txt`：按 key 升序排序，拼接 `key=value&...`，去掉末尾 `&` 后追加 KEY，MD5 小写。

与现有 `backend/internal/payment/provider/easypay.go` 对照：

- `easyPaySign` 已按 ASCII 排序、排除空值、`sign`、`sign_type`，追加 `pkey` 后 MD5 小写。
- `createAPIPayment` 已调用 `/mapi.php`，传 `pid/type/out_trade_no/notify_url/return_url/name/money/clientip/sign/sign_type`。
- `VerifyNotification` 已支持 GET query notify，校验签名、`trade_status=TRADE_SUCCESS`、`money`、`pid`。
- `QueryOrder` 已调用 `/api.php` 并用 `out_trade_no` 查询，支持 `status=1` 或 `trade_status=TRADE_SUCCESS`。

结论：本轮以运行态配置和端到端验收为主，不需要先改 EasyPay provider。若真实 `mapi.php` 因 Content-Type 拒绝 `application/x-www-form-urlencoded`，再单独补兼容 patch。

## Files And Runtime Surfaces

**Runtime DB:**

- Modify: `payment_provider_instances`
- Modify: `settings`

**Read-only code references:**

- `backend/internal/payment/provider/easypay.go`
- `backend/internal/service/payment_order.go`
- `backend/internal/service/payment_fulfillment.go`
- `backend/internal/service/payment_config_limits.go`
- `frontend/src/views/user/PaymentView.vue`

**Docs:**

- Create: `docs/ai/context/YYYYMMDD-HHMMSS-zpay-alipay-runtime-config-result_CN.md` after execution.
- Update: `AGENTS.md` after successful runtime verification.

## Task 1: Backup And Preflight

**Files:**

- Create runtime backup: `.tmp-sub2api-before-zpay-alipay-runtime-<timestamp>.dump`
- No source code changes.

- [ ] **Step 1: Verify containers and current branch**

Run:

```bash
git status --short --branch
/Applications/Docker.app/Contents/Resources/bin/docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Ports}}'
```

Expected:

- Current branch is the intended working branch.
- `sub2api` and `sub2api-postgres` are running.
- `sub2api` exposes `127.0.0.1:18080->8080/tcp`.

- [ ] **Step 2: Backup runtime PostgreSQL**

Run:

```bash
STAMP=$(date +%Y%m%d-%H%M%S)
/Applications/Docker.app/Contents/Resources/bin/docker exec sub2api-postgres \
  pg_dump -U sub2api -d sub2api \
  > ".tmp-sub2api-before-zpay-alipay-runtime-${STAMP}.dump"
ls -lh ".tmp-sub2api-before-zpay-alipay-runtime-${STAMP}.dump"
```

Expected:

- Dump file exists and is non-empty.

- [ ] **Step 3: Confirm current payment runtime state**

Run:

```bash
/Applications/Docker.app/Contents/Resources/bin/docker exec sub2api-postgres \
  psql -U sub2api -d sub2api -P pager=off -c "
SELECT key, value
FROM settings
WHERE key IN (
  'payment_enabled',
  'payment_visible_method_alipay_enabled',
  'payment_visible_method_alipay_source',
  'payment_visible_method_wxpay_enabled',
  'payment_visible_method_wxpay_source'
)
ORDER BY key;

SELECT id, provider_key, name, supported_types, enabled, payment_mode,
       refund_enabled, allow_user_refund
FROM payment_provider_instances
ORDER BY id;"
```

Expected before this task is implemented:

- It is acceptable if `payment_provider_instances` is empty.
- If an old EasyPay/ZPay instance exists, record its `id/name/supported_types/enabled/payment_mode/refund_enabled/allow_user_refund` in the result doc before changing it.

## Task 2: Write ZPay Alipay-Only Runtime Config

**Files:**

- Create: `scripts/configure-zpay-alipay-runtime.mjs`
- Create: `scripts/__tests__/configure-zpay-alipay-runtime.test.mjs`
- Modify runtime DB table: `payment_provider_instances`
- Modify runtime DB table: `settings`

- [x] **Step 1: Add a tested runtime config script**

Use TDD to add a script that:

- Builds the ZPay/EasyPay provider SQL without printing credentials.
- Enables only `alipay -> easypay_alipay`.
- Keeps `wxpay` hidden.
  - Keeps `refund_enabled=false` and `allow_user_refund=false`.
  - Uses `payment_mode=popup` so orders launch the ZPay hosted cashier via `submit.php`.

Verification:

```bash
node --test scripts/__tests__/configure-zpay-alipay-runtime.test.mjs
```

Expected:

```text
pass 3
fail 0
```

- [x] **Step 2: Collect secrets without writing them to files**

Use macOS hidden input or an interactive shell:

```bash
read -r -p "ZPay PID: " ZPAY_PID
read -r -s -p "ZPay KEY: " ZPAY_KEY
printf '\n'
export ZPAY_PID ZPAY_KEY
export ZPAY_API_BASE='https://zpayz.cn'
export ZPAY_NOTIFY_URL='https://api.aaccx.pw/api/v1/payment/webhook/easypay'
export ZPAY_RETURN_URL='https://aaccx.pw/payment/result'
```

Expected:

- `ZPAY_PID` and `ZPAY_KEY` are set in the current shell.
- Do not run `echo "$ZPAY_KEY"` or paste the key into docs/logs.

- [x] **Step 3: Upsert provider and visible payment settings**

Run:

```bash
node scripts/configure-zpay-alipay-runtime.mjs --apply
```

Expected:

- `COMMIT` succeeds.
- No secret is printed by the command itself.

- [x] **Step 4: Verify DB state without printing secrets**

Run:

```bash
/Applications/Docker.app/Contents/Resources/bin/docker exec sub2api-postgres \
  psql -U sub2api -d sub2api -P pager=off -c "
SELECT id, provider_key, name, supported_types, enabled, payment_mode,
       refund_enabled, allow_user_refund,
       config::jsonb ? 'pid' AS has_pid,
       config::jsonb ? 'pkey' AS has_pkey,
       config::jsonb ->> 'apiBase' AS api_base,
       config::jsonb ->> 'notifyUrl' AS notify_url,
       config::jsonb ->> 'returnUrl' AS return_url
FROM payment_provider_instances
WHERE provider_key = 'easypay'
ORDER BY id;

SELECT key, value
FROM settings
WHERE key IN (
  'payment_enabled',
  'payment_visible_method_alipay_enabled',
  'payment_visible_method_alipay_source',
  'payment_visible_method_wxpay_enabled',
  'payment_visible_method_wxpay_source'
)
ORDER BY key;"
```

Expected:

- One enabled `easypay` provider named `ZPay Alipay`.
- `supported_types=alipay`.
- `payment_mode=popup`.
- `refund_enabled=false`.
- `allow_user_refund=false`.
- `has_pid=true`.
- `has_pkey=true`.
- `api_base=https://zpayz.cn`.
- `payment_enabled=true`.
- `payment_visible_method_alipay_enabled=true`.
- `payment_visible_method_alipay_source=easypay_alipay`.
- `payment_visible_method_wxpay_enabled=false`.
- `payment_visible_method_wxpay_source` is empty.

## Task 3: Verify Checkout Surface

**Files:**

- No source code changes.

- [ ] **Step 1: Verify public payment flag**

Run:

```bash
curl -s http://127.0.0.1:5174/api/v1/settings/public \
  | python3 -m json.tool \
  | rg '"payment_enabled"|"purchase_subscription_enabled"'
```

Expected:

- `"payment_enabled": true`
- `purchase_subscription_enabled` may remain `false`; current `/purchase` uses internal payment page, not the old external iframe purchase URL.

- [ ] **Step 2: Verify authenticated checkout info**

Use the currently logged-in browser session at `http://127.0.0.1:5174/purchase`.

Expected UI state:

- Selecting `29 元订阅池` shows the payment method selector.
- Payment method includes 支付宝.
- It does not show 微信支付.
- It does not show “充值功能暂未开放”.
- “确认支付 ¥29.00” is enabled.

If command-line verification is preferred, obtain a user JWT from the same local browser session without saving it to disk, then run:

```bash
read -r -s -p "User JWT: " USER_JWT
printf '\n'
curl -s \
  -H "Authorization: Bearer ${USER_JWT}" \
  http://127.0.0.1:5174/api/v1/payment/checkout-info \
  | python3 -m json.tool \
  | rg '"methods"|"alipay"|"wxpay"|"plans"'
```

Expected:

- `methods.alipay` exists.
- `methods.wxpay` does not exist.
- `plans` contains the active sale plans.

## Task 4: Create A Real ZPay Alipay Test Order

**Files:**

- Runtime DB changes through normal application order creation.
- No source code changes.

- [ ] **Step 1: Create order from UI**

Manual UI flow:

```text
Open http://127.0.0.1:5174/purchase
Select 29 元订阅池
Select 支付宝
Click 确认支付
```

Expected:

- Page enters payment waiting state.
- It displays a ZPay QR image or QR content.
- It displays an order countdown.

- [ ] **Step 2: Verify order persisted with provider binding**

Run:

```bash
/Applications/Docker.app/Contents/Resources/bin/docker exec sub2api-postgres \
  psql -U sub2api -d sub2api -P pager=off -c "
SELECT id, user_id, amount, pay_amount, payment_type, order_type, plan_id,
       provider_instance_id, provider_key, status,
       pay_url IS NOT NULL AS has_pay_url,
       qr_code IS NOT NULL AS has_qr_code,
       qr_code_img IS NOT NULL AS has_qr_code_img,
       out_trade_no,
       created_at
FROM payment_orders
ORDER BY id DESC
LIMIT 5;"
```

Expected for the newest order:

- `pay_amount=29` for the 29 元 plan.
- `payment_type=alipay`.
- `order_type=subscription`.
- `plan_id` is the selected plan ID.
- `provider_key=easypay`.
- `provider_instance_id` is non-empty.
- At least one of `has_pay_url`, `has_qr_code`, `has_qr_code_img` is true.
- `status=PENDING` before payment.

- [ ] **Step 3: Pay the test order**

Use Alipay to scan/pay the generated ZPay QR code.

Expected:

- The ZPay payment completes successfully.
- The Sub2API payment waiting page eventually exits waiting state, or the order can be verified from the order/result page.

## Task 5: Verify Fulfillment And Recovery Paths

**Files:**

- No source code changes.

- [ ] **Step 1: Verify webhook completed order**

Run after payment:

```bash
/Applications/Docker.app/Contents/Resources/bin/docker exec sub2api-postgres \
  psql -U sub2api -d sub2api -P pager=off -c "
SELECT id, status, pay_amount, payment_trade_no, paid_at, completed_at,
       failed_at, failed_reason
FROM payment_orders
ORDER BY id DESC
LIMIT 5;"
```

Expected:

- Test order status becomes `COMPLETED`.
- `payment_trade_no` is non-empty.
- `paid_at` and `completed_at` are non-null.
- `failed_reason` is empty.

- [ ] **Step 2: If webhook is delayed, verify by active query**

Open the order/result page or the order list and trigger the existing verify flow.

If using API with a user JWT:

```bash
read -r -p "out_trade_no: " OUT_TRADE_NO
read -r -s -p "User JWT: " USER_JWT
printf '\n'
curl -s \
  -H "Authorization: Bearer ${USER_JWT}" \
  -H "Content-Type: application/json" \
  -X POST \
  -d "{\"out_trade_no\":\"${OUT_TRADE_NO}\"}" \
  http://127.0.0.1:5174/api/v1/payment/orders/verify \
  | python3 -m json.tool
```

Expected:

- Paid ZPay order reconciles to `COMPLETED`.
- If payment is not done, order remains `PENDING`.

- [ ] **Step 3: Verify user subscription**

Run:

```bash
/Applications/Docker.app/Contents/Resources/bin/docker exec sub2api-postgres \
  psql -U sub2api -d sub2api -P pager=off -c "
SELECT us.id, us.user_id, us.group_id, us.status, us.expires_at,
       g.name AS group_name, g.daily_limit_usd
FROM user_subscriptions us
JOIN groups g ON g.id = us.group_id
ORDER BY us.updated_at DESC
LIMIT 10;"
```

Expected:

- The paying user has an active subscription for the purchased plan’s group.
- The subscription expiry reflects the plan validity.
- `/subscriptions` page shows the new or extended subscription.

## Task 6: Negative And Safety Checks

**Files:**

- No source code changes unless a check fails and root cause points to code.

- [ ] **Step 1: Verify WeChat stays hidden**

Run authenticated checkout check again:

```bash
read -r -s -p "User JWT: " USER_JWT
printf '\n'
curl -s \
  -H "Authorization: Bearer ${USER_JWT}" \
  http://127.0.0.1:5174/api/v1/payment/checkout-info \
  | python3 -m json.tool \
  | rg '"alipay"|"wxpay"'
```

Expected:

- `alipay` appears.
- `wxpay` does not appear.

- [ ] **Step 2: Verify refund is closed**

Run:

```bash
/Applications/Docker.app/Contents/Resources/bin/docker exec sub2api-postgres \
  psql -U sub2api -d sub2api -P pager=off -c "
SELECT id, name, refund_enabled, allow_user_refund
FROM payment_provider_instances
WHERE provider_key='easypay'
ORDER BY id;"
```

Expected:

- ZPay instance has `refund_enabled=false`.
- ZPay instance has `allow_user_refund=false`.

- [ ] **Step 3: Verify no sensitive values are written to tracked files**

Run:

```bash
git diff -- . ':!docs/ai/context'
git status --short --untracked-files=all
rg -n 'ZPAY_KEY|pkey|商户密钥|Bearer |refresh_token|api_key' \
  AGENTS.md backend frontend scripts docs/ai/context \
  || true
```

Expected:

- No business source code diff unless a later task explicitly required a compatibility fix.
- The search does not reveal the ZPay secret.
- If the PID appears only in ignored local runtime notes, remove it before finalizing. Prefer not recording the full PID anywhere.

## Task 7: Optional Compatibility Patch If ZPay Rejects Existing Form Encoding

**Files:**

- Modify only if Task 4 fails due to `mapi.php` rejecting `application/x-www-form-urlencoded`.
- Modify: `backend/internal/payment/provider/easypay.go`
- Test: `backend/internal/payment/provider/easypay_create_test.go`

- [ ] **Step 1: Confirm root cause before editing**

Evidence required:

```text
ZPay create order response explicitly shows request format/content-type rejection, while signature and fields are correct.
```

If the error is missing field, invalid sign, invalid merchant, disabled channel, or amount issue, do not edit code; fix runtime config or ZPay account state.

- [ ] **Step 2: Add failing provider test for multipart form only if required**

Patch `backend/internal/payment/provider/easypay_create_test.go` with a test that starts an `httptest.Server`, rejects requests whose `Content-Type` is not multipart, and asserts `CreatePayment` succeeds after implementation.

Expected before implementation:

```bash
cd backend
go test ./internal/payment/provider -run TestEasyPayCreatePaymentSupportsMultipartFormData
```

Expected result before patch:

```text
FAIL
```

- [ ] **Step 3: Implement minimal multipart fallback only if required**

Implementation shape:

```go
func (e *EasyPay) postRaw(ctx context.Context, endpoint string, params map[string]string) ([]byte, int, error) {
    body, status, err := e.postURLEncoded(ctx, endpoint, params)
    if err == nil && !easyPayResponseLooksFormatRejected(body) {
        return body, status, nil
    }
    if !easyPayShouldRetryMultipart(status, body, err) {
        return body, status, err
    }
    return e.postMultipart(ctx, endpoint, params)
}
```

Keep this fallback narrowly gated by the provider’s actual rejection response. Do not switch all EasyPay traffic blindly unless the real ZPay evidence requires it.

- [ ] **Step 4: Run provider tests**

Run:

```bash
cd backend
go test ./internal/payment/provider
```

Expected:

```text
ok  	github.com/Wei-Shaw/sub2api/internal/payment/provider
```

## Task 8: Result Documentation

**Files:**

- Create: `docs/ai/context/YYYYMMDD-HHMMSS-zpay-alipay-runtime-config-result_CN.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: Write result doc**

Create a new result document with:

```markdown
# ZPay Alipay Runtime Config Result

## 已完成

- 已配置 `easypay` provider instance，名称 `ZPay Alipay`。
- 仅启用支付宝 `alipay`。
- 微信支付保持关闭。
- 退款保持关闭。
- `/purchase` 已能展示支付宝支付方式。
- 测试订单结果：填写订单 ID、状态、是否完成订阅履约。

## 验证命令

- 填写实际运行过的命令，不包含密钥。

## 风险与后续

- 如果未完成真实支付，说明原因。
- 如果回调未到达，记录查单补偿结果。
```

Do not include the PID, PKey, full payment URL query string, full QR URL if it embeds sensitive tokens, or complete JWT.

- [ ] **Step 2: Update `AGENTS.md`**

Add one concise bullet under “当前运行态提醒”:

```markdown
- 2026-06-26 已配置 ZPay/EasyPay 支付宝-only 运行态支付实例；微信支付和退款保持关闭。用户购买订阅走 `/purchase -> payment_orders -> ZPay alipay -> webhook/verify -> subscription fulfillment`，结果见 `docs/ai/context/<result-doc>.md`。
```

- [ ] **Step 3: Self-review for secret leakage**

Run:

```bash
rg -n 'ZPAY_KEY|pkey|商户密钥|Bearer |refresh_token|api_key' \
  docs/ai/context AGENTS.md backend frontend scripts \
  || true
```

Expected:

- No secret values appear.
- Generic field names such as `pkey` in source code are acceptable; actual secret values are not.

## Rollback Plan

If checkout breaks or ZPay config is wrong, disable the provider and visible method without deleting history:

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

Expected:

- `/purchase` no longer exposes支付宝 payment method.
- Existing completed payment orders remain auditable.

## Self-Review

- Spec coverage: covers Alipay-only provider config, WeChat disabled, refund disabled, checkout visibility, order creation, webhook/query fulfillment, subscription verification, rollback, and secret hygiene.
- Placeholder scan: no unresolved placeholder markers; sensitive PID/KEY are intentionally supplied at execution time through hidden input.
- Type consistency: provider key `easypay`, visible method source `easypay_alipay`, payment type `alipay`, payment mode `popup`, refund booleans `false`.
