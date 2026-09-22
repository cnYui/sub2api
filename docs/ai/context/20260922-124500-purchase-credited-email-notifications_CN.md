# 购买/到账成功邮件通知（余额套餐 / 流量卡 / 兑换码）

- 日期：2026-09-22
- 触发：管理员问「用户购买套餐或充值流量卡后，能不能也用 SMTP 发一封确认邮件」
- 状态：**代码已完成并全绿，未提交、未部署**（工作区还有其它未提交改动，见文末）

## 一、为什么便宜

项目里早就有一套「通知邮件」框架（`backend/internal/service/notification_email_service.go`），
注册验证码和忘记密码只是它的两个事件，另外还有订阅到期、余额不足、风控、运维报表、
报销发票共 12 个事件在用。框架自带：

- 后台可视化改模板 + 预览（`/admin/settings/email-templates`）
- 中英双语官方模板
- 占位符白名单校验
- 退订链接与退订记录（`Optional: true` 的事件才有）
- 按 `(事件, 来源类型, 来源 ID, 收件人, reminderKey)` 的发送去重

所以这次是**填空**，不是造轮子。`PaymentService` 上连 `notificationEmailService`
字段都早就有了（原本只用来记语言偏好）。

## 二、挂载点

`markCompleted`（`payment_fulfillment.go:362`）是余额套餐和流量卡两种订单**唯一的汇聚点**：

- 流量卡 → `ExecuteTrafficPackFulfillment` → `markCompleted(…, "TRAFFIC_PACK_SUCCESS")`
- 余额套餐 → `doBalancePackage` → `markCompleted(…, "BALANCE_PACKAGE_SUCCESS")`

它前面有 fulfillment lease + `status=recharging → completed` 的乐观锁，
所以同一订单只会成功一次，不管由支付回调、前端轮询还是管理员重试触发。
发信放在**赢下这一跳之后**，`updated == 0 && 已 completed` 的早返回分支不发。

管理员发放（`GrantBalancePackage`）不经 `markCompleted`，直接置 `completed`，
所以单独在提交事务后挂了一次。兑换码挂在 `RedeemService.Redeem` 提交之后。

## 三、改了什么

### 新增 3 个通知事件（`notification_email_service.go`）

| 事件 | 触发 | 关键占位符 |
| --- | --- | --- |
| `payment.balance_package_credited` | 余额套餐首期到账（新购 / 续费 / 管理员发放共用） | `purchase_kind` `plan_name` `pay_amount` `weekly_credit_usd` `remaining_usd` `refresh_count` `refresh_interval_days` `next_credit_at` `expires_at` `order_no` `dashboard_url` |
| `payment.traffic_pack_credited` | 流量卡支付完成、额度入账 | `pack_name` `pay_amount` `credit_usd` `validity_days` `expires_at` `order_no` `dashboard_url` |
| `redeem.balance_credited` | 兑换码加普通余额成功 | `redeem_code`（掩码） `credit_usd` `current_balance` `dashboard_url` |

三个都是 `Optional: true` / `Category: billing`，中英官方模板齐全，用户可自行退订。

### 新增服务 `purchase_notify_service.go`

`PurchaseNotifyService`，被 `PaymentService` 和 `RedeemService` 各持一份。
三个 `Notify*` 方法都是**异步**（`go` + 30s 超时的 `context.Background()`），
失败只 `slog.Warn`，绝不影响已经落库的订单或兑换。

变量构造抽成了三个纯函数（`buildBalancePackageNoticeVariables` 等），便于回归测试。

### 新增全局开关

`purchase_notify_enabled`，**缺省视为开启**（用 `!isFalseSettingValue`，与
`subscription_expiry_notify_enabled` 同款）。后台「设置 → 邮件」里新加了一张卡片。
改动链路：`domain_constants.go` → `settings_view.go` → `setting_parse.go` →
`setting_update.go` → `dto/settings.go` → 三个 `setting_handler*.go` →
前端 `api/admin/settings.ts` + `SettingsView.vue` + 中英 i18n。

### wire

新增 `ProvideRedeemService`（原来直接用 `NewRedeemService`），`ProvidePaymentService`
多一个参数。`wire_gen.go` 用 `go run -mod=mod github.com/google/wire/cmd/wire gen ./cmd/server`
重新生成，diff 只有 4 行增 3 行删。**改之前先跑过 `wire diff` 确认基线是干净的。**

## 四、踩到/避开的坑

1. **`runtimeVariables` 会先铺一层"预览示例值"再覆盖调用方传入的变量。**
   事件声明了却没传的占位符，会把 `张三` / `https://example.com/...` 这种示例值
   发给真实用户。`ops.scheduled_report` 那段代码就是专门防这个的。
   已加回归测试 `TestPurchaseNoticeVariablesCoverDeclaredPlaceholders`：
   逐事件断言「事件声明的占位符 ⊆ 服务实际传的占位符」。
   另一个测试真的渲染一遍模板，断言结果里不含任何示例值片段。
   **第一版就因为这个测试红了**（漏了框架侧的 `recipient_name` / `unsubscribe_url`，
   测试改成模拟框架补值后才是真阳性）。

2. **`EmailService.SendEmail` 是同步 SMTP。** 支付回调里同步连 SMTP，
   慢了会让易支付/微信判超时并重推回调，所以必须异步。

3. **负数兑换码绝不能发"到账"。** 后台手工补扣用的是负数 `admin_balance` 兑换码
   （AGENTS.md 坑 30），发出去就是「您已成功兑换 $-33.5」。
   `sendRedeemBalanceNotice` 里 `amountUSD <= 0` 直接返回，并有测试钉死。
   `create-and-redeem` 后台接口也走 `Redeem`，同样被这条挡住。

4. **续费与新购共用一条代码路径。** `creditInitialBalance` 内部可能走
   `renewBalancePackage`，调用方看不出区别。靠套餐行的 `renewal_count > 0` 判定"续费"，
   `PaymentType == admin_grant` 判定"管理员发放"，其余为"新购"，渲染成 `purchase_kind`。
   续费会把 `payment_order_id` 改绑到新订单，所以按订单 id 查套餐行对两种情况都成立。

5. **管理员发放是零金额订单**，`pay_amount` 会渲染成 `0.00 CNY`，已改写成「赠送」。

6. **前端路由是 `/subscriptions` 不是 `/subscription`**，写错就是死链，已加测试钉死。

7. **API 契约快照测试会红。** `internal/server/api_contract_test.go` 固定了
   `GET /admin/settings` 的完整字段集合，加一个设置项就要同步改两处。

## 五、验证

- `go build ./...` 通过
- `go test -tags=unit ./internal/service/` 全量通过（146s）
- `go test -tags=unit ./internal/server/... ./internal/handler/...` 通过
- `go test -tags=unit -run '^$' ./...` 全包编译通过
- `golangci-lint run --build-tags=unit`：59 条全是既有测试文件的问题，**新增文件零告警**
- `vue-tsc --noEmit` 通过；`SettingsView.spec.ts` 29 项通过

**没做的**：没有真的发一封邮件到真实邮箱。生产 SMTP 已配置（注册/忘密在用），
上线后建议先自己买一笔最小额度或用后台发放一次，确认收件与文案。

## 六、注意

- 这次只碰了上面列的文件。工作区里另有**一批与本次无关的未提交改动**
  （套餐「提前刷新」前端 + `balance_package_service.go` / `payment_handler.go` /
  `routes/payment.go` / `SubscriptionsView.vue` / `misc.ts` / `types/payment.ts`），
  是别的会话留下的，没有动。提交时别把两拨混在一个 commit 里。
- 出镜像才生效（push main → 自动部署到生产 Mac）。
