# yui.web 旧 Shop 页面公网残留盘点

## 结论

当前 `aaccx.pw` 仍把 `/shop/*` 交给 yui.web，但 `/api/*` 已由 nginx 代理给 Sub2API。因此 yui.web 旧 Shop 页面有一部分仍能在公网直接打开，页面内旧 `/api/account/*`、`/api/admin/*`、`/api/auth/*` 调用则不会再进入 yui.web。

这意味着公网存在“旧页面外壳仍可访问，但旧后端业务不在公网链路”的残留状态。

## 当前应保留

- `/shop/`：当前作为图片热区中转入口，跳到 Sub2API 控制台。
- `/shop/guide/`：公开 Sub2API 配置说明页；是否保留取决于是否还需要 yui.web 承载使用说明。

## 公网仍直接 200 的旧页面

- `/shop/login/`：旧手机号登录页。
- `/shop/register/`：旧手机号注册页。
- `/shop/reset-password/`：旧手机号 + 管理员重置码密码重置页。
- `/shop/guide/`：旧/过渡期使用说明页。

这些页面公网可直接打开，但它们依赖的 `/api/auth/*` 在当前 `aaccx.pw` nginx 下会进入 Sub2API，不会进入 yui.web，因此从用户视角很容易成为无效旧入口。

## 公网可访问但未登录会 302 到旧登录页

- `/shop/account/`：旧账户中心，包含订阅池、Token 用量、套餐购买、加量包、退款、API key、扣费流水等。
- `/shop/admin/`：旧管理员控制台，包含邀请码、API key 池、订阅/加量包订单、退款、充值、用户额度、美元消耗、用量监控、日志导入等。
- `/shop/redeem/`：旧邀请码兑换 API key 页面。
- `/shop/key/`：旧兑换结果 / API key 展示页面。
- `/shop/query/`：旧订单查询入口，已合并到账户页。
- `/shop/order/`：旧购买订单页，前端已跳到账户页。
- `/shop/pay/`：旧支付页，前端已跳到账户页。
- `/shop/result/`：旧支付/兑换结果页，前端已跳到账户页。
- `/shop/content/`：旧交付内容页，前端已跳到账户页。

## yui.web 本地仍存在的旧后端接口

`/Users/wujianxiang/CodeSpace/yui.web/server.js` 仍保留旧 Shop 后端接口：

- 账号认证：`/api/auth/register`、`/api/auth/login`、`/api/auth/password-reset`、`/api/auth/logout`。
- 用户账户：`/api/account/me`、`/api/account/balance`、`/api/account/subscription-state`、`/api/account/subscription-orders`、`/api/account/addon-orders`、`/api/account/subscription-refund-requests`、`/api/account/topups`、`/api/account/usage-summary`、`/api/account/model-overview`、`/api/account/invites/redeem`。
- 管理后台：`/api/admin/invite-console`、`/api/admin/subscription-users`、`/api/admin/subscription-orders`、`/api/admin/addon-orders`、`/api/admin/usd-charges`、`/api/admin/subscription-refund-requests`、`/api/admin/usage-summary`、`/api/admin/usage-import-status`、`/api/admin/usage-imports`、`/api/admin/topups`、`/api/admin/password-reset-codes`。
- 旧发 Key 写路径：`/api/admin/invites`、`/api/admin/api-keys`、`/api/admin/session-invites`、`/api/admin/session-api-keys`、`/api/account/invites/redeem`、`/api/invites/redeem`。其中写路径受 `SHOP_LEGACY_KEY_ISSUANCE_DISABLED=true` 时返回 `410 SHOP_LEGACY_KEY_ISSUANCE_DISABLED`。
- 内部接口：`/api/internal/api-keys/status`、`/api/internal/usage-events`。

当前公网 `aaccx.pw/api/*` 由 nginx 交给 Sub2API，因此这些 yui.web API 只在 `127.0.0.1:4173` 或绕过 nginx 直连 yui.web 时可用。

## 处理建议

最小公网收敛方案：保留 `/shop/`，其余 `/shop/login|register|reset-password|account|admin|redeem|key|query|order|pay|result|content` 在 nginx 层返回 410 或 404，避免用户看到旧登录和旧业务入口。

如果仍要保留公开说明页，可单独保留 `/shop/guide/`；如果说明已迁到 Sub2API，则一起下线。

代码清理方案：在 yui.web 中删除或归档上述旧 HTML 页面和旧 Shop API，但这会影响本地历史查询、测试和迁移脚本，需要单独设计迁移后的只读归档边界。
