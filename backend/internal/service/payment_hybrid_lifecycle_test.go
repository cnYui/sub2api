//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentbalancehold"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestResolveHybridPayment_QueryFailureStaysUnknownAndKeepsHold(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)
	order, userID := createHybridLifecycleOrder(t, ctx, client, time.Now().Add(-time.Minute), "")
	svc := newHybridLifecyclePaymentService(client, &hybridLifecycleProvider{queryErr: errors.New("provider timeout")})

	expired, err := svc.ExpireTimedOutOrders(ctx)
	require.NoError(t, err)
	require.Zero(t, expired)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPending, reloaded.Status)
	require.Equal(t, paymentResolutionStatusUnknown, reloaded.PaymentResolutionStatus)
	require.NotNil(t, reloaded.PaymentResolutionDeadline)

	hold, err := client.PaymentBalanceHold.Query().Only(ctx)
	require.NoError(t, err)
	require.Equal(t, balanceHoldStatusReserved, hold.Status)
	reloadedUser, err := client.User.Query().Where(user.IDEQ(userID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, 2.0, reloadedUser.Balance)
}

func TestResolveHybridPayment_ExplicitUnpaidReleasesHold(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)
	order, userID := createHybridLifecycleOrder(t, ctx, client, time.Now().Add(-time.Minute), "")
	svc := newHybridLifecyclePaymentService(client, &hybridLifecycleProvider{resp: &payment.QueryOrderResponse{Status: payment.ProviderStatusFailed}})

	expired, err := svc.ExpireTimedOutOrders(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, expired)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusExpired, reloaded.Status)
	require.Equal(t, paymentResolutionStatusUnpaid, reloaded.PaymentResolutionStatus)
	hold, err := client.PaymentBalanceHold.Query().Only(ctx)
	require.NoError(t, err)
	require.Equal(t, balanceHoldStatusReleased, hold.Status)
	require.NotNil(t, hold.ReleasedAt)
	reloadedUser, err := client.User.Query().Where(user.IDEQ(userID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, 8.32, reloadedUser.Balance)
}

func TestCancelHybridPayment_UnknownReturnsConfirmationPending(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)
	order, _ := createHybridLifecycleOrder(t, ctx, client, time.Now().Add(20*time.Minute), "")
	svc := newHybridLifecyclePaymentService(client, &hybridLifecycleProvider{queryErr: errors.New("provider timeout")})

	outcome, err := svc.CancelOrder(ctx, order.ID, order.UserID)
	require.NoError(t, err)
	require.Equal(t, checkPaidResultConfirmationPending, outcome)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPending, reloaded.Status)
	require.Equal(t, paymentResolutionStatusUnknown, reloaded.PaymentResolutionStatus)
	require.NotNil(t, reloaded.CancelRequestedAt)
	hold, err := client.PaymentBalanceHold.Query().Only(ctx)
	require.NoError(t, err)
	require.Equal(t, balanceHoldStatusReserved, hold.Status)
}

func TestResolveHybridPayment_ReleasesUnknownAfterFiveMinuteDeadline(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)
	deadline := time.Now().Add(-time.Minute)
	order, userID := createHybridLifecycleOrder(t, ctx, client, time.Now().Add(-10*time.Minute), &deadline)
	svc := newHybridLifecyclePaymentService(client, &hybridLifecycleProvider{queryErr: errors.New("provider timeout")})

	expired, err := svc.ExpireTimedOutOrders(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, expired)

	reloaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusExpired, reloaded.Status)
	require.Equal(t, paymentResolutionStatusUnpaid, reloaded.PaymentResolutionStatus)
	hold, err := client.PaymentBalanceHold.Query().Only(ctx)
	require.NoError(t, err)
	require.Equal(t, balanceHoldStatusReleased, hold.Status)
	reloadedUser, err := client.User.Query().Where(user.IDEQ(userID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, 8.32, reloadedUser.Balance)
}

type hybridLifecycleProvider struct {
	queryErr error
	resp     *payment.QueryOrderResponse
}

func (p *hybridLifecycleProvider) Name() string { return "hybrid-lifecycle-provider" }

func (p *hybridLifecycleProvider) ProviderKey() string { return payment.TypeAlipay }

func (p *hybridLifecycleProvider) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeAlipay}
}

func (p *hybridLifecycleProvider) CreatePayment(context.Context, payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	panic("unexpected CreatePayment")
}

func (p *hybridLifecycleProvider) QueryOrder(context.Context, string) (*payment.QueryOrderResponse, error) {
	if p.queryErr != nil {
		return nil, p.queryErr
	}
	if p.resp != nil {
		return p.resp, nil
	}
	return &payment.QueryOrderResponse{Status: payment.ProviderStatusPending}, nil
}

func (p *hybridLifecycleProvider) VerifyNotification(context.Context, string, map[string]string) (*payment.PaymentNotification, error) {
	panic("unexpected VerifyNotification")
}

func (p *hybridLifecycleProvider) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	panic("unexpected Refund")
}

func newHybridLifecyclePaymentService(client *dbent.Client, provider payment.Provider) *PaymentService {
	registry := payment.NewRegistry()
	registry.Register(provider)
	return &PaymentService{entClient: client, registry: registry, providersLoaded: true}
}

func createHybridLifecycleOrder(t *testing.T, ctx context.Context, client *dbent.Client, expiresAt time.Time, resolutionDeadline any) (*dbent.PaymentOrder, int64) {
	t.Helper()
	u, err := client.User.Create().
		SetEmail("hybrid-lifecycle@example.com").
		SetPasswordHash("hash").
		SetUsername("hybrid-lifecycle").
		SetBalance(2).
		SetStatus(payment.EntityStatusActive).
		Save(ctx)
	require.NoError(t, err)
	builder := client.PaymentOrder.Create().
		SetUserID(u.ID).
		SetUserEmail(u.Email).
		SetUserName(u.Username).
		SetAmount(79).
		SetPayAmount(79.79).
		SetFeeRate(1).
		SetFundingMode(paymentFundingModeMixed).
		SetBalanceAmount(6.32).
		SetGatewayAmount(73.47).
		SetRechargeCode("HYBRID-LIFECYCLE").
		SetOutTradeNo("sub2_hybrid_lifecycle").
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeSubscription).
		SetStatus(OrderStatusPending).
		SetExpiresAt(expiresAt).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com")
	if deadline, ok := resolutionDeadline.(*time.Time); ok && deadline != nil {
		builder.SetPaymentResolutionStatus(paymentResolutionStatusUnknown).
			SetPaymentResolutionDeadline(*deadline)
	}
	order, err := builder.Save(ctx)
	require.NoError(t, err)
	_, err = client.PaymentBalanceHold.Create().
		SetOrderID(order.ID).
		SetUserID(u.ID).
		SetAmount(6.32).
		SetStatus(balanceHoldStatusReserved).
		SetExpiresAt(expiresAt.Add(time.Duration(paymentGraceMinutes) * time.Minute)).
		Save(ctx)
	require.NoError(t, err)
	count, err := client.PaymentBalanceHold.Query().Where(paymentbalancehold.OrderIDEQ(order.ID)).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	return order, u.ID
}
