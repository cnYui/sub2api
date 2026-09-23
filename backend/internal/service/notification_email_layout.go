package service

import (
	"html"
	"strings"
)

// 通知邮件的统一外观：站点兑换卡白色版（白卡 + 青→靛→品红渐变描边 + 青/品红错位）。
//
// 邮件客户端对 CSS 的支持远不如浏览器，所以这里只用各家都认的写法：
//   - 表格布局、样式全部内联；<style> 只放媒体查询，被整段删掉时版面仍然完整。
//   - 有底色的单元格同时写 bgcolor 属性，Outlook Windows 只认属性。
//   - 渐变只写在 background-image 上并带纯色兜底，Outlook / Yahoo 不支持渐变。
//   - Gmail 不支持 text-shadow / box-shadow，青/品红错位只是锦上添花；标签前的小方块改用边框画，Gmail 里也在。
//
// 头像、字标和二维码是前端 public/email/ 下的静态文件，用 {{site_url}} 拼成绝对地址。
// 字标 PNG 外圈带白边：Gmail iOS、Windows 版 Outlook 深色模式会强行把白底翻成深色但不翻图片，
// 没有白边的黑字字标在那里会看不见。

const (
	emailFontSans = `'Noto Sans SC',-apple-system,BlinkMacSystemFont,'Segoe UI','PingFang SC','Hiragino Sans GB','Microsoft YaHei',sans-serif`
	emailFontMono = `'JetBrains Mono','SF Mono',Menlo,Consolas,'Liberation Mono',monospace`

	emailLabelStyle = `font-family:` + emailFontMono + `;font-size:12px;line-height:18px;font-weight:500;letter-spacing:.16em;color:#57606a;`
	emailBullet     = `<span style="display:inline-block;width:6px;height:6px;background-color:#0d1117;border-top:2px solid #17e0f8;border-left:2px solid #17e0f8;border-right:2px solid #ff1fd3;border-bottom:2px solid #ff1fd3;vertical-align:middle;margin:0 10px 2px 0;"></span>`

	emailAssetAvatar   = "{{site_url}}/email/avatar.png"
	emailAssetWordmark = "{{site_url}}/email/wordmark.png"
	emailAssetQRWechat = "{{site_url}}/email/qr-wechat.png"
	emailAssetQRSite   = "{{site_url}}/email/qr-site.png"

	// 空白占位字符把收件箱预览截在 preheader 处，避免后面的正文被拼进预览。
	emailPreheaderPadding = "&#8199;&#847;&#8199;&#847;&#8199;&#847;&#8199;&#847;&#8199;&#847;&#8199;&#847;&#8199;&#847;&#8199;&#847;&#8199;&#847;&#8199;&#847;&#8199;&#847;&#8199;&#847;&#8199;&#847;&#8199;&#847;&#8199;&#847;&#8199;&#847;&#8199;&#847;&#8199;&#847;&#8199;&#847;&#8199;&#847;"
)

const emailHeadStyle = `body{margin:0!important;padding:0!important;width:100%!important;-webkit-text-size-adjust:100%;-ms-text-size-adjust:100%}
a[x-apple-data-detectors]{color:inherit!important;text-decoration:none!important}
@media only screen and (min-width:600px){.amt{font-size:60px!important}}
@media only screen and (max-width:599px){.hc{max-width:100%!important}}
@media only screen and (max-width:480px){
.wr{padding:12px 8px 28px!important}
.sa{padding:28px 20px 26px!important}
.sb{padding:24px 20px 26px!important}
.sc{padding:24px 20px 28px!important}
.ac{width:52px!important;padding-right:14px!important}
.av{width:52px!important;height:52px!important}
.wm{max-width:220px!important}
.h1{font-size:25px!important}
.pn{padding:22px 14px 20px!important}
.amt{font-size:44px!important}
.no{font-size:14px!important}
.bt{width:100%!important}
.ba{display:block!important}
.qc{padding:0 8px!important}
.q{width:104px!important;height:104px!important}
}`

// emailPage 描述一封通知邮件。文本字段可以带 {{占位符}}，渲染时由通知框架统一转义替换。
type emailPage struct {
	locale    string
	title     string // <title> 与大标题
	eyebrow   string // 大标题上方的故障风小标签
	preheader string // 收件箱列表里的预览文字
	intro     string // 标题下的问候与说明，HTML
	band      string // 网格底纹带：玻璃面板、数据列、说明；可空
	body      string // 下半部分：明细、按钮等；可空
	// 卡片右下角的编号，仿兑换卡正面的 No.0001；serialValue 为空则不显示。
	serialLabel string
	serialValue string
	showQRCodes bool
	// 退订链接的文字；空表示这类邮件不可退订（验证码、风控、运维类）。
	unsubscribe string
	extraStyle  string
}

type emailStat struct {
	label string
	value string
	// narrow 用于「日期 时间」这类值：限宽让它在空格处折成两行，而不是在日期中间断开。
	narrow bool
	mono   bool
}

type emailRow struct {
	label  string
	value  string
	mono   bool
	accent bool
	// breakAll 用于上游返回的长串（URL、JSON），任意位置折行，避免撑破表格。
	breakAll bool
}

func (p emailPage) isChinese() bool {
	return normalizeNotificationLocale(p.locale) == notificationEmailLocaleChinese
}

func (p emailPage) render() string {
	lang := "en"
	if p.isChinese() {
		lang = "zh-CN"
	}
	hasBand := strings.TrimSpace(p.band) != ""
	hasLower := strings.TrimSpace(p.body) != "" || p.serialValue != ""

	var b strings.Builder
	_, _ = b.WriteString(`<!DOCTYPE html>
<html lang="` + lang + `">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="color-scheme" content="light only">
<meta name="supported-color-schemes" content="light only">
<meta name="format-detection" content="telephone=no,date=no,address=no,email=no">
<title>` + p.title + `</title>
<style>
` + emailHeadStyle + p.extraStyle + `
</style>
</head>
<body bgcolor="#f0f2f5" style="margin:0;padding:0;background-color:#f0f2f5;">
<div style="display:none;max-height:0;overflow:hidden;mso-hide:all;font-size:1px;line-height:1px;color:#f0f2f5;opacity:0;">` + p.preheader + emailPreheaderPadding + `</div>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" bgcolor="#f0f2f5" style="background-color:#f0f2f5;">
<tr><td class="wr" align="center" bgcolor="#f0f2f5" style="background-color:#f0f2f5;padding:36px 10px 40px;">
<!--[if mso]><table role="presentation" width="600" align="center" cellpadding="0" cellspacing="0" border="0"><tr><td><![endif]-->
<table role="presentation" width="600" cellpadding="0" cellspacing="0" border="0" style="width:100%;max-width:600px;">
<tr><td bgcolor="#3a3f6e" style="background-color:#3a3f6e;background-image:linear-gradient(135deg,#2b8a99 0%,#3a3f6e 50%,#8e2f72 100%);border-radius:24px;padding:3px;box-shadow:0 20px 48px #cdd2da;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0">
`)
	_, _ = b.WriteString(p.headerSection(!hasBand && !hasLower))
	if hasBand {
		_, _ = b.WriteString(emailBandSection(p.band, !hasLower))
	}
	if hasLower {
		_, _ = b.WriteString(p.lowerSection(!hasBand))
	}
	_, _ = b.WriteString(`</table>
</td></tr>
`)
	if p.showQRCodes {
		_, _ = b.WriteString(p.qrCodes())
	}
	_, _ = b.WriteString(p.footer())
	_, _ = b.WriteString(`</table>
<!--[if mso]></td></tr></table><![endif]-->
</td></tr>
</table>
</body>
</html>`)
	return b.String()
}

func (p emailPage) headerSection(isLast bool) string {
	radius := "21px 21px 0 0"
	if isLast {
		radius = "21px"
	}
	return `<tr><td class="sa" bgcolor="#ffffff" style="background-color:#ffffff;border-radius:` + radius + `;padding:40px 32px 34px;color:#0d1117;font-family:` + emailFontSans + `;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr>
<td class="ac" width="68" valign="middle" style="width:68px;padding-right:18px;"><img class="av" src="` + emailAssetAvatar + `" width="68" height="68" alt="" style="display:block;border:0;"></td>
<td valign="middle"><img class="wm" src="` + emailAssetWordmark + `" width="290" height="50" alt="{{site_name}}" style="display:block;border:0;width:100%;max-width:290px;height:auto;color:#0d1117;font-size:22px;font-weight:700;"></td>
</tr></table>
` + emailLabel(p.eyebrow, "padding-top:34px;") + `
<h1 class="h1" style="margin:8px 0 0;font-size:30px;line-height:1.3;font-weight:700;letter-spacing:.04em;color:#0d1117;">` + p.title + `</h1>
<p style="margin:12px 0 0;font-size:15px;line-height:1.8;color:#24292f;">` + p.intro + `</p>
</td></tr>
`
}

func emailBandSection(content string, isLast bool) string {
	edge := "border-bottom:1px solid #eaeef2;"
	if isLast {
		edge = "border-radius:0 0 21px 21px;"
	}
	return `<tr><td class="sb" bgcolor="#ffffff" style="background-color:#ffffff;background-image:linear-gradient(transparent 10px,#f1f1f1 10px,#f1f1f1 11px,transparent 11px),linear-gradient(90deg,#f1f1f1 1px,transparent 1px);background-size:20px 20px;border-top:1px solid #eaeef2;` + edge + `padding:32px 32px 30px;color:#0d1117;">
` + content + `
</td></tr>
`
}

func (p emailPage) lowerSection(needsDivider bool) string {
	divider := ""
	if needsDivider {
		divider = "border-top:1px solid #eaeef2;"
	}
	var b strings.Builder
	_, _ = b.WriteString(`<tr><td class="sc" bgcolor="#ffffff" style="background-color:#ffffff;` + divider + `border-radius:0 0 21px 21px;padding:30px 32px 34px;color:#0d1117;font-family:` + emailFontSans + `;">
`)
	_, _ = b.WriteString(p.body)
	if p.serialValue != "" {
		_, _ = b.WriteString(`
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr><td style="padding-top:28px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr><td align="right" style="padding-top:22px;border-top:1px dashed #d0d7de;font-family:` + emailFontMono + `;color:#0d1117;">
<div style="font-size:11px;line-height:16px;font-weight:500;letter-spacing:.16em;color:#57606a;">` + p.serialLabel + `</div>
<div class="no" style="padding-top:2px;font-size:15px;line-height:22px;font-weight:700;letter-spacing:.02em;color:#0d1117;word-break:break-all;">` + p.serialValue + `</div>
</td></tr></table>
</td></tr></table>`)
	}
	_, _ = b.WriteString(`
</td></tr>
`)
	return b.String()
}

func (p emailPage) qrCodes() string {
	wechatTitle, wechatCaption, siteTitle := "WeChat group", "天才程序员聚集地", "Website"
	wechatAlt, siteAlt := "WeChat group QR code", "Website QR code"
	if p.isChinese() {
		wechatTitle, siteTitle = "微信群", "官网"
		wechatAlt, siteAlt = "微信群二维码", "官网二维码"
	}
	return `<tr><td align="center" style="padding-top:30px;">
<table role="presentation" cellpadding="0" cellspacing="0" border="0"><tr>
` + emailQRCode(emailAssetQRWechat, wechatAlt, wechatTitle, wechatCaption) + `
` + emailQRCode(emailAssetQRSite, siteAlt, siteTitle, "aaccx.pw") + `
</tr></table>
</td></tr>
`
}

func emailQRCode(src, alt, title, caption string) string {
	return `<td class="qc" align="center" valign="top" style="padding:0 10px;color:#424a53;font-family:` + emailFontSans + `;">
<table role="presentation" cellpadding="0" cellspacing="0" border="0"><tr><td bgcolor="#ffffff" style="background-color:#ffffff;border:1px solid #dfe3e8;border-radius:16px;padding:8px;box-shadow:0 10px 24px #dce0e6;"><img class="q" src="` + src + `" width="112" height="112" alt="` + alt + `" style="display:block;border:0;"></td></tr></table>
<div style="padding-top:10px;font-size:13px;line-height:18px;font-weight:700;color:#0d1117;">` + title + `</div>
<div style="font-size:12px;line-height:18px;color:#57606a;">` + caption + `</div>
</td>`
}

func (p emailPage) footer() string {
	text := `This email was sent automatically by {{site_name}}. Please do not reply.`
	if p.isChinese() {
		text = `此邮件由 {{site_name}} 自动发送，<span style="white-space:nowrap;">请勿直接回复。</span>`
	}
	if p.unsubscribe != "" {
		text += `<br><a href="{{unsubscribe_url}}" target="_blank" style="color:#424a53;text-decoration:underline;">` + p.unsubscribe + `</a>`
	}
	return `<tr><td align="center" style="padding:26px 12px 0;font-family:` + emailFontSans + `;font-size:12px;line-height:1.8;color:#57606a;">` + text + `</td></tr>
`
}

// emailLabel 是等宽字体的故障风小标签；extraStyle 用来调整上下间距。
func emailLabel(text, extraStyle string) string {
	return `<div style="` + extraStyle + emailLabelStyle + `">` + emailBullet + text + `</div>`
}

// emailHero 是网格带中央的玻璃面板，放本封邮件最重要的一个数。
func emailHero(label, value string) string {
	return emailPanel(emailLabel(label, "") + `
<div class="amt" style="padding-top:8px;font-family:` + emailFontMono + `;font-size:54px;line-height:1.15;font-weight:700;color:#0d1117;text-shadow:-3px -2px 0 #17e0f8,3px 2px 0 #ff1fd3;word-break:break-all;">` + value + `</div>`)
}

// emailHeroCode 用于验证码：字距拉开，便于逐位抄写。
func emailHeroCode(label, value string) string {
	return emailPanel(emailLabel(label, "") + `
<div class="amt" style="padding-top:8px;font-family:` + emailFontMono + `;font-size:54px;line-height:1.15;font-weight:700;letter-spacing:.18em;color:#0d1117;text-shadow:-3px -2px 0 #17e0f8,3px 2px 0 #ff1fd3;word-break:break-all;">` + value + `</div>`)
}

func emailPanel(inner string) string {
	return `<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr>
<td class="pn" align="center" bgcolor="#f6f8fa" style="background-color:#f6f8fa;background-image:linear-gradient(160deg,#ffffff 0%,#f6f8fa 45%,#f0f3f6 100%);border:1px solid #e3e6ea;border-radius:18px;padding:26px 12px 24px;box-shadow:0 14px 32px #e1e5ea;color:#0d1117;font-family:` + emailFontSans + `;">
` + inner + `
</td></tr></table>`
}

// emailStats 是面板下方的数据列：一到两项占一行；四项在电脑上排一行，手机上折成 2×2。
func emailStats(stats ...emailStat) string {
	if len(stats) == 4 {
		return `
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr><td align="center" style="font-size:0;line-height:0;">
<!--[if mso]><table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr><td width="50%" valign="top"><![endif]-->
` + emailStatGroup(stats[0], stats[1]) + `
<!--[if mso]></td><td width="50%" valign="top"><![endif]-->
` + emailStatGroup(stats[2], stats[3]) + `
<!--[if mso]></td></tr></table><![endif]-->
</td></tr></table>`
	}
	cells := make([]string, 0, len(stats))
	for _, stat := range stats {
		cells = append(cells, emailStatCell(stat, len(stats)))
	}
	return `
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr><td style="padding-top:24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr>
` + strings.Join(cells, "\n") + `
</tr></table>
</td></tr></table>`
}

func emailStatGroup(left, right emailStat) string {
	return `<div class="hc" style="display:inline-block;vertical-align:top;width:100%;max-width:265px;padding-top:24px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr>
` + emailStatCell(left, 2) + `
` + emailStatCell(right, 2) + `
</tr></table>
</div>`
}

func emailStatCell(stat emailStat, columns int) string {
	width := "50%"
	switch columns {
	case 1:
		width = "100%"
	case 3:
		width = "33%"
	}
	valueStyle := "padding-top:4px;font-size:16px;line-height:22px;font-weight:700;color:#0d1117;word-break:break-word;"
	if stat.narrow {
		valueStyle += "max-width:7em;margin:0 auto;"
	}
	if stat.mono {
		valueStyle += "font-family:" + emailFontMono + ";"
	}
	return `<td width="` + width + `" align="center" valign="top" style="padding:0 4px;color:#0d1117;font-family:` + emailFontSans + `;"><div style="font-size:12px;line-height:18px;font-weight:500;color:#57606a;">` + stat.label + `</div><div style="` + valueStyle + `">` + stat.value + `</div></td>`
}

// emailPanelNote 是玻璃面板里、主内容下方的一行小字。
func emailPanelNote(text string) string {
	return `
<div style="padding-top:14px;font-size:12px;line-height:20px;color:#57606a;">` + text + `</div>`
}

// emailBandNote 是网格带底部居中的小字说明。
func emailBandNote(text string) string {
	return `
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr><td align="center" style="padding-top:22px;font-family:` + emailFontSans + `;font-size:12px;line-height:20px;color:#57606a;">` + text + `</td></tr></table>`
}

// emailDetails 是带标题的左右两列明细表。
func emailDetails(label string, rows ...emailRow) string {
	return emailDetailsTable(label, false, rows)
}

// emailDetailsFixed 固定表格布局，给可能出现超长值（上游报错原文）的明细用。
func emailDetailsFixed(label string, rows ...emailRow) string {
	return emailDetailsTable(label, true, rows)
}

func emailDetailsTable(label string, fixed bool, rows []emailRow) string {
	tableStyle := ""
	labelWidth := ""
	if fixed {
		tableStyle = ` style="table-layout:fixed;"`
		labelWidth = `width="112" `
	}
	var b strings.Builder
	_, _ = b.WriteString(emailLabel(label, "padding-bottom:6px;"))
	_, _ = b.WriteString("\n<table role=\"presentation\" width=\"100%\" cellpadding=\"0\" cellspacing=\"0\" border=\"0\"" + tableStyle + ">\n")
	for i, row := range rows {
		border := "border-bottom:1px solid #eaeef2;"
		if i == len(rows)-1 {
			border = ""
		}
		valueStyle := "padding:12px 0;" + border + "font-size:14px;line-height:20px;font-weight:700;color:#0d1117;overflow-wrap:anywhere;word-break:break-word;"
		if row.breakAll {
			valueStyle = "padding:12px 0;" + border + "font-size:13px;line-height:20px;font-weight:500;color:#0d1117;overflow-wrap:anywhere;word-break:break-all;white-space:pre-wrap;"
		}
		if row.mono {
			valueStyle += "font-family:" + emailFontMono + ";"
		}
		if row.accent {
			valueStyle += "color:#1a7f37;"
		}
		_, _ = b.WriteString(`<tr><td ` + labelWidth + `valign="top" style="padding:12px 16px 12px 0;` + border + `font-size:14px;line-height:20px;color:#57606a;white-space:nowrap;">` + row.label + `</td><td align="right" style="` + valueStyle + `">` + row.value + "</td></tr>\n")
	}
	_, _ = b.WriteString("</table>")
	return b.String()
}

// emailButton 是卡片里的主按钮：黑底白字，Apple 邮件里带青/品红错位描边。
func emailButton(text, href string) string {
	return emailButtonAt(text, href, "30px")
}

// emailButtonAt 可以调整按钮上方的间距，放进玻璃面板时用更小的值。
func emailButtonAt(text, href, paddingTop string) string {
	return `
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0"><tr><td align="center" style="padding-top:` + paddingTop + `;">
<table class="bt" role="presentation" cellpadding="0" cellspacing="0" border="0"><tr>
<td align="center" bgcolor="#0d1117" style="background-color:#0d1117;border-radius:12px;color:#ffffff;mso-padding-alt:15px 44px;box-shadow:-3px -3px 0 #17e0f8,3px 3px 0 #ff1fd3;"><a class="ba" href="` + href + `" target="_blank" style="display:inline-block;padding:15px 44px;font-size:16px;line-height:22px;font-weight:700;letter-spacing:.08em;color:#ffffff;text-decoration:none;border-radius:12px;">` + text + ` &rarr;</a></td>
</tr></table>
</td></tr></table>`
}

func emailParagraph(text string) string {
	return `
<p style="margin:18px 0 0;font-size:14px;line-height:1.8;color:#24292f;">` + text + `</p>`
}

func emailMuted(text string) string {
	return `
<p style="margin:16px 0 0;font-size:12px;line-height:1.8;color:#57606a;">` + text + `</p>`
}

// emailLinkFallback 给按钮点不动的客户端留一条可复制的原始链接。
func emailLinkFallback(prefix, href string) string {
	return `
<p style="margin:18px 0 0;font-size:12px;line-height:1.7;color:#57606a;">` + prefix + `<br><a href="` + href + `" target="_blank" style="color:#0969da;text-decoration:underline;word-break:break-all;">` + href + `</a></p>`
}

// renderStaticEmail 用于不走通知框架的邮件（例如 SMTP 测试邮件），手工替换少量占位符。
func renderStaticEmail(page string, variables map[string]string) string {
	return notificationEmailPlaceholderPattern.ReplaceAllStringFunc(page, func(match string) string {
		parts := notificationEmailPlaceholderPattern.FindStringSubmatch(match)
		if len(parts) != 2 {
			return ""
		}
		value := variables[parts[1]]
		if strings.HasSuffix(parts[1], "_url") && !isSafeNotificationEmailURL(value) {
			value = ""
		}
		return html.EscapeString(value)
	})
}
