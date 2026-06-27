# ZPay 购买支付与自动履约需求文档

## 背景

当前 `/purchase` 页面已经切到 Sub2API 内置支付页，人工静态收款码链路已下线。用户目标是：在当前前端中让用户选择套餐、创建 ZPay 支付订单、完成支付，并在支付成功后自动确认金额，把用户购买的套餐添加到其账户中。

2026-06-26 排查结论：ZPay 相关代码已合并到本地 `main`，但运行态 DB 没有 `payment_provider_instances`，支付宝/微信可见支付方式也未启用，因此前端显示“充值功能暂未开放”。详见 `docs/ai/context/20260626-124242-purchase-zpay-merge-runtime-diagnosis_CN.md`。

## 目标

- 用户在 `/purchase` 选择订阅套餐后，可以看到可用支付方式。
- 用户点击“确认支付”后，系统创建本地 `payment_orders` 订单，并调用 ZPay/EasyPay 创建固定金额动态二维码或跳转链接。
- 前端展示 ZPay 返回的二维码图片、二维码内容或支付链接，并轮询订单状态。
- ZPay 支付成功后，系统通过回调或主动查单确认支付成功。
- 系统必须校验订单号、支付渠道、商户身份、支付金额与本地订单 `pay_amount` 一致。
- 校验通过后，系统自动把订单对应的订阅套餐添加或续期到用户账户。
- 用户刷新“我的订阅”后能看到新购买或续期后的套餐。

## 非目标

- 不新增独立 `zpay` provider；ZPay 继续作为 EasyPay 协议 provider instance 接入。
- 不恢复人工静态收款码和人工发码链路。
- 不在代码、文档、日志、提交中记录完整 ZPay 商户号、PKey、内部 token 或 API Key。
- 不根据用户实际支付金额临时改套餐；订单创建时选择哪个套餐，支付成功后只履约该套餐。
- 不把 yui.web/shop、CLIProxyAPI 加回用户 Key、计费或支付事实源。

## 当前已有能力

### 前端

- `/purchase` 当前使用 `frontend/src/views/user/PaymentView.vue`。
- 有支付方式时，前端会调用 `/api/v1/payment/orders` 创建订单。
- 支付中状态由 `PaymentStatusPanel` 展示二维码、倒计时、取消和成功态。
- `qr_image_url` 已支持展示 ZPay `img` 字段返回的二维码图片。

### 后端

- `PaymentService.CreateOrder` 会根据 `plan_id` 读取后端套餐价格，使用 `plan.Price` 作为订单金额事实源。
- `payment_orders` 保存 `pay_amount`、`out_trade_no`、`plan_id`、`subscription_group_id`、`subscription_days`、provider snapshot。
- `EasyPay.CreatePayment` 支持 `mapi.php` 和 `submit.php`，并传递 `money=req.Amount`。
- EasyPay 已解析 `payurl`、`payurl2`、`qrcode`、`img`。
- `HandlePaymentNotification` 通过 `out_trade_no` 找本地订单。
- `confirmPayment` 已校验 provider、metadata 和金额，金额不匹配会拒绝履约。
- `executeFulfillment` 会按订单类型分发到订阅、流量包或余额履约。
- 订阅订单成功后会进入 `ExecuteSubscriptionFulfillment`，给用户添加或续期套餐。

## 必须补齐的工作

### 1. 运行态支付配置

当前最直接缺口不是代码，而是运行态配置。

必须配置至少一个 ZPay EasyPay provider instance：

- `provider_key=easypay`
- `name=ZPay` 或 `ZPay EasyPay`
- `enabled=true`
- `supported_types` 至少包含 `alipay`；如果 ZPay 微信通道可用，再包含 `wxpay`
- `payment_mode=qrcode` 优先，确认跳转链路后可评估 `popup`
- `config` 必须包含：
  - `pid`
  - `pkey`
  - `apiBase`
  - `notifyUrl`
  - `returnUrl`
  - 可选 `cidAlipay`
  - 可选 `cidWxpay`

必须启用前台可见支付方式：

- 支付宝：
  - `payment_visible_method_alipay_enabled=true`
  - `payment_visible_method_alipay_source=easypay_alipay`
- 微信支付，如启用：
  - `payment_visible_method_wxpay_enabled=true`
  - `payment_visible_method_wxpay_source=easypay_wxpay`

验收要求：

- `/api/v1/payment/checkout-info` 返回 `methods.alipay` 或 `methods.wxpay`。
- `/purchase` 选择套餐后不再显示“充值功能暂未开放”。
- “确认支付”按钮可点击。

### 2. 支付创建链路验收

用户点击套餐支付时，前端请求必须包含：

- `order_type=subscription`
- `plan_id=<用户选择的套餐 ID>`
- `payment_type=alipay` 或 `wxpay`
- `amount` 可以随前端传递，但后端必须以 `plan.Price` 为准。

后端创建订单时必须：

- 从 DB 读取在售套餐。
- 使用套餐价格计算 `pay_amount`。
- 写入订单和订阅履约字段。
- 调 ZPay/EasyPay 创建固定金额订单。
- 把 ZPay 返回的二维码图片或二维码内容返回给前端。

验收要求：

- 29 元套餐创建本地订单 `pay_amount=29`。
- 39 元套餐创建本地订单 `pay_amount=39`。
- 返回结果至少包含 `order_id`、`out_trade_no`、`pay_amount`、`expires_at`，以及 `qr_image_url`、`qr_code` 或 `pay_url` 三者之一。

### 3. 支付成功确认与金额校验

支付成功确认必须支持两条路径：

- ZPay 异步回调到 `/api/v1/payment/webhook/easypay`。
- 用户订单页或支付等待页主动查单，补偿回调丢失。

系统必须校验：

- 签名有效。
- 回调里的 `out_trade_no` 能匹配本地订单。
- 回调商户身份与订单 provider snapshot 一致。
- 回调或查单返回的支付金额等于本地订单 `pay_amount`，允许最小货币精度容差。
- 重复回调不重复发放套餐。

验收要求：

- 金额匹配：订单进入 `COMPLETED`，用户获得或续期套餐。
- 金额不匹配：订单不履约，并写入 `PAYMENT_AMOUNT_MISMATCH` 审计日志。
- 商户身份不匹配：订单不履约，并写入 provider metadata mismatch 类审计日志。
- 重复回调：订单保持完成态，不重复延长订阅。

### 4. 订阅履约

支付成功后，系统必须按订单中的套餐快照履约：

- `plan_id` 决定套餐。
- `subscription_group_id` 决定发放到哪个订阅分组。
- `subscription_days` 决定有效期。
- 如果用户已有同一分组有效订阅，应按现有订阅逻辑续期或复用。
- 如果用户没有同一分组有效订阅，应创建新订阅。

验收要求：

- 支付成功后 `/api/v1/subscriptions/active` 返回对应订阅。
- `/subscriptions` 页面展示新订阅或续期后的到期时间。
- API Key 调用计费时能识别新订阅额度。

### 5. 前端体验

无支付方式时：

- 页面应明确显示“充值功能暂未开放”或更具体的配置提示。
- 确认按钮禁用。
- 不展示人工收款码。

有支付方式时：

- 选择套餐后显示支付方式选择器。
- 点击确认后进入支付等待态。
- 二维码图片、二维码内容或跳转链接按 provider 返回值正确展示。
- 支付成功后刷新用户余额/订阅数据，并引导用户查看“我的订阅”或订单结果页。

建议改进：

- 无支付方式提示可以从“充值功能暂未开放”改为“支付通道暂未配置”，减少误解。
- 支付等待页可显示订单号、实付金额、套餐名称和过期时间。

## 需要用户提供的 ZPay 信息

### 必须提供

以下信息需要用于运行态配置，但不要写入仓库文件或聊天长期文档：

- ZPay EasyPay API 基础地址，例如 `https://zpayz.cn`。
- 商户 PID。
- 商户 PKey。
- 支付宝通道是否已开通。
- 微信支付通道是否已开通。
- 是否有固定通道 ID：
  - 支付宝 `cid`
  - 微信 `cid`

### 必须提供文档或截图

请提供 ZPay/EasyPay 兼容接口文档中这些部分：

- 创建订单接口：
  - endpoint 是 `mapi.php` 还是其他路径。
  - 请求方法：`GET` / `POST` / form-urlencoded / JSON。
  - 必填字段名：`pid`、`type`、`out_trade_no`、`notify_url`、`return_url`、`name`、`money`、`clientip`、`sign`、`sign_type` 等。
  - 可选字段：`cid`、`device`、`param`、`sitename` 等。
  - 成功响应字段：`code`、`msg`、`trade_no`、`payurl`、`payurl2`、`qrcode`、`img`。
  - `code=1` 是否一定代表创建成功。
- 签名规则：
  - 参数排序方式。
  - 是否排除空值、`sign`、`sign_type`。
  - 拼接格式。
  - MD5 大小写要求。
  - PKey 拼接方式。
- 异步通知接口：
  - 回调方法：GET、POST 或都支持。
  - 回调字段列表。
  - 支付成功状态字段和值，例如 `trade_status=TRADE_SUCCESS` 或 `status=1`。
  - 金额字段名和单位，例如 `money` 是否为元。
  - 通知成功响应内容，是否必须返回纯文本 `success`。
  - 失败重试策略。
- 查单接口：
  - endpoint，例如 `api.php?act=order`。
  - 查询字段用 `out_trade_no` 还是 `trade_no`。
  - 成功支付状态字段和值。
  - 金额字段名和单位。
- 退款接口，如后续要开放用户退款：
  - endpoint。
  - 字段与签名。
  - 状态返回。

### 建议提供

- ZPay 后台回调地址配置页面截图，隐藏敏感值。
- 一笔测试订单的脱敏创建响应。
- 一笔测试订单的脱敏异步通知 payload。
- 一笔测试订单的脱敏查单响应。
- ZPay 是否要求公网域名配置白名单。
- ZPay 是否区分沙箱/生产环境。
- ZPay 二维码有效期和订单过期策略。

## 实现方案对比

### 方案 A：运行态配置 + 现有 EasyPay 链路验收

推荐。

- 不新增 provider。
- 复用现有订单、回调、金额校验和订阅履约逻辑。
- 主要补运行态配置、测试与端到端验收。

优点：改动最小，符合当前架构，风险最低。

风险：如果 ZPay 实际协议与 EasyPay 标准有偏差，需要对 `EasyPay` provider 做小补丁。

### 方案 B：新增独立 ZPay provider

不推荐，除非文档证明 ZPay 与 EasyPay 协议明显不兼容。

优点：可以隔离 ZPay 特有字段。

风险：重复实现签名、回调、查单，增加维护成本，也偏离当前“ZPay 作为 EasyPay 接入”的定论。

### 方案 C：前端直接接 ZPay

禁止。

前端不能持有 PKey，也不能承担订单金额事实源和履约事实源。

## 验收清单

- 后台配置 EasyPay/ZPay provider instance 后，`/purchase` 有支付方式。
- 用户选择 29 元套餐可以生成 ZPay 动态二维码。
- 用户完成真实小额支付后，订单状态变为 `COMPLETED`。
- 用户的订阅列表新增或续期对应套餐。
- 金额不匹配的伪造回调不会发放套餐。
- 回调重复发送不会重复发放套餐。
- 回调丢失时，主动查单可以补偿完成履约。
- 日志和文档中不出现完整 PID/PKey。

## 待确认问题

1. 当前只需要先开通支付宝，还是支付宝和微信都要同时开通？
2. ZPay 是否提供沙箱环境？如果没有，是否允许用 0.01 元或 1 元真实订单做端到端测试？
3. ZPay 的 `mapi.php` 返回 `img` 时，该 URL 是否有访问时效或防盗链限制？
4. ZPay 回调金额字段是否始终为人民币“元”的十进制字符串？
5. ZPay 是否会在回调里带 `pid`，用于商户身份校验？
6. 查单接口使用 `out_trade_no` 是否稳定可用？
7. 订单支付成功后，是否需要给用户发送邮件通知？
8. 购买成功后，前端应停留在支付结果页，还是自动跳转到“我的订阅”？

## 下一步建议

1. 用户提供 ZPay 文档中“创建订单、签名、回调、查单”四段内容，以及脱敏样例。
2. 在运行态配置 EasyPay/ZPay provider instance，不把密钥写入仓库。
3. 用测试套餐或低额真实订单完成一次端到端验证。
4. 若协议与现有 EasyPay provider 不一致，再补代码兼容。
5. 通过单元测试锁定金额校验、metadata 校验、重复回调和订阅履约。
