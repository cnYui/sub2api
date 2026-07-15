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

func TestReserveBalanceForHybridOrder_OnlyOneConcurrentReservationSucceeds(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentService{entClient: client}
	u := createHybridBalanceHoldTestUser(t, ctx, client, "hybrid-reserve@example.com", 10)
	firstOrderID := createHybridBalanceHoldTestOrder(t, ctx, client, u.ID, "sub2_hold_1")
	secondOrderID := createHybridBalanceHoldTestOrder(t, ctx, client, u.ID, "sub2_hold_2")

	firstErr := svc.reserveBalanceForHybridOrder(ctx, firstOrderID, u.ID, 8, time.Now().Add(35*time.Minute))
	secondErr := svc.reserveBalanceForHybridOrder(ctx, secondOrderID, u.ID, 8, time.Now().Add(35*time.Minute))

	successes := 0
	for _, err := range []error{firstErr, secondErr} {
		if err == nil {
			successes++
			continue
		}
		require.ErrorIs(t, err, errCheckoutChanged)
	}
	require.Equal(t, 1, successes)

	holds, err := client.PaymentBalanceHold.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, holds, 1)
	require.Equal(t, balanceHoldStatusReserved, holds[0].Status)
	require.Equal(t, 8.0, holds[0].Amount)

	reloaded, err := client.User.Query().Where(user.IDEQ(u.ID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, 2.0, reloaded.Balance)
}

func TestPaymentBalanceHold_TransitionsOnlyFromReserved(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentService{entClient: client}
	u := createHybridBalanceHoldTestUser(t, ctx, client, "hybrid-transition@example.com", 10)
	orderID := createHybridBalanceHoldTestOrder(t, ctx, client, u.ID, "sub2_hold_transition")
	_, err := client.PaymentBalanceHold.Create().
		SetOrderID(orderID).
		SetUserID(u.ID).
		SetAmount(8).
		SetStatus(balanceHoldStatusReserved).
		SetExpiresAt(time.Now().Add(35 * time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	captured, err := svc.capturePaymentBalanceHold(ctx, orderID)
	require.NoError(t, err)
	require.Equal(t, 1, captured)
	released, err := svc.releasePaymentBalanceHold(ctx, orderID, "captured_hold_cannot_release")
	require.NoError(t, err)
	require.Zero(t, released)

	hold, err := client.PaymentBalanceHold.Query().Only(ctx)
	require.NoError(t, err)
	require.Equal(t, balanceHoldStatusCaptured, hold.Status)
	require.NotNil(t, hold.CapturedAt)
	require.Nil(t, hold.ReleasedAt)
}

func createHybridBalanceHoldTestUser(t *testing.T, ctx context.Context, client *dbent.Client, email string, balance float64) *dbent.User {
	t.Helper()
	u, err := client.User.Create().
		SetEmail(email).
		SetUsername(email).
		SetPasswordHash("hash").
		SetBalance(balance).
		SetStatus(payment.EntityStatusActive).
		Save(ctx)
	require.NoError(t, err)
	return u
}

func createHybridBalanceHoldTestOrder(t *testing.T, ctx context.Context, client *dbent.Client, userID int64, outTradeNo string) int64 {
	t.Helper()
	order, err := client.PaymentOrder.Create().
		SetUserID(userID).
		SetUserEmail("hybrid-hold@example.com").
		SetUserName("hybrid-hold").
		SetAmount(8).
		SetPayAmount(8).
		SetFeeRate(0).
		SetRechargeCode("PAY-HOLD").
		SetOutTradeNo(outTradeNo).
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeSubscription).
		SetStatus(OrderStatusPending).
		SetExpiresAt(time.Now().Add(30 * time.Minute)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com").
		Save(ctx)
	require.NoError(t, err)
	return order.ID
}
