package service

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestEnsureOfflinePaymentBackfillSchemaRejectsMissingMigration(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`(?s)SELECT filename.*FROM schema_migrations`).
		WithArgs("162_refund_state_machine.sql", "163_alipay_balance_hybrid_payment.sql").
		WillReturnRows(sqlmock.NewRows([]string{"filename"}).AddRow("163_alipay_balance_hybrid_payment.sql"))

	err = ensureOfflinePaymentBackfillSchema(context.Background(), db)
	require.Error(t, err)
	require.Equal(t, "OFFLINE_PAYMENT_BACKFILL_SCHEMA_NOT_READY", infraerrors.Reason(err))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnsureOfflinePaymentBackfillSchemaRejectsMissingRefundColumn(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`(?s)SELECT filename.*FROM schema_migrations`).
		WithArgs("162_refund_state_machine.sql", "163_alipay_balance_hybrid_payment.sql").
		WillReturnRows(sqlmock.NewRows([]string{"filename"}).
			AddRow("162_refund_state_machine.sql").
			AddRow("163_alipay_balance_hybrid_payment.sql"))
	mock.ExpectQuery(`(?s)SELECT column_name.*FROM information_schema.columns`).
		WillReturnRows(sqlmock.NewRows([]string{"column_name"}).
			AddRow("subscription_id").
			AddRow("funding_mode").
			AddRow("balance_amount").
			AddRow("gateway_amount").
			AddRow("provider_snapshot").
			AddRow("refund_request_id").
			AddRow("refund_gateway_status").
			AddRow("refund_entitlement_status").
			AddRow("refund_balance_amount").
			AddRow("refund_gateway_amount").
			AddRow("refund_balance_status"))

	err = ensureOfflinePaymentBackfillSchema(context.Background(), db)
	require.Error(t, err)
	require.Equal(t, "OFFLINE_PAYMENT_BACKFILL_SCHEMA_NOT_READY", infraerrors.Reason(err))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDefaultOfflinePaymentBackfillBatchContainsOnlyApprovedPaymentFacts(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	expected := []struct {
		subscriptionID int64
		userID         int64
		paidAt         time.Time
		expectedExpiry time.Time
		outTradeNo     string
	}{
		{2, 3, time.Date(2026, time.July, 16, 12, 8, 33, 371_000_000, shanghai), time.Date(2026, time.August, 16, 0, 0, 0, 0, shanghai), "offline_paid_backfill_20260716_s2"},
		{4, 6, time.Date(2026, time.July, 16, 12, 6, 25, 442_000_000, shanghai), time.Date(2026, time.August, 16, 0, 0, 0, 0, shanghai), "offline_paid_backfill_20260716_s4"},
		{7, 12, time.Date(2026, time.July, 16, 12, 5, 16, 893_000_000, shanghai), time.Date(2026, time.August, 16, 0, 0, 0, 0, shanghai), "offline_paid_backfill_20260716_s7"},
		{9, 15, time.Date(2026, time.July, 16, 11, 49, 52, 625_000_000, shanghai), time.Date(2026, time.October, 15, 0, 0, 0, 0, shanghai), "offline_paid_backfill_20260716_s9"},
		{13, 21, time.Date(2026, time.July, 16, 13, 30, 29, 288_000_000, shanghai), time.Date(2026, time.October, 15, 0, 0, 0, 0, shanghai), "offline_paid_backfill_20260716_s13"},
	}

	batch := defaultOfflinePaymentBackfillBatch()
	require.Equal(t, "offline_paid_backfill_20260716", batch.Source)
	require.EqualValues(t, 1, batch.PlanID)
	require.EqualValues(t, 2, batch.GroupID)
	require.EqualValues(t, 29.00, offlinePaymentBackfillAmount)
	require.EqualValues(t, 30, offlinePaymentBackfillDays)
	require.Len(t, batch.Entries, len(expected))
	for index, want := range expected {
		got := batch.Entries[index]
		require.Equal(t, want.subscriptionID, got.SubscriptionID)
		require.Equal(t, want.userID, got.UserID)
		require.True(t, got.PaidAt.Equal(want.paidAt))
		require.True(t, got.ExpectedExpiry.Equal(want.expectedExpiry))
		require.Equal(t, want.outTradeNo, offlinePaymentBackfillOutTradeNo(batch.Source, got.SubscriptionID))
	}
}

func TestEnsureOfflinePaymentBackfillSchemaRejectsMissingProviderSnapshot(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`(?s)SELECT filename.*FROM schema_migrations`).
		WithArgs("162_refund_state_machine.sql", "163_alipay_balance_hybrid_payment.sql").
		WillReturnRows(sqlmock.NewRows([]string{"filename"}).
			AddRow("162_refund_state_machine.sql").
			AddRow("163_alipay_balance_hybrid_payment.sql"))
	mock.ExpectQuery(`(?s)SELECT column_name.*FROM information_schema.columns`).
		WillReturnRows(sqlmock.NewRows([]string{"column_name"}).
			AddRow("subscription_id").
			AddRow("funding_mode").
			AddRow("balance_amount").
			AddRow("gateway_amount").
			AddRow("refund_request_id").
			AddRow("refund_gateway_status").
			AddRow("refund_entitlement_status").
			AddRow("refund_provider_ref").
			AddRow("refund_balance_amount").
			AddRow("refund_gateway_amount").
			AddRow("refund_balance_status"))

	err = ensureOfflinePaymentBackfillSchema(context.Background(), db)
	require.Error(t, err)
	require.Equal(t, "OFFLINE_PAYMENT_BACKFILL_SCHEMA_NOT_READY", infraerrors.Reason(err))
	require.NoError(t, mock.ExpectationsWereMet())
}
