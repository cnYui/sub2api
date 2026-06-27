# ZPay 订阅自动履约与本地分支冲突调查

## 背景

- 用户提到 `docs/ai/content` 下有两个相关改动：模型价格页改为 29 / 39 / 59 / 99 元，以及集成 ZPay 支付。
- 实际检查结果：`docs/ai/content` 当前只有早期前端合并记录；99 元套餐和 ZPay 细节主要记录在 `docs/ai/context`。
- 当前工作区分支是 `codex/zpay-payment-20260625`，工作区干净。
- 本地 `main` 是合并提交 `561a0feb`，父提交分别是：
  - `9a2f21b18 codex/add-99-plan-20260625`
  - `a536dc9d codex/zpay-payment-20260625`

## 当前代码事实

### 1. ZPay 自动支付主链路已经存在

- ZPay 被当作 EasyPay 协议实现接入，不新增独立 `zpay` provider。
- `frontend/src/views/user/PaymentView.vue` 的订阅确认按钮在有可用支付方式时会调用 `/api/v1/payment/orders` 创建订单。
- `frontend/src/components/payment/PaymentStatusPanel.vue` 会轮询订单，并调用 `/api/v1/payment/orders/verify` 主动查单。
- `backend/internal/service/payment_order_lifecycle.go` 的 `VerifyOrderByOutTradeNo` 会查上游订单状态。
- `backend/internal/service/payment_fulfillment.go` 的 `confirmPayment` 会校验：
  - provider 是否匹配；
  - 商户元数据是否匹配；
  - provider 返回实付金额是否有效；
  - 实付金额与订单 `pay_amount` 是否在币种容差内一致。
- 金额校验通过后，订阅订单进入 `ExecuteSubscriptionFulfillment`，最终调用 `AssignOrExtendSubscription` 给当前用户开通或续期套餐。

结论：后端不需要重写一套“付款后计算金额并发放套餐”的新链路；应该复用现有订单、查单、金额校验、履约链路。

### 2. 人工支付链路仍然存在，并被测试保护

- `frontend/src/components/payment/ManualPaymentDialog.vue` 仍展示静态支付宝收款图。
- 用户点击“已付款”后只是显示“去兑换”，不会创建支付订单，也不会自动履约。
- `PaymentView.vue` 中：
  - `confirmSubscribe()` 在 `enabledMethods.length === 0` 时打开 `ManualPaymentDialog`；
  - `confirmTrafficPack()` 也有同样人工兜底。
- `frontend/src/views/user/__tests__/PaymentView.spec.ts` 当前有测试断言：
  - 无支付方式时订阅套餐不创建订单；
  - 打开人工支付弹窗；
  - `main` 上 99 元套餐测试也断言会打开人工支付。

结论：用户要求“不再需要人工发码链路”与现有测试预期冲突。实现时必须先改测试，否则测试会继续保护旧流程。

### 3. ZPay 入口是否出现取决于运行态配置

- `/purchase` 是否展示支付宝/微信按钮取决于 `/api/v1/payment/checkout-info` 返回的 `methods`。
- `methods` 来自 enabled 的 `payment_provider_instances`。
- EasyPay/ZPay 实例必须：
  - `provider_key = easypay`；
  - `enabled = true`；
  - `supported_types` 包含 `alipay` 或 `wxpay`；
  - 配置里有 `pid`、`pkey`、`apiBase`、`notifyUrl`、`returnUrl`；
  - `apiBase` 指向 ZPay 网关，如 `https://zpayz.cn`。
- 如果同时存在官方支付宝/微信和 EasyPay，还要确认前台可见支付来源设置，否则系统可能隐藏冲突方法。

结论：如果线上仍弹人工付款码，第一根因大概率是 ZPay/EasyPay 未被配置成前台可见支付方式，而不是金额校验逻辑缺失。

## 分支冲突调查

### 当前分支状态

- `codex/zpay-payment-20260625` 不包含 99 元套餐 migration。
- 本地 `main` 已经合并 99 元分支和 ZPay 分支。
- 从当前分支继续做实现前，应先切到或基于 `main`，否则容易遗漏 99 元套餐数据 seed。

### 文本冲突

- `git merge-tree` 显示两个分支没有需要人工解决的明显文本冲突。
- 合并提交中双边修改文件包括：
  - `frontend/src/views/HomeView.vue`
  - `frontend/src/views/__tests__/HomeView.spec.ts`
  - `frontend/src/views/user/PaymentView.vue`

### 逻辑冲突

1. 99 元套餐分支新增四档展示和 99 元套餐 seed。
2. ZPay 分支新增 `qr_image_url` 支持和人工支付宝静态码调整。
3. 合并后的 `main` 同时保留：
   - 四档套餐；
   - ZPay 二维码图片展示；
   - 无支付方式时弹人工码并去兑换。
4. 这与当前新要求“不再人工发码，付款后自动应用套餐”不一致。

结论：真正冲突不是 Git 冲突，而是业务语义冲突：99 元套餐测试把人工支付兜底作为正确行为，而 ZPay 自动履约目标要求移除或限制该兜底。

## 推荐实现方向

### 方案 A：下线订阅和流量包的人工支付兜底，强制走支付订单

推荐。

- 删除或停用 `PaymentView.vue` 中订阅和流量包的 `enabledMethods.length === 0 -> ManualPaymentDialog` 分支。
- 当没有可用支付方式时，按钮 disabled 或点击后显示 `payment.notAvailable` / 明确错误提示。
- 订阅、流量包全部必须通过 `/api/v1/payment/orders` 创建订单。
- ZPay/EasyPay 返回 `qr_code` 或 `qr_image_url` 后，前端进入 `PaymentStatusPanel`。
- 付款后依赖 webhook 或主动查单自动履约。
- 更新测试：
  - 有 ZPay/EasyPay 方法时，29 / 39 套餐点击后创建 `order_type=subscription`、正确 `plan_id` 和金额；
  - ZPay 只返回 `qr_image_url` 时进入付款等待态；
  - 无支付方式时不再展示 `ManualPaymentDialog`；
  - 99 元四档布局保留，但不再断言打开人工支付。

优点：

- 与“不再人工发码”一致。
- 避免用户付款到静态码后系统无法自动识别。
- 复用现有金额校验、查单、履约和审计。

风险：

- ZPay 配置不可用时用户无法购买；这是正确失败，比人工收款后无法自动履约更安全。

### 方案 B：只对 29 / 39 走 ZPay 自动订单，59 / 99 保留人工兜底

不推荐。

- 会让同一个购买页出现两套支付事实源。
- 用户会困惑，后续运营仍要处理人工发码。
- 与“整个支付流程已经不再需要手动分发兑换码”不一致。

### 方案 C：把人工支付弹窗改造成“创建订单后的静态二维码展示”

不推荐。

- 静态个人收款码无法给后端提供可验证的 `out_trade_no`、上游交易号和实付金额。
- 后端无法可靠自动确认付款归属。
- 除非 ZPay 对这张二维码提供可查单交易记录，否则只是包装旧人工流程。

## 推荐落地顺序

1. 基于本地 `main` 开新分支，避免遗漏 99 元 migration 和 ZPay 已合并代码。
2. 先写前端失败测试：
   - 29 元套餐有 `alipay` 方法时创建订阅订单；
   - 39 元套餐有 `alipay` 方法时创建订阅订单；
   - 99 元四档布局仍存在；
   - 无支付方式时不出现人工付款弹窗。
3. 修改 `PaymentView.vue`：
   - 移除订阅和流量包确认阶段的人工兜底；
   - 无支付方式时禁用确认按钮或显示不可用提示；
   - 移除未使用的 `ManualPaymentDialog` 引用和 `goRedeem` 入口，除非余额充值仍明确保留人工入口。
4. 保留 `PaymentStatusPanel`、`paymentFlow.ts`、EasyPay `qr_image_url` 逻辑。
5. 加后端回归测试（如当前覆盖不足）：
   - 订阅订单 provider 查询返回金额不匹配时不履约；
   - 金额匹配时自动开通订阅。
6. 更新上下文文档与 AGENTS 长期记忆：
   - 99 元套餐已上线；
   - 订阅购买必须走 ZPay/EasyPay 自动订单；
   - 不再通过人工兑换码发放订阅。

## 需要用户确认

- 是否接受“订阅和 GPT 流量包都不再使用人工支付兜底”。推荐接受。
- 如果只想先处理 29 / 39 套餐，需要明确 59 / 99 是否继续人工兜底；该方向有明显一致性风险。
