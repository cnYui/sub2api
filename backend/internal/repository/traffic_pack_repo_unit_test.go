//go:build unit

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	trafficPackUserLockSQL = `(?s)SELECT id FROM users WHERE id=\$1 AND deleted_at IS NULL FOR UPDATE`
	trafficPackDebtNetSQL  = `(?s)SELECT COALESCE\(SUM\(CASE WHEN entry_type='debt' THEN amount_usd ELSE -amount_usd END\),0\) FROM traffic_credit_debt_ledger WHERE user_id=\$1`
	trafficPackCreditSQL   = `(?s)INSERT INTO user_traffic_credits\(user_id,order_id,pack_id,platform,initial_usd,remaining_usd,credited_at,expires_at,created_at,updated_at\).*ON CONFLICT\(order_id\) DO NOTHING RETURNING id`
	trafficPackLedgerSQL   = `(?s)INSERT INTO traffic_credit_ledger\(user_id,credit_id,order_id,request_id,entry_type,amount_usd,balance_after_usd,created_at\)`
	trafficPackRepaySQL    = `(?s)INSERT INTO traffic_credit_debt_ledger\(user_id,entry_type,amount_usd,balance_after_usd,source_type,source_ref,created_at\)`
	trafficPackSummarySQL  = `(?s)SELECT COALESCE\(SUM\(initial_usd\),0\), COALESCE\(SUM\(remaining_usd\),0\) FROM user_traffic_credits WHERE user_id=\$1 AND remaining_usd>0 AND expires_at>\$2`
	trafficPackExpirySQL   = `(?s)SELECT expires_at, COALESCE\(SUM\(remaining_usd\),0\) FROM user_traffic_credits WHERE user_id=\$1 AND remaining_usd>0 AND expires_at>\$2 GROUP BY expires_at ORDER BY expires_at LIMIT 1`
)

func TestTrafficPackCreditPurchaseRepaysTrafficDebtBeforeCreatingAvailableCredit(t *testing.T) {
	for _, tc := range []struct {
		name              string
		creditUSD         float64
		debtBefore        float64
		wantRemaining     float64
		wantDebtRepayment float64
		wantDebtAfter     float64
	}{
		{name: "full repayment leaves available credit", creditUSD: 10, debtBefore: 8, wantRemaining: 2, wantDebtRepayment: 8, wantDebtAfter: 0},
		{name: "partial repayment leaves debt outstanding", creditUSD: 5, debtBefore: 8, wantRemaining: 0, wantDebtRepayment: 5, wantDebtAfter: 3},
		{name: "no debt credits the full amount", creditUSD: 5, debtBefore: 0, wantRemaining: 5, wantDebtRepayment: 0, wantDebtAfter: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()

			creditedAt := time.Date(2026, time.August, 19, 12, 0, 0, 0, time.UTC)
			mock.ExpectBegin()
			mock.ExpectQuery(trafficPackUserLockSQL).
				WithArgs(int64(42)).
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
			mock.ExpectQuery(trafficPackDebtNetSQL).
				WithArgs(int64(42)).
				WillReturnRows(sqlmock.NewRows([]string{"debt"}).AddRow(tc.debtBefore))
			mock.ExpectQuery(trafficPackCreditSQL).
				WithArgs(int64(42), int64(88), int64(9), service.TrafficPackPlatformAll, tc.creditUSD, tc.wantRemaining, creditedAt, creditedAt.AddDate(0, 0, 28)).
				WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(101))
			mock.ExpectExec(trafficPackLedgerSQL).
				WithArgs(int64(42), int64(101), int64(88), service.TrafficCreditLedgerTypePurchase, tc.creditUSD, tc.wantRemaining, creditedAt).
				WillReturnResult(sqlmock.NewResult(1, 1))
			if tc.wantDebtRepayment > 0 {
				mock.ExpectExec(trafficPackRepaySQL).
					WithArgs(int64(42), tc.wantDebtRepayment, tc.wantDebtAfter, "order:88", creditedAt).
					WillReturnResult(sqlmock.NewResult(1, 1))
			}
			mock.ExpectCommit()

			repo := &trafficPackRepository{db: db}
			err = repo.CreditPurchase(ctx, service.CreditTrafficPackInput{
				UserID: 42, OrderID: 88, PackID: 9, CreditUSD: tc.creditUSD, ValidityDays: 28, CreditedAt: creditedAt,
			})
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestTrafficPackCreditPurchaseIsIdempotentAfterDebtLookup(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	creditedAt := time.Date(2026, time.August, 19, 12, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(trafficPackUserLockSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
	mock.ExpectQuery(trafficPackDebtNetSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"debt"}).AddRow(8.0))
	mock.ExpectQuery(trafficPackCreditSQL).
		WithArgs(int64(42), int64(88), int64(9), service.TrafficPackPlatformAll, 10.0, 2.0, creditedAt, creditedAt.AddDate(0, 0, 28)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectCommit()

	repo := &trafficPackRepository{db: db}
	err = repo.CreditPurchase(ctx, service.CreditTrafficPackInput{
		UserID: 42, OrderID: 88, PackID: 9, CreditUSD: 10, ValidityDays: 28, CreditedAt: creditedAt,
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTrafficPackSummarySubtractsOutstandingTrafficDebt(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	now := time.Date(2026, time.August, 19, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(trafficPackSummarySQL).
		WithArgs(int64(42), now).
		WillReturnRows(sqlmock.NewRows([]string{"initial", "remaining"}).AddRow(10.0, 3.0))
	mock.ExpectQuery(trafficPackDebtNetSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"debt"}).AddRow(5.0))
	mock.ExpectQuery(trafficPackExpirySQL).
		WithArgs(int64(42), now).
		WillReturnError(sql.ErrNoRows)

	summary, err := (&trafficPackRepository{db: db}).GetSummary(ctx, 42, now)
	require.NoError(t, err)
	require.InDelta(t, 10.0, summary.TotalInitialUSD, 0.000001)
	require.Zero(t, summary.TotalRemainingUSD)
	require.InDelta(t, 5.0, summary.TrafficDebtUSD, 0.000001)
	require.NoError(t, mock.ExpectationsWereMet())
}
