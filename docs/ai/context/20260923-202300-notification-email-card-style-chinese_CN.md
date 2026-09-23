# 通知邮件改为兑换卡白色版外观，并全部用中文发送（2026-09-23）

## 需求

站长当天的要求，按时间顺序：

1. 先后看过 Anthropic 配色、深色兑换卡、白色兑换卡三轮样稿，最后选定白色兑换卡的「方案一」，去掉 GitHub 活跃度热力图，其余保持不变。
2. 按方案一给所有 SMTP 邮件做同一套外观，重点是流量卡到账、兑换码到账等到账邮件。
3. 测试邮件里显示的套餐信息，以及「查看我的套餐」这类按钮的跳转。
4. 全部用中文发送邮件。
5. 测完直接提 PR。本地起完整环境、在浏览器里点按钮需要下载 Redis 镜像，站长选择不下载，所以跳转只做到服务层和前端路由表两级验证。

## 改动

- `backend/internal/service/notification_email_layout.go`（新增）：外观渲染器。页面分三段：头部（头像 + 字标 + 故障风标签 + 标题）、
  网格底纹带（玻璃面板大数字 + 数据列 + 说明）、下半部分（明细表、按钮、右下角 `No.` 编号），卡片下方是两个二维码和页脚。
- `backend/internal/service/notification_email_templates.go`（新增）：15 个事件的中英文官方模板，SMTP 测试邮件正文。
  旧的 `notificationEmailCard` 和运维报表模板已删除。
- `notification_email_service.go`：
  - 新增公共占位符 `site_url`，先取 `frontend_url`，再取 `api_base_url`。预览和发送都会填这个值。
  - `ResolveRecipientLocale` 恒返回中文，`Send` 不再看 `input.Locale`。
  - 新增 `renderForSend`：自定义模板损坏时退回官方模板。
  - 预览示例补了 `model` 和 `upstream_message` 两个值。
- `purchase_notify_service.go`：流量卡按钮从 `/subscriptions` 改为 `/orders`。「我的订阅」页只列余额套餐，流量卡只能在订单页逐张看到，剩余额度显示在页头。
- `balance_notify_service.go`：账号限额维度标签从「日限额 / Daily」这类双语写法改成纯中文。
- `ops_scheduled_report_service.go`：错误摘要、账号健康报表的明细 HTML 改成中文。
- `handler/admin/setting_handler_email.go`：后台「发送测试邮件」改用新外观，主题改为「SMTP 测试邮件」。
- `frontend/public/email/`（新增）：`avatar.png`、`wordmark.png`、`qr-wechat.png`、`qr-site.png`，都已压缩到 8–22KB。
  字标用的是白色版原图，外圈加了白边。

## 设计取舍

- 只用邮件客户端普遍支持的写法：表格布局、样式内联；有底色的单元格同时写 `bgcolor`，因为 Outlook Windows 只认这个属性。
  渐变只写在 `background-image` 上，并带纯色兜底。
- 根据 caniemail 的数据，Gmail 不支持 `text-shadow` 和 `box-shadow`，所以：
  - 大数字的青/品红错位、按钮的错位描边，在 Gmail 里会变成纯黑；
  - 标签前的小方块改用四条彩色边框画，Gmail 里也能显示；
  - 字标直接用带错位效果的图片。
- 预览时 `site_url` 也用真实站点地址，否则后台预览里的图片是裂的。现有测试会检查示例值里的 `example.com` 不能漏进正式邮件，这样也不会被它拦下。
- 订阅到期提醒新增了按钮「查看我的订阅」，链接是 `{{site_url}}/subscriptions`。原来这封邮件没有任何站内链接。
- 各服务里旧的兜底正文（大多是英文或中英双语）没有改。有了 `renderForSend` 之后，正常的生产接线下已经走不到这些分支。

## 测试

- `go test -tags=unit ./...` 全部通过。golangci-lint 对改动包的检查结果为 0 条。
- 新增 `purchase_notify_email_e2e_test.go`，流程是 SQLite 真实库表 → `PurchaseNotifyService` → 测试 SMTP 服务器收信 → 解码主题和正文后断言。覆盖：
  - 余额套餐的四种来源：新购、续费、管理员发放、兑换码兑换。断言套餐名、类型、实付（`赠送` / `—`）、本期可用、每期额度、`1 / 4 期`、下次到账、有效期、订单号，以及按钮 `/subscriptions`；
  - 流量卡：断言卡名、额度、实付、有效期和按钮 `/orders`；
  - 兑换码：断言到账金额、当前余额、掩码后的兑换码，以及按钮 `/redeem`；
  - 浏览器语言为英文的用户，收到的仍是纯中文邮件；
  - 路由检查：读取 `frontend/src/router/index.ts`，确认上述路径都存在。
- 新增 `notification_email_layout_test.go`：
  - 对全部 30 份官方模板检查邮件兼容约束；
  - `site_url` 的取值顺序、去掉尾斜杠、拒绝 `javascript:`；
  - 全部中文发信；
  - 自定义模板损坏时退回官方模板；
  - SMTP 测试邮件。
- 反向验证：把流量卡路径临时改回 `/subscriptions`，E2E 测试和路径测试都会失败。
- 截图检查：用无头 Edge 渲染全部模板，在电脑宽度和 375px 手机宽度下逐张看过。

## 生产事实（2026-09-23 只读核对）

- `frontend_url = https://aaccx.pw`，`api_base_url` 为空，`site_name = 天才程序员小站`，SMTP 已配置。
- `settings` 里没有任何 `notification_email_template:*` 键，也就是没有自定义模板，上线后新官方模板直接生效。
- 用浏览器、GoogleImageProxy、Microsoft Office 三种 UA 请求 `https://aaccx.pw/logo.png` 都返回 200 `image/png`，
  说明邮箱图片代理能取到站点静态图，Cloudflare 规则没有拦截。

## 上线后核对

1. 下面四个地址都应返回 200 `image/png`：`https://aaccx.pw/email/avatar.png`、`wordmark.png`、`qr-wechat.png`、`qr-site.png`。
2. 在后台「设置 → 邮件模板」预览几个事件，图片要能正常加载。
3. 用后台「发送测试邮件」发到 Gmail 和 QQ 邮箱各一封，看真实效果。

## 已知限制

- 微信群二维码约 7 天过期，需要定期替换 `frontend/public/email/qr-wechat.png` 后推 main。
- 这次没有真实点击邮件按钮，只验证到链接地址和路由表两级。
- 回滚：直接 revert 这个 PR。没有迁移，也没有数据改动。
