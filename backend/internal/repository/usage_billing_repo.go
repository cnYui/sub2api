package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type usageBillingRepository struct {
	db *sql.DB
}

func NewUsageBillingRepository(_ *dbent.Client, sqlDB *sql.DB) service.UsageBillingRepository {
	return &usageBillingRepository{db: sqlDB}
}

func (r *usageBillingRepository) Apply(ctx context.Context, cmd *service.UsageBillingCommand) (_ *service.UsageBillingApplyResult, err error) {
	if cmd == nil {
		return &service.UsageBillingApplyResult{}, nil
	}
	if r == nil || r.db == nil {
		return nil, errors.New("usage billing repository db is nil")
	}

	cmd.Normalize()
	if cmd.RequestID == "" {
		return nil, service.ErrUsageBillingRequestIDRequired
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	applied, err := r.claimUsageBillingKey(ctx, tx, cmd)
	if err != nil {
		return nil, err
	}
	if !applied {
		return &service.UsageBillingApplyResult{Applied: false}, nil
	}

	result := &service.UsageBillingApplyResult{Applied: true}
	if err := r.applyUsageBillingEffects(ctx, tx, cmd, result); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil
	return result, nil
}

func (r *usageBillingRepository) claimUsageBillingKey(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) (bool, error) {
	return r.claimUsageBillingRequest(ctx, tx, cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint)
}

func (r *usageBillingRepository) claimUsageBillingRequest(ctx context.Context, tx *sql.Tx, requestID string, apiKeyID int64, requestFingerprint string) (bool, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO usage_billing_dedup (request_id, api_key_id, request_fingerprint)
		VALUES ($1, $2, $3)
		ON CONFLICT (request_id, api_key_id) DO NOTHING
		RETURNING id
	`, requestID, apiKeyID, requestFingerprint).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		var existingFingerprint string
		if err := tx.QueryRowContext(ctx, `
			SELECT request_fingerprint
			FROM usage_billing_dedup
			WHERE request_id = $1 AND api_key_id = $2
		`, requestID, apiKeyID).Scan(&existingFingerprint); err != nil {
			return false, err
		}
		if strings.TrimSpace(existingFingerprint) != strings.TrimSpace(requestFingerprint) {
			return false, service.ErrUsageBillingRequestConflict
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var archivedFingerprint string
	err = tx.QueryRowContext(ctx, `
		SELECT request_fingerprint
		FROM usage_billing_dedup_archive
		WHERE request_id = $1 AND api_key_id = $2
	`, requestID, apiKeyID).Scan(&archivedFingerprint)
	if err == nil {
		if strings.TrimSpace(archivedFingerprint) != strings.TrimSpace(requestFingerprint) {
			return false, service.ErrUsageBillingRequestConflict
		}
		return false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	return true, nil
}

func (r *usageBillingRepository) ReserveBatchImageBalance(ctx context.Context, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	return r.applyBatchImageBalanceHold(ctx, cmd, reserveUsageBillingBatchImageBalance)
}

func (r *usageBillingRepository) CaptureBatchImageBalance(ctx context.Context, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	return r.applyBatchImageBalanceHold(ctx, cmd, captureUsageBillingBatchImageBalance)
}

func (r *usageBillingRepository) ReleaseBatchImageBalance(ctx context.Context, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	return r.applyBatchImageBalanceHold(ctx, cmd, releaseUsageBillingBatchImageBalance)
}

func (r *usageBillingRepository) applyBatchImageBalanceHold(
	ctx context.Context,
	cmd *service.BatchImageBalanceHoldCommand,
	apply func(context.Context, *sql.Tx, *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error),
) (_ *service.BatchImageBalanceHoldResult, err error) {
	if cmd == nil {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	if r == nil || r.db == nil {
		return nil, errors.New("usage billing repository db is nil")
	}
	cmd.Normalize()
	if cmd.RequestID == "" {
		return nil, service.ErrUsageBillingRequestIDRequired
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	applied, err := r.claimUsageBillingRequest(ctx, tx, cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint)
	if err != nil {
		return nil, err
	}
	if !applied {
		return &service.BatchImageBalanceHoldResult{Applied: false}, nil
	}

	result, err := apply(ctx, tx, cmd)
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = &service.BatchImageBalanceHoldResult{}
	}
	result.Applied = true

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil
	return result, nil
}

func (r *usageBillingRepository) applyUsageBillingEffects(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, result *service.UsageBillingApplyResult) error {
	if cmd.SubscriptionCost > 0 && cmd.SubscriptionID != nil {
		if err := incrementUsageBillingSubscription(ctx, tx, *cmd.SubscriptionID, cmd.SubscriptionCost); err != nil {
			return err
		}
	}

	if cmd.BalanceCost > 0 {
		newBalance, sufficient, err := deductUsageBillingBalanceIfSufficient(ctx, tx, cmd.UserID, cmd.BalanceCost)
		if err != nil {
			return err
		}
		if sufficient {
			result.NewBalance = &newBalance
		} else {
			balanceBefore, balanceErr := lockedUsageBillingBalance(ctx, tx, cmd.UserID)
			if balanceErr != nil {
				return balanceErr
			}
			if balanceBefore < 0 {
				trafficCharged, trafficErr := deductUsageBillingTrafficPack(ctx, tx, cmd.UserID, cmd.BalanceCost, cmd.RequestID, true)
				if trafficErr != nil {
					return trafficErr
				}
				trafficDebt := cmd.BalanceCost - trafficCharged
				if trafficDebt > 0 {
					if err := recordTrafficCreditDebt(ctx, tx, cmd.UserID, trafficDebt, cmd.RequestID); err != nil {
						return err
					}
				}
				result.NewBalance = &balanceBefore
				result.TrafficCreditCharged = true
			} else {
				covered, trafficErr := deductUsageBillingTrafficPack(ctx, tx, cmd.UserID, cmd.BalanceCost, cmd.RequestID, false)
				if trafficErr != nil {
					return trafficErr
				}
				if covered >= cmd.BalanceCost-0.0000000001 {
					result.NewBalance = &balanceBefore
					result.TrafficCreditCharged = true
				} else {
					newBalance, _, err = deductUsageBillingBalance(ctx, tx, cmd.UserID, cmd.BalanceCost)
					if err != nil {
						return err
					}
					result.NewBalance = &newBalance
					result.BalanceOverdrafted = true
				}
			}
		}
	}

	if cmd.APIKeyQuotaCost > 0 {
		exhausted, err := incrementUsageBillingAPIKeyQuota(ctx, tx, cmd.APIKeyID, cmd.APIKeyQuotaCost)
		if err != nil {
			return err
		}
		result.APIKeyQuotaExhausted = exhausted
	}

	if cmd.APIKeyRateLimitCost > 0 {
		if err := incrementUsageBillingAPIKeyRateLimit(ctx, tx, cmd.APIKeyID, cmd.APIKeyRateLimitCost); err != nil {
			return err
		}
	}

	if cmd.AccountQuotaCost > 0 && (strings.EqualFold(cmd.AccountType, service.AccountTypeAPIKey) || strings.EqualFold(cmd.AccountType, service.AccountTypeBedrock)) {
		quotaState, err := incrementUsageBillingAccountQuota(ctx, tx, cmd.AccountID, cmd.AccountQuotaCost)
		if err != nil {
			return err
		}
		result.QuotaState = quotaState
	}

	return nil
}

func deductUsageBillingBalanceIfSufficient(ctx context.Context, tx *sql.Tx, userID int64, amount float64) (float64, bool, error) {
	if err := lockUsageBillingUser(ctx, tx, userID); err != nil {
		return 0, false, err
	}
	packageID, packageRemaining, err := lockCurrentBalancePackage(ctx, tx, userID)
	if err != nil {
		return 0, false, err
	}
	var balance float64
	err = tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance - $1, updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL AND balance >= $1
		RETURNING balance
	`, amount, userID).Scan(&balance)
	if errors.Is(err, sql.ErrNoRows) {
		if exists, existsErr := userExistsForBilling(ctx, tx, userID); existsErr != nil {
			return 0, false, existsErr
		} else if !exists {
			return 0, false, service.ErrUserNotFound
		}
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if packageID > 0 {
		if err := consumeCurrentBalancePackage(ctx, tx, packageID, minFloat(amount, packageRemaining)); err != nil {
			return 0, false, err
		}
	}
	return balance, true, nil
}

func lockUsageBillingUser(ctx context.Context, tx *sql.Tx, userID int64) error {
	var id int64
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, userID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrUserNotFound
	}
	return err
}

func lockedUsageBillingBalance(ctx context.Context, tx *sql.Tx, userID int64) (float64, error) {
	var balance float64
	err := tx.QueryRowContext(ctx, `SELECT balance FROM users WHERE id = $1 AND deleted_at IS NULL`, userID).Scan(&balance)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, service.ErrUserNotFound
	}
	return balance, err
}

func lockCurrentBalancePackage(ctx context.Context, tx *sql.Tx, userID int64) (int64, float64, error) {
	var packageID int64
	var remaining float64
	err := tx.QueryRowContext(ctx, `
		SELECT id, remaining_usd
		FROM user_balance_packages
		WHERE user_id = $1
		  AND status IN ('active', 'completed')
		  AND expires_at > NOW()
		  AND remaining_usd > 0
		ORDER BY expires_at ASC, created_at ASC, id ASC
		LIMIT 1
		FOR UPDATE
	`, userID).Scan(&packageID, &remaining)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	return packageID, remaining, nil
}

func consumeCurrentBalancePackage(ctx context.Context, tx *sql.Tx, packageID int64, amount float64) error {
	if packageID <= 0 || amount <= 0 {
		return nil
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE user_balance_packages
		SET remaining_usd = GREATEST(remaining_usd - $1, 0), updated_at = NOW()
		WHERE id = $2 AND remaining_usd > 0
	`, amount, packageID)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected == 0 {
		return errors.New("balance package remaining quota changed concurrently")
	}
	return nil
}

func deductUsageBillingTrafficPack(ctx context.Context, tx *sql.Tx, userID int64, amountUSD float64, requestID string, allowPartial bool) (float64, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, user_id, order_id, pack_id, initial_usd, remaining_usd, credited_at, expires_at
		FROM user_traffic_credits
		WHERE user_id = $1 AND remaining_usd > 0 AND expires_at > NOW()
		ORDER BY expires_at ASC, credited_at ASC, id ASC
		FOR UPDATE
	`, userID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	batches := []service.TrafficCreditBatch{}
	for rows.Next() {
		var batch service.TrafficCreditBatch
		var orderID, packID sql.NullInt64
		if err := rows.Scan(&batch.ID, &batch.UserID, &orderID, &packID, &batch.InitialUSD, &batch.RemainingUSD, &batch.CreditedAt, &batch.ExpiresAt); err != nil {
			return 0, err
		}
		if orderID.Valid {
			batch.OrderID = &orderID.Int64
		}
		if packID.Valid {
			batch.PackID = &packID.Int64
		}
		batches = append(batches, batch)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	deductions, covered := service.PlanTrafficCreditDeductions(batches, amountUSD)
	if !covered && !allowPartial {
		return 0, nil
	}
	charged := 0.0
	for _, deduction := range deductions {
		var balanceAfter float64
		if err := tx.QueryRowContext(ctx, `
			UPDATE user_traffic_credits
			SET remaining_usd = remaining_usd - $1, updated_at = NOW()
			WHERE id = $2 AND remaining_usd + 0.0000000001 >= $1
			RETURNING remaining_usd
		`, deduction.AmountUSD, deduction.CreditID).Scan(&balanceAfter); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO traffic_credit_ledger (user_id, credit_id, order_id, request_id, entry_type, amount_usd, balance_after_usd, created_at)
			VALUES ($1, $2, NULL, $3, $4, $5, $6, NOW())
		`, userID, deduction.CreditID, requestID, service.TrafficCreditLedgerTypeDeduction, deduction.AmountUSD, balanceAfter); err != nil {
			return 0, err
		}
		charged += deduction.AmountUSD
	}
	return charged, nil
}

func recordTrafficCreditDebt(ctx context.Context, tx *sql.Tx, userID int64, amountUSD float64, requestID string) error {
	if amountUSD <= 0 {
		return nil
	}
	var debtBefore float64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN entry_type='debt' THEN amount_usd ELSE -amount_usd END),0) FROM traffic_credit_debt_ledger WHERE user_id=$1`, userID).Scan(&debtBefore); err != nil {
		return err
	}
	if debtBefore < 0 {
		debtBefore = 0
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO traffic_credit_debt_ledger(user_id,entry_type,amount_usd,balance_after_usd,source_type,source_ref,created_at) VALUES($1,'debt',$2,$3,'usage_billing',$4,NOW())`, userID, amountUSD, debtBefore+amountUSD, requestID)
	return err
}

func incrementUsageBillingSubscription(ctx context.Context, tx *sql.Tx, subscriptionID int64, costUSD float64) error {
	const updateSQL = `
		UPDATE user_subscriptions us
		SET
			daily_usage_usd = us.daily_usage_usd + $1,
			weekly_usage_usd = us.weekly_usage_usd + $1,
			monthly_usage_usd = us.monthly_usage_usd + $1,
			updated_at = NOW()
		FROM groups g
		WHERE us.id = $2
			AND us.deleted_at IS NULL
			AND us.group_id = g.id
			AND g.deleted_at IS NULL
	`
	res, err := tx.ExecContext(ctx, updateSQL, costUSD, subscriptionID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}
	return service.ErrSubscriptionNotFound
}

func deductUsageBillingBalance(ctx context.Context, tx *sql.Tx, userID int64, amount float64) (float64, bool, error) {
	if err := lockUsageBillingUser(ctx, tx, userID); err != nil {
		return 0, false, err
	}
	packageID, packageRemaining, err := lockCurrentBalancePackage(ctx, tx, userID)
	if err != nil {
		return 0, false, err
	}
	var newBalance float64
	err = tx.QueryRowContext(ctx, `
		WITH charged AS (
			UPDATE users
			SET balance = balance - $1,
				updated_at = NOW()
			WHERE id = $2 AND deleted_at IS NULL AND balance >= $1
			RETURNING id, balance, balance + $1 AS balance_before
		), frozen_rebate_consumed AS (
			UPDATE user_affiliates ua
			SET aff_frozen_quota = GREATEST(
					ua.aff_frozen_quota - LEAST(
						ua.aff_frozen_quota,
						GREATEST($1 - GREATEST(charged.balance_before - ua.aff_frozen_quota, 0), 0)
					),
					0
				),
				updated_at = NOW()
			FROM charged
			WHERE ua.user_id = charged.id AND ua.aff_frozen_quota > 0
			RETURNING ua.user_id
		)
		SELECT charged.balance FROM charged
		LEFT JOIN frozen_rebate_consumed ON TRUE
	`, amount, userID).Scan(&newBalance)
	if err == nil {
		if err := consumeCurrentBalancePackage(ctx, tx, packageID, minFloat(amount, packageRemaining)); err != nil {
			return 0, false, err
		}
		return newBalance, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}

	err = tx.QueryRowContext(ctx, `
		WITH charged AS (
			UPDATE users
			SET balance = balance - $1,
				updated_at = NOW()
			WHERE id = $2 AND deleted_at IS NULL
			RETURNING id, balance, balance + $1 AS balance_before
		), frozen_rebate_consumed AS (
			UPDATE user_affiliates ua
			SET aff_frozen_quota = GREATEST(
					ua.aff_frozen_quota - LEAST(
						ua.aff_frozen_quota,
						GREATEST($1 - GREATEST(charged.balance_before - ua.aff_frozen_quota, 0), 0)
					),
					0
				),
				updated_at = NOW()
			FROM charged
			WHERE ua.user_id = charged.id AND ua.aff_frozen_quota > 0
			RETURNING ua.user_id
		)
		SELECT charged.balance FROM charged
		LEFT JOIN frozen_rebate_consumed ON TRUE
	`, amount, userID).Scan(&newBalance)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, service.ErrUserNotFound
	}
	if err != nil {
		return 0, false, err
	}
	availableBeforeOverdraft := maxFloat(newBalance+amount, 0)
	if err := consumeCurrentBalancePackage(ctx, tx, packageID, minFloat(packageRemaining, availableBeforeOverdraft)); err != nil {
		return 0, false, err
	}
	return newBalance, false, nil
}

func minFloat(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

func maxFloat(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}

func reserveUsageBillingBatchImageBalance(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	if cmd.HoldAmount <= 0 {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	packageID := int64(0)
	packageHold := 0.0
	if strings.TrimSpace(cmd.BatchID) != "" {
		if err := lockUsageBillingUser(ctx, tx, cmd.UserID); err != nil {
			return nil, err
		}
		var packageRemaining float64
		var err error
		packageID, packageRemaining, err = lockCurrentBalancePackage(ctx, tx, cmd.UserID)
		if err != nil {
			return nil, err
		}
		packageHold = minFloat(cmd.HoldAmount, packageRemaining)
	}
	var balance, frozen float64
	err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance - $1,
			frozen_balance = COALESCE(frozen_balance, 0) + $1,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL AND balance >= $1
		RETURNING balance, frozen_balance
	`, cmd.HoldAmount, cmd.UserID).Scan(&balance, &frozen)
	if err == nil {
		if packageID > 0 && packageHold > 0 {
			if err := consumeCurrentBalancePackage(ctx, tx, packageID, packageHold); err != nil {
				return nil, err
			}
		}
		if strings.TrimSpace(cmd.BatchID) != "" {
			if err := recordBatchImageBalancePackageSource(ctx, tx, cmd.BatchID, cmd.UserID, packageID, packageHold); err != nil {
				return nil, err
			}
		}
		return &service.BatchImageBalanceHoldResult{NewBalance: &balance, FrozenBalance: &frozen}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if exists, existsErr := userExistsForBilling(ctx, tx, cmd.UserID); existsErr != nil {
		return nil, existsErr
	} else if !exists {
		return nil, service.ErrUserNotFound
	}
	return nil, service.ErrBatchImageInsufficientBalance
}

func captureUsageBillingBatchImageBalance(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	if cmd.HoldAmount <= 0 && cmd.ActualAmount <= 0 {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	if cmd.ActualAmount-cmd.HoldAmount > 0.00000001 {
		return nil, service.ErrBatchImageSettlementCostExceedsHold
	}
	source, err := lockBatchImageBalancePackageSource(ctx, tx, cmd.BatchID, cmd.UserID)
	if err != nil {
		return nil, err
	}
	var balance, frozen float64
	err = tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance
				+ CASE WHEN $1 > $2 THEN $1 - $2 ELSE 0 END
				- CASE WHEN $2 > $1 THEN $2 - $1 ELSE 0 END,
			frozen_balance = COALESCE(frozen_balance, 0) - $1,
			updated_at = NOW()
		WHERE id = $3 AND deleted_at IS NULL AND COALESCE(frozen_balance, 0) >= $1
		RETURNING balance, frozen_balance
	`, cmd.HoldAmount, cmd.ActualAmount, cmd.UserID).Scan(&balance, &frozen)
	if err == nil {
		packageConsumed := minFloat(cmd.ActualAmount, source.holdUSD)
		packageUnused := source.holdUSD - packageConsumed
		restored, err := restoreBatchImageBalancePackageSource(ctx, tx, source, cmd.UserID, packageUnused)
		if err != nil {
			return nil, err
		}
		if !restored {
			balance, frozen, err = discardExpiredBatchImagePackageRefund(ctx, tx, cmd.UserID, packageUnused)
			if err != nil {
				return nil, err
			}
		}
		if err := clearBatchImageBalancePackageSource(ctx, tx, cmd.BatchID, cmd.UserID); err != nil {
			return nil, err
		}
		return &service.BatchImageBalanceHoldResult{NewBalance: &balance, FrozenBalance: &frozen}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if exists, existsErr := userExistsForBilling(ctx, tx, cmd.UserID); existsErr != nil {
		return nil, existsErr
	} else if !exists {
		return nil, service.ErrUserNotFound
	}
	return nil, errors.New("batch image frozen balance is insufficient")
}

func releaseUsageBillingBatchImageBalance(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	if cmd.HoldAmount <= 0 {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	// 释放前校验该 job 确实预留过 hold（hold request id 已被 claim），
	// 防止从未成功冻结的 job 触发"幻影释放"，从其他用户的冻结资金池中凭空生成余额。
	held, heldErr := batchImageHoldClaimExists(ctx, tx, service.BatchImageHoldRequestID(cmd.BatchID), cmd.APIKeyID)
	if heldErr != nil {
		return nil, heldErr
	}
	if !held {
		logger.LegacyPrintf("repository.usage_billing", "[BatchImage] release skipped, hold was never reserved: batch=%s", cmd.BatchID)
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	source, err := lockBatchImageBalancePackageSource(ctx, tx, cmd.BatchID, cmd.UserID)
	if err != nil {
		return nil, err
	}
	var balance, frozen float64
	err = tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance + $1,
			frozen_balance = COALESCE(frozen_balance, 0) - $1,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL AND COALESCE(frozen_balance, 0) >= $1
		RETURNING balance, frozen_balance
	`, cmd.HoldAmount, cmd.UserID).Scan(&balance, &frozen)
	if err == nil {
		restored, err := restoreBatchImageBalancePackageSource(ctx, tx, source, cmd.UserID, source.holdUSD)
		if err != nil {
			return nil, err
		}
		if !restored {
			balance, frozen, err = discardExpiredBatchImagePackageRefund(ctx, tx, cmd.UserID, source.holdUSD)
			if err != nil {
				return nil, err
			}
		}
		if err := clearBatchImageBalancePackageSource(ctx, tx, cmd.BatchID, cmd.UserID); err != nil {
			return nil, err
		}
		return &service.BatchImageBalanceHoldResult{NewBalance: &balance, FrozenBalance: &frozen}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if exists, existsErr := userExistsForBilling(ctx, tx, cmd.UserID); existsErr != nil {
		return nil, existsErr
	} else if !exists {
		return nil, service.ErrUserNotFound
	}
	return nil, errors.New("batch image frozen balance is insufficient")
}

type batchImageBalancePackageSource struct {
	packageID int64
	holdUSD   float64
}

func recordBatchImageBalancePackageSource(ctx context.Context, tx *sql.Tx, batchID string, userID, packageID int64, holdUSD float64) error {
	var packageValue any
	if packageID > 0 {
		packageValue = packageID
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE batch_image_jobs
		SET balance_package_id = $1,
			balance_package_hold_usd = $2,
			updated_at = NOW()
		WHERE batch_id = $3 AND user_id = $4
	`, packageValue, holdUSD, strings.TrimSpace(batchID), userID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return errors.New("batch image job is missing while recording balance package source")
	}
	return nil
}

func lockBatchImageBalancePackageSource(ctx context.Context, tx *sql.Tx, batchID string, userID int64) (batchImageBalancePackageSource, error) {
	if strings.TrimSpace(batchID) == "" {
		return batchImageBalancePackageSource{}, nil
	}
	var source batchImageBalancePackageSource
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(balance_package_id, 0), balance_package_hold_usd
		FROM batch_image_jobs
		WHERE batch_id = $1 AND user_id = $2
		FOR UPDATE
	`, strings.TrimSpace(batchID), userID).Scan(&source.packageID, &source.holdUSD)
	if errors.Is(err, sql.ErrNoRows) {
		return source, errors.New("batch image job is missing while loading balance package source")
	}
	return source, err
}

func restoreBatchImageBalancePackageSource(ctx context.Context, tx *sql.Tx, source batchImageBalancePackageSource, userID int64, amount float64) (bool, error) {
	if source.packageID <= 0 || amount <= 0 {
		return true, nil
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE user_balance_packages
		SET remaining_usd = LEAST(
				weekly_credit_usd,
				remaining_usd + LEAST(
					$1,
					GREATEST((SELECT balance FROM users WHERE id = $3 AND deleted_at IS NULL) - remaining_usd, 0)
				)
			),
			updated_at = NOW()
		WHERE id = $2
		  AND status IN ('active', 'completed', 'debt_paused')
		  AND expires_at > NOW()
	`, amount, source.packageID, userID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

func discardExpiredBatchImagePackageRefund(ctx context.Context, tx *sql.Tx, userID int64, amount float64) (float64, float64, error) {
	if amount <= 0 {
		return 0, 0, nil
	}
	var balance, frozen float64
	err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance - $1, updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING balance, frozen_balance
	`, amount, userID).Scan(&balance, &frozen)
	return balance, frozen, err
}

func clearBatchImageBalancePackageSource(ctx context.Context, tx *sql.Tx, batchID string, userID int64) error {
	if strings.TrimSpace(batchID) == "" {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE batch_image_jobs
		SET balance_package_id = NULL,
			balance_package_hold_usd = 0,
			updated_at = NOW()
		WHERE batch_id = $1 AND user_id = $2
	`, strings.TrimSpace(batchID), userID)
	return err
}

// batchImageHoldClaimExists 检查 hold request id 是否已在 dedup（或归档）表中被 claim，
// 即该 batch 的冻结操作确实成功提交过。
func batchImageHoldClaimExists(ctx context.Context, tx *sql.Tx, holdRequestID string, apiKeyID int64) (bool, error) {
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM usage_billing_dedup
		WHERE request_id = $1 AND api_key_id = $2
	`, holdRequestID, apiKeyID).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	err = tx.QueryRowContext(ctx, `
		SELECT 1
		FROM usage_billing_dedup_archive
		WHERE request_id = $1 AND api_key_id = $2
	`, holdRequestID, apiKeyID).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

func userExistsForBilling(ctx context.Context, tx *sql.Tx, userID int64) (bool, error) {
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, userID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func incrementUsageBillingAPIKeyQuota(ctx context.Context, tx *sql.Tx, apiKeyID int64, amount float64) (bool, error) {
	var exhausted bool
	err := tx.QueryRowContext(ctx, `
		UPDATE api_keys
		SET quota_used = quota_used + $1,
			status = CASE
				WHEN quota > 0
					AND status = $3
					AND quota_used < quota
					AND quota_used + $1 >= quota
				THEN $4
				ELSE status
			END,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING quota > 0 AND quota_used >= quota AND quota_used - $1 < quota
	`, amount, apiKeyID, service.StatusAPIKeyActive, service.StatusAPIKeyQuotaExhausted).Scan(&exhausted)
	if errors.Is(err, sql.ErrNoRows) {
		return false, service.ErrAPIKeyNotFound
	}
	if err != nil {
		return false, err
	}
	return exhausted, nil
}

func incrementUsageBillingAPIKeyRateLimit(ctx context.Context, tx *sql.Tx, apiKeyID int64, cost float64) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE api_keys SET
			usage_5h = CASE WHEN window_5h_start IS NOT NULL AND window_5h_start + INTERVAL '5 hours' <= NOW() THEN $1 ELSE usage_5h + $1 END,
			usage_1d = CASE WHEN window_1d_start IS NOT NULL AND window_1d_start + INTERVAL '24 hours' <= NOW() THEN $1 ELSE usage_1d + $1 END,
			usage_7d = CASE WHEN window_7d_start IS NOT NULL AND window_7d_start + INTERVAL '7 days' <= NOW() THEN $1 ELSE usage_7d + $1 END,
			window_5h_start = CASE WHEN window_5h_start IS NULL OR window_5h_start + INTERVAL '5 hours' <= NOW() THEN NOW() ELSE window_5h_start END,
			window_1d_start = CASE WHEN window_1d_start IS NULL OR window_1d_start + INTERVAL '24 hours' <= NOW() THEN date_trunc('day', NOW()) ELSE window_1d_start END,
			window_7d_start = CASE WHEN window_7d_start IS NULL OR window_7d_start + INTERVAL '7 days' <= NOW() THEN date_trunc('day', NOW()) ELSE window_7d_start END,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
	`, cost, apiKeyID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrAPIKeyNotFound
	}
	return nil
}

func incrementUsageBillingAccountQuota(ctx context.Context, tx *sql.Tx, accountID int64, amount float64) (*service.AccountQuotaState, error) {
	rows, err := tx.QueryContext(ctx,
		`UPDATE accounts SET extra = (
			COALESCE(extra, '{}'::jsonb)
			|| jsonb_build_object('quota_used', COALESCE((extra->>'quota_used')::numeric, 0) + $1)
			|| CASE WHEN COALESCE((extra->>'quota_daily_limit')::numeric, 0) > 0 THEN
				jsonb_build_object(
					'quota_daily_used',
					CASE WHEN `+dailyExpiredExpr+`
					THEN $1
					ELSE COALESCE((extra->>'quota_daily_used')::numeric, 0) + $1 END,
					'quota_daily_start',
					CASE WHEN `+dailyExpiredExpr+`
					THEN `+nowUTC+`
					ELSE COALESCE(extra->>'quota_daily_start', `+nowUTC+`) END
				)
				|| CASE WHEN `+dailyExpiredExpr+` AND `+nextDailyResetAtExpr+` IS NOT NULL
				   THEN jsonb_build_object('quota_daily_reset_at', `+nextDailyResetAtExpr+`)
				   ELSE '{}'::jsonb END
			ELSE '{}'::jsonb END
			|| CASE WHEN COALESCE((extra->>'quota_weekly_limit')::numeric, 0) > 0 THEN
				jsonb_build_object(
					'quota_weekly_used',
					CASE WHEN `+weeklyExpiredExpr+`
					THEN $1
					ELSE COALESCE((extra->>'quota_weekly_used')::numeric, 0) + $1 END,
					'quota_weekly_start',
					CASE WHEN `+weeklyExpiredExpr+`
					THEN `+nowUTC+`
					ELSE COALESCE(extra->>'quota_weekly_start', `+nowUTC+`) END
				)
				|| CASE WHEN `+weeklyExpiredExpr+` AND `+nextWeeklyResetAtExpr+` IS NOT NULL
				   THEN jsonb_build_object('quota_weekly_reset_at', `+nextWeeklyResetAtExpr+`)
				   ELSE '{}'::jsonb END
			ELSE '{}'::jsonb END
		), updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING
			COALESCE((extra->>'quota_used')::numeric, 0),
			COALESCE((extra->>'quota_limit')::numeric, 0),
			COALESCE((extra->>'quota_daily_used')::numeric, 0),
			COALESCE((extra->>'quota_daily_limit')::numeric, 0),
			COALESCE((extra->>'quota_weekly_used')::numeric, 0),
			COALESCE((extra->>'quota_weekly_limit')::numeric, 0)`,
		amount, accountID)
	if err != nil {
		return nil, err
	}

	var state service.AccountQuotaState
	if rows.Next() {
		if err := rows.Scan(
			&state.TotalUsed, &state.TotalLimit,
			&state.DailyUsed, &state.DailyLimit,
			&state.WeeklyUsed, &state.WeeklyLimit,
		); err != nil {
			_ = rows.Close()
			return nil, err
		}
	} else {
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
		return nil, service.ErrAccountNotFound
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	// 必须在执行下一条 SQL 前显式关闭 rows：pq 驱动在同一连接上
	// 不允许前一条查询的结果集未耗尽时启动新查询，否则会返回
	// "unexpected Parse response" 错误。
	if err := rows.Close(); err != nil {
		return nil, err
	}
	// 任意维度额度在本次递增中从"未超"跨越到"已超"时，必须刷新调度快照，
	// 否则 Redis 中缓存的 Account 仍显示旧的 used 值，后续请求会继续选中本账号，
	// 最终观察到 daily_used / weekly_used 大幅超过配置的 limit。
	// 对于日/周额度，即使本次触发了周期重置（pre=0、post=amount），
	// 判定式 (post-amount) < limit 同样成立，逻辑与总额度保持一致。
	crossedTotal := state.TotalLimit > 0 && state.TotalUsed >= state.TotalLimit && (state.TotalUsed-amount) < state.TotalLimit
	crossedDaily := state.DailyLimit > 0 && state.DailyUsed >= state.DailyLimit && (state.DailyUsed-amount) < state.DailyLimit
	crossedWeekly := state.WeeklyLimit > 0 && state.WeeklyUsed >= state.WeeklyLimit && (state.WeeklyUsed-amount) < state.WeeklyLimit
	if crossedTotal || crossedDaily || crossedWeekly {
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil); err != nil {
			logger.LegacyPrintf("repository.usage_billing", "[SchedulerOutbox] enqueue quota exceeded failed: account=%d err=%v", accountID, err)
			return nil, err
		}
	}
	return &state, nil
}
