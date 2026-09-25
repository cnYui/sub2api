//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

// 兑换码邮件里，下面这些值只会来自预览示例；渲染结果命中任何一个就说明有占位符没传。
var redeemCodeDeliverySampleMarkers = []string{
	"3f9a1c7e5b2d4a6c8e0f1a2b3c4d5e6f", "example.com", "张三", "Alex", "Claude Pro", "每期 $76.00 · 共 4 期",
}

func redeemCodeDeliveryTestPackageCode() *RedeemCode {
	planID := int64(21)
	return &RedeemCode{
		ID:                   501,
		Code:                 "9c2e7a41d03b4f58a6e1c5d7b28f90ae",
		Type:                 RedeemTypeBalancePackage,
		Status:               StatusUnused,
		BalancePackagePlanID: &planID,
		BalancePackagePlan: &RedeemBalancePackagePlan{
			ID: 21, Code: "balance-29", Name: "余额套餐 ¥29", PriceCNY: 29,
			WeeklyCreditUSD: 76, ValidityDays: 28, RefreshCount: 4, RefreshIntervalDays: 7, ForSale: true,
		},
	}
}

func TestRedeemCodeDeliveryVariablesCoverDeclaredPlaceholders(t *testing.T) {
	info, ok := notificationEmailEventDefinitions[NotificationEmailEventRedeemCodeDelivery]
	require.True(t, ok)
	// 码写在邮件里，用户退订了就等于没发。
	require.False(t, info.Optional)

	expires := time.Date(2026, 10, 25, 15, 59, 0, 0, time.UTC)
	balance := &RedeemCode{ID: 7, Code: "0123456789abcdef0123456789abcdef", Type: RedeemTypeBalance, Value: 20, Status: StatusUnused, ExpiresAt: &expires}
	for _, code := range []*RedeemCode{redeemCodeDeliveryTestPackageCode(), balance} {
		for _, locale := range []string{notificationEmailLocaleChinese, notificationEmailDefaultLocale} {
			variables := buildRedeemCodeDeliveryVariables(code, locale)
			for _, placeholder := range info.Placeholders {
				if _, framework := purchaseNoticeFrameworkPlaceholders[placeholder]; framework {
					continue
				}
				require.NotEmptyf(t, variables[placeholder], "%s/%s does not supply %q", code.Type, locale, placeholder)
			}
		}
	}

	zh := buildRedeemCodeDeliveryVariables(redeemCodeDeliveryTestPackageCode(), notificationEmailLocaleChinese)
	require.Equal(t, "9c2e7a41d03b4f58a6e1c5d7b28f90ae", zh["full_redeem_code"])
	require.Equal(t, "余额套餐 ¥29", zh["reward_name"])
	require.Equal(t, "每期 $76.00 · 共 4 期", zh["reward_value"])
	require.Equal(t, "长期有效", zh["code_expires_at"])
	require.Contains(t, zh["reward_note"], "每 7 天到账一期，共 4 期")

	balanceZH := buildRedeemCodeDeliveryVariables(balance, notificationEmailLocaleChinese)
	require.Equal(t, "账户余额 $20.00", balanceZH["reward_name"])
	require.Equal(t, "2026-10-25 15:59 UTC", balanceZH["code_expires_at"])
}

func TestRedeemCodeDeliveryTemplateRendersFullCodeWithoutSampleLeakage(t *testing.T) {
	ctx := context.Background()
	svc := NewNotificationEmailService(newNotificationEmailMemorySettingRepo(), nil)
	code := redeemCodeDeliveryTestPackageCode()

	for _, locale := range []string{notificationEmailLocaleChinese, notificationEmailDefaultLocale} {
		variables := map[string]string{
			"site_url":        "https://panel.test",
			"recipient_name":  "someone",
			"recipient_email": "someone@panel.test",
		}
		for key, value := range buildRedeemCodeDeliveryVariables(code, locale) {
			variables[key] = value
		}
		preview, err := svc.PreviewTemplate(ctx, NotificationEmailPreviewInput{
			Event:     NotificationEmailEventRedeemCodeDelivery,
			Locale:    locale,
			Variables: variables,
		})
		require.NoError(t, err, locale)
		// 兑换按原文精确匹配，邮件里的码不能被拆分、插空格或改大小写。
		require.Contains(t, preview.HTML, ">9c2e7a41d03b4f58a6e1c5d7b28f90ae</div>", locale)
		require.Contains(t, preview.HTML, `href="https://panel.test/redeem"`, locale)
		require.Contains(t, preview.Subject, "¥29", locale)
		require.NotContains(t, preview.HTML, "{{unsubscribe_url}}", locale)
		body := preview.Subject + preview.HTML
		for _, marker := range redeemCodeDeliverySampleMarkers {
			if locale == notificationEmailLocaleChinese && marker == "每期 $76.00 · 共 4 期" {
				// 这张码的真实额度恰好与示例值相同，不能作为泄漏判据。
				continue
			}
			require.NotContainsf(t, body, marker, "%s leaked preview sample value %q", locale, marker)
		}
	}
}

type redeemDeliveryCodeRepo struct {
	RedeemCodeRepository
	codes map[int64]*RedeemCode
}

func (r *redeemDeliveryCodeRepo) GetByID(_ context.Context, id int64) (*RedeemCode, error) {
	code, ok := r.codes[id]
	if !ok {
		return nil, ErrRedeemCodeNotFound
	}
	copied := *code
	return &copied, nil
}

type redeemDeliveryUserRepo struct {
	UserRepository
	users map[int64]*User
}

func (r *redeemDeliveryUserRepo) GetByID(_ context.Context, id int64) (*User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	copied := *user
	return &copied, nil
}

func (r *redeemDeliveryUserRepo) GetByEmail(_ context.Context, email string) (*User, error) {
	for _, user := range r.users {
		if user.Email == email {
			copied := *user
			return &copied, nil
		}
	}
	return nil, ErrUserNotFound
}

func newRedeemDeliveryTestService(t *testing.T, codes ...*RedeemCode) (*RedeemService, *notificationEmailTestSMTPServer, *notificationEmailMemorySettingRepo) {
	t.Helper()
	ctx := context.Background()
	settings := newNotificationEmailMemorySettingRepo()
	smtpServer := startNotificationEmailTestSMTPServer(t)
	require.NoError(t, settings.SetMultiple(ctx, smtpServer.settings()))
	require.NoError(t, settings.Set(ctx, SettingKeyFrontendURL, "https://panel.test"))

	notificationEmail := NewNotificationEmailService(settings, NewEmailService(settings, nil))
	byID := make(map[int64]*RedeemCode, len(codes))
	for _, code := range codes {
		byID[code.ID] = code
	}
	svc := &RedeemService{
		redeemRepo: &redeemDeliveryCodeRepo{codes: byID},
		userRepo: &redeemDeliveryUserRepo{users: map[int64]*User{
			705: {ID: 705, Email: "first@panel.test", Status: StatusActive},
			706: {ID: 706, Email: "second@panel.test", Status: StatusActive},
			707: {ID: 707, Email: "disabled@panel.test", Status: StatusDisabled},
		}},
		purchaseNotify: NewPurchaseNotifyService(notificationEmail, settings, nil, nil, nil),
	}
	return svc, smtpServer, settings
}

func requireInfraReason(t *testing.T, err error, reason string) {
	t.Helper()
	require.Error(t, err)
	var appErr *infraerrors.ApplicationError
	require.Truef(t, errors.As(err, &appErr), "want %s, got %v", reason, err)
	require.Equal(t, reason, appErr.Reason)
}

func TestDeliverRedeemCodeByEmailSendsOnceAndGuardsRecipient(t *testing.T) {
	ctx := context.Background()
	code := redeemCodeDeliveryTestPackageCode()
	svc, smtpServer, settings := newRedeemDeliveryTestService(t, code)

	result, err := svc.DeliverByEmail(ctx, RedeemCodeDeliveryInput{CodeID: code.ID, UserID: 705})
	require.NoError(t, err)
	require.False(t, result.AlreadySent)
	require.Equal(t, "first@panel.test", result.Email)
	require.Equal(t, int64(1), smtpServer.messageCount())
	body := smtpServer.lastMessageBody(t)
	require.Contains(t, body, "9c2e7a41d03b4f58a6e1c5d7b28f90ae")
	require.Contains(t, body, "余额套餐 ¥29")
	require.Contains(t, body, `href="https://panel.test/redeem"`)

	record, err := settings.GetValue(ctx, redeemCodeDeliveryKey(code.ID))
	require.NoError(t, err)
	require.Contains(t, record, `"user_id":705`)

	// 同一个人再调一次不会重复发信；按邮箱指定同一个人也一样。
	again, err := svc.DeliverByEmail(ctx, RedeemCodeDeliveryInput{CodeID: code.ID, Email: "first@panel.test"})
	require.NoError(t, err)
	require.True(t, again.AlreadySent)
	require.Equal(t, int64(1), smtpServer.messageCount())

	// 明确要求重发才会再发一封。
	resent, err := svc.DeliverByEmail(ctx, RedeemCodeDeliveryInput{CodeID: code.ID, UserID: 705, Resend: true})
	require.NoError(t, err)
	require.False(t, resent.AlreadySent)
	require.Equal(t, int64(2), smtpServer.messageCount())

	// 已经发给 705 的码不能再发给别人，重发标记也不行。
	_, err = svc.DeliverByEmail(ctx, RedeemCodeDeliveryInput{CodeID: code.ID, UserID: 706, Resend: true})
	requireInfraReason(t, err, "REDEEM_CODE_ALREADY_DELIVERED")
	require.Equal(t, int64(2), smtpServer.messageCount())
}

func TestDeliverRedeemCodeByEmailRejectsUnusableCodesAndTargets(t *testing.T) {
	ctx := context.Background()
	usedBy := int64(9)
	past := time.Now().Add(-time.Hour)
	used := &RedeemCode{ID: 1, Code: "used", Type: RedeemTypeBalance, Value: 10, Status: StatusUsed, UsedBy: &usedBy}
	expired := &RedeemCode{ID: 2, Code: "expired", Type: RedeemTypeBalance, Value: 10, Status: StatusUnused, ExpiresAt: &past}
	backCharge := &RedeemCode{ID: 3, Code: "back-charge", Type: RedeemTypeBalance, Value: -5, Status: StatusUnused}
	subscription := &RedeemCode{ID: 4, Code: "subscription", Type: RedeemTypeSubscription, Value: 1, Status: StatusUnused}
	balance := &RedeemCode{ID: 5, Code: "0123456789abcdef0123456789abcdef", Type: RedeemTypeBalance, Value: 20, Status: StatusUnused}
	svc, smtpServer, _ := newRedeemDeliveryTestService(t, used, expired, backCharge, subscription, balance)

	_, err := svc.DeliverByEmail(ctx, RedeemCodeDeliveryInput{CodeID: used.ID, UserID: 705})
	require.ErrorIs(t, err, ErrRedeemCodeUsed)
	_, err = svc.DeliverByEmail(ctx, RedeemCodeDeliveryInput{CodeID: expired.ID, UserID: 705})
	require.ErrorIs(t, err, ErrRedeemCodeExpired)
	_, err = svc.DeliverByEmail(ctx, RedeemCodeDeliveryInput{CodeID: backCharge.ID, UserID: 705})
	requireInfraReason(t, err, "REDEEM_CODE_DELIVERY_UNSUPPORTED")
	_, err = svc.DeliverByEmail(ctx, RedeemCodeDeliveryInput{CodeID: subscription.ID, UserID: 705})
	requireInfraReason(t, err, "REDEEM_CODE_DELIVERY_UNSUPPORTED")

	_, err = svc.DeliverByEmail(ctx, RedeemCodeDeliveryInput{CodeID: balance.ID})
	requireInfraReason(t, err, "REDEEM_CODE_DELIVERY_TARGET_REQUIRED")
	_, err = svc.DeliverByEmail(ctx, RedeemCodeDeliveryInput{CodeID: balance.ID, UserID: 705, Email: "first@panel.test"})
	requireInfraReason(t, err, "REDEEM_CODE_DELIVERY_TARGET_REQUIRED")
	_, err = svc.DeliverByEmail(ctx, RedeemCodeDeliveryInput{CodeID: balance.ID, UserID: 707})
	requireInfraReason(t, err, "USER_INACTIVE")
	_, err = svc.DeliverByEmail(ctx, RedeemCodeDeliveryInput{CodeID: balance.ID, Email: "stranger@panel.test"})
	require.ErrorIs(t, err, ErrUserNotFound)
	require.Equal(t, int64(0), smtpServer.messageCount())

	// 普通余额码可以发，邮件里写的是美元额度。
	_, err = svc.DeliverByEmail(ctx, RedeemCodeDeliveryInput{CodeID: balance.ID, Email: "second@panel.test"})
	require.NoError(t, err)
	require.Equal(t, int64(1), smtpServer.messageCount())
	require.Contains(t, smtpServer.lastMessageBody(t), "账户余额 $20.00")
}
