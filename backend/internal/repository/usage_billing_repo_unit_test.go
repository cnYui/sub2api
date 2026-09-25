//go:build unit

package repository

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	conditionalBalanceDeductSQL  = `(?s)WITH charged AS \(.*UPDATE users.*balance = balance - \$1,.*balance >= \$1.*\), frozen_rebate_consumed AS \(.*UPDATE user_affiliates.*aff_frozen_quota.*\).*SELECT charged\.balance FROM charged`
	overdraftBalanceDeductSQL    = `(?s)WITH charged AS \(.*UPDATE users.*balance = balance - \$1,.*deleted_at IS NULL.*\), frozen_rebate_consumed AS \(.*UPDATE user_affiliates.*aff_frozen_quota.*\).*SELECT charged\.balance FROM charged`
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
	apiKeyQuotaIncrementSQL      = `(?s)UPDATE api_keys\s+SET quota_used = quota_used \+ \$1,.*WHERE id = \$2 AND deleted_at IS NULL\s+RETURNING`
	apiKeyRateLimitIncrementSQL  = `(?s)UPDATE api_keys SET\s+usage_5h = .*WHERE id = \$2 AND deleted_at IS NULL`
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

	newBalance, sufficient, _, err := deductUsageBillingBalanceWithLedger(ctx, tx, 42, 2.5, "", 0)
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

	newBalance, sufficient, _, err := deductUsageBillingBalanceWithLedger(ctx, tx, 42, 10, "", 0)
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
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(userExistsForBillingSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(1))
	mock.ExpectQuery(lockedUsageBillingBalanceSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(0.0))
	// 0 余额先看流量卡；一张都没有时才透支，留给下一期套餐到账抵扣。
	mock.ExpectQuery(trafficCreditBatchesSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "order_id", "pack_id", "initial_usd", "remaining_usd", "credited_at", "expires_at",
		}))
	expectUsageBillingUserAndPackageLocks(mock, 42)
	mock.ExpectQuery(overdraftBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(-10.0))
	mock.ExpectCommit()

	result := &service.UsageBillingApplyResult{Applied: true}
	err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:      42,
		BalanceCost: 10,
	}, result)
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.InDelta(t, -10.0, *result.NewBalance, 0.000001)
	require.True(t, result.BalanceOverdrafted)
	require.False(t, result.TrafficCreditCharged)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUsageBillingEffectsDoesNotUseTrafficPackForPositiveBalance(t *testing.T) {
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
	mock.ExpectQuery(userExistsForBillingSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(1))
	mock.ExpectQuery(lockedUsageBillingBalanceSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(0.5))
	expectUsageBillingUserAndPackageLocks(mock, 42)
	mock.ExpectQuery(overdraftBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(-9.5))
	mock.ExpectCommit()

	result := &service.UsageBillingApplyResult{Applied: true}
	err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:      42,
		Platform:    service.PlatformAnthropic,
		BalanceCost: 10,
	}, result)
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.InDelta(t, -9.5, *result.NewBalance, 0.000001)
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
	mock.ExpectQuery(conditionalBalanceDeductSQL).
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

func expectTrafficPackCharge(mock sqlmock.Sqlmock, balance, cost, cardRemaining, charged float64, requestID string) {
	expectUsageBillingUserAndPackageLocks(mock, 42)
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(cost, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(userExistsForBillingSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(1))
	mock.ExpectQuery(lockedUsageBillingBalanceSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(balance))
	mock.ExpectQuery(trafficCreditBatchesSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "order_id", "pack_id", "initial_usd", "remaining_usd", "credited_at", "expires_at",
		}).AddRow(55, 42, nil, nil, 30.0, cardRemaining, time.Now().Add(-time.Hour), time.Now().Add(time.Hour)))
	mock.ExpectQuery(trafficCreditDeductSQL).
		WithArgs(charged, int64(55)).
		WillReturnRows(sqlmock.NewRows([]string{"remaining_usd"}).AddRow(cardRemaining - charged))
	mock.ExpectExec(trafficCreditLedgerSQL).
		WithArgs(int64(42), int64(55), requestID, service.TrafficCreditLedgerTypeDeduction, charged, cardRemaining-charged).
		WillReturnResult(sqlmock.NewResult(1, 1))
}

// 流量卡足额时，扣款按 10 位小数舍入，与原始费用只差不足一个精度单位；
// 这个尾差若被当成欠费写库，会违反 amount_usd > 0，整笔计费回滚、请求白用。
func TestApplyUsageBillingEffectsSkipsSubPrecisionTrafficDebt(t *testing.T) {
	cases := []struct {
		name    string
		cost    float64
		charged float64
	}{
		{name: "float noise above ledger precision", cost: math.Nextafter(0.3, 1), charged: 0.3},
		{name: "eleventh decimal rounds down", cost: 0.51663276004, charged: 0.51663276},
		{name: "eleventh decimal rounds up", cost: 0.51663276006, charged: 0.5166327601},
		{name: "cost below ledger precision", cost: 0.00000000004, charged: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()

			mock.ExpectBegin()
			tx, err := db.BeginTx(ctx, nil)
			require.NoError(t, err)
			if tc.charged > 0 {
				expectTrafficPackCharge(mock, -2.0, tc.cost, 5.716406078, tc.charged, "traffic-request")
			} else {
				expectUsageBillingUserAndPackageLocks(mock, 42)
				mock.ExpectQuery(conditionalBalanceDeductSQL).
					WithArgs(tc.cost, int64(42)).
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
					}).AddRow(55, 42, nil, nil, 30.0, 5.716406078, time.Now().Add(-time.Hour), time.Now().Add(time.Hour)))
			}
			mock.ExpectCommit()

			result := &service.UsageBillingApplyResult{Applied: true}
			err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
				UserID:      42,
				Platform:    service.PlatformOpenAI,
				RequestID:   "traffic-request",
				BalanceCost: tc.cost,
			}, result)
			require.NoError(t, err)
			require.True(t, result.TrafficCreditCharged)
			require.NoError(t, tx.Commit())
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestApplyUsageBillingEffectsRecordsTrafficDebtAtLedgerPrecision(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	expectTrafficPackCharge(mock, -2.0, 10.00000000004, 3.0, 3.0, "partial-request")
	mock.ExpectQuery(trafficDebtNetSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"debt"}).AddRow(1.25))
	mock.ExpectExec(trafficDebtLedgerSQL).
		WithArgs(int64(42), 7.0, 8.25, "partial-request").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result := &service.UsageBillingApplyResult{Applied: true}
	err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:      42,
		Platform:    service.PlatformOpenAI,
		RequestID:   "partial-request",
		BalanceCost: 10.00000000004,
	}, result)
	require.NoError(t, err)
	require.True(t, result.TrafficCreditCharged)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

// 余额恰为 0 又有流量卡时必须扣流量卡；旧逻辑把这笔整笔透支到余额，只买流量卡的用户永远抵不回来。
func TestApplyUsageBillingEffectsChargesTrafficPackAtZeroBalance(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	expectTrafficPackCharge(mock, 0, 10.0, 30.0, 10.0, "zero-balance-request")
	mock.ExpectCommit()

	result := &service.UsageBillingApplyResult{Applied: true}
	err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:      42,
		Platform:    service.PlatformOpenAI,
		RequestID:   "zero-balance-request",
		BalanceCost: 10,
	}, result)
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.Zero(t, *result.NewBalance)
	require.True(t, result.TrafficCreditCharged)
	require.False(t, result.BalanceOverdrafted)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUsageBillingEffectsRecordsTrafficDebtAtZeroBalanceWhenCardRunsShort(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	expectTrafficPackCharge(mock, 0, 10.0, 3.0, 3.0, "zero-balance-short")
	mock.ExpectQuery(trafficDebtNetSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"debt"}).AddRow(0.0))
	mock.ExpectExec(trafficDebtLedgerSQL).
		WithArgs(int64(42), 7.0, 7.0, "zero-balance-short").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	result := &service.UsageBillingApplyResult{Applied: true}
	err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:      42,
		Platform:    service.PlatformOpenAI,
		RequestID:   "zero-balance-short",
		BalanceCost: 10,
	}, result)
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.Zero(t, *result.NewBalance)
	require.True(t, result.TrafficCreditCharged)
	require.False(t, result.BalanceOverdrafted)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectSufficientBalanceCharge(mock sqlmock.Sqlmock, cost, balanceAfter float64) {
	expectUsageBillingUserAndPackageLocks(mock, 42)
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(cost, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(balanceAfter))
}

func TestApplyUsageBillingEffectsChargesBalanceWhenQuotaAPIKeyDeletedMidRequest(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	expectSufficientBalanceCharge(mock, 2.5, 7.5)
	// Key 已被软删，额度 UPDATE 命中 0 行；限速那一步随之跳过，不再发 SQL。
	mock.ExpectQuery(apiKeyQuotaIncrementSQL).
		WithArgs(2.5, int64(7), service.StatusAPIKeyActive, service.StatusAPIKeyQuotaExhausted).
		WillReturnRows(sqlmock.NewRows([]string{"exhausted"}))
	mock.ExpectCommit()

	result := &service.UsageBillingApplyResult{Applied: true}
	err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:              42,
		APIKeyID:            7,
		RequestID:           "deleted-quota-key",
		BalanceCost:         2.5,
		APIKeyQuotaCost:     2.5,
		APIKeyRateLimitCost: 2.5,
	}, result)
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.InDelta(t, 7.5, *result.NewBalance, 0.000001)
	require.False(t, result.APIKeyQuotaExhausted)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUsageBillingEffectsChargesBalanceWhenRateLimitedAPIKeyDeletedMidRequest(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	expectSufficientBalanceCharge(mock, 2.5, 7.5)
	mock.ExpectExec(apiKeyRateLimitIncrementSQL).
		WithArgs(2.5, int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	result := &service.UsageBillingApplyResult{Applied: true}
	err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:              42,
		APIKeyID:            7,
		RequestID:           "deleted-rate-limited-key",
		BalanceCost:         2.5,
		APIKeyRateLimitCost: 2.5,
	}, result)
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.InDelta(t, 7.5, *result.NewBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUsageBillingEffectsStillFailsOnAPIKeyUpdateError(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	expectSufficientBalanceCharge(mock, 2.5, 7.5)
	mock.ExpectQuery(apiKeyQuotaIncrementSQL).
		WithArgs(2.5, int64(7), service.StatusAPIKeyActive, service.StatusAPIKeyQuotaExhausted).
		WillReturnError(errors.New("connection reset"))
	mock.ExpectRollback()

	err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:          42,
		APIKeyID:        7,
		RequestID:       "key-update-error",
		BalanceCost:     2.5,
		APIKeyQuotaCost: 2.5,
	}, &service.UsageBillingApplyResult{Applied: true})
	require.ErrorContains(t, err, "connection reset")
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRecordTrafficCreditDebtIgnoresAmountBelowLedgerPrecision(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectCommit()

	require.NoError(t, recordTrafficCreditDebt(ctx, tx, 42, 0.00000000004, "tiny-request"))
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

	_, _, _, err = deductUsageBillingBalanceWithLedger(ctx, tx, 42, 10, "", 0)
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
