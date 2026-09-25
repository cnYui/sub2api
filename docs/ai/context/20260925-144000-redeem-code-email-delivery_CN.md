# 兑换码邮件发放（2026-09-25）

## 背景

站长要给一批活动参与者（问卷收集、已在站内注册的用户）各发一张 ¥29 余额套餐兑换码，
要求：直接发到对方注册邮箱，**不用 3D 兑换卡链接**，重新写一个邮件模板，兑换码直接写在邮件里。

此前站内能「发码给用户」的只有 #55 的 3D 兑换卡分享链接（`/card/<token>`），通知邮件框架里没有
「把码发出去」的事件：`redeem.balance_credited` 是兑换**之后**的到账回执，码还做了掩码。

## 做法

按 AGENTS.md「加新通知应该往这个框架里加事件」的约定，没有另起发信逻辑：

- 新事件 `redeem.code_delivery`（`notification_email_service.go`），`Optional: false`：码写在信里，
  用户退订了就等于没发；也不挂 `purchase_notify_enabled` 开关，这是管理员当场触发的投递，不是自动通知。
- 占位符：`full_redeem_code`、`reward_name`、`reward_value`、`reward_note`、`code_expires_at`（外加框架公共的 4 个）。
  名字刻意不复用 `redeem_code`——那个在到账回执里是掩码值，预览示例也是掩码，复用会让后台预览看起来像发了掩码。
- 模板 `officialEmailRedeemCodeDelivery`（中英），外观沿用 #54 的白卡版式；码放进新 helper `emailHeroRedeemCode`：
  验证码那种 54px 大字放不下 32 位码，改为 22px 等宽（手机 16px、字距 0），`word-break:break-all`，
  **不插空格或分隔符**（兑换按原文精确匹配），32 位十六进制是一个"单词"，双击 / 长按即可整串选中。
- `reward_*` 由 `buildRedeemCodeDeliveryVariables` 按码类型生成：余额套餐取档位**当前值**（与兑换时
  `GrantByRedeemCode` 读的是同一份），普通余额取面值。说明里提示「已有生效中的余额套餐时需等它结束」，
  因为 `creditInitialBalance` 对零金额发放一律拒绝续费（`BALANCE_PACKAGE_ACTIVE`）。
- 接口 `POST /api/v1/admin/redeem-codes/:id/send-email`，body `{user_id}` / `{email}` 二选一、可选 `resend`，
  走 `executeAdminIdempotentJSON`（要 `Idempotency-Key`，幂等载荷带码 ID）。
- 只收未使用的正数余额码与余额套餐码：负数余额码是后台补扣，订阅 / 并发 / 邀请码在信里说不清到账内容。
- **一码一收件人**：settings `redeem_code_delivery:<码ID>` 记下第一个收件人；同一人再调返回 `already_sent`，
  `resend:true` 才重发（换一个 `ReminderKey`，否则通知框架按投递键去重直接跳过）；换人一律 409。
  这道闸只防管理员手滑发给两个人——码本身不绑定用户，谁拿到都能兑换。

## 测试

`redeem_code_delivery_test.go`（unit 标签）：占位符传满且无示例值泄漏、邮件里的码与原文逐字一致、
用本地测试 SMTP 实收一遍信、重复调用不重发、`resend` 重发、换人 409、已用 / 过期 / 负数 / 订阅码与
缺收件人、禁用用户、站外邮箱全部拒绝。`TestOfficialEmailTemplatesStayEmailClientSafe` 自动覆盖新模板。
