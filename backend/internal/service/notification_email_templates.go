package service

import "context"

// 各通知事件的官方模板。外观由 notification_email_layout.go 统一渲染，这里只描述每封邮件放什么。
//
// 占位符在发送时换成真实取值，有几种会让句子读不通，写文案时要避开：
//   - next_credit_at / expires_at 缺失时是「—」，只能放在「标签：值」里，不能写进句子；
//   - pay_amount 在管理员发放时是「赠送」，兑换码兑换时是「—」；
//   - purchase_kind 可能是 新购 / 续费 / 管理员发放 / 兑换码兑换。
var notificationEmailOfficialTemplates = map[string]map[string]notificationEmailOfficialTemplate{
	NotificationEmailEventAuthVerifyCode: {
		notificationEmailDefaultLocale: {Subject: "[{{site_name}}] Email verification code", HTML: officialEmailVerifyCode(notificationEmailDefaultLocale, false)},
		notificationEmailLocaleChinese: {Subject: "[{{site_name}}] 邮箱验证码", HTML: officialEmailVerifyCode(notificationEmailLocaleChinese, false)},
	},
	NotificationEmailEventAuthPasswordReset: {
		notificationEmailDefaultLocale: {Subject: "[{{site_name}}] Password reset request", HTML: officialEmailPasswordReset(notificationEmailDefaultLocale)},
		notificationEmailLocaleChinese: {Subject: "[{{site_name}}] 密码重置请求", HTML: officialEmailPasswordReset(notificationEmailLocaleChinese)},
	},
	NotificationEmailEventNotificationEmailVerifyCode: {
		notificationEmailDefaultLocale: {Subject: "[{{site_name}}] Notification email verification code", HTML: officialEmailVerifyCode(notificationEmailDefaultLocale, true)},
		notificationEmailLocaleChinese: {Subject: "[{{site_name}}] 通知邮箱验证码", HTML: officialEmailVerifyCode(notificationEmailLocaleChinese, true)},
	},
	NotificationEmailEventSubscriptionExpiryReminder: {
		notificationEmailDefaultLocale: {Subject: "[{{site_name}}] Subscription expires in {{days_remaining}} day(s)", HTML: officialEmailSubscriptionExpiry(notificationEmailDefaultLocale)},
		notificationEmailLocaleChinese: {Subject: "[{{site_name}}] 订阅将在 {{days_remaining}} 天后到期", HTML: officialEmailSubscriptionExpiry(notificationEmailLocaleChinese)},
	},
	NotificationEmailEventBalanceLow: {
		notificationEmailDefaultLocale: {Subject: "[{{site_name}}] Low balance alert", HTML: officialEmailBalanceLow(notificationEmailDefaultLocale)},
		notificationEmailLocaleChinese: {Subject: "[{{site_name}}] 余额不足提醒", HTML: officialEmailBalanceLow(notificationEmailLocaleChinese)},
	},
	NotificationEmailEventAccountQuotaAlert: {
		notificationEmailDefaultLocale: {Subject: "[{{site_name}}] Account quota alert - {{account_name}}", HTML: officialEmailAccountQuotaAlert(notificationEmailDefaultLocale)},
		notificationEmailLocaleChinese: {Subject: "[{{site_name}}] 账号限额告警 - {{account_name}}", HTML: officialEmailAccountQuotaAlert(notificationEmailLocaleChinese)},
	},
	NotificationEmailEventContentModerationViolation: {
		notificationEmailDefaultLocale: {Subject: "[{{site_name}}] Risk control notice", HTML: officialEmailModerationViolation(notificationEmailDefaultLocale)},
		notificationEmailLocaleChinese: {Subject: "[{{site_name}}] 账户风控提醒", HTML: officialEmailModerationViolation(notificationEmailLocaleChinese)},
	},
	NotificationEmailEventContentModerationDisabled: {
		notificationEmailDefaultLocale: {Subject: "[{{site_name}}] Account disabled by risk control", HTML: officialEmailModerationDisabled(notificationEmailDefaultLocale)},
		notificationEmailLocaleChinese: {Subject: "[{{site_name}}] 账户已被禁用", HTML: officialEmailModerationDisabled(notificationEmailLocaleChinese)},
	},
	NotificationEmailEventCyberPolicyNotice: {
		notificationEmailDefaultLocale: {Subject: "[{{site_name}}] Cyber-security policy notice", HTML: officialEmailCyberPolicyNotice(notificationEmailDefaultLocale)},
		notificationEmailLocaleChinese: {Subject: "[{{site_name}}] 网络安全策略拦截提醒", HTML: officialEmailCyberPolicyNotice(notificationEmailLocaleChinese)},
	},
	NotificationEmailEventOpsAlert: {
		notificationEmailDefaultLocale: {Subject: "[Ops Alert][{{severity}}] {{rule_name}}", HTML: officialEmailOpsAlert(notificationEmailDefaultLocale)},
		notificationEmailLocaleChinese: {Subject: "[运维告警][{{severity}}] {{rule_name}}", HTML: officialEmailOpsAlert(notificationEmailLocaleChinese)},
	},
	NotificationEmailEventOpsScheduledReport: {
		notificationEmailDefaultLocale: {Subject: "[Ops Report] {{report_name}}", HTML: officialEmailOpsScheduledReport(notificationEmailDefaultLocale)},
		notificationEmailLocaleChinese: {Subject: "[运维报表] {{report_name}}", HTML: officialEmailOpsScheduledReport(notificationEmailLocaleChinese)},
	},
	NotificationEmailEventBalancePackageCredited: {
		notificationEmailDefaultLocale: {Subject: "[{{site_name}}] {{plan_name}} is active - {{purchase_kind}}", HTML: officialEmailBalancePackageCredited(notificationEmailDefaultLocale)},
		notificationEmailLocaleChinese: {Subject: "[{{site_name}}] {{plan_name}} 已生效（{{purchase_kind}}）", HTML: officialEmailBalancePackageCredited(notificationEmailLocaleChinese)},
	},
	NotificationEmailEventTrafficPackCredited: {
		notificationEmailDefaultLocale: {Subject: "[{{site_name}}] {{pack_name}} credited", HTML: officialEmailTrafficPackCredited(notificationEmailDefaultLocale)},
		notificationEmailLocaleChinese: {Subject: "[{{site_name}}] {{pack_name}} 已到账", HTML: officialEmailTrafficPackCredited(notificationEmailLocaleChinese)},
	},
	NotificationEmailEventRedeemBalanceCredited: {
		notificationEmailDefaultLocale: {Subject: "[{{site_name}}] ${{credit_usd}} credited to your balance", HTML: officialEmailRedeemBalanceCredited(notificationEmailDefaultLocale)},
		notificationEmailLocaleChinese: {Subject: "[{{site_name}}] 兑换成功，余额到账 ${{credit_usd}}", HTML: officialEmailRedeemBalanceCredited(notificationEmailLocaleChinese)},
	},
	NotificationEmailEventReimbursementCompleted: {
		notificationEmailDefaultLocale: {Subject: "[{{site_name}}] Invoice ready for request #{{request_id}}", HTML: officialEmailReimbursementCompleted(notificationEmailDefaultLocale)},
		notificationEmailLocaleChinese: {Subject: "[{{site_name}}] 开票申请 #{{request_id}} 的发票已上传", HTML: officialEmailReimbursementCompleted(notificationEmailLocaleChinese)},
	},
}

func emailIsChinese(locale string) bool {
	return normalizeNotificationLocale(locale) == notificationEmailLocaleChinese
}

func emailGreeting(zh bool, text string) string {
	if zh {
		return "{{recipient_name}}，您好：<br>" + text
	}
	return "Hello {{recipient_name}},<br>" + text
}

// officialEmailVerifyCode 同时用于注册/绑定验证码和通知邮箱验证码，两者只有标题和说明不同。
func officialEmailVerifyCode(locale string, notificationAddress bool) string {
	if emailIsChinese(locale) {
		title, intro := "邮箱验证码", "请使用下面的验证码完成验证。"
		if notificationAddress {
			title, intro = "通知邮箱验证", "您正在添加额外的通知邮箱，请输入下面的验证码完成验证。"
		}
		return emailPage{
			locale:      locale,
			title:       title,
			eyebrow:     "安全验证",
			preheader:   "您的验证码是 {{verification_code}}，{{expires_in_minutes}} 分钟内有效。",
			intro:       emailGreeting(true, intro),
			band:        emailHeroCode("验证码", "{{verification_code}}") + emailBandNote("验证码 {{expires_in_minutes}} 分钟内有效，请勿告诉任何人。"),
			body:        emailMuted("如果不是您本人操作，请忽略此邮件。"),
			showQRCodes: true,
		}.render()
	}
	title, intro := "Email verification code", "Use the code below to finish verification."
	if notificationAddress {
		title, intro = "Verify your notification email", "You are adding this address as an extra notification email. Enter the code below to confirm it."
	}
	return emailPage{
		locale:      locale,
		title:       title,
		eyebrow:     "VERIFICATION",
		preheader:   "Your verification code is {{verification_code}}. It expires in {{expires_in_minutes}} minutes.",
		intro:       emailGreeting(false, intro),
		band:        emailHeroCode("Code", "{{verification_code}}") + emailBandNote("The code expires in {{expires_in_minutes}} minutes. Never share it with anyone."),
		body:        emailMuted("If you did not request this code, you can safely ignore this email."),
		showQRCodes: true,
	}.render()
}

func officialEmailPasswordReset(locale string) string {
	if emailIsChinese(locale) {
		return emailPage{
			locale:    locale,
			title:     "密码重置",
			eyebrow:   "安全验证",
			preheader: "点击邮件里的按钮设置新密码，链接 {{expires_in_minutes}} 分钟内有效。",
			intro:     emailGreeting(true, "我们收到了您的密码重置请求，点击下方按钮设置新密码。"),
			band: emailPanel(emailLabel("重置链接", "") + emailButtonAt("重置密码", "{{reset_url}}", "18px") +
				emailPanelNote("链接 {{expires_in_minutes}} 分钟内有效")),
			body: emailLinkFallback("如果按钮无法点击，请复制以下链接到浏览器中打开：", "{{reset_url}}") +
				emailMuted("如果不是您本人操作，请忽略此邮件，您的密码不会改变。"),
			showQRCodes: true,
		}.render()
	}
	return emailPage{
		locale:    locale,
		title:     "Reset your password",
		eyebrow:   "SECURITY",
		preheader: "Use the button in this email to choose a new password. The link expires in {{expires_in_minutes}} minutes.",
		intro:     emailGreeting(false, "We received a request to reset your password. Use the button below to choose a new one."),
		band: emailPanel(emailLabel("Reset link", "") + emailButtonAt("Reset password", "{{reset_url}}", "18px") +
			emailPanelNote("The link expires in {{expires_in_minutes}} minutes")),
		body: emailLinkFallback("If the button does not work, copy this link into your browser:", "{{reset_url}}") +
			emailMuted("If you did not request this, you can ignore this email and your password will stay the same."),
		showQRCodes: true,
	}.render()
}

func officialEmailSubscriptionExpiry(locale string) string {
	if emailIsChinese(locale) {
		return emailPage{
			locale:    locale,
			title:     "订阅即将到期",
			eyebrow:   "到期提醒",
			preheader: "您的 {{subscription_group}} 订阅将在 {{days_remaining}} 天后到期。",
			intro:     emailGreeting(true, "您的 <strong>{{subscription_group}}</strong> 订阅将在 {{days_remaining}} 天后到期。"),
			band: emailHero("剩余天数", "{{days_remaining}} 天") + emailStats(
				emailStat{label: "订阅分组", value: "{{subscription_group}}"},
				emailStat{label: "到期时间", value: "{{expiry_time}}", narrow: true},
			),
			body:        emailButton("查看我的订阅", "{{site_url}}/subscriptions"),
			showQRCodes: true,
			unsubscribe: "退订此类订阅提醒",
		}.render()
	}
	return emailPage{
		locale:    locale,
		title:     "Subscription expiring soon",
		eyebrow:   "REMINDER",
		preheader: "Your {{subscription_group}} subscription expires in {{days_remaining}} day(s).",
		intro:     emailGreeting(false, "Your <strong>{{subscription_group}}</strong> subscription expires in {{days_remaining}} day(s)."),
		band: emailHero("Days left", "{{days_remaining}}") + emailStats(
			emailStat{label: "Group", value: "{{subscription_group}}"},
			emailStat{label: "Expires at", value: "{{expiry_time}}", narrow: true},
		),
		body:        emailButton("View my subscriptions", "{{site_url}}/subscriptions"),
		showQRCodes: true,
		unsubscribe: "Unsubscribe from subscription reminders",
	}.render()
}

func officialEmailBalanceLow(locale string) string {
	if emailIsChinese(locale) {
		return emailPage{
			locale:      locale,
			title:       "余额不足提醒",
			eyebrow:     "余额提醒",
			preheader:   "您的余额已低于 ${{threshold}}，请及时充值。",
			intro:       emailGreeting(true, "您当前的余额已低于提醒阈值，请及时充值，以免服务中断。"),
			band:        emailHero("当前余额", "${{current_balance}}") + emailStats(emailStat{label: "提醒阈值", value: "${{threshold}}"}),
			body:        emailButton("立即充值", "{{recharge_url}}"),
			showQRCodes: true,
			unsubscribe: "退订此类余额提醒",
		}.render()
	}
	return emailPage{
		locale:      locale,
		title:       "Low balance alert",
		eyebrow:     "BALANCE ALERT",
		preheader:   "Your balance is below ${{threshold}}. Please recharge soon.",
		intro:       emailGreeting(false, "Your balance has dropped below your alert threshold. Recharge soon to avoid service interruptions."),
		band:        emailHero("Current balance", "${{current_balance}}") + emailStats(emailStat{label: "Alert threshold", value: "${{threshold}}"}),
		body:        emailButton("Recharge now", "{{recharge_url}}"),
		showQRCodes: true,
		unsubscribe: "Unsubscribe from balance alerts",
	}.render()
}

// officialEmailAccountQuotaAlert 发给管理员，不带问候、二维码和退订。
func officialEmailAccountQuotaAlert(locale string) string {
	if emailIsChinese(locale) {
		return emailPage{
			locale:    locale,
			title:     "账号限额告警",
			eyebrow:   "运维告警",
			preheader: "上游账号 {{account_name}} 已触发额度告警。",
			intro:     "上游账号 <strong>{{account_name}}</strong> 已触发配置的额度告警阈值。",
			band: emailHero("剩余额度", "{{quota_remaining}}") + emailStats(
				emailStat{label: "已用", value: "{{quota_used}}"},
				emailStat{label: "限额", value: "{{quota_limit}}"},
				emailStat{label: "告警阈值", value: "{{quota_threshold}}"},
				emailStat{label: "维度", value: "{{quota_dimension}}"},
			),
			body: emailDetails("账号信息",
				emailRow{label: "账号 ID", value: "{{account_id}}", mono: true},
				emailRow{label: "账号名称", value: "{{account_name}}"},
				emailRow{label: "平台", value: "{{platform}}"},
			),
		}.render()
	}
	return emailPage{
		locale:    locale,
		title:     "Account quota alert",
		eyebrow:   "OPS ALERT",
		preheader: "Upstream account {{account_name}} crossed its quota alert threshold.",
		intro:     "The upstream account <strong>{{account_name}}</strong> has crossed its configured quota alert threshold.",
		band: emailHero("Remaining", "{{quota_remaining}}") + emailStats(
			emailStat{label: "Used", value: "{{quota_used}}"},
			emailStat{label: "Limit", value: "{{quota_limit}}"},
			emailStat{label: "Threshold", value: "{{quota_threshold}}"},
			emailStat{label: "Dimension", value: "{{quota_dimension}}"},
		),
		body: emailDetails("ACCOUNT",
			emailRow{label: "Account ID", value: "{{account_id}}", mono: true},
			emailRow{label: "Account name", value: "{{account_name}}"},
			emailRow{label: "Platform", value: "{{platform}}"},
		),
	}.render()
}

func officialEmailModerationViolation(locale string) string {
	if emailIsChinese(locale) {
		return emailPage{
			locale:    locale,
			title:     "账户风控提醒",
			eyebrow:   "风控提醒",
			preheader: "您的 API 请求触发了平台风控策略，请检查请求内容。",
			intro:     emailGreeting(true, "您的 API 请求触发了平台内容审核/风控策略。"),
			// 自动禁用受 AutoBanEnabled 开关控制，所以只能说「可能」。
			band: emailHero("累计触发次数", "{{violation_count}} / {{ban_threshold}}") + emailBandNote("累计次数达到上限后，账户可能会被自动禁用。"),
			body: emailDetails("触发详情",
				emailRow{label: "触发时间", value: "{{triggered_at}}"},
				emailRow{label: "所属分组", value: "{{group_name}}"},
				emailRow{label: "命中类别", value: "{{moderation_category}}"},
				emailRow{label: "风险分数", value: "{{moderation_score}}", mono: true},
			) + emailParagraph("请检查请求内容，避免后续服务受到影响。"),
			showQRCodes: true,
		}.render()
	}
	return emailPage{
		locale:    locale,
		title:     "Risk control notice",
		eyebrow:   "RISK CONTROL",
		preheader: "Your API request triggered the platform risk-control policy.",
		intro:     emailGreeting(false, "Your API request triggered the platform content moderation / risk-control policy."),
		band:      emailHero("Violations", "{{violation_count}} / {{ban_threshold}}") + emailBandNote("Your account may be disabled automatically once the limit is reached."),
		body: emailDetails("DETAILS",
			emailRow{label: "Triggered at", value: "{{triggered_at}}"},
			emailRow{label: "Group", value: "{{group_name}}"},
			emailRow{label: "Category", value: "{{moderation_category}}"},
			emailRow{label: "Score", value: "{{moderation_score}}", mono: true},
		) + emailParagraph("Please review your request content to avoid future service interruptions."),
		showQRCodes: true,
	}.render()
}

func officialEmailModerationDisabled(locale string) string {
	if emailIsChinese(locale) {
		return emailPage{
			locale:    locale,
			title:     "账户已被禁用",
			eyebrow:   "风控提醒",
			preheader: "您的账户多次触发风控规则，已被系统自动禁用。",
			intro:     emailGreeting(true, "您的账户在统计周期内多次触发平台内容审核/风控规则，<br>系统已自动禁用该账户。"),
			band: emailHero("账户状态", "已禁用") + emailStats(
				emailStat{label: "累计触发次数", value: "{{violation_count}} / {{ban_threshold}}"},
				emailStat{label: "禁用时间", value: "{{triggered_at}}", narrow: true},
			),
			body: emailDetails("触发详情",
				emailRow{label: "所属分组", value: "{{group_name}}"},
				emailRow{label: "命中类别", value: "{{moderation_category}}"},
				emailRow{label: "风险分数", value: "{{moderation_score}}", mono: true},
			) + emailParagraph("如需申诉或恢复账号，请联系平台管理员处理。"),
			showQRCodes: true,
		}.render()
	}
	return emailPage{
		locale:    locale,
		title:     "Account disabled",
		eyebrow:   "RISK CONTROL",
		preheader: "Your account repeatedly triggered risk-control rules and has been disabled.",
		intro:     emailGreeting(false, "Your account repeatedly triggered content moderation / risk-control rules and has been disabled automatically."),
		band: emailHero("Account status", "Disabled") + emailStats(
			emailStat{label: "Violations", value: "{{violation_count}} / {{ban_threshold}}"},
			emailStat{label: "Disabled at", value: "{{triggered_at}}", narrow: true},
		),
		body: emailDetails("DETAILS",
			emailRow{label: "Group", value: "{{group_name}}"},
			emailRow{label: "Category", value: "{{moderation_category}}"},
			emailRow{label: "Score", value: "{{moderation_score}}", mono: true},
		) + emailParagraph("Please contact the administrator if you need to appeal or restore access."),
		showQRCodes: true,
	}.render()
}

// officialEmailCyberPolicyNotice 的上游说明可能是很长的原文，明细表必须固定布局并允许任意位置折行。
func officialEmailCyberPolicyNotice(locale string) string {
	if emailIsChinese(locale) {
		return emailPage{
			locale:    locale,
			title:     "请求被安全策略拦截",
			eyebrow:   "安全策略",
			preheader: "您的请求被上游服务商的网络安全策略拦截。",
			intro:     emailGreeting(true, "您的请求被上游服务商的网络安全策略（cyber policy）拦截。"),
			body: emailDetailsFixed("拦截详情",
				emailRow{label: "触发时间", value: "{{triggered_at}}"},
				emailRow{label: "模型", value: "{{model}}", mono: true},
				emailRow{label: "所属分组", value: "{{group_name}}"},
				emailRow{label: "上游说明", value: "{{upstream_message}}", breakAll: true},
			) + emailParagraph("如认为系误判，可调整请求措辞后重试，或申请获得授权的安全访问权限。"),
			showQRCodes: true,
		}.render()
	}
	return emailPage{
		locale:    locale,
		title:     "Request blocked by security policy",
		eyebrow:   "SECURITY POLICY",
		preheader: "Your request was blocked by the upstream provider's cyber-security policy.",
		intro:     emailGreeting(false, "Your request was blocked by the upstream provider's cyber-security policy."),
		body: emailDetailsFixed("DETAILS",
			emailRow{label: "Triggered at", value: "{{triggered_at}}"},
			emailRow{label: "Model", value: "{{model}}", mono: true},
			emailRow{label: "Group", value: "{{group_name}}"},
			emailRow{label: "Upstream message", value: "{{upstream_message}}", breakAll: true},
		) + emailParagraph("If you believe this is a mistake, try rephrasing your request, or apply for authorized security access."),
		showQRCodes: true,
	}.render()
}

func officialEmailOpsAlert(locale string) string {
	if emailIsChinese(locale) {
		return emailPage{
			locale:    locale,
			title:     "{{rule_name}}",
			eyebrow:   "运维告警 · {{severity}}",
			preheader: "{{rule_name}}：{{metric_type}} {{operator}} {{threshold_value}}，当前 {{metric_value}}。",
			intro:     "告警规则已触发，当前状态：<strong>{{alert_status}}</strong>。",
			band: emailHero("{{metric_type}}", "{{metric_value}}") + emailStats(
				emailStat{label: "阈值", value: "{{operator}} {{threshold_value}}", mono: true},
				emailStat{label: "严重级别", value: "{{severity}}"},
				emailStat{label: "状态", value: "{{alert_status}}"},
				emailStat{label: "触发时间", value: "{{triggered_at}}", narrow: true},
			),
			body: emailLabel("告警说明", "padding-bottom:6px;") + emailParagraph("{{alert_description}}"),
		}.render()
	}
	return emailPage{
		locale:    locale,
		title:     "{{rule_name}}",
		eyebrow:   "OPS ALERT · {{severity}}",
		preheader: "{{rule_name}}: {{metric_type}} {{operator}} {{threshold_value}}, currently {{metric_value}}.",
		intro:     "The alert rule fired. Current status: <strong>{{alert_status}}</strong>.",
		band: emailHero("{{metric_type}}", "{{metric_value}}") + emailStats(
			emailStat{label: "Threshold", value: "{{operator}} {{threshold_value}}", mono: true},
			emailStat{label: "Severity", value: "{{severity}}"},
			emailStat{label: "Status", value: "{{alert_status}}"},
			emailStat{label: "Fired at", value: "{{triggered_at}}", narrow: true},
		),
		body: emailLabel("DESCRIPTION", "padding-bottom:6px;") + emailParagraph("{{alert_description}}"),
	}.render()
}

// officialEmailOpsScheduledReport 的两个 display 占位符必须保持 `display: {{...}};` 的写法：
// 发送侧只填 report_html 时靠它隐藏汇总区（见 runtimeVariables），后台也允许单独隐藏某一块。
func officialEmailOpsScheduledReport(locale string) string {
	zh := emailIsChinese(locale)
	text := func(zhText, enText string) string {
		if zh {
			return zhText
		}
		return enText
	}
	metric := func(label, value, color string) string {
		return `<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr><td bgcolor="#f6f8fa" style="background-color:#f6f8fa;border:1px solid #eaeef2;border-radius:12px;padding:14px 16px;color:#0d1117;font-family:` + emailFontSans + `;"><div style="font-size:12px;line-height:18px;color:#57606a;">` + label + `</div><div style="padding-top:6px;font-family:` + emailFontMono + `;font-size:20px;line-height:26px;font-weight:700;color:` + color + `;">` + value + `</div></td></tr></table>`
	}
	spacer := `<div style="height:22px;line-height:22px;font-size:0;">&nbsp;</div>`
	summary := `<div style="display: {{report_summary_display}};">` +
		emailLabel(text("请求概览", "REQUESTS"), "padding-bottom:12px;") + `
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0">
<tr><td width="50%" valign="top" style="padding:0 5px 10px 0;">` + metric(text("总请求数", "Total requests"), "{{report_total_requests}}", "#0d1117") + `</td><td width="50%" valign="top" style="padding:0 0 10px 5px;">` + metric(text("成功请求", "Successful"), "{{report_success_count}}", "#1a7f37") + `</td></tr>
<tr><td width="50%" valign="top" style="padding:0 5px 0 0;">` + metric(text("SLA 错误", "SLA errors"), "{{report_sla_error_count}}", "#cf222e") + `</td><td width="50%" valign="top" style="padding:0 0 0 5px;">` + metric(text("业务限流", "Business limited"), "{{report_business_limited_count}}", "#0d1117") + `</td></tr>
</table>` + spacer +
		emailDetails(text("可靠性", "RELIABILITY"),
			emailRow{label: "SLA", value: "{{report_sla}}", mono: true},
			emailRow{label: text("错误率", "Error rate"), value: "{{report_error_rate}}", mono: true},
			emailRow{label: text("上游错误率（不含 429 / 529）", "Upstream error rate (excl. 429 / 529)"), value: "{{report_upstream_error_rate}}", mono: true},
			emailRow{label: text("上游错误（不含 429 / 529）", "Upstream errors (excl. 429 / 529)"), value: "{{report_upstream_error_count_excl_429_529}}", mono: true},
			emailRow{label: text("上游 429 / 529", "Upstream 429 / 529"), value: "{{report_upstream_429_count}} / {{report_upstream_529_count}}", mono: true},
		) + spacer +
		emailDetails(text("延迟表现", "LATENCY"),
			emailRow{label: text("请求延迟 p50 / p99", "Latency p50 / p99"), value: "{{report_latency_p50}} / {{report_latency_p99}}", mono: true},
			emailRow{label: text("首 Token 时间 p50 / p99", "TTFT p50 / p99"), value: "{{report_ttft_p50}} / {{report_ttft_p99}}", mono: true},
		) + spacer +
		emailDetails(text("吞吐量", "THROUGHPUT"),
			emailRow{label: text("Token 消耗", "Tokens"), value: "{{report_tokens}}", mono: true},
			emailRow{label: text("QPS（当前 / 峰值 / 平均）", "QPS (now / peak / avg)"), value: "{{report_qps_current}} / {{report_qps_peak}} / {{report_qps_avg}}", mono: true},
			emailRow{label: text("TPS（当前 / 峰值 / 平均）", "TPS (now / peak / avg)"), value: "{{report_tps_current}} / {{report_tps_peak}} / {{report_tps_avg}}", mono: true},
		) + `</div>
<div class="report-detail" style="display: {{report_detail_display}};">{{report_html}}</div>`

	return emailPage{
		locale:    locale,
		title:     "{{report_name}}",
		eyebrow:   text("运维报表", "OPS REPORT"),
		preheader: text("{{site_name}} 的运行概览：{{report_start_time}} 至 {{report_end_time}}（UTC）", "{{site_name}} runtime overview: {{report_start_time}} to {{report_end_time}} (UTC)"),
		intro:     text("{{site_name}} 的运行概览。", "Runtime overview for {{site_name}}."),
		band: emailStats(
			emailStat{label: text("报表", "Report"), value: "{{report_name}}"},
			emailStat{label: text("类型", "Type"), value: "{{report_type}}", mono: true},
		) + emailBandNote(text(
			`统计周期：<span style="white-space:nowrap;">{{report_start_time}}</span> 至 <span style="white-space:nowrap;">{{report_end_time}}</span>（UTC）`,
			`Period: <span style="white-space:nowrap;">{{report_start_time}}</span> to <span style="white-space:nowrap;">{{report_end_time}}</span> (UTC)`,
		)),
		body:       summary,
		extraStyle: "\n.report-detail{margin-top:24px}\n.report-detail:empty{display:none}",
	}.render()
}

func officialEmailBalancePackageCredited(locale string) string {
	if emailIsChinese(locale) {
		return emailPage{
			locale:    locale,
			title:     "余额套餐已生效",
			eyebrow:   "到账通知",
			preheader: "{{plan_name}} 已生效，本期可用 ${{remaining_usd}}。",
			intro:     emailGreeting(true, "您的余额套餐已生效，首期额度已到账。"),
			band: emailHero("本期可用", "${{remaining_usd}}") + emailStats(
				emailStat{label: "每期额度", value: "${{weekly_credit_usd}}"},
				emailStat{label: "下次到账", value: "{{next_credit_at}}", narrow: true},
				emailStat{label: "发放周期", value: "共 {{refresh_count}} 期<br>每 {{refresh_interval_days}} 天一期"},
				emailStat{label: "有效期至", value: "{{expires_at}}", narrow: true},
			) + emailBandNote("如果此前余额为负，本次额度会先抵扣欠费，<br>所以「本期可用」可能小于「每期额度」。"),
			// 这封信只在首期到账时发（新购、续费、发放、兑换都会把期数重置为 1），所以进度恒为 1。
			body: emailDetails("套餐明细",
				emailRow{label: "套餐", value: "{{plan_name}}"},
				emailRow{label: "类型", value: "{{purchase_kind}}"},
				emailRow{label: "实付", value: "{{pay_amount}}"},
				emailRow{label: "到账进度", value: "1 / {{refresh_count}} 期", mono: true, accent: true},
			) + emailButton("查看我的套餐", "{{dashboard_url}}"),
			serialLabel: "订单号",
			serialValue: "No.{{order_no}}",
			showQRCodes: true,
			unsubscribe: "退订此类通知",
		}.render()
	}
	return emailPage{
		locale:    locale,
		title:     "Balance package active",
		eyebrow:   "CREDITED",
		preheader: "{{plan_name}} is active. ${{remaining_usd}} is available this period.",
		intro:     emailGreeting(false, "Your balance package is active and the first period has been credited."),
		band: emailHero("Available this period", "${{remaining_usd}}") + emailStats(
			emailStat{label: "Credit per period", value: "${{weekly_credit_usd}}"},
			emailStat{label: "Next credit", value: "{{next_credit_at}}", narrow: true},
			emailStat{label: "Schedule", value: "{{refresh_count}} periods<br>every {{refresh_interval_days}} days"},
			emailStat{label: "Valid until", value: "{{expires_at}}", narrow: true},
		) + emailBandNote("If you had a negative balance, this credit repaid it first,<br>so the amount available this period can be lower."),
		body: emailDetails("PACKAGE",
			emailRow{label: "Package", value: "{{plan_name}}"},
			emailRow{label: "Type", value: "{{purchase_kind}}"},
			emailRow{label: "Paid", value: "{{pay_amount}}"},
			emailRow{label: "Credited", value: "1 / {{refresh_count}}", mono: true, accent: true},
		) + emailButton("View my package", "{{dashboard_url}}"),
		serialLabel: "ORDER",
		serialValue: "No.{{order_no}}",
		showQRCodes: true,
		unsubscribe: "Unsubscribe from these notices",
	}.render()
}

// officialEmailTrafficPackCredited 的按钮指向订单页：流量卡只在订单列表里逐张展示，剩余额度在页头。
func officialEmailTrafficPackCredited(locale string) string {
	if emailIsChinese(locale) {
		return emailPage{
			locale:    locale,
			title:     "流量卡已到账",
			eyebrow:   "到账通知",
			preheader: "{{pack_name}} 已到账，额度 ${{credit_usd}}。",
			intro:     emailGreeting(true, "您购买的流量卡额度已到账，可以直接使用。"),
			band: emailHero("流量卡额度", "${{credit_usd}}") + emailStats(
				emailStat{label: "流量卡", value: "{{pack_name}}"},
				emailStat{label: "实付", value: "{{pay_amount}}"},
				emailStat{label: "有效期", value: "{{validity_days}} 天"},
				emailStat{label: "到期时间", value: "{{expires_at}}", narrow: true},
			) + emailBandNote("流量卡额度在普通余额为负时启用，<br>到期未用完的部分会失效。"),
			body:        emailButton("查看我的订单", "{{dashboard_url}}"),
			serialLabel: "订单号",
			serialValue: "No.{{order_no}}",
			showQRCodes: true,
			unsubscribe: "退订此类通知",
		}.render()
	}
	return emailPage{
		locale:    locale,
		title:     "Traffic pack credited",
		eyebrow:   "CREDITED",
		preheader: "{{pack_name}} has been credited with ${{credit_usd}}.",
		intro:     emailGreeting(false, "Your traffic pack has been credited and is ready to use."),
		band: emailHero("Quota", "${{credit_usd}}") + emailStats(
			emailStat{label: "Pack", value: "{{pack_name}}"},
			emailStat{label: "Paid", value: "{{pay_amount}}"},
			emailStat{label: "Validity", value: "{{validity_days}} days"},
			emailStat{label: "Expires", value: "{{expires_at}}", narrow: true},
		) + emailBandNote("Traffic pack quota is used once your regular balance goes negative.<br>Unused quota is forfeited at expiry."),
		body:        emailButton("View my orders", "{{dashboard_url}}"),
		serialLabel: "ORDER",
		serialValue: "No.{{order_no}}",
		showQRCodes: true,
		unsubscribe: "Unsubscribe from these notices",
	}.render()
}

func officialEmailRedeemBalanceCredited(locale string) string {
	if emailIsChinese(locale) {
		return emailPage{
			locale:    locale,
			title:     "兑换成功",
			eyebrow:   "到账通知",
			preheader: "兑换成功，${{credit_usd}} 已加入账户余额。",
			intro:     emailGreeting(true, "您的兑换码已使用成功，额度已加入账户余额。"),
			band: emailHero("本次到账", "${{credit_usd}}") + emailStats(
				emailStat{label: "当前余额", value: "${{current_balance}}"},
				emailStat{label: "兑换码", value: "{{redeem_code}}", mono: true},
			),
			body:        emailButton("查看我的余额", "{{dashboard_url}}"),
			showQRCodes: true,
			unsubscribe: "退订此类通知",
		}.render()
	}
	return emailPage{
		locale:    locale,
		title:     "Redeem code applied",
		eyebrow:   "CREDITED",
		preheader: "${{credit_usd}} has been added to your balance.",
		intro:     emailGreeting(false, "Your redeem code has been applied and the credit is now in your balance."),
		band: emailHero("Credited", "${{credit_usd}}") + emailStats(
			emailStat{label: "Current balance", value: "${{current_balance}}"},
			emailStat{label: "Redeem code", value: "{{redeem_code}}", mono: true},
		),
		body:        emailButton("View my balance", "{{dashboard_url}}"),
		showQRCodes: true,
		unsubscribe: "Unsubscribe from these notices",
	}.render()
}

func officialEmailReimbursementCompleted(locale string) string {
	if emailIsChinese(locale) {
		return emailPage{
			locale:    locale,
			title:     "发票已上传",
			eyebrow:   "开票通知",
			preheader: "开票申请 #{{request_id}} 的发票已上传，可以下载了。",
			intro:     emailGreeting(true, "您提交的报销/开票申请已处理完成，发票 PDF 已上传。"),
			band: emailHero("开票金额", "¥{{amount}}") + emailStats(
				emailStat{label: "申请编号", value: "#{{request_id}}", mono: true},
				emailStat{label: "开票抬头", value: "{{company_name}}"},
			),
			body:        emailButton("前往下载", "{{download_page_url}}") + emailMuted("也可以登录后在「报销/开票」页面下载发票 PDF。"),
			showQRCodes: true,
			unsubscribe: "退订此类通知",
		}.render()
	}
	return emailPage{
		locale:    locale,
		title:     "Your invoice is ready",
		eyebrow:   "INVOICE",
		preheader: "The invoice for request #{{request_id}} is ready to download.",
		intro:     emailGreeting(false, "Your reimbursement / invoice request is complete and the invoice PDF has been uploaded."),
		band: emailHero("Amount", "¥{{amount}}") + emailStats(
			emailStat{label: "Request ID", value: "#{{request_id}}", mono: true},
			emailStat{label: "Company", value: "{{company_name}}"},
		),
		body:        emailButton("Download invoice", "{{download_page_url}}") + emailMuted("You can also sign in and download the invoice PDF from the reimbursement page."),
		showQRCodes: true,
		unsubscribe: "Unsubscribe from these notices",
	}.render()
}

// SMTPTestEmailHTML 渲染后台「发送测试邮件」的正文，外观与通知邮件一致，方便管理员一眼看到真实效果。
func (s *NotificationEmailService) SMTPTestEmailHTML(ctx context.Context, siteName string) string {
	page := emailPage{
		locale:    notificationEmailLocaleChinese,
		title:     "SMTP 配置成功",
		eyebrow:   "系统通知",
		preheader: "这是一封测试邮件，SMTP 设置可以正常发信。",
		intro:     "这是一封测试邮件，收到它说明当前的 SMTP 设置可以正常发信。",
		band:      emailHero("发信状态", "OK"),
	}.render()
	return renderStaticEmail(page, map[string]string{
		"site_name": siteName,
		"site_url":  s.siteURL(ctx),
	})
}
