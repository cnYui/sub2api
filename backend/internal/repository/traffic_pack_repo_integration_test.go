//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestTrafficPackRepository_CreditPurchase_ReusesOuterTransaction(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	uniqueID := time.Now().UnixNano()

	user, err := integrationEntClient.User.Create().
		SetEmail(fmt.Sprintf("traffic-pack-tx-%d@example.com", uniqueID)).
		SetPasswordHash("test-password-hash").
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, cleanupErr := integrationDB.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
		require.NoError(t, cleanupErr)
	})

	outerTx, err := integrationEntClient.Tx(ctx)
	require.NoError(t, err, "begin outer tx")
	t.Cleanup(func() { _ = outerTx.Rollback() })
	txCtx := dbent.NewTxContext(ctx, outerTx)

	order, err := outerTx.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName("traffic-pack-tx-user").
		SetAmount(3).
		SetPayAmount(3).
		SetFeeRate(0).
		SetRechargeCode(fmt.Sprintf("PAY-BALANCE-TRAFFIC-PACK-%d", uniqueID)).
		SetOutTradeNo(fmt.Sprintf("sub2_traffic_pack_tx_%d", uniqueID)).
		SetPaymentType(payment.TypeBalance).
		SetPaymentTradeNo("balance").
		SetOrderType(payment.OrderTypeTrafficPack).
		SetStatus(payment.OrderStatusRecharging).
		SetExpiresAt(now).
		SetPaidAt(now).
		SetClientIP("127.0.0.1").
		SetSrcHost("integration.test").
		Save(txCtx)
	require.NoError(t, err)

	repo := NewTrafficPackRepository(integrationDB)
	err = repo.CreditPurchase(txCtx, service.CreditTrafficPackInput{
		UserID:       user.ID,
		OrderID:      order.ID,
		PackID:       2,
		CreditUSD:    10,
		ValidityDays: 365,
		CreditedAt:   now,
	})
	require.NoError(t, err)

	rows, err := outerTx.Client().QueryContext(txCtx, `
		SELECT
			(SELECT COUNT(*) FROM user_traffic_credits WHERE order_id = $1),
			(SELECT COUNT(*) FROM traffic_credit_ledger WHERE order_id = $1 AND entry_type = $2)
	`, order.ID, service.TrafficCreditLedgerTypePurchase)
	require.NoError(t, err)
	require.True(t, rows.Next())
	var creditCount int
	var ledgerCount int
	require.NoError(t, rows.Scan(&creditCount, &ledgerCount))
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	require.Equal(t, 1, creditCount)
	require.Equal(t, 1, ledgerCount)

	require.NoError(t, outerTx.Rollback(), "rollback outer tx")

	var orderCountAfterRollback int
	var creditCountAfterRollback int
	var ledgerCountAfterRollback int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM payment_orders WHERE id = $1),
			(SELECT COUNT(*) FROM user_traffic_credits WHERE order_id = $1),
			(SELECT COUNT(*) FROM traffic_credit_ledger WHERE order_id = $1)
	`, order.ID).Scan(&orderCountAfterRollback, &creditCountAfterRollback, &ledgerCountAfterRollback))
	require.Zero(t, orderCountAfterRollback)
	require.Zero(t, creditCountAfterRollback)
	require.Zero(t, ledgerCountAfterRollback)
}
