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
	conditionalBalanceDeductSQL  = `(?s)WITH charged AS \(.*UPDATE users.*balance = balance - \$1,.*balance >= \$1.*\), frozen_rebate_consumed AS \(.*UPDATE user_affiliates.*aff_frozen_quota.*\).*SELECT charged\.balance FROM charged`
	overdraftBalanceDeductSQL    = `(?s)WITH charged AS \(.*UPDATE users.*balance = balance - \$1,.*deleted_at IS NULL.*\), frozen_rebate_consumed AS \(.*UPDATE user_affiliates.*aff_frozen_quota.*\).*SELECT charged\.balance FROM charged`
	sufficientBalanceDeductSQL   = `(?s)UPDATE users\s+SET balance = balance - \$1, updated_at = NOW\(\)\s+WHERE id = \$2 AND deleted_at IS NULL AND balance >= \$1\s+RETURNING balance`
	reserveBatchImageHoldSQL     = `(?s)UPDATE users\s+SET balance = balance - \$1,\s+frozen_balance = COALESCE\(frozen_balance, 0\) \+ \$1,\s+updated_at = NOW\(\)\s+WHERE id = \$2 AND deleted_at IS NULL AND balance >= \$1\s+RETURNING balance, frozen_balance`
	captureBatchImageHoldSQL     = `(?s)UPDATE users\s+SET balance = balance\s+\+ CASE WHEN \$1 > \$2 THEN \$1 - \$2 ELSE 0 END\s+- CASE WHEN \$2 > \$1 THEN \$2 - \$1 ELSE 0 END,\s+frozen_balance = COALESCE\(frozen_balance, 0\) - \$1,\s+updated_at = NOW\(\)\s+WHERE id = \$3 AND deleted_at IS NULL AND COALESCE\(frozen_balance, 0\) >= \$1\s+RETURNING balance, frozen_balance`
	releaseBatchImageHoldSQL     = `(?s)UPDATE users\s+SET balance = balance \+ \$1,\s+frozen_balance = COALESCE\(frozen_balance, 0\) - \$1,\s+updated_at = NOW\(\)\s+WHERE id = \$2 AND deleted_at IS NULL AND COALESCE\(frozen_balance, 0\) >= \$1\s+RETURNING balance, frozen_balance`
	userExistsForBillingSQL      = `(?s)SELECT 1\s+FROM users\s+WHERE id = \$1 AND deleted_at IS NULL`
	trafficCreditBatchesSQL      = `(?s)SELECT id, user_id, order_id, pack_id, initial_usd, remaining_usd, credited_at, expires_at\s+FROM user_traffic_credits\s+WHERE user_id = \$1 AND remaining_usd > 0 AND expires_at > NOW\(\)`
	usageBillingUserLockSQL      = `(?s)SELECT id\s+FROM users\s+WHERE id = \$1 AND deleted_at IS NULL\s+FOR UPDATE`
	lockedUsageBillingBalanceSQL = `(?s)SELECT balance FROM users\s+WHERE id = \$1 AND deleted_at IS NULL`
	trafficCreditDeductSQL       = `(?s)UPDATE user_traffic_credits\s+SET remaining_usd = remaining_usd - \$1, updated_at = NOW\(\)\s+WHERE id = \$2 AND remaining_usd \+ 0\.0000000001 >= \$1\s+RETURNING remaining_usd`
	trafficCreditLedgerSQL       = `(?s)INSERT INTO traffic_credit_ledger \(user_id, credit_id, order_id, request_id, entry_type, amount_usd, balance_after_usd, created_at\)`
	trafficDebtNetSQL            = `(?s)SELECT COALESCE\(SUM\(CASE WHEN entry_type='debt' THEN amount_usd ELSE -amount_usd END\),0\) FROM traffic_credit_debt_ledger WHERE user_id=\$1`
	trafficDebtLedgerSQL         = `(?s)INSERT INTO traffic_credit_debt_ledger\(user_id,entry_type,amount_usd,balance_after_usd,source_type,source_ref,created_at\)`
	currentBalancePackageLockSQL = `(?s)SELECT id, remaining_usd\s+FROM user_balance_packages\s+WHERE user_id = \$1.*FOR UPDATE`
	consumeBalancePackageSQL     = `(?s)UPDATE user_balance_packages\s+SET remaining_usd = GREATEST\(remaining_usd - \$1, 0\), updated_at = NOW\(\)\s+WHERE id = \$2 AND remaining_usd > 0`
	recordBatchImageSourceSQL    = `(?s)UPDATE batch_image_jobs\s+SET balance_package_id = \$1,\s+balance_package_hold_usd = \$2,\s+updated_at = NOW\(\)\s+WHERE batch_id = \$3 AND user_id = \$4`
	loadBatchImageSourceSQL      = `(?s)SELECT COALESCE\(balance_package_id, 0\), balance_package_hold_usd\s+FROM batch_image_jobs\s+WHERE batch_id = \$1 AND user_id = \$2\s+FOR UPDATE`
	clearBatchImageSourceSQL     = `(?s)UPDATE batch_image_jobs\s+SET balance_package_id = NULL,\s+balance_package_hold_usd = 0,\s+updated_at = NOW\(\)\s+WHERE batch_id = \$1 AND user_id = \$2`
	restoreBalancePackageSQL     = `(?s)UPDATE user_balance_packages\s+SET remaining_usd = LEAST\(.*weekly_credit_usd.*remaining_usd.*SELECT balance FROM users.*\$3.*\).*WHERE id = \$2.*status IN \('active', 'completed', 'debt_paused'\).*expires_at > NOW\(\)`
	discardExpiredPackageSQL     = `(?s)UPDATE users\s+SET balance = balance - \$1, updated_at = NOW\(\)\s+WHERE id = \$2 AND deleted_at IS NULL\s+RETURNING balance, frozen_balance`
)

func expectUsageBillingUserAndPackageLocks(mock sqlmock.Sqlmock, userID int64) {
	mock.ExpectQuery(usageBillingUserLockSQL).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(userID))
	mock.ExpectQuery(currentBalancePackageLockSQL).
		WithArgs(userID).
		WillReturnError(sql.ErrNoRows)
}

func TestDeductUsageBillingBalance_UsesSufficientBalanceGuard(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	expectUsageBillingUserAndPackageLocks(mock, 42)
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(2.5, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(7.5))
	mock.ExpectCommit()

	newBalance, sufficient, err := deductUsageBillingBalance(ctx, tx, 42, 2.5)
	require.NoError(t, err)
	require.True(t, sufficient)
	require.InDelta(t, 7.5, newBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeductUsageBillingBalance_RecordsOverdraftWhenGuardMisses(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	expectUsageBillingUserAndPackageLocks(mock, 42)
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(overdraftBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(-5.0))
	mock.ExpectCommit()

	newBalance, sufficient, err := deductUsageBillingBalance(ctx, tx, 42, 10)
	require.NoError(t, err)
	require.False(t, sufficient)
	require.InDelta(t, -5.0, newBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUsageBillingEffects_FlagsBalanceOverdraft(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	expectUsageBillingUserAndPackageLocks(mock, 42)
	mock.ExpectQuery(sufficientBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(userExistsForBillingSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(1))
	mock.ExpectQuery(lockedUsageBillingBalanceSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(0.0))
	expectUsageBillingUserAndPackageLocks(mock, 42)
	mock.ExpectQuery(overdraftBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(-5.0))
	mock.ExpectCommit()

	result := &service.UsageBillingApplyResult{Applied: true}
	err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:      42,
		BalanceCost: 10,
	}, result)
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.InDelta(t, -5.0, *result.NewBalance, 0.000001)
	require.True(t, result.BalanceOverdrafted)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUsageBillingEffectsDoesNotUseTrafficPackForNonDebtBalance(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	expectUsageBillingUserAndPackageLocks(mock, 42)
	mock.ExpectQuery(sufficientBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(userExistsForBillingSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(1))
	mock.ExpectQuery(lockedUsageBillingBalanceSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(0.0))
	expectUsageBillingUserAndPackageLocks(mock, 42)
	mock.ExpectQuery(overdraftBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(-5.0))
	mock.ExpectCommit()

	result := &service.UsageBillingApplyResult{Applied: true}
	err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:      42,
		Platform:    service.PlatformAnthropic,
		BalanceCost: 10,
	}, result)
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.InDelta(t, -5.0, *result.NewBalance, 0.000001)
	require.True(t, result.BalanceOverdrafted)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUsageBillingEffectsUsesTrafficPackForDebtAcrossPlatforms(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	expectUsageBillingUserAndPackageLocks(mock, 42)
	mock.ExpectQuery(sufficientBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(userExistsForBillingSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(1))
	mock.ExpectQuery(lockedUsageBillingBalanceSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(-2.0))
	mock.ExpectQuery(trafficCreditBatchesSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "order_id", "pack_id", "initial_usd", "remaining_usd", "credited_at", "expires_at",
		}).AddRow(55, 42, nil, nil, 3.0, 3.0, time.Now().Add(-time.Hour), time.Now().Add(time.Hour)))
	mock.ExpectQuery(trafficCreditDeductSQL).
		WithArgs(3.0, int64(55)).
		WillReturnRows(sqlmock.NewRows([]string{"remaining_usd"}).AddRow(0.0))
	mock.ExpectExec(trafficCreditLedgerSQL).
		WithArgs(int64(42), int64(55), "anthropic-request", service.TrafficCreditLedgerTypeDeduction, 3.0, 0.0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(trafficDebtNetSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"debt"}).AddRow(0.0))
	mock.ExpectExec(trafficDebtLedgerSQL).
		WithArgs(int64(42), 7.0, 7.0, "anthropic-request").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result := &service.UsageBillingApplyResult{Applied: true}
	err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:      42,
		Platform:    service.PlatformAnthropic,
		RequestID:   "anthropic-request",
		BalanceCost: 10,
	}, result)
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.InDelta(t, -2.0, *result.NewBalance, 0.000001)
	require.True(t, result.TrafficCreditCharged)
	require.False(t, result.BalanceOverdrafted)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeductUsageBillingBalance_ReturnsUserNotFoundWhenNoUserUpdated(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(usageBillingUserLockSQL).
		WithArgs(int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	_, _, err = deductUsageBillingBalance(ctx, tx, 42, 10)
	require.ErrorIs(t, err, service.ErrUserNotFound)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveUsageBillingBatchImageBalance_MovesAvailableToFrozen(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(reserveBatchImageHoldSQL).
		WithArgs(2.5, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(7.5, 2.5))
	mock.ExpectCommit()

	result, err := reserveUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, HoldAmount: 2.5})
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.NotNil(t, result.FrozenBalance)
	require.InDelta(t, 7.5, *result.NewBalance, 0.000001)
	require.InDelta(t, 2.5, *result.FrozenBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveUsageBillingBatchImageBalance_TracksPackageSource(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(usageBillingUserLockSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
	mock.ExpectQuery(currentBalancePackageLockSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "remaining_usd"}).AddRow(99, 4.0))
	mock.ExpectQuery(reserveBatchImageHoldSQL).
		WithArgs(2.5, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(7.5, 2.5))
	mock.ExpectExec(consumeBalancePackageSQL).
		WithArgs(2.5, int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(recordBatchImageSourceSQL).
		WithArgs(int64(99), 2.5, "imgbatch_source", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := reserveUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, BatchID: "imgbatch_source", HoldAmount: 2.5})
	require.NoError(t, err)
	require.InDelta(t, 7.5, *result.NewBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveUsageBillingBatchImageBalance_InsufficientBalance(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(reserveBatchImageHoldSQL).
		WithArgs(10.0, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(userExistsForBillingSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"?column?"}).AddRow(1))
	mock.ExpectRollback()

	_, err = reserveUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, HoldAmount: 10})
	require.ErrorIs(t, err, service.ErrBatchImageInsufficientBalance)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCaptureUsageBillingBatchImageBalance_ReleasesRemainder(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(captureBatchImageHoldSQL).
		WithArgs(1.0, 0.25, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(9.75, 0.0))
	mock.ExpectCommit()

	result, err := captureUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, HoldAmount: 1, ActualAmount: 0.25})
	require.NoError(t, err)
	require.InDelta(t, 9.75, *result.NewBalance, 0.000001)
	require.InDelta(t, 0.0, *result.FrozenBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCaptureUsageBillingBatchImageBalance_RestoresUnusedPackageSource(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(loadBatchImageSourceSQL).
		WithArgs("imgbatch_capture_source", int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance_package_id", "balance_package_hold_usd"}).AddRow(99, 2.0))
	mock.ExpectQuery(captureBatchImageHoldSQL).
		WithArgs(2.0, 0.5, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(9.5, 0.0))
	mock.ExpectExec(restoreBalancePackageSQL).
		WithArgs(1.5, int64(99), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(clearBatchImageSourceSQL).
		WithArgs("imgbatch_capture_source", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := captureUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, BatchID: "imgbatch_capture_source", HoldAmount: 2, ActualAmount: 0.5})
	require.NoError(t, err)
	require.InDelta(t, 9.5, *result.NewBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCaptureUsageBillingBatchImageBalance_DoesNotRefundExpiredPackageToWallet(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(loadBatchImageSourceSQL).
		WithArgs("imgbatch_capture_expired_source", int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance_package_id", "balance_package_hold_usd"}).AddRow(99, 2.0))
	mock.ExpectQuery(captureBatchImageHoldSQL).
		WithArgs(2.0, 0.5, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(9.5, 0.0))
	mock.ExpectExec(restoreBalancePackageSQL).
		WithArgs(1.5, int64(99), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(discardExpiredPackageSQL).
		WithArgs(1.5, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(8.0, 0.0))
	mock.ExpectExec(clearBatchImageSourceSQL).
		WithArgs("imgbatch_capture_expired_source", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := captureUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{
		UserID: 42, BatchID: "imgbatch_capture_expired_source", HoldAmount: 2, ActualAmount: 0.5,
	})
	require.NoError(t, err)
	require.InDelta(t, 8.0, *result.NewBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCaptureUsageBillingBatchImageBalance_RejectsActualCostOverHold(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectRollback()

	_, err = captureUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, HoldAmount: 0.5, ActualAmount: 1})
	require.ErrorIs(t, err, service.ErrBatchImageSettlementCostExceedsHold)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseUsageBillingBatchImageBalance_ReturnsFrozenToAvailable(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(`SELECT 1\s+FROM usage_billing_dedup\s+WHERE request_id = \$1 AND api_key_id = \$2`).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_release"), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"?column?"}).AddRow(1))
	mock.ExpectQuery(loadBatchImageSourceSQL).
		WithArgs("imgbatch_release", int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance_package_id", "balance_package_hold_usd"}).AddRow(0, 0))
	mock.ExpectQuery(releaseBatchImageHoldSQL).
		WithArgs(1.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(10.0, 0.0))
	mock.ExpectExec(clearBatchImageSourceSQL).
		WithArgs("imgbatch_release", int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := releaseUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, APIKeyID: 7, BatchID: "imgbatch_release", HoldAmount: 1})
	require.NoError(t, err)
	require.InDelta(t, 10.0, *result.NewBalance, 0.000001)
	require.InDelta(t, 0.0, *result.FrozenBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseUsageBillingBatchImageBalance_SkipsWhenHoldNeverReserved(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	// dedup 与归档表均无 hold claim：说明该 job 从未成功冻结，
	// 释放必须跳过，不得从他人冻结资金池中凭空生成余额。
	mock.ExpectQuery(`SELECT 1\s+FROM usage_billing_dedup\s+WHERE request_id = \$1 AND api_key_id = \$2`).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_phantom"), int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT 1\s+FROM usage_billing_dedup_archive\s+WHERE request_id = \$1 AND api_key_id = \$2`).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_phantom"), int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectCommit()

	result, err := releaseUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, APIKeyID: 7, BatchID: "imgbatch_phantom", HoldAmount: 1})
	require.NoError(t, err)
	require.Nil(t, result.NewBalance)
	require.Nil(t, result.FrozenBalance)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}
