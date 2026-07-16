//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestCreateHybridOrder_ReservesBalanceAndSendsOnlyGatewayDifference(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	u := createHybridBalanceHoldTestUser(t, ctx, client, "hybrid-create@example.com", 6.32)
	planID := createBalancePayTestPlan(t, ctx, client, 7, 79)
	plan, err := client.SubscriptionPlan.Get(ctx, planID)
	require.NoError(t, err)
	capturingProvider := &hybridOrderCreateProvider{}
	svc := &PaymentService{
		entClient: client,
		createProvider: func(string, string, map[string]string) (payment.Provider, error) {
			return capturingProvider, nil
		},
	}
	cfg := &PaymentConfig{MaxPendingOrders: 3, OrderTimeoutMin: 30}
	sel := &payment.InstanceSelection{InstanceID: "hybrid-alipay", ProviderKey: payment.TypeAlipay, PaymentMode: "redirect", Config: map[string]string{}}
	req := CreateOrderRequest{
		UserID:                u.ID,
		PaymentType:           payment.TypeAlipay,
		OrderType:             payment.OrderTypeSubscription,
		UseBalance:            true,
		ExpectedPayAmount:     "79.79",
		ExpectedBalanceAmount: "6.32",
		ClientIP:              "127.0.0.1",
		SrcHost:               "api.example.com",
	}

	order, err := svc.createOrderInTx(ctx, req, hybridServiceUser(u), plan, nil, cfg, 79, 79, 1, 79.79, sel)
	require.NoError(t, err)
	require.Equal(t, paymentFundingModeMixed, order.FundingMode)
	require.Equal(t, 79.79, order.PayAmount)
	require.Equal(t, 6.32, order.BalanceAmount)
	require.Equal(t, 73.47, order.GatewayAmount)

	hold, err := client.PaymentBalanceHold.Query().Only(ctx)
	require.NoError(t, err)
	require.Equal(t, order.ID, hold.OrderID)
	require.Equal(t, balanceHoldStatusReserved, hold.Status)
	require.Equal(t, 6.32, hold.Amount)

	_, err = svc.invokeProvider(ctx, order, req, cfg, 79, "79.79", 79.79, plan, nil, sel)
	require.NoError(t, err)
	require.Equal(t, "73.47", capturingProvider.lastRequest.Amount)
}

func TestClaimProviderInitialization_AllowsOnlyOneCaller(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	u := createHybridBalanceHoldTestUser(t, ctx, client, "hybrid-claim@example.com", 10)
	orderID := createHybridBalanceHoldTestOrder(t, ctx, client, u.ID, "sub2_claim_provider")
	svc := &PaymentService{entClient: client}
	now := time.Now()

	firstClaimed, err := svc.claimProviderInitialization(ctx, orderID, now, 2*time.Minute)
	require.NoError(t, err)
	secondClaimed, err := svc.claimProviderInitialization(ctx, orderID, now, 2*time.Minute)
	require.NoError(t, err)

	require.True(t, firstClaimed)
	require.False(t, secondClaimed)
	order, err := client.PaymentOrder.Get(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, providerInitStatusCreating, order.ProviderInitStatus)
	require.NotNil(t, order.ProviderInitAttemptedAt)
	require.NotNil(t, order.ProviderInitLeaseUntil)
}

func TestCreateHybridOrder_CheckoutChangedDoesNotReserveBalance(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	u := createHybridBalanceHoldTestUser(t, ctx, client, "hybrid-changed@example.com", 5.32)
	planID := createBalancePayTestPlan(t, ctx, client, 7, 79)
	plan, err := client.SubscriptionPlan.Get(ctx, planID)
	require.NoError(t, err)
	svc := &PaymentService{entClient: client}
	req := CreateOrderRequest{
		UserID:                u.ID,
		PaymentType:           payment.TypeAlipay,
		OrderType:             payment.OrderTypeSubscription,
		UseBalance:            true,
		ExpectedPayAmount:     "79.79",
		ExpectedBalanceAmount: "6.32",
		ClientIP:              "127.0.0.1",
		SrcHost:               "api.example.com",
	}

	_, err = svc.createOrderInTx(ctx, req, &User{
		ID:       u.ID,
		Email:    u.Email,
		Username: u.Username,
		Balance:  6.32,
		Status:   payment.EntityStatusActive,
	}, plan, nil, &PaymentConfig{MaxPendingOrders: 3, OrderTimeoutMin: 30}, 79, 79, 1, 79.79, &payment.InstanceSelection{ProviderKey: payment.TypeAlipay})

	require.ErrorIs(t, err, errCheckoutChanged)
	orderCount, err := client.PaymentOrder.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, orderCount)
	holdCount, err := client.PaymentBalanceHold.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, holdCount)
	reloaded, err := client.User.Query().Where(user.IDEQ(u.ID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, 5.32, reloaded.Balance)
}

type hybridOrderCreateProvider struct {
	lastRequest payment.CreatePaymentRequest
}

func (p *hybridOrderCreateProvider) Name() string { return "hybrid-order-provider" }

func (p *hybridOrderCreateProvider) ProviderKey() string { return payment.TypeAlipay }

func (p *hybridOrderCreateProvider) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeAlipay}
}

func (p *hybridOrderCreateProvider) CreatePayment(_ context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	p.lastRequest = req
	return &payment.CreatePaymentResponse{TradeNo: req.OrderID, PayURL: "https://pay.example.com/" + req.OrderID}, nil
}

func (p *hybridOrderCreateProvider) QueryOrder(context.Context, string) (*payment.QueryOrderResponse, error) {
	return nil, nil
}

func (p *hybridOrderCreateProvider) VerifyNotification(context.Context, string, map[string]string) (*payment.PaymentNotification, error) {
	return nil, nil
}

func (p *hybridOrderCreateProvider) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	return nil, nil
}

func hybridServiceUser(u *dbent.User) *User {
	return &User{
		ID:       u.ID,
		Email:    u.Email,
		Username: u.Username,
		Balance:  u.Balance,
		Status:   payment.EntityStatusActive,
	}
}
