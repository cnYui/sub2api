//go:build unit

package service

import (
	"context"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

// 通知框架提供的占位符，业务侧不需要传。
var purchaseNoticeFrameworkPlaceholders = map[string]struct{}{
	"site_name":       {},
	"recipient_name":  {},
	"recipient_email": {},
	"unsubscribe_url": {},
}

func purchaseNoticeTestOrder() *dbent.PaymentOrder {
	paidAt := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	return &dbent.PaymentOrder{
		ID:           1024,
		UserID:       77,
		UserEmail:    "buyer@example.org",
		RechargeCode: "sub2_20260611aB3kX9mQ",
		Amount:       29,
		PayAmount:    29.29,
		PaymentType:  payment.TypeAlipay,
		OrderType:    payment.OrderTypeBalanceSubscription,
		PaidAt:       &paidAt,
	}
}

func purchaseNoticeTestPackage() *dbent.UserBalancePackage {
	next := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	return &dbent.UserBalancePackage{
		ID:                  9,
		PlanID:              3,
		PaymentOrderID:      1024,
		WeeklyCreditUsd:     76,
		RemainingUsd:        70.5,
		RefreshCount:        4,
		RefreshIntervalDays: 7,
		NextCreditAt:        &next,
		ExpiresAt:           time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
	}
}

// 事件声明了却没传的占位符会被 runtimeVariables 填成预览示例值（"张三"、
// "https://example.com/..."）并发给真实用户，所以每个事件都必须传满。
func TestPurchaseNoticeVariablesCoverDeclaredPlaceholders(t *testing.T) {
	expiresAt := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		event     string
		variables map[string]string
	}{
		{
			event: NotificationEmailEventBalancePackageCredited,
			variables: buildBalancePackageNoticeVariables(
				purchaseNoticeTestOrder(), purchaseNoticeTestPackage(),
				purchaseKindNew, notificationEmailLocaleChinese,
				"余额套餐 ¥29", "https://panel.test/subscriptions"),
		},
		{
			event: NotificationEmailEventTrafficPackCredited,
			variables: buildTrafficPackNoticeVariables(
				purchaseNoticeTestOrder(),
				&TrafficPackOrderInfo{ID: 2, Name: "流量卡 $10", CreditUSD: 10, ValidityDays: 30, Platform: TrafficPackPlatformAll},
				expiresAt, notificationEmailLocaleChinese, "https://panel.test/subscriptions"),
		},
		{
			event:     NotificationEmailEventRedeemBalanceCredited,
			variables: buildRedeemBalanceNoticeVariables("ABCD-EFGH-9F2C", 25, 133.25, "https://panel.test/redeem"),
		},
	}

	for _, tc := range cases {
		info, ok := notificationEmailEventDefinitions[tc.event]
		require.Truef(t, ok, "event %s is not registered", tc.event)
		require.Truef(t, info.Optional, "event %s must be optional so users can unsubscribe", tc.event)
		for _, placeholder := range info.Placeholders {
			if _, framework := purchaseNoticeFrameworkPlaceholders[placeholder]; framework {
				continue
			}
			require.Containsf(t, tc.variables, placeholder,
				"event %s declares %q but the service never supplies it; it would render a preview sample value", tc.event, placeholder)
		}
	}
}

// 官方模板 zh/en 都必须存在，并且渲染出来不能残留任何预览示例值。
func TestPurchaseNoticeTemplatesRenderWithoutSampleLeakage(t *testing.T) {
	ctx := context.Background()
	svc := NewNotificationEmailService(newNotificationEmailMemorySettingRepo(), nil)
	expiresAt := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	// 这些片段只会出现在预览示例值里；渲染结果命中任何一个就说明有占位符没传。
	sampleMarkers := []string{"example.com", "张三", "Alex", "Claude Pro", "openai-main", "414.10"}

	cases := []struct {
		event     string
		locale    string
		variables map[string]string
		expect    []string
	}{
		{
			event:  NotificationEmailEventBalancePackageCredited,
			locale: notificationEmailLocaleChinese,
			variables: buildBalancePackageNoticeVariables(
				purchaseNoticeTestOrder(), purchaseNoticeTestPackage(),
				purchaseKindRenewal, notificationEmailLocaleChinese,
				"余额套餐 ¥29", "https://panel.test/subscriptions"),
			expect: []string{"续费", "余额套餐 ¥29", "29.29 CNY", "76.00", "70.50", "sub2_20260611aB3kX9mQ", "2026-07-09 12:00 UTC"},
		},
		{
			event:  NotificationEmailEventBalancePackageCredited,
			locale: notificationEmailDefaultLocale,
			variables: buildBalancePackageNoticeVariables(
				purchaseNoticeTestOrder(), purchaseNoticeTestPackage(),
				purchaseKindNew, notificationEmailDefaultLocale,
				"Balance package 29", "https://panel.test/subscriptions"),
			expect: []string{"New purchase", "Balance package 29", "29.29 CNY"},
		},
		{
			event:  NotificationEmailEventTrafficPackCredited,
			locale: notificationEmailLocaleChinese,
			variables: buildTrafficPackNoticeVariables(
				purchaseNoticeTestOrder(),
				&TrafficPackOrderInfo{ID: 2, Name: "流量卡 $10", CreditUSD: 10, ValidityDays: 30, Platform: TrafficPackPlatformAll},
				expiresAt, notificationEmailLocaleChinese, "https://panel.test/subscriptions"),
			expect: []string{"流量卡 $10", "10.00", "30", "2026-07-11 12:00 UTC"},
		},
		{
			event:  NotificationEmailEventTrafficPackCredited,
			locale: notificationEmailDefaultLocale,
			variables: buildTrafficPackNoticeVariables(
				purchaseNoticeTestOrder(),
				&TrafficPackOrderInfo{ID: 2, Name: "Traffic pack 10", CreditUSD: 10, ValidityDays: 30, Platform: TrafficPackPlatformAll},
				expiresAt, notificationEmailDefaultLocale, "https://panel.test/subscriptions"),
			expect: []string{"Traffic pack 10", "10.00"},
		},
		{
			event:     NotificationEmailEventRedeemBalanceCredited,
			locale:    notificationEmailLocaleChinese,
			variables: buildRedeemBalanceNoticeVariables("ABCD-EFGH-9F2C", 25, 133.25, "https://panel.test/redeem"),
			expect:    []string{"**********9F2C", "25.00", "133.25"},
		},
		{
			event:     NotificationEmailEventRedeemBalanceCredited,
			locale:    notificationEmailDefaultLocale,
			variables: buildRedeemBalanceNoticeVariables("ABCD-EFGH-9F2C", 25, 133.25, "https://panel.test/redeem"),
			expect:    []string{"**********9F2C", "25.00"},
		},
	}

	for _, tc := range cases {
		// 真实发送时 runtimeVariables 会补齐这几个框架占位符，预览要模拟一遍，
		// 否则它们的示例值会把"业务占位符漏传"这个真正的断言淹掉。
		variables := map[string]string{
			"recipient_name":  "Buyer",
			"recipient_email": "buyer@panel.test",
			"unsubscribe_url": "https://panel.test/unsubscribe?token=x",
		}
		for key, value := range tc.variables {
			variables[key] = value
		}
		preview, err := svc.PreviewTemplate(ctx, NotificationEmailPreviewInput{
			Event:     tc.event,
			Locale:    tc.locale,
			Variables: variables,
		})
		require.NoErrorf(t, err, "event %s locale %s", tc.event, tc.locale)
		require.NotEmpty(t, preview.Subject)
		for _, want := range tc.expect {
			require.Containsf(t, preview.HTML, want, "event %s locale %s missing %q", tc.event, tc.locale, want)
		}
		body := preview.Subject + preview.HTML
		for _, marker := range sampleMarkers {
			require.NotContainsf(t, body, marker,
				"event %s locale %s leaked preview sample value %q", tc.event, tc.locale, marker)
		}
	}
}

// 管理员发放是零金额订单，不能在邮件里写成 "0.00 CNY"。
func TestPurchasePayAmountTextForAdminGrant(t *testing.T) {
	order := purchaseNoticeTestOrder()
	order.PaymentType = payment.PaymentTypeAdminGrant
	order.PayAmount = 0

	require.Equal(t, "赠送", purchasePayAmountText(order, purchaseKindAdminGrant, notificationEmailLocaleChinese))
	require.Equal(t, "Complimentary", purchasePayAmountText(order, purchaseKindAdminGrant, notificationEmailDefaultLocale))

	paid := purchaseNoticeTestOrder()
	require.Equal(t, "29.29 CNY", purchasePayAmountText(paid, purchaseKindNew, notificationEmailLocaleChinese))
}

// 后台手工补扣用的是负数 admin_balance 兑换码，绝不能给用户发"到账"通知。
func TestSendRedeemBalanceNoticeSkipsNonPositiveAmounts(t *testing.T) {
	svc := &PurchaseNotifyService{
		notificationEmail: NewNotificationEmailService(newNotificationEmailMemorySettingRepo(), nil),
		settingRepo:       newNotificationEmailMemorySettingRepo(),
		entClient:         &dbent.Client{},
		userRepo:          purchaseNotifyPanicUserRepo{},
	}

	for _, amount := range []float64{-33.5, 0} {
		require.NoError(t, svc.sendRedeemBalanceNotice(context.Background(), 77, 5, "ABCD-EFGH-9F2C", amount))
	}
}

// 全局开关关掉后不查库、不发信。
func TestPurchaseNotifyRespectsGlobalToggle(t *testing.T) {
	ctx := context.Background()
	settingRepo := newNotificationEmailMemorySettingRepo()
	svc := &PurchaseNotifyService{
		notificationEmail: NewNotificationEmailService(settingRepo, nil),
		settingRepo:       settingRepo,
		entClient:         &dbent.Client{},
		userRepo:          purchaseNotifyPanicUserRepo{},
	}

	// 缺省（从未写过这个 key）视为开启。
	require.True(t, svc.ready(ctx))

	require.NoError(t, settingRepo.Set(ctx, SettingKeyPurchaseNotifyEnabled, "false"))
	require.False(t, svc.ready(ctx))
	// 关掉后即使金额为正也不会碰 userRepo（碰了会 panic）。
	require.NoError(t, svc.sendRedeemBalanceNotice(ctx, 77, 5, "ABCD-EFGH-9F2C", 25))

	require.NoError(t, settingRepo.Set(ctx, SettingKeyPurchaseNotifyEnabled, "true"))
	require.True(t, svc.ready(ctx))
}

func TestMaskRedeemCode(t *testing.T) {
	require.Equal(t, "**********9F2C", maskRedeemCode("ABCD-EFGH-9F2C"))
	require.Equal(t, "****", maskRedeemCode("ABCD"))
	require.Equal(t, "***", maskRedeemCode("ABC"))
	require.Equal(t, "—", maskRedeemCode("   "))
	require.NotContains(t, maskRedeemCode("SECRETCODE1234"), "SECRET")
}

func TestPurchaseOrderNoFallsBackToID(t *testing.T) {
	order := purchaseNoticeTestOrder()
	require.Equal(t, "sub2_20260611aB3kX9mQ", purchaseOrderNo(order))
	order.RechargeCode = "  "
	require.Equal(t, "1024", purchaseOrderNo(order))
}

func TestFormatPurchaseTimeHandlesMissingValue(t *testing.T) {
	require.Equal(t, "—", formatPurchaseTime(nil))
	require.Equal(t, "—", formatPurchaseTime(&time.Time{}))
	at := time.Date(2026, 6, 18, 12, 30, 0, 0, time.UTC)
	require.Equal(t, "2026-06-18 12:30 UTC", formatPurchaseTime(&at))
}

func TestPurchaseNoticeDashboardPathsMatchFrontendRoutes(t *testing.T) {
	// frontend/src/router/index.ts 里是 /subscriptions 和 /redeem，写错就是死链。
	require.Equal(t, "/subscriptions", balancePackageDashboardPath)
	require.Equal(t, "/subscriptions", trafficPackDashboardPath)
	require.Equal(t, "/redeem", redeemDashboardPath)
	require.False(t, strings.HasSuffix(balancePackageDashboardPath, "/"))
}

// purchaseNotifyPanicUserRepo 在被调用时 panic，用来证明某条分支根本没走到用户查询。
type purchaseNotifyPanicUserRepo struct {
	UserRepository
}

func (purchaseNotifyPanicUserRepo) GetByID(context.Context, int64) (*User, error) {
	panic("unexpected user lookup")
}

// 后台「邮件模板」页调 ListTemplates，它会遍历所有事件 × 所有语言；
// 任何一个事件缺官方模板都会让整页报错，所以这里全量兜一遍。
func TestNotificationEmailListTemplatesCoversEveryRegisteredEvent(t *testing.T) {
	ctx := context.Background()
	svc := NewNotificationEmailService(newNotificationEmailMemorySettingRepo(), nil)

	templates, err := svc.ListTemplates(ctx)
	require.NoError(t, err)

	seen := make(map[string]map[string]bool)
	for _, tmpl := range templates {
		if seen[tmpl.Event] == nil {
			seen[tmpl.Event] = make(map[string]bool)
		}
		seen[tmpl.Event][tmpl.Locale] = true
		require.NotEmptyf(t, tmpl.Subject, "event %s locale %s has an empty subject", tmpl.Event, tmpl.Locale)
		require.NotEmptyf(t, tmpl.HTML, "event %s locale %s has an empty body", tmpl.Event, tmpl.Locale)
	}

	for _, event := range notificationEmailEventOrder {
		for _, locale := range svc.SupportedLocales() {
			require.Truef(t, seen[event][locale], "event %s is missing the %s official template", event, locale)
		}
	}
}
