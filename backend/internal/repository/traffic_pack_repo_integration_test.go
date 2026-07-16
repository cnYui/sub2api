//go:build integration

package repository

import (
	"context"
	"database/sql"
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

func TestTrafficPackRepository_CreditPurchaseAcknowledgesPendingExhaustionEvents(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	uniqueID := time.Now().UnixNano()

	user, err := integrationEntClient.User.Create().
		SetEmail(fmt.Sprintf("traffic-pack-ack-%d@example.com", uniqueID)).
		SetPasswordHash("test-password-hash").
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, cleanupErr := integrationDB.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
		require.NoError(t, cleanupErr)
	})

	repo := NewTrafficPackRepository(integrationDB, service.TrafficCreditPolicy{MinimumReserveUSD: 0.01})
	oldOrder := createTrafficPackPaymentOrderForTest(t, user.ID, user.Email, fmt.Sprintf("old-%d", uniqueID), now)
	require.NoError(t, repo.CreditPurchase(ctx, service.CreditTrafficPackInput{
		UserID:       user.ID,
		OrderID:      oldOrder.ID,
		PackID:       2,
		CreditUSD:    1,
		ValidityDays: 365,
		CreditedAt:   now,
	}))
	var oldCreditID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT id FROM user_traffic_credits WHERE order_id = $1", oldOrder.ID).Scan(&oldCreditID))
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO traffic_credit_exhaustion_events (user_id, credit_id, request_id, batch_key)
		VALUES ($1, $2, $3, $4)
	`, user.ID, oldCreditID, "req-old", "batch-old")
	require.NoError(t, err)

	newOrder := createTrafficPackPaymentOrderForTest(t, user.ID, user.Email, fmt.Sprintf("new-%d", uniqueID), now.Add(time.Minute))
	require.NoError(t, repo.CreditPurchase(ctx, service.CreditTrafficPackInput{
		UserID:       user.ID,
		OrderID:      newOrder.ID,
		PackID:       2,
		CreditUSD:    1,
		ValidityDays: 365,
		CreditedAt:   now.Add(time.Minute),
	}))

	var acknowledgedAt sql.NullTime
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT acknowledged_at
		FROM traffic_credit_exhaustion_events
		WHERE user_id = $1 AND credit_id = $2
	`, user.ID, oldCreditID).Scan(&acknowledgedAt))
	require.True(t, acknowledgedAt.Valid)
}

func createTrafficPackPaymentOrderForTest(t *testing.T, userID int64, email, suffix string, now time.Time) *dbent.PaymentOrder {
	t.Helper()
	order, err := integrationEntClient.PaymentOrder.Create().
		SetUserID(userID).
		SetUserEmail(email).
		SetUserName("traffic-pack-ack-user").
		SetAmount(3).
		SetPayAmount(3).
		SetFeeRate(0).
		SetRechargeCode("TRAFFIC-PACK-ACK-" + suffix).
		SetOutTradeNo("traffic_pack_ack_" + suffix).
		SetPaymentType(payment.TypeBalance).
		SetPaymentTradeNo("balance").
		SetOrderType(payment.OrderTypeTrafficPack).
		SetStatus(payment.OrderStatusCompleted).
		SetExpiresAt(now.Add(time.Hour)).
		SetPaidAt(now).
		SetClientIP("127.0.0.1").
		SetSrcHost("integration.test").
		Save(context.Background())
	require.NoError(t, err)
	return order
}
