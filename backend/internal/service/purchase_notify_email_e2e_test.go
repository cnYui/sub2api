//go:build unit

package service

import (
	"context"
	"mime"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

// 到账邮件端到端：真实库表 → 到账通知服务 → SMTP 投递。
// 断言的是用户真正收到的邮件：显示的套餐/流量卡/兑换信息是否与订单一致，按钮是否跳到对应页面。

const purchaseNotifyE2ESiteURL = "https://panel.test"

type purchaseNotifyE2E struct {
	ctx    context.Context
	client *dbent.Client
	smtp   *notificationEmailTestSMTPServer
	svc    *PurchaseNotifyService
	user   *dbent.User
}

type purchaseNotifyE2EUserRepo struct {
	UserRepository
	user *dbent.User
}

func (r purchaseNotifyE2EUserRepo) GetByID(_ context.Context, id int64) (*User, error) {
	if r.user == nil || r.user.ID != id {
		return nil, ErrUserNotFound
	}
	return &User{ID: r.user.ID, Email: r.user.Email, Balance: r.user.Balance}, nil
}

func newPurchaseNotifyE2E(t *testing.T, locale string) *purchaseNotifyE2E {
	t.Helper()
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	repo := newNotificationEmailMemorySettingRepo()
	smtp := startNotificationEmailTestSMTPServer(t)
	require.NoError(t, repo.SetMultiple(ctx, smtp.settings()))
	// 故意带尾斜杠，确认拼出来的链接不会出现双斜杠。
	require.NoError(t, repo.SetMultiple(ctx, map[string]string{
		SettingKeyFrontendURL: purchaseNotifyE2ESiteURL + "/",
		SettingKeySiteName:    "天才程序员小站",
	}))
	notification := NewNotificationEmailService(repo, NewEmailService(repo, nil))

	user, err := client.User.Create().SetEmail("buyer@panel.test").SetPasswordHash("hash").SetBalance(133.25).Save(ctx)
	require.NoError(t, err)
	notification.RememberRecipientLocale(ctx, user.ID, user.Email, locale)

	return &purchaseNotifyE2E{
		ctx:    ctx,
		client: client,
		smtp:   smtp,
		user:   user,
		svc: &PurchaseNotifyService{
			notificationEmail: notification,
			settingRepo:       repo,
			entClient:         client,
			userRepo:          purchaseNotifyE2EUserRepo{user: user},
		},
	}
}

func (e *purchaseNotifyE2E) createOrder(t *testing.T, code, paymentType, orderType string, payAmount float64, snapshot map[string]interface{}) *dbent.PaymentOrder {
	t.Helper()
	paidAt := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	create := e.client.PaymentOrder.Create().
		SetUserID(e.user.ID).
		SetUserEmail(e.user.Email).
		SetUserName("buyer").
		SetAmount(29).
		SetPayAmount(payAmount).
		SetRechargeCode(code).
		SetPaymentType(paymentType).
		SetPaymentTradeNo("trade-" + code).
		SetOrderType(orderType).
		SetStatus(OrderStatusCompleted).
		SetClientIP("127.0.0.1").
		SetSrcHost("panel.test").
		SetPaidAt(paidAt).
		SetExpiresAt(paidAt.Add(time.Hour))
	if snapshot != nil {
		create = create.SetProviderSnapshot(snapshot)
	}
	order, err := create.Save(e.ctx)
	require.NoError(t, err)
	return order
}

func (e *purchaseNotifyE2E) createBalancePackage(t *testing.T, order *dbent.PaymentOrder, renewalCount int) {
	t.Helper()
	plan, err := e.client.BalancePackagePlan.Create().
		SetCode("e2e-" + order.RechargeCode).SetName("Pro 余额套餐").SetPriceCny(29).
		SetWeeklyCreditUsd(76).SetValidityDays(28).SetRefreshCount(4).
		SetRefreshIntervalDays(7).SetForSale(true).Save(e.ctx)
	require.NoError(t, err)
	next := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	_, err = e.client.UserBalancePackage.Create().
		SetUserID(e.user.ID).SetPlanID(plan.ID).SetPaymentOrderID(order.ID).
		SetWeeklyCreditUsd(76).SetRemainingUsd(70.5).SetCreditedCount(1).SetRefreshCount(4).
		SetRefreshIntervalDays(7).SetRenewalCount(renewalCount).
		SetStartsAt(time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)).SetNextCreditAt(next).
		SetExpiresAt(time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)).
		SetStatus(balancePackageStatusActive).Save(e.ctx)
	require.NoError(t, err)
}

// lastEmail 返回最后一封邮件解码后的主题和正文。
func (e *purchaseNotifyE2E) lastEmail(t *testing.T) (string, string) {
	t.Helper()
	require.Equal(t, int64(1), e.smtp.messageCount(), "exactly one email should be delivered")
	message, err := mail.ReadMessage(strings.NewReader(e.smtp.lastMessage()))
	require.NoError(t, err)
	subject, err := new(mime.WordDecoder).DecodeHeader(message.Header.Get("Subject"))
	require.NoError(t, err)
	return subject, e.smtp.lastMessageBody(t)
}

func requirePurchaseEmailChrome(t *testing.T, body string) {
	t.Helper()
	require.Contains(t, body, `src="`+purchaseNotifyE2ESiteURL+`/email/avatar.png"`)
	require.Contains(t, body, `src="`+purchaseNotifyE2ESiteURL+`/email/wordmark.png"`)
	require.Contains(t, body, `src="`+purchaseNotifyE2ESiteURL+`/email/qr-wechat.png"`)
	require.Contains(t, body, `href="`+purchaseNotifyE2ESiteURL+`/api/v1/settings/email-unsubscribe?token=`)
	require.NotContains(t, body, "{{", "every placeholder must be rendered")
	require.NotContains(t, body, "panel.test//", "base URL trailing slash must be trimmed")
	require.NotContains(t, body, "example.com", "preview sample values must never reach a real email")
}

func TestPurchaseNoticeE2EBalancePackageShowsOrderAndLinksToSubscriptions(t *testing.T) {
	cases := []struct {
		name        string
		paymentType string
		payAmount   float64
		renewal     int
		wantKind    string
		wantPaid    string
	}{
		{name: "new purchase", paymentType: string(payment.TypeAlipay), payAmount: 29.29, wantKind: "新购", wantPaid: "29.29 CNY"},
		{name: "renewal", paymentType: string(payment.TypeAlipay), payAmount: 29.29, renewal: 1, wantKind: "续费", wantPaid: "29.29 CNY"},
		{name: "admin grant", paymentType: payment.PaymentTypeAdminGrant, payAmount: 0, wantKind: "管理员发放", wantPaid: "赠送"},
		{name: "redeem code", paymentType: payment.PaymentTypeRedeemCode, payAmount: 0, wantKind: "兑换码兑换", wantPaid: "—"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newPurchaseNotifyE2E(t, "zh-CN")
			order := e.createOrder(t, "sub2_E2E_BP", tc.paymentType, payment.OrderTypeBalanceSubscription, tc.payAmount, nil)
			e.createBalancePackage(t, order, tc.renewal)

			require.NoError(t, e.svc.sendBalancePackageNotice(e.ctx, order))

			subject, body := e.lastEmail(t)
			require.Equal(t, "[天才程序员小站] Pro 余额套餐 已生效（"+tc.wantKind+"）", subject)
			for _, want := range []string{
				"余额套餐已生效",
				"Pro 余额套餐",
				">" + tc.wantKind + "<",
				">" + tc.wantPaid + "<",
				"$70.50", // 本期可用
				"$76.00", // 每期额度
				"共 4 期<br>每 7 天一期",
				"1 / 4 期",
				"2026-06-18 12:00 UTC",
				"2026-07-09 12:00 UTC",
				"No.sub2_E2E_BP",
				`href="` + purchaseNotifyE2ESiteURL + `/subscriptions"`,
				"查看我的套餐",
			} {
				require.Containsf(t, body, want, "balance package email should contain %q", want)
			}
			requirePurchaseEmailChrome(t, body)
		})
	}
}

// 站长要求全部中文发信：浏览器是英文的用户，模板和变量里的文字也都得是中文。
func TestPurchaseNoticeE2EEnglishBrowserStillGetsChinese(t *testing.T) {
	e := newPurchaseNotifyE2E(t, "en-US")
	order := e.createOrder(t, "sub2_E2E_EN", string(payment.TypeAlipay), payment.OrderTypeBalanceSubscription, 29.29, nil)
	e.createBalancePackage(t, order, 0)

	require.NoError(t, e.svc.sendBalancePackageNotice(e.ctx, order))

	subject, body := e.lastEmail(t)
	require.Equal(t, "[天才程序员小站] Pro 余额套餐 已生效（新购）", subject)
	require.Contains(t, body, "余额套餐已生效")
	require.Contains(t, body, "查看我的套餐")
	require.Contains(t, body, "此邮件由 天才程序员小站 自动发送")
	for _, english := range []string{"New purchase", "View my package", "Balance package active", "This email was sent"} {
		require.NotContains(t, body, english)
	}
	require.Contains(t, body, `href="`+purchaseNotifyE2ESiteURL+`/subscriptions"`)
	requirePurchaseEmailChrome(t, body)
}

func TestPurchaseNoticeE2ETrafficPackShowsPackAndLinksToOrders(t *testing.T) {
	e := newPurchaseNotifyE2E(t, "zh-CN")
	order := e.createOrder(t, "sub2_E2E_TP", string(payment.TypeAlipay), payment.OrderTypeTrafficPack, 10.1, map[string]interface{}{
		"traffic_pack_id":            2,
		"traffic_pack_name":          "流量卡 $10",
		"traffic_pack_credit_usd":    10,
		"traffic_pack_validity_days": 30,
		"traffic_pack_platform":      TrafficPackPlatformAll,
	})

	require.NoError(t, e.svc.sendTrafficPackNotice(e.ctx, order))

	subject, body := e.lastEmail(t)
	require.Equal(t, "[天才程序员小站] 流量卡 $10 已到账", subject)
	for _, want := range []string{
		"流量卡已到账",
		">流量卡 $10<",
		"$10.00",
		"10.10 CNY",
		"30 天",
		// 有效期从支付时间起算，与实际入账口径一致。
		"2026-07-11 12:00 UTC",
		"No.sub2_E2E_TP",
		`href="` + purchaseNotifyE2ESiteURL + `/orders"`,
		"查看我的订单",
	} {
		require.Containsf(t, body, want, "traffic pack email should contain %q", want)
	}
	requirePurchaseEmailChrome(t, body)
}

func TestPurchaseNoticeE2ERedeemBalanceShowsCreditAndLinksToRedeem(t *testing.T) {
	e := newPurchaseNotifyE2E(t, "zh-CN")

	require.NoError(t, e.svc.sendRedeemBalanceNotice(e.ctx, e.user.ID, 5, "ABCD-EFGH-9F2C", 25))

	subject, body := e.lastEmail(t)
	require.Equal(t, "[天才程序员小站] 兑换成功，余额到账 $25.00", subject)
	for _, want := range []string{
		"兑换成功",
		"$25.00",
		"$133.25",
		"**********9F2C",
		`href="` + purchaseNotifyE2ESiteURL + `/redeem"`,
		"查看我的余额",
	} {
		require.Containsf(t, body, want, "redeem email should contain %q", want)
	}
	require.NotContains(t, body, "ABCD-EFGH", "the full redeem code must not stay in the inbox")
	requirePurchaseEmailChrome(t, body)
}

// 按钮路径写错就是死链：逐个确认它们在前端路由表里真的存在。
func TestPurchaseNoticeDashboardPathsExistInFrontendRouter(t *testing.T) {
	router, err := os.ReadFile(filepath.Join("..", "..", "..", "frontend", "src", "router", "index.ts"))
	if err != nil {
		t.Skipf("frontend router not available: %v", err)
	}
	for _, path := range []string{balancePackageDashboardPath, trafficPackDashboardPath, redeemDashboardPath, "/subscriptions"} {
		require.Containsf(t, string(router), "path: '"+path+"'", "frontend router has no route %s", path)
	}
}
