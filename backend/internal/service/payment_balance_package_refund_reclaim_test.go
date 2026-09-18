//go:build unit

package service

import (
	"context"
	"strconv"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

// 以下用例覆盖两类多退：本周未用额度在退款后仍留在余额里；退款失败后按当时存下的金额重试。

func TestMarkRefundOKReclaimsRemainingCreditOnlyOnce(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	fx := newBalancePackageRefundFixture(t, ctx, client, "reclaim-once", balancePackageRefundFixtureOptions{
		orderStatus:   OrderStatusRefunding,
		balance:       150,
		remaining:     100,
		packageStatus: balancePackageStatusActive,
	})
	svc := &PaymentService{entClient: client}
	plan := &RefundPlan{OrderID: fx.order.ID, Order: fx.order, RefundAmount: 30, GatewayAmount: 30, Reason: "refund"}

	_, err := svc.markRefundOk(ctx, plan)
	require.NoError(t, err)
	// 重复确认（例如查询补偿再次确认成功）不能再扣一次。
	_, err = svc.markRefundOk(ctx, plan)
	require.NoError(t, err)

	require.InDelta(t, 50, balancePackageRefundUserBalance(t, ctx, client, fx.user.ID), 1e-9)
}

func TestMarkRefundOKAllowsReclaimIntoNegativeBalance(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	// 余额低于未用额度，说明普通余额已是负数；收回后如实为负，由下一次到账抵扣。
	fx := newBalancePackageRefundFixture(t, ctx, client, "reclaim-negative", balancePackageRefundFixtureOptions{
		orderStatus:   OrderStatusRefunding,
		balance:       20,
		remaining:     50,
		packageStatus: balancePackageStatusActive,
	})
	svc := &PaymentService{entClient: client}

	_, err := svc.markRefundOk(ctx, &RefundPlan{OrderID: fx.order.ID, Order: fx.order, RefundAmount: 10, GatewayAmount: 10})
	require.NoError(t, err)

	require.InDelta(t, -30, balancePackageRefundUserBalance(t, ctx, client, fx.user.ID), 1e-9)
}

func TestMarkRefundOKRevokesPackageOfDeletedUserWithoutReclaim(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	fx := newBalancePackageRefundFixture(t, ctx, client, "reclaim-deleted-user", balancePackageRefundFixtureOptions{
		orderStatus:   OrderStatusRefunding,
		balance:       150,
		remaining:     100,
		packageStatus: balancePackageStatusActive,
	})
	require.NoError(t, client.User.DeleteOneID(fx.user.ID).Exec(ctx))
	svc := &PaymentService{entClient: client}

	// 网关已经退款成功，用户被软删除也不能让本地确认失败。
	result, err := svc.markRefundOk(ctx, &RefundPlan{OrderID: fx.order.ID, Order: fx.order, RefundAmount: 30, GatewayAmount: 30})
	require.NoError(t, err)
	require.True(t, result.Success)

	pkg, err := client.UserBalancePackage.Get(ctx, fx.pkg.ID)
	require.NoError(t, err)
	require.Equal(t, balancePackageStatusRefunded, pkg.Status)
	order, err := client.PaymentOrder.Get(ctx, fx.order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefunded, order.Status)
	require.InDelta(t, 150, balancePackageRefundUserBalance(t, ctx, client, fx.user.ID), 1e-9)
}

func TestPrepareRefundRequotesFailedRefund(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	createBalancePackageUsageLedgerTableForTest(t, ctx, client)
	now := time.Now().UTC()
	fx := newBalancePackageRefundFixture(t, ctx, client, "requote-failed", balancePackageRefundFixtureOptions{
		orderStatus:   OrderStatusRefundFailed,
		storedRefund:  36.75,
		balance:       100,
		remaining:     100,
		packageStatus: balancePackageStatusActive,
		startsAt:      now.Add(-14 * 24 * time.Hour),
		creditedCount: 3,
	})
	// 上一周期的用量不属于这张订单。
	insertBalancePackageUsageForTest(t, ctx, client, fx.pkg, "previous-cycle", 500, "request", fx.pkg.StartsAt.Add(-24*time.Hour))
	insertBalancePackageUsageForTest(t, ctx, client, fx.pkg, "week-1", 128, "request", now.Add(-13*24*time.Hour))
	insertBalancePackageUsageForTest(t, ctx, client, fx.pkg, "week-2", 128, "request", now.Add(-6*24*time.Hour))
	insertBalancePackageUsageForTest(t, ctx, client, fx.pkg, "week-3", 28, "request", now.Add(-time.Hour))
	svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}}

	plan, err := svc.PrepareRefund(ctx, fx.order.ID, "")
	require.NoError(t, err)
	// 失败后套餐仍在用：用量比例 284/512 已超过时间比例 0.5，不能再按失败时存下的 36.75 退。
	require.InDelta(t, 49*(1-284.0/512), plan.RefundAmount, 1e-9)

	restore := replacePaymentProviderFactoryForTest(t, refundProviderTestDouble{})
	defer restore()
	_, err = svc.ExecuteRefund(ctx, plan)
	require.Error(t, err)

	reloaded, err := client.PaymentOrder.Get(ctx, fx.order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusRefundFailed, reloaded.Status)
	require.InDelta(t, plan.RefundAmount, reloaded.RefundAmount, 0.01)
}

func TestPrepareRefundFailedLegacyPackageRequiresManualReview(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	createBalancePackageUsageLedgerTableForTest(t, ctx, client)
	fx := newBalancePackageRefundFixture(t, ctx, client, "requote-legacy", balancePackageRefundFixtureOptions{
		orderStatus:   OrderStatusRefundFailed,
		storedRefund:  44.10,
		packageStatus: balancePackageStatusExpired,
		startsAt:      time.Now().UTC().Add(-40 * 24 * time.Hour),
		creditedCount: 4,
	})
	insertBalancePackageUsageForTest(t, ctx, client, fx.pkg, "legacy_unattributed:"+strconv.FormatInt(fx.pkg.ID, 10), 0, "legacy_unattributed", time.Now().UTC())
	svc := &PaymentService{entClient: client, loadBalancer: &captureLoadBalancer{}}

	plan, err := svc.PrepareRefund(ctx, fx.order.ID, "")
	require.Nil(t, plan)
	require.Error(t, err)
	require.Equal(t, "REFUND_MANUAL_REVIEW_REQUIRED", infraerrors.Reason(err))
}

func TestBalancePackageRefundQuoteReportsReclaimForActivePackage(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	createBalancePackageUsageLedgerTableForTest(t, ctx, client)
	now := time.Now().UTC()
	fx := newBalancePackageRefundFixture(t, ctx, client, "quote-reclaim", balancePackageRefundFixtureOptions{
		orderStatus:   OrderStatusCompleted,
		balance:       46.19,
		remaining:     46.19,
		packageStatus: balancePackageStatusActive,
		startsAt:      now.Add(-2 * 24 * time.Hour),
	})
	insertBalancePackageUsageForTest(t, ctx, client, fx.pkg, "used", 29.66, "request", now.Add(-24*time.Hour))
	svc := &PaymentService{entClient: client}

	quote, err := svc.GetBalancePackageRefundQuoteForAdmin(ctx, fx.order.ID)
	require.NoError(t, err)
	require.False(t, quote.ManualReviewRequired)
	require.InDelta(t, 29.66, quote.UsedQuotaUSD, 1e-9)
	require.Zero(t, quote.RetainedQuotaUSD)
	require.InDelta(t, 46.19, quote.ReclaimQuotaUSD, 1e-9)
}

func TestBalancePackageRefundQuoteCountsQuotaRetainedOnCancellation(t *testing.T) {
	for _, tc := range []struct {
		name         string
		detail       string
		wantRetained float64
	}{
		{name: "remaining kept in balance", detail: `{"remaining_usd_before":40,"balance_unchanged_usd":55}`, wantRetained: 40},
		{name: "balance below remaining", detail: `{"remaining_usd_before":40,"balance_unchanged_usd":15}`, wantRetained: 15},
		{name: "negative balance", detail: `{"remaining_usd_before":40,"balance_unchanged_usd":-3}`, wantRetained: 0},
		{name: "detail without balance", detail: `{"remaining_usd_before":0.45}`, wantRetained: 0.45},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			createBalancePackageUsageLedgerTableForTest(t, ctx, client)
			now := time.Now().UTC()
			fx := newBalancePackageRefundFixture(t, ctx, client, "quote-retained", balancePackageRefundFixtureOptions{
				orderStatus:   OrderStatusRefundFailed,
				storedRefund:  36.75,
				packageStatus: balancePackageStatusCancelled,
				startsAt:      now.Add(-24 * time.Hour),
			})
			_, err := client.PaymentAuditLog.Create().
				SetOrderID(strconv.FormatInt(fx.order.ID, 10)).
				SetAction(balancePackageManualCancelAudit).
				SetOperator("admin:1").
				SetDetail(tc.detail).
				Save(ctx)
			require.NoError(t, err)
			insertBalancePackageUsageForTest(t, ctx, client, fx.pkg, "used", 88, "request", now.Add(-time.Hour))
			svc := &PaymentService{entClient: client}

			quote, err := svc.GetBalancePackageRefundQuoteForAdmin(ctx, fx.order.ID)
			require.NoError(t, err)
			require.False(t, quote.ManualReviewRequired)
			require.InDelta(t, tc.wantRetained, quote.RetainedQuotaUSD, 1e-9)
			require.InDelta(t, 88+tc.wantRetained, quote.UsedQuotaUSD, 1e-9)
			require.Zero(t, quote.ReclaimQuotaUSD)
			require.InDelta(t, 49*(1-(88+tc.wantRetained)/512), quote.EstimatedRefundAmount, 1e-9)
		})
	}
}

func TestBalancePackageRefundQuoteRequiresReviewForCancellationWithoutAudit(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	createBalancePackageUsageLedgerTableForTest(t, ctx, client)
	fx := newBalancePackageRefundFixture(t, ctx, client, "quote-cancel-no-audit", balancePackageRefundFixtureOptions{
		orderStatus:   OrderStatusCompleted,
		packageStatus: balancePackageStatusCancelled,
	})
	svc := &PaymentService{entClient: client}

	quote, err := svc.GetBalancePackageRefundQuoteForAdmin(ctx, fx.order.ID)
	require.NoError(t, err)
	require.True(t, quote.ManualReviewRequired)
	require.False(t, quote.Eligible)
}

type balancePackageRefundFixtureOptions struct {
	orderStatus   string
	storedRefund  float64
	balance       float64
	remaining     float64
	packageStatus string
	startsAt      time.Time
	creditedCount int
}

type balancePackageRefundFixture struct {
	user  *dbent.User
	order *dbent.PaymentOrder
	pkg   *dbent.UserBalancePackage
}

// newBalancePackageRefundFixture 建一张 ¥49、每周 $128、共 4 期的真实支付套餐订单。
func newBalancePackageRefundFixture(t *testing.T, ctx context.Context, client *dbent.Client, suffix string, opts balancePackageRefundFixtureOptions) balancePackageRefundFixture {
	t.Helper()
	now := time.Now().UTC()
	if opts.startsAt.IsZero() {
		opts.startsAt = now.Add(-time.Hour)
	}
	if opts.creditedCount == 0 {
		opts.creditedCount = 1
	}

	u, err := client.User.Create().
		SetEmail(suffix + "@example.com").
		SetPasswordHash("hash").
		SetUsername(suffix).
		SetBalance(opts.balance).
		Save(ctx)
	require.NoError(t, err)

	inst, err := client.PaymentProviderInstance.Create().
		SetProviderKey(payment.TypeStripe).
		SetName(suffix + "-provider").
		SetConfig("{}").
		SetSupportedTypes("stripe").
		SetEnabled(true).
		SetRefundEnabled(true).
		Save(ctx)
	require.NoError(t, err)

	order, err := client.PaymentOrder.Create().
		SetUserID(u.ID).
		SetUserEmail(u.Email).
		SetUserName(u.Username).
		SetAmount(49).
		SetPayAmount(49.49).
		SetFeeRate(0.01).
		SetRechargeCode("REFUND-" + suffix).
		SetOutTradeNo("sub2_" + suffix).
		SetPaymentType(payment.TypeStripe).
		SetPaymentTradeNo("pi_" + suffix).
		SetOrderType(payment.OrderTypeBalanceSubscription).
		SetStatus(opts.orderStatus).
		SetRefundAmount(opts.storedRefund).
		SetExpiresAt(now.Add(time.Hour)).
		SetPaidAt(opts.startsAt).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		SetProviderInstanceID(strconv.FormatInt(inst.ID, 10)).
		Save(ctx)
	require.NoError(t, err)

	builder := client.UserBalancePackage.Create().
		SetUserID(u.ID).
		SetPlanID(1).
		SetPaymentOrderID(order.ID).
		SetWeeklyCreditUsd(128).
		SetRemainingUsd(opts.remaining).
		SetCreditedCount(opts.creditedCount).
		SetRefreshCount(4).
		SetRefreshIntervalDays(7).
		SetStartsAt(opts.startsAt).
		SetExpiresAt(opts.startsAt.Add(28 * 24 * time.Hour)).
		SetStatus(opts.packageStatus)
	if opts.packageStatus == balancePackageStatusActive {
		builder.SetNextCreditAt(now.Add(24 * time.Hour))
	}
	pkg, err := builder.Save(ctx)
	require.NoError(t, err)
	return balancePackageRefundFixture{user: u, order: order, pkg: pkg}
}

// balance_package_usage_ledger 由迁移建表、不在 ent schema 里，SQLite 测试库要手工建。
func createBalancePackageUsageLedgerTableForTest(t *testing.T, ctx context.Context, client *dbent.Client) {
	t.Helper()
	_, err := client.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS balance_package_usage_ledger (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			balance_package_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			api_key_id INTEGER NOT NULL DEFAULT 0,
			request_id TEXT NOT NULL,
			amount_usd REAL NOT NULL,
			source_type TEXT NOT NULL DEFAULT 'request',
			created_at DATETIME NOT NULL
		)
	`)
	require.NoError(t, err)
}

func insertBalancePackageUsageForTest(t *testing.T, ctx context.Context, client *dbent.Client, pkg *dbent.UserBalancePackage, requestID string, amountUSD float64, sourceType string, createdAt time.Time) {
	t.Helper()
	_, err := client.ExecContext(ctx, `
		INSERT INTO balance_package_usage_ledger (balance_package_id, user_id, request_id, amount_usd, source_type, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, pkg.ID, pkg.UserID, requestID, amountUSD, sourceType, createdAt)
	require.NoError(t, err)
}

// balancePackageRefundUserBalance 直接读表，软删除的用户也能读到。
func balancePackageRefundUserBalance(t *testing.T, ctx context.Context, client *dbent.Client, userID int64) float64 {
	t.Helper()
	rows, err := client.QueryContext(ctx, `SELECT balance FROM users WHERE id = ?`, userID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next())
	var balance float64
	require.NoError(t, rows.Scan(&balance))
	require.NoError(t, rows.Err())
	return balance
}
