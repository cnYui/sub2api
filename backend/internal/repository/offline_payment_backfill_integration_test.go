//go:build integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/tool/offlinepaymentbackfill"
	"github.com/stretchr/testify/require"
)

const offlinePaymentBackfillTestOperator = "integration:offline-payment-backfill"

type offlinePaymentBackfillFixture struct {
	batch    offlinepaymentbackfill.OfflinePaymentBackfillBatch
	groupIDs []int64
	userIDs  []int64
}

type offlinePaymentBackfillOrderSnapshot struct {
	Orders    int
	Audits    int
	Holds     int
	Traffic   int
	Affiliate int
}

func TestOfflinePaymentBackfillCreatesFixedBatch(t *testing.T) {
	ctx := context.Background()
	fixture := newOfflinePaymentBackfillFixture(t)
	before := snapshotOfflinePaymentBackfill(t, ctx, fixture)
	startedAt := time.Now().UTC()

	result, err := offlinepaymentbackfill.RunOfflinePaymentBackfillBatch(ctx, integrationDB, fixture.batch, offlinePaymentBackfillTestOperator, true)
	finishedAt := time.Now().UTC()
	require.NoError(t, err)
	require.Equal(t, 5, result.Created)
	require.Zero(t, result.Planned)
	require.Zero(t, result.Existing)
	require.False(t, result.DryRun)

	after := snapshotOfflinePaymentBackfill(t, ctx, fixture)
	require.Equal(t, before.Orders+5, after.Orders)
	require.Equal(t, before.Audits+5, after.Audits)
	require.Equal(t, before.Holds, after.Holds)
	require.Equal(t, before.Traffic, after.Traffic)
	require.Equal(t, before.Affiliate, after.Affiliate)

	var totalPayAmount float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(pay_amount), 0)
		FROM payment_orders
		WHERE out_trade_no LIKE $1
	`, fixture.batch.Source+"%").Scan(&totalPayAmount))
	require.InDelta(t, 145, totalPayAmount, 0.000001)

	for _, entry := range fixture.batch.Entries {
		assertOfflinePaymentBackfillOrder(t, ctx, fixture, entry, startedAt, finishedAt)
		assertOfflinePaymentBackfillEntitlementUnchanged(t, ctx, fixture, entry)
	}
}

func TestOfflinePaymentBackfillDryRunDoesNotWrite(t *testing.T) {
	ctx := context.Background()
	fixture := newOfflinePaymentBackfillFixture(t)
	before := snapshotOfflinePaymentBackfill(t, ctx, fixture)

	result, err := offlinepaymentbackfill.RunOfflinePaymentBackfillBatch(ctx, integrationDB, fixture.batch, offlinePaymentBackfillTestOperator, false)
	require.NoError(t, err)
	require.Zero(t, result.Created)
	require.Equal(t, 5, result.Planned)
	require.Zero(t, result.Existing)
	require.True(t, result.DryRun)

	after := snapshotOfflinePaymentBackfill(t, ctx, fixture)
	require.Equal(t, before, after)
	for _, entry := range fixture.batch.Entries {
		assertOfflinePaymentBackfillEntitlementUnchanged(t, ctx, fixture, entry)
	}
}

func TestOfflinePaymentBackfillDryRunDoesNotAdvanceOrderSequence(t *testing.T) {
	ctx := context.Background()
	fixture := newOfflinePaymentBackfillFixture(t)

	var before int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT nextval(pg_get_serial_sequence('payment_orders', 'id'))
	`).Scan(&before))

	result, err := offlinepaymentbackfill.RunOfflinePaymentBackfillBatch(ctx, integrationDB, fixture.batch, offlinePaymentBackfillTestOperator, false)
	require.NoError(t, err)
	require.Equal(t, 5, result.Planned)

	var after int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT nextval(pg_get_serial_sequence('payment_orders', 'id'))
	`).Scan(&after))
	require.Equal(t, before+1, after)
}

func TestOfflinePaymentBackfillExactRerunIsNoop(t *testing.T) {
	ctx := context.Background()
	fixture := newOfflinePaymentBackfillFixture(t)

	first, err := offlinepaymentbackfill.RunOfflinePaymentBackfillBatch(ctx, integrationDB, fixture.batch, offlinePaymentBackfillTestOperator, true)
	require.NoError(t, err)
	require.Equal(t, 5, first.Created)
	before := snapshotOfflinePaymentBackfill(t, ctx, fixture)

	result, err := offlinepaymentbackfill.RunOfflinePaymentBackfillBatch(ctx, integrationDB, fixture.batch, "another-authorized-operator", true)
	require.NoError(t, err)
	require.Zero(t, result.Created)
	require.Zero(t, result.Planned)
	require.Equal(t, 5, result.Existing)
	require.True(t, result.Noop)

	after := snapshotOfflinePaymentBackfill(t, ctx, fixture)
	require.Equal(t, before, after)
}

func TestOfflinePaymentBackfillRejectsEmptyOperator(t *testing.T) {
	ctx := context.Background()
	fixture := newOfflinePaymentBackfillFixture(t)
	before := snapshotOfflinePaymentBackfill(t, ctx, fixture)

	_, err := offlinepaymentbackfill.RunOfflinePaymentBackfillBatch(ctx, integrationDB, fixture.batch, "  ", true)
	require.Error(t, err)
	require.Equal(t, "OFFLINE_PAYMENT_BACKFILL_OPERATOR_REQUIRED", infraerrors.Reason(err))
	require.Equal(t, before, snapshotOfflinePaymentBackfill(t, ctx, fixture))
}

func TestOfflinePaymentBackfillFailsClosedOnPreconditions(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, fixture *offlinePaymentBackfillFixture)
	}{
		{
			name: "partial existing batch",
			mutate: func(t *testing.T, fixture *offlinePaymentBackfillFixture) {
				insertOfflinePaymentBackfillOrder(t, fixture, fixture.batch.Entries[0], offlinePaymentBackfillOutTradeNo(fixture.batch.Source, fixture.batch.Entries[0].SubscriptionID))
			},
		},
		{
			name: "other subscription order",
			mutate: func(t *testing.T, fixture *offlinePaymentBackfillFixture) {
				insertOfflinePaymentBackfillOrder(t, fixture, fixture.batch.Entries[0], fixture.batch.Source+"_other")
			},
		},
		{
			name: "subscription missing",
			mutate: func(t *testing.T, fixture *offlinePaymentBackfillFixture) {
				_, err := integrationDB.ExecContext(context.Background(), "DELETE FROM user_subscriptions WHERE id = $1", fixture.batch.Entries[0].SubscriptionID)
				require.NoError(t, err)
			},
		},
		{
			name: "user deleted",
			mutate: func(t *testing.T, fixture *offlinePaymentBackfillFixture) {
				_, err := integrationDB.ExecContext(context.Background(), "UPDATE users SET deleted_at = NOW() WHERE id = $1", fixture.batch.Entries[0].UserID)
				require.NoError(t, err)
			},
		},
		{
			name: "subscription group mismatch",
			mutate: func(t *testing.T, fixture *offlinePaymentBackfillFixture) {
				otherGroupID := insertOfflinePaymentBackfillGroup(t, fixture.batch.Source+"-other-group")
				fixture.groupIDs = append(fixture.groupIDs, otherGroupID)
				_, err := integrationDB.ExecContext(context.Background(), "UPDATE user_subscriptions SET group_id = $1 WHERE id = $2", otherGroupID, fixture.batch.Entries[0].SubscriptionID)
				require.NoError(t, err)
			},
		},
		{
			name: "subscription status mismatch",
			mutate: func(t *testing.T, fixture *offlinePaymentBackfillFixture) {
				_, err := integrationDB.ExecContext(context.Background(), "UPDATE user_subscriptions SET status = 'suspended' WHERE id = $1", fixture.batch.Entries[0].SubscriptionID)
				require.NoError(t, err)
			},
		},
		{
			name: "subscription expiry mismatch",
			mutate: func(t *testing.T, fixture *offlinePaymentBackfillFixture) {
				entry := fixture.batch.Entries[0]
				_, err := integrationDB.ExecContext(context.Background(), "UPDATE user_subscriptions SET expires_at = $1 WHERE id = $2", entry.ExpectedExpiry.Add(time.Minute), entry.SubscriptionID)
				require.NoError(t, err)
			},
		},
		{
			name: "plan mismatch",
			mutate: func(t *testing.T, fixture *offlinePaymentBackfillFixture) {
				_, err := integrationDB.ExecContext(context.Background(), "UPDATE subscription_plans SET price = 30 WHERE id = $1", fixture.batch.PlanID)
				require.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			fixture := newOfflinePaymentBackfillFixture(t)
			tt.mutate(t, fixture)
			before := snapshotOfflinePaymentBackfill(t, ctx, fixture)

			_, err := offlinepaymentbackfill.RunOfflinePaymentBackfillBatch(ctx, integrationDB, fixture.batch, offlinePaymentBackfillTestOperator, true)
			require.Error(t, err)
			require.Equal(t, "OFFLINE_PAYMENT_BACKFILL_PRECONDITION_FAILED", infraerrors.Reason(err))
			require.Equal(t, before, snapshotOfflinePaymentBackfill(t, ctx, fixture))
		})
	}
}

func TestOfflinePaymentBackfillRejectsExistingOrderAndAuditMismatch(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, fixture *offlinePaymentBackfillFixture)
	}{
		{
			name: "order fields",
			mutate: func(t *testing.T, fixture *offlinePaymentBackfillFixture) {
				_, err := integrationDB.ExecContext(context.Background(), `
					UPDATE payment_orders
					SET amount = 28
					WHERE out_trade_no = $1
				`, offlinePaymentBackfillOutTradeNo(fixture.batch.Source, fixture.batch.Entries[0].SubscriptionID))
				require.NoError(t, err)
			},
		},
		{
			name: "audit fields",
			mutate: func(t *testing.T, fixture *offlinePaymentBackfillFixture) {
				entry := fixture.batch.Entries[0]
				_, err := integrationDB.ExecContext(context.Background(), `
					UPDATE payment_audit_logs
					SET detail = '{}'
					WHERE order_id = (
						SELECT id::text
						FROM payment_orders
						WHERE out_trade_no = $1
					)
				`, offlinePaymentBackfillOutTradeNo(fixture.batch.Source, entry.SubscriptionID))
				require.NoError(t, err)
			},
		},
		{
			name: "audit missing",
			mutate: func(t *testing.T, fixture *offlinePaymentBackfillFixture) {
				entry := fixture.batch.Entries[0]
				_, err := integrationDB.ExecContext(context.Background(), `
					DELETE FROM payment_audit_logs
					WHERE order_id = (
						SELECT id::text
						FROM payment_orders
						WHERE out_trade_no = $1
					)
					  AND action = 'OFFLINE_PAYMENT_RECORDED'
				`, offlinePaymentBackfillOutTradeNo(fixture.batch.Source, entry.SubscriptionID))
				require.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			fixture := newOfflinePaymentBackfillFixture(t)
			_, err := offlinepaymentbackfill.RunOfflinePaymentBackfillBatch(ctx, integrationDB, fixture.batch, offlinePaymentBackfillTestOperator, true)
			require.NoError(t, err)
			tt.mutate(t, fixture)
			before := snapshotOfflinePaymentBackfill(t, ctx, fixture)

			_, err = offlinepaymentbackfill.RunOfflinePaymentBackfillBatch(ctx, integrationDB, fixture.batch, offlinePaymentBackfillTestOperator, true)
			require.Error(t, err)
			require.Equal(t, "OFFLINE_PAYMENT_BACKFILL_EXISTING_RECORD_MISMATCH", infraerrors.Reason(err))
			require.Equal(t, before, snapshotOfflinePaymentBackfill(t, ctx, fixture))
		})
	}
}

func TestOfflinePaymentBackfillRejectsDuplicateOfflinePaymentAudit(t *testing.T) {
	ctx := context.Background()
	fixture := newOfflinePaymentBackfillFixture(t)
	_, err := offlinepaymentbackfill.RunOfflinePaymentBackfillBatch(ctx, integrationDB, fixture.batch, offlinePaymentBackfillTestOperator, true)
	require.NoError(t, err)

	entry := fixture.batch.Entries[0]
	var orderID string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT id::text
		FROM payment_orders
		WHERE out_trade_no = $1
	`, offlinePaymentBackfillOutTradeNo(fixture.batch.Source, entry.SubscriptionID)).Scan(&orderID))

	_, err = integrationDB.ExecContext(ctx, `DROP INDEX idx_payment_audit_logs_order_action_uniq`)
	require.NoError(t, err)

	var duplicateAuditID int64
	t.Cleanup(func() {
		if duplicateAuditID != 0 {
			_, cleanupErr := integrationDB.ExecContext(ctx, "DELETE FROM payment_audit_logs WHERE id = $1", duplicateAuditID)
			require.NoError(t, cleanupErr)
		}
		_, cleanupErr := integrationDB.ExecContext(ctx, `
			CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_audit_logs_order_action_uniq
			ON payment_audit_logs(order_id, action)
		`)
		require.NoError(t, cleanupErr)
	})

	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO payment_audit_logs (order_id, action, detail, operator, created_at)
		SELECT order_id, action, detail, operator, created_at
		FROM payment_audit_logs
		WHERE order_id = $1
		  AND action = $2
		RETURNING id
	`, orderID, "OFFLINE_PAYMENT_RECORDED").Scan(&duplicateAuditID))

	before := snapshotOfflinePaymentBackfill(t, ctx, fixture)
	_, err = offlinepaymentbackfill.RunOfflinePaymentBackfillBatch(ctx, integrationDB, fixture.batch, offlinePaymentBackfillTestOperator, true)
	require.Error(t, err)
	require.Equal(t, "OFFLINE_PAYMENT_BACKFILL_SCHEMA_NOT_READY", infraerrors.Reason(err))
	require.Equal(t, before, snapshotOfflinePaymentBackfill(t, ctx, fixture))
}

func TestOfflinePaymentBackfillRejectsDuplicateExistingOrder(t *testing.T) {
	ctx := context.Background()
	fixture := newOfflinePaymentBackfillFixture(t)
	_, err := offlinepaymentbackfill.RunOfflinePaymentBackfillBatch(ctx, integrationDB, fixture.batch, offlinePaymentBackfillTestOperator, true)
	require.NoError(t, err)

	entry := fixture.batch.Entries[0]
	var orderID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT id
		FROM payment_orders
		WHERE out_trade_no = $1
	`, offlinePaymentBackfillOutTradeNo(fixture.batch.Source, entry.SubscriptionID)).Scan(&orderID))

	_, err = integrationDB.ExecContext(ctx, `DROP INDEX paymentorder_out_trade_no`)
	require.NoError(t, err)

	var duplicateOrderID int64
	t.Cleanup(func() {
		if duplicateOrderID != 0 {
			_, cleanupErr := integrationDB.ExecContext(ctx, "DELETE FROM payment_audit_logs WHERE order_id = $1", fmt.Sprint(duplicateOrderID))
			require.NoError(t, cleanupErr)
			_, cleanupErr = integrationDB.ExecContext(ctx, "DELETE FROM payment_orders WHERE id = $1", duplicateOrderID)
			require.NoError(t, cleanupErr)
		}
		_, cleanupErr := integrationDB.ExecContext(ctx, `
			CREATE UNIQUE INDEX IF NOT EXISTS paymentorder_out_trade_no
			ON payment_orders (out_trade_no)
			WHERE out_trade_no <> ''
		`)
		require.NoError(t, cleanupErr)
	})

	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO payment_orders (
			user_id, user_email, user_name, user_notes,
			amount, pay_amount, fee_rate, funding_mode, balance_amount, gateway_amount,
			recharge_code, out_trade_no, payment_type, payment_trade_no, order_type,
			plan_id, subscription_group_id, subscription_days, subscription_id,
			provider_instance_id, provider_key, provider_snapshot, status,
			expires_at, paid_at, completed_at, client_ip, src_host, src_url, created_at, updated_at
		)
		SELECT
			user_id, user_email, user_name, user_notes,
			amount, pay_amount, fee_rate, funding_mode, balance_amount, gateway_amount,
			recharge_code, out_trade_no, payment_type, payment_trade_no, order_type,
			plan_id, subscription_group_id, subscription_days, subscription_id,
			provider_instance_id, provider_key, provider_snapshot, status,
			expires_at, paid_at, completed_at, client_ip, src_host, src_url, created_at, updated_at
		FROM payment_orders
		WHERE id = $1
		RETURNING id
	`, orderID).Scan(&duplicateOrderID))
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO payment_audit_logs (order_id, action, detail, operator, created_at)
		SELECT $1, action, detail, operator, created_at
		FROM payment_audit_logs
		WHERE order_id = $2
		  AND action = $3
	`, fmt.Sprint(duplicateOrderID), fmt.Sprint(orderID), "OFFLINE_PAYMENT_RECORDED")
	require.NoError(t, err)

	before := snapshotOfflinePaymentBackfill(t, ctx, fixture)
	_, err = offlinepaymentbackfill.RunOfflinePaymentBackfillBatch(ctx, integrationDB, fixture.batch, offlinePaymentBackfillTestOperator, true)
	require.Error(t, err)
	require.Equal(t, "OFFLINE_PAYMENT_BACKFILL_SCHEMA_NOT_READY", infraerrors.Reason(err))
	require.Equal(t, before, snapshotOfflinePaymentBackfill(t, ctx, fixture))
}

func newOfflinePaymentBackfillFixture(t *testing.T) *offlinePaymentBackfillFixture {
	t.Helper()
	ctx := context.Background()
	source := fmt.Sprintf("offlinebackfilltest%d", time.Now().UnixNano())
	groupID := insertOfflinePaymentBackfillGroup(t, source+"-group")
	planID := insertOfflinePaymentBackfillPlan(t, groupID, source+"-plan")
	fixture := &offlinePaymentBackfillFixture{
		batch: offlinepaymentbackfill.OfflinePaymentBackfillBatch{
			Source:  source,
			PlanID:  planID,
			GroupID: groupID,
			Entries: make([]offlinepaymentbackfill.OfflinePaymentBackfillEntry, 0, 5),
		},
		groupIDs: []int64{groupID},
	}

	for i := 0; i < 5; i++ {
		paidAt := time.Date(2026, time.July, 16, 10+i, i, i*1000, 0, time.FixedZone("CST", 8*60*60))
		expectedExpiry := time.Date(2026, time.August, 16+i, 0, 0, 0, 0, time.FixedZone("CST", 8*60*60))
		userID := insertOfflinePaymentBackfillUser(t, fmt.Sprintf("%s-user-%d@example.test", source, i), float64(i)+10.25, float64(i)+100.5)
		subscriptionID := insertOfflinePaymentBackfillSubscription(t, userID, groupID, paidAt.Add(-24*time.Hour), expectedExpiry)
		fixture.userIDs = append(fixture.userIDs, userID)
		fixture.batch.Entries = append(fixture.batch.Entries, offlinepaymentbackfill.OfflinePaymentBackfillEntry{
			SubscriptionID: subscriptionID,
			UserID:         userID,
			PaidAt:         paidAt,
			ExpectedExpiry: expectedExpiry,
		})
	}

	t.Cleanup(func() {
		cleanupOfflinePaymentBackfillFixture(t, ctx, fixture)
	})
	return fixture
}

func insertOfflinePaymentBackfillGroup(t *testing.T, name string) int64 {
	t.Helper()
	var id int64
	err := integrationDB.QueryRowContext(context.Background(), `
		INSERT INTO groups (name, platform, subscription_type, status, rate_multiplier, is_exclusive)
		VALUES ($1, 'openai', 'standard', 'active', 1, false)
		RETURNING id
	`, name).Scan(&id)
	require.NoError(t, err)
	return id
}

func insertOfflinePaymentBackfillPlan(t *testing.T, groupID int64, name string) int64 {
	t.Helper()
	var id int64
	err := integrationDB.QueryRowContext(context.Background(), `
		INSERT INTO subscription_plans (group_id, name, price, validity_days)
		VALUES ($1, $2, 29.00, 30)
		RETURNING id
	`, groupID, name).Scan(&id)
	require.NoError(t, err)
	return id
}

func insertOfflinePaymentBackfillUser(t *testing.T, email string, balance, totalRecharged float64) int64 {
	t.Helper()
	var id int64
	err := integrationDB.QueryRowContext(context.Background(), `
		INSERT INTO users (email, password_hash, role, status, balance, total_recharged, username, notes)
		VALUES ($1, 'test-password-hash', 'user', 'active', $2, $3, 'offline-backfill-user', 'offline-backfill-notes')
		RETURNING id
	`, email, balance, totalRecharged).Scan(&id)
	require.NoError(t, err)
	return id
}

func insertOfflinePaymentBackfillSubscription(t *testing.T, userID, groupID int64, startsAt, expiresAt time.Time) int64 {
	t.Helper()
	var id int64
	err := integrationDB.QueryRowContext(context.Background(), `
		INSERT INTO user_subscriptions (user_id, group_id, starts_at, expires_at, status, assigned_at)
		VALUES ($1, $2, $3, $4, 'active', $3)
		RETURNING id
	`, userID, groupID, startsAt, expiresAt).Scan(&id)
	require.NoError(t, err)
	return id
}

func insertOfflinePaymentBackfillOrder(t *testing.T, fixture *offlinePaymentBackfillFixture, entry offlinepaymentbackfill.OfflinePaymentBackfillEntry, outTradeNo string) int64 {
	t.Helper()
	var email string
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), "SELECT email FROM users WHERE id = $1", entry.UserID).Scan(&email))
	var id int64
	err := integrationDB.QueryRowContext(context.Background(), `
		INSERT INTO payment_orders (
			user_id, user_email, user_name, user_notes,
			amount, pay_amount, fee_rate, funding_mode, balance_amount, gateway_amount,
			recharge_code, out_trade_no, payment_type, payment_trade_no, order_type,
			plan_id, subscription_group_id, subscription_days, subscription_id,
			provider_instance_id, provider_key, status,
			expires_at, paid_at, completed_at, client_ip, src_host, created_at, updated_at
		) VALUES (
			$1, $2, 'offline-backfill-user', 'offline-backfill-notes',
			29.00, 29.00, 0, 'offline', 0, 0,
			'', $3, 'offline', '', 'subscription',
			$4, $5, 30, $6,
			NULL, NULL, 'COMPLETED',
			$7, $7, $7, '', '', $7, $7
		)
		RETURNING id
	`, entry.UserID, email, outTradeNo, fixture.batch.PlanID, fixture.batch.GroupID, entry.SubscriptionID, entry.PaidAt).Scan(&id)
	require.NoError(t, err)
	return id
}

func cleanupOfflinePaymentBackfillFixture(t *testing.T, ctx context.Context, fixture *offlinePaymentBackfillFixture) {
	t.Helper()
	prefix := fixture.batch.Source + "%"
	_, err := integrationDB.ExecContext(ctx, `
		DELETE FROM payment_balance_holds
		WHERE order_id IN (
			SELECT id FROM payment_orders WHERE out_trade_no LIKE $1
		)
	`, prefix)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `
		DELETE FROM payment_audit_logs
		WHERE order_id IN (
			SELECT id::text FROM payment_orders WHERE out_trade_no LIKE $1
		)
	`, prefix)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, "DELETE FROM payment_orders WHERE out_trade_no LIKE $1", prefix)
	require.NoError(t, err)
	for _, subscriptionID := range fixture.batch.Entries {
		_, err = integrationDB.ExecContext(ctx, "DELETE FROM user_subscriptions WHERE id = $1", subscriptionID.SubscriptionID)
		require.NoError(t, err)
	}
	for _, userID := range fixture.userIDs {
		_, err = integrationDB.ExecContext(ctx, "DELETE FROM users WHERE id = $1", userID)
		require.NoError(t, err)
	}
	_, err = integrationDB.ExecContext(ctx, "DELETE FROM subscription_plans WHERE id = $1", fixture.batch.PlanID)
	require.NoError(t, err)
	for _, groupID := range fixture.groupIDs {
		_, err = integrationDB.ExecContext(ctx, "DELETE FROM groups WHERE id = $1", groupID)
		require.NoError(t, err)
	}
}

func snapshotOfflinePaymentBackfill(t *testing.T, ctx context.Context, fixture *offlinePaymentBackfillFixture) offlinePaymentBackfillOrderSnapshot {
	t.Helper()
	prefix := fixture.batch.Source + "%"
	snapshot := offlinePaymentBackfillOrderSnapshot{}
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM payment_orders WHERE out_trade_no LIKE $1", prefix).Scan(&snapshot.Orders))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM payment_audit_logs
		WHERE order_id IN (
			SELECT id::text FROM payment_orders WHERE out_trade_no LIKE $1
		)
	`, prefix).Scan(&snapshot.Audits))
	for _, userID := range fixture.userIDs {
		var holds, traffic, affiliate int
		require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM payment_balance_holds WHERE user_id = $1", userID).Scan(&holds))
		require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM traffic_credit_ledger WHERE user_id = $1", userID).Scan(&traffic))
		require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_affiliate_ledger WHERE user_id = $1 OR source_user_id = $1", userID).Scan(&affiliate))
		snapshot.Holds += holds
		snapshot.Traffic += traffic
		snapshot.Affiliate += affiliate
	}
	return snapshot
}

func assertOfflinePaymentBackfillOrder(t *testing.T, ctx context.Context, fixture *offlinePaymentBackfillFixture, entry offlinepaymentbackfill.OfflinePaymentBackfillEntry, startedAt, finishedAt time.Time) {
	t.Helper()
	var order struct {
		ID                  int64
		UserID              int64
		Amount              float64
		PayAmount           float64
		FeeRate             float64
		FundingMode         string
		BalanceAmount       float64
		GatewayAmount       float64
		OutTradeNo          string
		PaymentType         string
		PaymentTradeNo      string
		OrderType           string
		PlanID              sql.NullInt64
		SubscriptionGroupID sql.NullInt64
		SubscriptionDays    sql.NullInt64
		SubscriptionID      sql.NullInt64
		ProviderInstanceID  sql.NullString
		ProviderKey         sql.NullString
		Status              string
		ExpiresAt           time.Time
		PaidAt              sql.NullTime
		CompletedAt         sql.NullTime
		CreatedAt           time.Time
		UpdatedAt           time.Time
	}
	err := integrationDB.QueryRowContext(ctx, `
		SELECT id, user_id, amount, pay_amount, fee_rate, funding_mode, balance_amount, gateway_amount,
			out_trade_no, payment_type, payment_trade_no, order_type, plan_id, subscription_group_id,
			subscription_days, subscription_id, provider_instance_id, provider_key, status,
			expires_at, paid_at, completed_at, created_at, updated_at
		FROM payment_orders
		WHERE out_trade_no = $1
	`, offlinePaymentBackfillOutTradeNo(fixture.batch.Source, entry.SubscriptionID)).Scan(
		&order.ID, &order.UserID, &order.Amount, &order.PayAmount, &order.FeeRate, &order.FundingMode, &order.BalanceAmount, &order.GatewayAmount,
		&order.OutTradeNo, &order.PaymentType, &order.PaymentTradeNo, &order.OrderType, &order.PlanID, &order.SubscriptionGroupID,
		&order.SubscriptionDays, &order.SubscriptionID, &order.ProviderInstanceID, &order.ProviderKey, &order.Status,
		&order.ExpiresAt, &order.PaidAt, &order.CompletedAt, &order.CreatedAt, &order.UpdatedAt,
	)
	require.NoError(t, err)
	require.Equal(t, entry.UserID, order.UserID)
	require.InDelta(t, 29, order.Amount, 0.000001)
	require.InDelta(t, 29, order.PayAmount, 0.000001)
	require.Zero(t, order.FeeRate)
	require.Equal(t, "offline", order.FundingMode)
	require.Zero(t, order.BalanceAmount)
	require.Zero(t, order.GatewayAmount)
	require.Equal(t, offlinePaymentBackfillOutTradeNo(fixture.batch.Source, entry.SubscriptionID), order.OutTradeNo)
	require.Equal(t, payment.TypeOffline, order.PaymentType)
	require.Empty(t, order.PaymentTradeNo)
	require.Equal(t, payment.OrderTypeSubscription, order.OrderType)
	require.True(t, order.PlanID.Valid)
	require.Equal(t, fixture.batch.PlanID, order.PlanID.Int64)
	require.True(t, order.SubscriptionGroupID.Valid)
	require.Equal(t, fixture.batch.GroupID, order.SubscriptionGroupID.Int64)
	require.True(t, order.SubscriptionDays.Valid)
	require.EqualValues(t, 30, order.SubscriptionDays.Int64)
	require.True(t, order.SubscriptionID.Valid)
	require.Equal(t, entry.SubscriptionID, order.SubscriptionID.Int64)
	require.False(t, order.ProviderInstanceID.Valid)
	require.False(t, order.ProviderKey.Valid)
	require.Equal(t, payment.OrderStatusCompleted, order.Status)
	require.True(t, order.ExpiresAt.Equal(entry.PaidAt))
	require.True(t, order.PaidAt.Valid)
	require.True(t, order.PaidAt.Time.Equal(entry.PaidAt))
	require.True(t, order.CompletedAt.Valid)
	require.True(t, order.CompletedAt.Time.Equal(entry.PaidAt))
	require.True(t, order.CreatedAt.Equal(entry.PaidAt))
	require.True(t, order.UpdatedAt.Equal(entry.PaidAt))

	var action, orderID, detail, operator string
	var auditCreatedAt time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT action, order_id, detail, operator, created_at
		FROM payment_audit_logs
		WHERE order_id = $1
	`, fmt.Sprint(order.ID)).Scan(&action, &orderID, &detail, &operator, &auditCreatedAt))
	require.Equal(t, "OFFLINE_PAYMENT_RECORDED", action)
	require.Equal(t, fmt.Sprint(order.ID), orderID)
	require.Equal(t, offlinePaymentBackfillTestOperator, operator)
	require.False(t, auditCreatedAt.Before(startedAt))
	require.False(t, auditCreatedAt.After(finishedAt))
	require.False(t, auditCreatedAt.Equal(entry.PaidAt))
	assertOfflinePaymentBackfillAuditDetail(t, detail, fixture.batch.Source, entry)
}

func assertOfflinePaymentBackfillAuditDetail(t *testing.T, detail, source string, entry offlinepaymentbackfill.OfflinePaymentBackfillEntry) {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(detail))
	decoder.UseNumber()
	var got map[string]any
	require.NoError(t, decoder.Decode(&got))
	require.Equal(t, source, got["source"])
	require.Equal(t, json.Number(fmt.Sprint(entry.SubscriptionID)), got["subscription_id"])
	require.Equal(t, json.Number(fmt.Sprint(entry.UserID)), got["user_id"])
	require.Equal(t, entry.PaidAt.Format(time.RFC3339Nano), got["paid_at"])
	require.Equal(t, json.Number("29.00"), got["amount"])
	require.Equal(t, "CNY", got["currency"])
	require.Equal(t, "manual_only", got["refund_policy"])
}

func assertOfflinePaymentBackfillEntitlementUnchanged(t *testing.T, ctx context.Context, fixture *offlinePaymentBackfillFixture, entry offlinepaymentbackfill.OfflinePaymentBackfillEntry) {
	t.Helper()
	var expiresAt time.Time
	var balance, totalRecharged float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT expires_at FROM user_subscriptions WHERE id = $1", entry.SubscriptionID).Scan(&expiresAt))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance, total_recharged FROM users WHERE id = $1", entry.UserID).Scan(&balance, &totalRecharged))
	require.True(t, expiresAt.Equal(entry.ExpectedExpiry))
	index := -1
	for i, userID := range fixture.userIDs {
		if userID == entry.UserID {
			index = i
			break
		}
	}
	require.GreaterOrEqual(t, index, 0)
	require.InDelta(t, float64(index)+10.25, balance, 0.000001)
	require.InDelta(t, float64(index)+100.5, totalRecharged, 0.000001)
}

func offlinePaymentBackfillOutTradeNo(source string, subscriptionID int64) string {
	return fmt.Sprintf("%s_s%d", source, subscriptionID)
}
