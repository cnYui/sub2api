//go:build unit

package service

import (
	"context"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// 邮件客户端只认很窄的一组 CSS，破坏这些约束的模板会在某些邮箱里整块错位或看不见，
// 靠肉眼预览很难发现（浏览器里一切正常），所以这里对全部官方模板逐条钉死。
func TestOfficialEmailTemplatesStayEmailClientSafe(t *testing.T) {
	styleBlock := regexp.MustCompile(`(?is)<style\b.*?</style>`)
	forbidden := map[string]*regexp.Regexp{
		"flex/grid layout":      regexp.MustCompile(`display\s*:\s*(inline-)?(flex|grid)`),
		"position":              regexp.MustCompile(`(^|[;"\s])position\s*:`),
		"float":                 regexp.MustCompile(`(^|[;"\s])float\s*:`),
		"translucent colors":    regexp.MustCompile(`\b(rgba|hsla)\s*\(`),
		"script":                regexp.MustCompile(`(?i)<script\b`),
		"external css or fonts": regexp.MustCompile(`(?i)<link\b|@import|@font-face`),
	}
	tableTag := regexp.MustCompile(`<table\b[^>]*>`)
	coloredCell := regexp.MustCompile(`<td\b[^>]*background-color:[^>]*>`)
	imgTag := regexp.MustCompile(`<img\b[^>]*>`)
	anchorTag := regexp.MustCompile(`<a\b[^>]*>`)
	assetSrc := regexp.MustCompile(`src="\{\{site_url\}\}/email/[a-z-]+\.png"`)

	svc := NewNotificationEmailService(newNotificationEmailMemorySettingRepo(), nil)
	for _, event := range notificationEmailEventOrder {
		for _, locale := range svc.SupportedLocales() {
			tmpl, ok := notificationEmailOfficialTemplates[event][locale]
			require.Truef(t, ok, "%s/%s has no official template", event, locale)
			where := event + "/" + locale

			require.NoError(t, validateNotificationEmailTemplate(event, tmpl.Subject, tmpl.HTML), where)
			// 模板硬上限 30000 字节，官方模板要给后台自定义和较长的占位符取值留余量。
			require.LessOrEqualf(t, len(tmpl.HTML), 24000, "%s is %d bytes", where, len(tmpl.HTML))
			require.Containsf(t, tmpl.HTML, `<meta name="color-scheme" content="light only">`, where)
			wantLang := `lang="en"`
			if locale == notificationEmailLocaleChinese {
				wantLang = `lang="zh-CN"`
			}
			require.Containsf(t, tmpl.HTML, wantLang, where)

			inline := styleBlock.ReplaceAllString(tmpl.HTML, "")
			for name, pattern := range forbidden {
				require.Falsef(t, pattern.MatchString(inline), "%s uses %s", where, name)
			}
			for _, tag := range tableTag.FindAllString(tmpl.HTML, -1) {
				require.Containsf(t, tag, `role="presentation"`, "%s: %s", where, tag)
			}
			// Outlook Windows 只认 bgcolor 属性，只写 CSS 底色的单元格在那里是透明的。
			for _, tag := range coloredCell.FindAllString(tmpl.HTML, -1) {
				require.Containsf(t, tag, `bgcolor="`, "%s: %s", where, tag)
			}
			for _, tag := range imgTag.FindAllString(tmpl.HTML, -1) {
				require.Regexpf(t, assetSrc, tag, "%s: images must be site assets", where)
				require.Containsf(t, tag, `width="`, "%s: %s", where, tag)
				require.Containsf(t, tag, `height="`, "%s: %s", where, tag)
				require.Containsf(t, tag, `alt="`, "%s: %s", where, tag)
			}
			for _, tag := range anchorTag.FindAllString(tmpl.HTML, -1) {
				require.Regexpf(t, `color:#[0-9a-f]{6}`, tag, "%s: link color must be explicit: %s", where, tag)
				require.Containsf(t, tag, "text-decoration:", "%s: %s", where, tag)
			}

			if notificationEmailEventDefinitions[event].Optional {
				require.Containsf(t, tmpl.HTML, `href="{{unsubscribe_url}}"`, "%s is optional and must offer unsubscribe", where)
			} else {
				require.NotContainsf(t, tmpl.HTML, "{{unsubscribe_url}}", "%s is transactional and must not offer unsubscribe", where)
			}
		}
	}
}

func TestNotificationEmailSiteURLPrefersFrontendURL(t *testing.T) {
	ctx := context.Background()
	repo := newNotificationEmailMemorySettingRepo()
	svc := NewNotificationEmailService(repo, nil)

	require.Equal(t, "", svc.siteURL(ctx))
	require.NoError(t, repo.Set(ctx, SettingKeyAPIBaseURL, "https://api.panel.test/"))
	require.Equal(t, "https://api.panel.test", svc.siteURL(ctx))
	require.NoError(t, repo.Set(ctx, SettingKeyFrontendURL, " https://panel.test/ "))
	require.Equal(t, "https://panel.test", svc.siteURL(ctx))

	// 后台预览同样用真实站点地址，否则预览里的头像和字标是裂图。
	preview, err := svc.PreviewTemplate(ctx, NotificationEmailPreviewInput{
		Event:  NotificationEmailEventSubscriptionExpiryReminder,
		Locale: notificationEmailLocaleChinese,
	})
	require.NoError(t, err)
	require.Contains(t, preview.HTML, `src="https://panel.test/email/avatar.png"`)
	require.Contains(t, preview.HTML, `href="https://panel.test/subscriptions"`)
	require.NotContains(t, preview.HTML, "{{")
}

func TestNotificationEmailSiteURLRejectsUnsafeValues(t *testing.T) {
	ctx := context.Background()
	repo := newNotificationEmailMemorySettingRepo()
	require.NoError(t, repo.Set(ctx, SettingKeyFrontendURL, "javascript:alert(1)"))
	svc := NewNotificationEmailService(repo, nil)

	preview, err := svc.PreviewTemplate(ctx, NotificationEmailPreviewInput{
		Event:  NotificationEmailEventAuthVerifyCode,
		Locale: notificationEmailDefaultLocale,
	})
	require.NoError(t, err)
	require.NotContains(t, preview.HTML, "javascript:")
	require.Contains(t, preview.HTML, `src="/email/avatar.png"`)
}

// 全部中文发信：调用方传的请求头语言和记住的浏览器语言都不再影响模板选择。
func TestNotificationEmailSendAlwaysUsesChinese(t *testing.T) {
	ctx := context.Background()
	repo := newNotificationEmailMemorySettingRepo()
	smtpServer := startNotificationEmailTestSMTPServer(t)
	require.NoError(t, repo.SetMultiple(ctx, smtpServer.settings()))
	svc := NewNotificationEmailService(repo, NewEmailService(repo, nil))
	svc.RememberRecipientLocale(ctx, 42, "user@panel.test", "en-US")

	require.Equal(t, notificationEmailLocaleChinese, svc.ResolveRecipientLocale(ctx, 42, "user@panel.test"))
	require.NoError(t, svc.Send(ctx, NotificationEmailSendInput{
		Event:          NotificationEmailEventAuthVerifyCode,
		Locale:         "en-US",
		RecipientEmail: "user@panel.test",
		RecipientName:  "user",
		UserID:         42,
		Variables:      map[string]string{"verification_code": "654321", "expires_in_minutes": "15"},
	}))

	body := smtpServer.lastMessageBody(t)
	require.Contains(t, body, "邮箱验证码")
	require.Contains(t, body, "654321")
	require.NotContains(t, body, "Email verification code")
}

// 后台自定义模板坏掉时退回官方模板，而不是让调用方走旧版的英文/双语兜底正文。
func TestNotificationEmailSendFallsBackToOfficialTemplateWhenCustomIsBroken(t *testing.T) {
	ctx := context.Background()
	repo := newNotificationEmailMemorySettingRepo()
	smtpServer := startNotificationEmailTestSMTPServer(t)
	require.NoError(t, repo.SetMultiple(ctx, smtpServer.settings()))
	require.NoError(t, repo.Set(ctx,
		notificationEmailTemplateKey(NotificationEmailEventAuthVerifyCode, notificationEmailLocaleChinese),
		`{"subject":"坏模板 {{not_a_placeholder}}","html":"<p>{{not_a_placeholder}}</p>"}`))
	svc := NewNotificationEmailService(repo, NewEmailService(repo, nil))

	require.NoError(t, svc.Send(ctx, NotificationEmailSendInput{
		Event:          NotificationEmailEventAuthVerifyCode,
		RecipientEmail: "user@panel.test",
		Variables:      map[string]string{"verification_code": "112233", "expires_in_minutes": "15"},
	}))

	body := smtpServer.lastMessageBody(t)
	require.Contains(t, body, "112233")
	require.Contains(t, body, "请使用下面的验证码完成验证")
	require.NotContains(t, body, "坏模板")
}

func TestSMTPTestEmailHTMLUsesSiteBranding(t *testing.T) {
	ctx := context.Background()
	repo := newNotificationEmailMemorySettingRepo()
	require.NoError(t, repo.Set(ctx, SettingKeyFrontendURL, "https://panel.test"))
	svc := NewNotificationEmailService(repo, nil)

	body := svc.SMTPTestEmailHTML(ctx, `天才<程序员>`)
	require.Contains(t, body, "天才&lt;程序员&gt;")
	require.Contains(t, body, `src="https://panel.test/email/wordmark.png"`)
	require.Contains(t, body, "SMTP 配置成功")
	require.NotContains(t, body, "{{")
}
