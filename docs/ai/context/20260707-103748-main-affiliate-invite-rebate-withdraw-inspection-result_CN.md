# 本地 main 邀请码、邀请返现与提现方式排查

## 结论

- 当前本地 `main` 分支工作树干净，已实现邀请返利闭环。
- 项目里有两套“邀请码”：
  - 注册准入邀请码：`invitation_code`，基于 `redeem_codes.type=invitation`，只用于限制注册，不产生返利。
  - 邀请返利码：`aff_code`，基于 `user_affiliates.aff_code`，用于邀请关系绑定和返现。
- 邀请返利默认开启：`AffiliateEnabledDefault=true`，且迁移 `158_enable_affiliate_default.sql` 会把 `affiliate_enabled` 写为 `true`。
- 默认返利比例为 20%，范围 0-100；管理员可以给单个用户设置专属 `aff_code` 和专属返利比例。
- 用户“提现”当前不是外部打款，而是把可用返利额度转入 Sub2API 站内余额；没有实现支付宝、银行卡、USDT 等外部提现流程。

## 用户侧链路

- 用户页面：`/affiliate`。
- 用户 API：
  - `GET /api/v1/user/aff`：获取自己的返利码、邀请链接、返利比例、可用返利、冻结返利、历史返利和邀请用户列表。
  - `POST /api/v1/user/aff/transfer`：把全部可用返利额度转入用户余额。
- 前端邀请链接格式为 `/register?aff=AFF_CODE`；注册页也识别 `aff_code` 参数。
- 邮箱注册、邮箱验证注册、OAuth 新建账号都会尝试携带 `aff_code`，绑定失败只写日志，不阻断注册。
- 前端会把邀请来源码保存 30 天，用于跨页面和 OAuth 注册流程。

## 后端返利规则

- 新用户注册后会初始化自己的 affiliate profile，并按传入 `aff_code` 绑定邀请人。
- 绑定规则：
  - `aff_code` 会 trim 并转大写。
  - 格式允许 4-32 位大写字母、数字、下划线和短横线。
  - 不能邀请自己。
  - 已绑定邀请人后不会重复绑定。
  - 总开关关闭时注册阶段会静默忽略 `aff_code`。
- 返利触发点：
  - 支付订单：余额充值和订阅购买完成后触发。
  - 余额兑换码：正数余额兑换码兑换成功后触发。
- 返利基数：
  - `payment_orders.amount`，仅 `order_type=balance` 和 `order_type=subscription`。
  - 非 balance/subscription 的订单当前返回 0，不产生返利；因此流量包订单如果是独立 order type，当前不会返利。
- 返利公式：`rebate = baseAmount * rebateRatePercent / 100`，保留 8 位小数。
- 返利限制：
  - 全局比例：`affiliate_rebate_rate`，默认 20%。
  - 专属比例：`user_affiliates.aff_rebate_rate_percent`，存在时覆盖全局比例。
  - 冻结期：`affiliate_rebate_freeze_hours`，默认 0，最大 720 小时。
  - 有效期：`affiliate_rebate_duration_days`，默认 0 表示永久。
  - 单被邀请人返利上限：`affiliate_rebate_per_invitee_cap`，默认 0 表示无上限。
- 支付订单返利通过 `payment_audit_logs` 做订单级去重，避免同一订单重复返利。

## 提现/转余额方式

- 返利不会直接进入 `users.balance`。
- 无冻结期时返利进入 `user_affiliates.aff_quota`；有冻结期时先进入 `aff_frozen_quota`，到期后 lazy thaw 到可用额度。
- 用户点击 `/affiliate` 页面“转入余额”，调用 `POST /api/v1/user/aff/transfer`。
- 后端事务内：
  - 先解冻已到期返利。
  - `FOR UPDATE` 锁定并领取全部 `aff_quota`。
  - 把 `aff_quota` 清零。
  - 给 `users.balance` 和 `users.total_recharged` 增加同等金额。
  - 写入 `user_affiliate_ledger(action='transfer')`，包含余额和返利额度快照。
- 若没有可用返利，返回 `AFFILIATE_QUOTA_EMPTY`。

## 管理端能力

- 管理路由：
  - `/admin/affiliates/invites`
  - `/admin/affiliates/rebates`
  - `/admin/affiliates/transfers`
- 管理 API：
  - `GET /api/v1/admin/affiliates/invites`
  - `GET /api/v1/admin/affiliates/rebates`
  - `GET /api/v1/admin/affiliates/transfers`
  - `GET /api/v1/admin/affiliates/users`
  - `GET /api/v1/admin/affiliates/users/lookup`
  - `GET /api/v1/admin/affiliates/users/:user_id/overview`
  - `PUT /api/v1/admin/affiliates/users/:user_id`
  - `DELETE /api/v1/admin/affiliates/users/:user_id`
  - `POST /api/v1/admin/affiliates/users/batch-rate`
- 管理后台设置页支持开关 `affiliate_enabled`、全局比例、冻结期、有效期、单被邀请人上限，以及用户专属码/专属比例管理。

## 关键代码位置

- 后端核心：`backend/internal/service/affiliate_service.go`
- 后端存储：`backend/internal/repository/affiliate_repo.go`
- 支付订单触发返利：`backend/internal/service/payment_fulfillment.go`
- 兑换码触发返利：`backend/internal/service/redeem_service.go`
- 注册绑定返利码：`backend/internal/service/auth_service.go`
- 用户 API：`backend/internal/handler/user_handler.go`
- 路由：`backend/internal/server/routes/user.go`、`backend/internal/server/routes/admin.go`
- 用户页：`frontend/src/views/user/AffiliateView.vue`
- 注册页 aff 参数：`frontend/src/views/auth/RegisterView.vue`
- 前端 aff 持久化：`frontend/src/utils/oauthAffiliate.ts`
- 数据迁移：`backend/migrations/130_add_user_affiliates.sql`、`131_affiliate_rebate_hardening.sql`、`132_affiliate_custom_settings.sql`、`133_affiliate_rebate_freeze.sql`、`134_affiliate_ledger_audit_snapshots.sql`、`158_enable_affiliate_default.sql`
