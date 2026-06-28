# 邀请码与邀请返利能力梳理

## 结论

- 当前项目里有两套容易混淆的“邀请”能力：
  - `invitation_code`：注册准入邀请码，基于 `redeem_codes.type=invitation`，用于限制谁能注册。
  - `aff_code`：邀请返利码，基于 `user_affiliates.aff_code`，用于老用户邀请新用户并按新用户后续金额产生返利。
- 用户描述的“旧用户邀请新用户，新用户使用旧用户邀请码，旧用户获得新用户金额 xx%”对应的是 `affiliate` 邀请返利体系，不是 `invitation_code`。
- 源码已经有完整邀请返利链路；但 2026-06-28 公网公开设置显示 `affiliate_enabled=false`，当前线上总开关关闭；`invitation_code_enabled=false`，注册准入邀请码也关闭。

## 用户侧链路

- 用户页：`/affiliate`
- 用户 API：
  - `GET /api/v1/user/aff` 获取自己的邀请码、邀请链接、返利比例、可转余额度、冻结额度、邀请用户列表。
  - `POST /api/v1/user/aff/transfer` 把可用返利额度转入用户余额。
- 前端邀请链接为 `/register?aff=AFF_CODE`。
- 邮箱注册和 OAuth 创建账号时都会携带 `aff_code` 并尝试绑定邀请人。
- 绑定失败只记录日志，不阻断注册。

## 管理侧链路

- 管理路由：
  - `/admin/affiliates/invites`
  - `/admin/affiliates/rebates`
  - `/admin/affiliates/transfers`
- 管理 API：
  - `GET /api/v1/admin/affiliates/invites`
  - `GET /api/v1/admin/affiliates/rebates`
  - `GET /api/v1/admin/affiliates/transfers`
  - `GET /api/v1/admin/affiliates/users`
  - `PUT /api/v1/admin/affiliates/users/:user_id`
  - `DELETE /api/v1/admin/affiliates/users/:user_id`
  - `POST /api/v1/admin/affiliates/users/batch-rate`
- 管理端可设置用户专属 `aff_code` 和专属返利比例，专属比例优先于全局比例。

## 返利计算

- 全局开关：`affiliate_enabled`，默认关闭。
- 全局返利比例：`affiliate_rebate_rate`，默认 20%，范围 0-100。
- 用户专属比例：`user_affiliates.aff_rebate_rate_percent`，存在时覆盖全局比例。
- 返利公式：`rebate = baseAmount * rebateRatePercent / 100`，保留 8 位小数。
- 返利基准：
  - 支付订单：`payment_orders.amount`，仅 `balance` 和 `subscription` 订单触发。
  - 余额兑换码：正数 `RedeemTypeBalance` 兑换码触发。
  - 流量包订单当前不触发，因为 `affiliateRebateBaseAmount()` 对非 balance/subscription 返回 0。
- 支付订单层有 `payment_audit_logs` 去重，防止同一订单重复返利。

## 额度与提现方式

- 返利不会直接进用户余额，而是进入 `user_affiliates.aff_quota`。
- 如果配置了冻结期，会先进入 `aff_frozen_quota`，到期后在读取详情时 lazy thaw 到可用额度。
- 用户需要在 `/affiliate` 页面手动执行转入余额，转入记录写入 `user_affiliate_ledger`。
- 支持返利有效期 `affiliate_rebate_duration_days` 和单被邀请人返利上限 `affiliate_rebate_per_invitee_cap`。

## 数据表

- `user_affiliates`
  - `user_id`
  - `aff_code`
  - `inviter_id`
  - `aff_count`
  - `aff_quota`
  - `aff_frozen_quota`
  - `aff_history_quota`
  - `aff_rebate_rate_percent`
  - `aff_code_custom`
- `user_affiliate_ledger`
  - `action=accrue|transfer`
  - `amount`
  - `source_user_id`
  - `source_order_id`
  - 冻结和转余额后的快照字段。

## 注意点

- `invitation_code` 只是注册准入，不产生返利。
- `aff_code` 是返利邀请码，默认使用系统生成 12 位邀请码，也可由管理员改成专属码。
- 当前公网开关是关闭状态；即使用户带 `?aff=...` 注册，`BindInviterByCode()` 在开关关闭时会静默忽略。
- 若要启用，需要在后台设置打开 `affiliate_enabled`，并确认全局比例、冻结期、有效期和单人上限。
