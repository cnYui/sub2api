//go:build unit

package service

import (
	"context"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	"github.com/Wei-Shaw/sub2api/ent/userbalancepackage"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

// entRedeemCodeTestRepo 是最小的 ent 版兑换码仓储：只实现 Redeem 走到的方法，
// 并且和生产仓储一样从 context 里取事务，这样才能验证失败时兑换码会随事务回滚。
type entRedeemCodeTestRepo struct {
	RedeemCodeRepository
	client *dbent.Client
}

func (r *entRedeemCodeTestRepo) clientFor(ctx context.Context) *dbent.Client {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return tx.Client()
	}
	return r.client
}

func (r *entRedeemCodeTestRepo) toService(m *dbent.RedeemCode) *RedeemCode {
	return &RedeemCode{
		ID:                   m.ID,
		Code:                 m.Code,
		Type:                 m.Type,
		Value:                m.Value,
		Status:               m.Status,
		UsedBy:               m.UsedBy,
		UsedAt:               m.UsedAt,
		CreatedAt:            m.CreatedAt,
		ExpiresAt:            m.ExpiresAt,
		GroupID:              m.GroupID,
		ValidityDays:         m.ValidityDays,
		BalancePackagePlanID: m.BalancePackagePlanID,
	}
}

func (r *entRedeemCodeTestRepo) GetByID(ctx context.Context, id int64) (*RedeemCode, error) {
	m, err := r.clientFor(ctx).RedeemCode.Query().Where(redeemcode.IDEQ(id)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrRedeemCodeNotFound
		}
		return nil, err
	}
	return r.toService(m), nil
}

func (r *entRedeemCodeTestRepo) GetByCode(ctx context.Context, code string) (*RedeemCode, error) {
	m, err := r.clientFor(ctx).RedeemCode.Query().Where(redeemcode.CodeEQ(code)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrRedeemCodeNotFound
		}
		return nil, err
	}
	return r.toService(m), nil
}

func (r *entRedeemCodeTestRepo) Use(ctx context.Context, id, userID int64) error {
	affected, err := r.clientFor(ctx).RedeemCode.Update().
		Where(redeemcode.IDEQ(id), redeemcode.StatusEQ(StatusUnused)).
		SetStatus(StatusUsed).
		SetUsedBy(userID).
		SetUsedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrRedeemCodeUsed
	}
	return nil
}

func (r *entRedeemCodeTestRepo) ListByUser(ctx context.Context, userID int64, limit int) ([]RedeemCode, error) {
	panic("unexpected ListByUser call")
}

func (r *entRedeemCodeTestRepo) ListWithFilters(ctx context.Context, params pagination.PaginationParams, codeType, status, search string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

// purchaseNotifyProbeSettingRepo 记录 PurchaseNotifyService.ready() 读过的开关，
// 用来证明兑换成功后确实走到了发信逻辑（真发信需要 SMTP，这里只验证接线）。
type purchaseNotifyProbeSettingRepo struct {
	*notificationEmailMemorySettingRepo
	mu   sync.Mutex
	keys []string
}

func newPurchaseNotifyProbeSettingRepo() *purchaseNotifyProbeSettingRepo {
	return &purchaseNotifyProbeSettingRepo{notificationEmailMemorySettingRepo: newNotificationEmailMemorySettingRepo()}
}

func (r *purchaseNotifyProbeSettingRepo) GetValue(ctx context.Context, key string) (string, error) {
	r.mu.Lock()
	r.keys = append(r.keys, key)
	r.mu.Unlock()
	return r.notificationEmailMemorySettingRepo.GetValue(ctx, key)
}

func (r *purchaseNotifyProbeSettingRepo) asked(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, k := range r.keys {
		if k == key {
			return true
		}
	}
	return false
}

type balancePackageRedeemFixture struct {
	client    *dbent.Client
	service   *RedeemService
	repo      *entRedeemCodeTestRepo
	plan      *dbent.BalancePackagePlan
	userID    int64
	entClient *dbent.Client

	notifySettings *purchaseNotifyProbeSettingRepo
	notifyDone     chan struct{}
}

func newBalancePackageRedeemFixture(t *testing.T) *balancePackageRedeemFixture {
	t.Helper()
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)

	target, err := client.User.Create().
		SetEmail("package-redeemer@example.com").
		SetPasswordHash("hash").
		Save(ctx)
	require.NoError(t, err)

	plan, err := client.BalancePackagePlan.Create().
		SetCode("redeem-test").
		SetName("Redeem Test 29").
		SetPriceCny(29).
		SetWeeklyCreditUsd(76).
		SetValidityDays(28).
		SetRefreshCount(4).
		SetRefreshIntervalDays(7).
		SetForSale(true).
		Save(ctx)
	require.NoError(t, err)

	repo := &entRedeemCodeTestRepo{client: client}
	svc := NewRedeemService(
		repo,
		&mockUserRepo{getByIDUser: &User{ID: target.ID}},
		nil,
		NewBalancePackageService(client),
		nil,
		nil,
		client,
		nil,
	)

	notifySettings := newPurchaseNotifyProbeSettingRepo()
	// 发信是后台 goroutine，用带缓冲的 channel 收敛完成信号；没人等也不会阻塞。
	notifyDone := make(chan struct{}, 8)
	notify := NewPurchaseNotifyService(
		NewNotificationEmailService(notifySettings, nil),
		notifySettings,
		&mockUserRepo{getByIDUser: &User{ID: target.ID}},
		client,
		nil,
	)
	notify.notifyDone = func() {
		select {
		case notifyDone <- struct{}{}:
		default:
		}
	}
	svc.SetPurchaseNotifyService(notify)

	return &balancePackageRedeemFixture{
		client:         client,
		service:        svc,
		repo:           repo,
		plan:           plan,
		userID:         target.ID,
		entClient:      client,
		notifySettings: notifySettings,
		notifyDone:     notifyDone,
	}
}

func (f *balancePackageRedeemFixture) createCode(t *testing.T, code string, planID *int64) *dbent.RedeemCode {
	t.Helper()
	created, err := f.client.RedeemCode.Create().
		SetCode(code).
		SetType(RedeemTypeBalancePackage).
		SetValue(0).
		SetStatus(StatusUnused).
		SetNillableBalancePackagePlanID(planID).
		Save(context.Background())
	require.NoError(t, err)
	return created
}

func TestRedeemBalancePackageCodeGrantsPackageAndIsNotRefundable(t *testing.T) {
	ctx := context.Background()
	f := newBalancePackageRedeemFixture(t)
	created := f.createCode(t, "PKG-0001", &f.plan.ID)

	redeemed, err := f.service.Redeem(ctx, f.userID, created.Code)
	require.NoError(t, err)
	require.Equal(t, StatusUsed, redeemed.Status)
	require.NotNil(t, redeemed.UsedBy)
	require.Equal(t, f.userID, *redeemed.UsedBy)

	pkg, err := f.client.UserBalancePackage.Query().
		Where(userbalancepackage.UserIDEQ(f.userID)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, f.plan.ID, pkg.PlanID)
	require.Equal(t, 1, pkg.CreditedCount)
	require.Equal(t, f.plan.RefreshCount, pkg.RefreshCount)
	require.Equal(t, f.plan.WeeklyCreditUsd, pkg.WeeklyCreditUsd)
	require.Equal(t, f.plan.WeeklyCreditUsd, pkg.RemainingUsd)
	require.NotNil(t, pkg.NextCreditAt)

	creditedUser, err := f.client.User.Get(ctx, f.userID)
	require.NoError(t, err)
	require.Equal(t, f.plan.WeeklyCreditUsd, creditedUser.Balance)

	order, err := f.client.PaymentOrder.Get(ctx, pkg.PaymentOrderID)
	require.NoError(t, err)
	require.Equal(t, payment.PaymentTypeRedeemCode, order.PaymentType)
	require.Equal(t, payment.OrderTypeBalanceSubscription, order.OrderType)
	require.Equal(t, OrderStatusCompleted, order.Status)
	require.Zero(t, order.Amount)
	require.Zero(t, order.PayAmount)

	// 零金额的兑换订单不能走退款：退款只认真实支付凭证。
	require.Equal(t, "REFUND_REQUIRES_REAL_PAYMENT", infraerrors.Reason(validateRealPaidBalancePackageOrder(order)))

	auditCount, err := f.client.PaymentAuditLog.Query().
		Where(paymentauditlog.ActionEQ("REDEEM_BALANCE_PACKAGE_GRANTED")).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, auditCount)

	// 兑换套餐码也要发到账邮件（和购买、后台发放同一封），这里验证接线确实触发。
	select {
	case <-f.notifyDone:
	case <-time.After(5 * time.Second):
		t.Fatal("balance package redeem did not trigger the credited-notice goroutine")
	}
	require.True(t, f.notifySettings.asked(SettingKeyPurchaseNotifyEnabled),
		"balance package redeem should hand the order to PurchaseNotifyService")
	require.Equal(t, purchaseKindRedeem, balancePackagePurchaseKind(order, pkg))
}

func TestRedeemBalancePackageCodeRejectedWhenUserAlreadyHasPackage(t *testing.T) {
	ctx := context.Background()
	f := newBalancePackageRedeemFixture(t)

	first := f.createCode(t, "PKG-FIRST", &f.plan.ID)
	_, err := f.service.Redeem(ctx, f.userID, first.Code)
	require.NoError(t, err)

	second := f.createCode(t, "PKG-SECOND", &f.plan.ID)
	_, err = f.service.Redeem(ctx, f.userID, second.Code)
	require.Error(t, err)
	require.Equal(t, "BALANCE_PACKAGE_ACTIVE", infraerrors.Reason(err))

	// 事务回滚后第二张码必须还是未使用，用户可以等本期套餐失效后再兑换。
	reloaded, err := f.client.RedeemCode.Get(ctx, second.ID)
	require.NoError(t, err)
	require.Equal(t, StatusUnused, reloaded.Status)
	require.Nil(t, reloaded.UsedBy)

	packages, err := f.client.UserBalancePackage.Query().Where(userbalancepackage.UserIDEQ(f.userID)).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, packages)

	creditedUser, err := f.client.User.Get(ctx, f.userID)
	require.NoError(t, err)
	require.Equal(t, f.plan.WeeklyCreditUsd, creditedUser.Balance)
}

func TestRedeemBalancePackageCodeWithoutPlanIsRejectedBeforeTransaction(t *testing.T) {
	ctx := context.Background()
	f := newBalancePackageRedeemFixture(t)
	created := f.createCode(t, "PKG-NOPLAN", nil)

	_, err := f.service.Redeem(ctx, f.userID, created.Code)
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
	require.Equal(t, "REDEEM_CODE_INVALID", infraerrors.Reason(err))

	reloaded, err := f.client.RedeemCode.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, StatusUnused, reloaded.Status)
}
