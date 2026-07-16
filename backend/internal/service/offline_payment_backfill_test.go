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

func TestRunOfflinePaymentBackfillRejectsMissingRequiredUniqueIndexBeforeTransaction(t *testing.T) {
	tests := []struct {
		name            string
		missingIndex    string
		paymentIndexRow []any
		auditIndexRow   []any
	}{
		{
			name:         "payment orders out trade number",
			missingIndex: "payment_orders",
		},
		{
			name:            "payment audit order action",
			missingIndex:    "payment_audit_logs",
			paymentIndexRow: []any{true, "out_trade_no", "out_trade_no <> ''"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })

			expectOfflinePaymentBackfillSchemaBase(mock)
			mock.ExpectQuery(`(?s)SELECT.*indisunique.*FROM pg_class AS index_class`).
				WithArgs("payment_orders", "paymentorder_out_trade_no").
				WillReturnRows(offlinePaymentBackfillIndexRows(tt.paymentIndexRow))
			if tt.missingIndex == "payment_audit_logs" {
				mock.ExpectQuery(`(?s)SELECT.*indisunique.*FROM pg_class AS index_class`).
					WithArgs("payment_audit_logs", "idx_payment_audit_logs_order_action_uniq").
					WillReturnRows(offlinePaymentBackfillIndexRows(tt.auditIndexRow))
			}

			_, err = runOfflinePaymentBackfillBatch(context.Background(), db, defaultOfflinePaymentBackfillBatch(), "unit:operator", true)
			require.Error(t, err)
			require.Equal(t, "OFFLINE_PAYMENT_BACKFILL_SCHEMA_NOT_READY", infraerrors.Reason(err))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestEnsureOfflinePaymentBackfillSchemaRejectsMissingAuditLogSchema(t *testing.T) {
	tests := []struct {
		name            string
		auditTableRows  *sqlmock.Rows
		auditColumnRows *sqlmock.Rows
	}{
		{
			name:           "table",
			auditTableRows: sqlmock.NewRows([]string{"table_name"}),
		},
		{
			name:            "operator column",
			auditTableRows:  sqlmock.NewRows([]string{"table_name"}).AddRow("payment_audit_logs"),
			auditColumnRows: offlinePaymentBackfillColumnRows([]string{"id", "order_id", "action", "detail", "created_at"}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })

			mock.ExpectQuery(`(?s)SELECT filename.*FROM schema_migrations`).
				WithArgs("162_refund_state_machine.sql", "163_alipay_balance_hybrid_payment.sql").
				WillReturnRows(sqlmock.NewRows([]string{"filename"}).
					AddRow("162_refund_state_machine.sql").
					AddRow("163_alipay_balance_hybrid_payment.sql"))
			mock.ExpectQuery(`(?s)SELECT column_name.*FROM information_schema.columns.*table_name = 'payment_orders'`).
				WillReturnRows(offlinePaymentBackfillColumnRows(offlinePaymentBackfillRequiredColumns))
			mock.ExpectQuery(`(?s)SELECT table_name.*FROM information_schema.tables`).
				WithArgs("payment_audit_logs").
				WillReturnRows(tt.auditTableRows)
			if tt.auditColumnRows != nil {
				mock.ExpectQuery(`(?s)SELECT column_name.*FROM information_schema.columns.*table_name = 'payment_audit_logs'`).
					WillReturnRows(tt.auditColumnRows)
			}

			err = ensureOfflinePaymentBackfillSchema(context.Background(), db)
			require.Error(t, err)
			require.Equal(t, "OFFLINE_PAYMENT_BACKFILL_SCHEMA_NOT_READY", infraerrors.Reason(err))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestEnsureOfflinePaymentBackfillSchemaRejectsInvalidRequiredUniqueIndexDefinition(t *testing.T) {
	tests := []struct {
		name            string
		paymentIndexRow []any
		auditIndexRow   []any
	}{
		{
			name:            "payment orders index is not unique",
			paymentIndexRow: []any{false, "out_trade_no", "out_trade_no <> ''"},
		},
		{
			name:            "payment orders predicate differs",
			paymentIndexRow: []any{true, "out_trade_no", "out_trade_no <> 'placeholder'"},
		},
		{
			name:            "payment audit columns differ",
			paymentIndexRow: []any{true, "out_trade_no", "out_trade_no <> ''"},
			auditIndexRow:   []any{true, "action,order_id", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })

			expectOfflinePaymentBackfillSchemaBase(mock)
			mock.ExpectQuery(`(?s)SELECT.*indisunique.*FROM pg_class AS index_class`).
				WithArgs("payment_orders", "paymentorder_out_trade_no").
				WillReturnRows(offlinePaymentBackfillIndexRows(tt.paymentIndexRow))
			if tt.auditIndexRow != nil {
				mock.ExpectQuery(`(?s)SELECT.*indisunique.*FROM pg_class AS index_class`).
					WithArgs("payment_audit_logs", "idx_payment_audit_logs_order_action_uniq").
					WillReturnRows(offlinePaymentBackfillIndexRows(tt.auditIndexRow))
			}

			err = ensureOfflinePaymentBackfillSchema(context.Background(), db)
			require.Error(t, err)
			require.Equal(t, "OFFLINE_PAYMENT_BACKFILL_SCHEMA_NOT_READY", infraerrors.Reason(err))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func expectOfflinePaymentBackfillSchemaBase(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`(?s)SELECT filename.*FROM schema_migrations`).
		WithArgs("162_refund_state_machine.sql", "163_alipay_balance_hybrid_payment.sql").
		WillReturnRows(sqlmock.NewRows([]string{"filename"}).
			AddRow("162_refund_state_machine.sql").
			AddRow("163_alipay_balance_hybrid_payment.sql"))
	mock.ExpectQuery(`(?s)SELECT column_name.*FROM information_schema.columns.*table_name = 'payment_orders'`).
		WillReturnRows(offlinePaymentBackfillColumnRows(offlinePaymentBackfillRequiredColumns))
	mock.ExpectQuery(`(?s)SELECT table_name.*FROM information_schema.tables`).
		WithArgs("payment_audit_logs").
		WillReturnRows(sqlmock.NewRows([]string{"table_name"}).AddRow("payment_audit_logs"))
	mock.ExpectQuery(`(?s)SELECT column_name.*FROM information_schema.columns.*table_name = 'payment_audit_logs'`).
		WillReturnRows(offlinePaymentBackfillColumnRows([]string{"id", "order_id", "action", "detail", "operator", "created_at"}))
}

func offlinePaymentBackfillColumnRows(columns []string) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"column_name"})
	for _, column := range columns {
		rows.AddRow(column)
	}
	return rows
}

func offlinePaymentBackfillIndexRows(row []any) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"indisunique", "columns", "predicate"})
	if row != nil {
		rows.AddRow(row[0], row[1], row[2])
	}
	return rows
}
