package repository

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type trafficPackRepository struct {
	db         *sql.DB
	isPostgres bool
	policy     service.TrafficCreditPolicy
}

func NewTrafficPackRepository(db *sql.DB, policies ...service.TrafficCreditPolicy) service.TrafficPackRepository {
	return &trafficPackRepository{db: db, isPostgres: isPostgresDriver(db), policy: firstTrafficCreditPolicy(policies)}
}

func (r *trafficPackRepository) ListForSale(ctx context.Context) ([]service.TrafficPack, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, code, name, description, price, credit_usd, validity_days, platform, for_sale, sort_order
		FROM traffic_packs
		WHERE for_sale = TRUE
		ORDER BY sort_order ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	packs := []service.TrafficPack{}
	for rows.Next() {
		var pack service.TrafficPack
		if err := rows.Scan(&pack.ID, &pack.Code, &pack.Name, &pack.Description, &pack.Price, &pack.CreditUSD, &pack.ValidityDays, &pack.Platform, &pack.ForSale, &pack.SortOrder); err != nil {
			return nil, err
		}
		packs = append(packs, pack)
	}
	return packs, rows.Err()
}

func (r *trafficPackRepository) GetForSaleByID(ctx context.Context, id int64) (*service.TrafficPack, error) {
	var pack service.TrafficPack
	err := r.db.QueryRowContext(ctx, `
		SELECT id, code, name, description, price, credit_usd, validity_days, platform, for_sale, sort_order
		FROM traffic_packs
		WHERE id = $1 AND for_sale = TRUE
	`, id).Scan(&pack.ID, &pack.Code, &pack.Name, &pack.Description, &pack.Price, &pack.CreditUSD, &pack.ValidityDays, &pack.Platform, &pack.ForSale, &pack.SortOrder)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrInvalidInput
	}
	if err != nil {
		return nil, err
	}
	return &pack, nil
}

func (r *trafficPackRepository) GetSummary(ctx context.Context, userID int64, now time.Time) (*service.TrafficCreditSummary, error) {
	summary := &service.TrafficCreditSummary{}
	if err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(initial_usd), 0), COALESCE(SUM(remaining_usd - reserved_usd), 0)
		FROM user_traffic_credits
		WHERE user_id = $1 AND platform = $2
			AND remaining_usd > $4
			AND remaining_usd - reserved_usd > 0
			AND expires_at > $3
	`, userID, service.TrafficPackPlatformOpenAI, now, r.policy.MinimumReserveUSD).Scan(&summary.TotalInitialUSD, &summary.TotalRemainingUSD); err != nil {
		return nil, err
	}
	var nextExpiresAt time.Time
	err := r.db.QueryRowContext(ctx, `
		SELECT expires_at, COALESCE(SUM(remaining_usd - reserved_usd), 0)
		FROM user_traffic_credits
		WHERE user_id = $1 AND platform = $2
			AND remaining_usd > $4
			AND remaining_usd - reserved_usd > 0
			AND expires_at > $3
		GROUP BY expires_at
		ORDER BY expires_at ASC
		LIMIT 1
	`, userID, service.TrafficPackPlatformOpenAI, now, r.policy.MinimumReserveUSD).Scan(&nextExpiresAt, &summary.NextExpiringUSD)
	if errors.Is(err, sql.ErrNoRows) {
		return summary, nil
	}
	if err != nil {
		return nil, err
	}
	summary.NextExpiresAt = &nextExpiresAt
	return summary, nil
}

func (r *trafficPackRepository) HasAvailableCredit(ctx context.Context, userID int64, now time.Time) (bool, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `
		SELECT id FROM user_traffic_credits
		WHERE user_id = $1 AND platform = $2
			AND remaining_usd > $4
			AND remaining_usd - reserved_usd > 0
			AND expires_at > $3
		ORDER BY expires_at ASC, credited_at ASC, id ASC
		LIMIT 1
	`, userID, service.TrafficPackPlatformOpenAI, now, r.policy.MinimumReserveUSD).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (r *trafficPackRepository) CreditPurchase(ctx context.Context, input service.CreditTrafficPackInput) error {
	validityDays := input.ValidityDays
	if validityDays <= 0 {
		validityDays = service.TrafficPackValidityDays
	}
	creditUSD := roundTrafficPackUSD(input.CreditUSD)
	if input.UserID <= 0 || input.OrderID <= 0 || creditUSD <= 0 {
		return service.ErrInvalidInput
	}
	expiresAt := input.CreditedAt.AddDate(0, 0, validityDays)
	return r.withCreditPurchaseTx(ctx, func(txCtx context.Context, exec sqlExecutor) error {
		var creditID int64
		err := scanSingleRow(txCtx, exec, `
			INSERT INTO user_traffic_credits (user_id, order_id, pack_id, platform, initial_usd, remaining_usd, credited_at, expires_at, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $5, $6, $7, $6, $6)
			ON CONFLICT (order_id) DO NOTHING
			RETURNING id
		`, []any{input.UserID, input.OrderID, input.PackID, service.TrafficPackPlatformOpenAI, creditUSD, input.CreditedAt, expiresAt}, &creditID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		_, err = exec.ExecContext(txCtx, `
			INSERT INTO traffic_credit_ledger (user_id, credit_id, order_id, request_id, entry_type, amount_usd, balance_after_usd, created_at)
			VALUES ($1, $2, $3, '', $4, $5, $5, $6)
		`, input.UserID, creditID, input.OrderID, service.TrafficCreditLedgerTypePurchase, creditUSD, input.CreditedAt)
		if err != nil {
			return err
		}
		return acknowledgeAllTrafficCreditExhaustionEvents(txCtx, exec, input.UserID, input.CreditedAt)
	})
}

func (r *trafficPackRepository) withCreditPurchaseTx(ctx context.Context, fn func(context.Context, sqlExecutor) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		// 余额支付会在外层事务内先创建订单，入账必须复用同一事务才能满足外键约束。
		return fn(ctx, tx.Client())
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *trafficPackRepository) Deduct(ctx context.Context, userID int64, amountUSD float64, requestID string, now time.Time) (bool, []service.TrafficCreditDeduction, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, nil, err
	}
	defer func() { _ = tx.Rollback() }()
	batches, err := r.listDeductibleCredits(ctx, tx, userID, now)
	if err != nil {
		return false, nil, err
	}
	deductions, covered := service.PlanTrafficCreditDeductions(batches, roundTrafficPackUSD(amountUSD), r.policy)
	if !covered {
		return false, nil, nil
	}
	for _, deduction := range deductions {
		balanceBefore, balanceAfter, err := decrementTrafficCredit(ctx, tx, deduction.CreditID, deduction.AmountUSD, now, r.policy)
		if err != nil {
			return false, nil, err
		}
		if err := recordTrafficCreditExhaustion(ctx, tx, r.policy, userID, deduction.CreditID, requestID, trafficCreditExhaustionBatchKey(requestID, 0), balanceBefore, balanceAfter); err != nil {
			return false, nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO traffic_credit_ledger (user_id, credit_id, order_id, request_id, entry_type, amount_usd, balance_after_usd, created_at)
			VALUES ($1, $2, NULL, $3, $4, $5, $6, $7)
		`, userID, deduction.CreditID, requestID, service.TrafficCreditLedgerTypeDeduction, deduction.AmountUSD, balanceAfter, now); err != nil {
			return false, nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return false, nil, err
	}
	return true, deductions, nil
}

func (r *trafficPackRepository) listDeductibleCredits(ctx context.Context, tx *sql.Tx, userID int64, now time.Time) ([]service.TrafficCreditBatch, error) {
	lockClause := ""
	if r.isPostgres {
		lockClause = " FOR UPDATE"
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT id, user_id, order_id, pack_id, initial_usd, remaining_usd - reserved_usd, credited_at, expires_at
		FROM user_traffic_credits
		WHERE user_id = $1 AND platform = $2
			AND remaining_usd > $4
			AND remaining_usd - reserved_usd > 0
			AND expires_at > $3
		ORDER BY expires_at ASC, credited_at ASC, id ASC
	`+lockClause, userID, service.TrafficPackPlatformOpenAI, now, r.policy.MinimumReserveUSD)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	batches := []service.TrafficCreditBatch{}
	for rows.Next() {
		var batch service.TrafficCreditBatch
		var orderID sql.NullInt64
		var packID sql.NullInt64
		if err := rows.Scan(&batch.ID, &batch.UserID, &orderID, &packID, &batch.InitialUSD, &batch.RemainingUSD, &batch.CreditedAt, &batch.ExpiresAt); err != nil {
			return nil, err
		}
		if orderID.Valid {
			batch.OrderID = &orderID.Int64
		}
		if packID.Valid {
			batch.PackID = &packID.Int64
		}
		batches = append(batches, batch)
	}
	return batches, rows.Err()
}

func decrementTrafficCredit(ctx context.Context, tx *sql.Tx, creditID int64, amountUSD float64, now time.Time, policy service.TrafficCreditPolicy) (float64, float64, error) {
	var balanceBefore float64
	if err := tx.QueryRowContext(ctx, `
		SELECT remaining_usd
		FROM user_traffic_credits
		WHERE id = $1
	`, creditID).Scan(&balanceBefore); err != nil {
		return 0, 0, err
	}
	var balanceAfter float64
	err := tx.QueryRowContext(ctx, `
		UPDATE user_traffic_credits SET remaining_usd = remaining_usd - $1, updated_at = $2
		WHERE id = $3
			AND remaining_usd > $4
			AND remaining_usd - reserved_usd + 0.0000000001 >= $1
		RETURNING remaining_usd
	`, amountUSD, now, creditID, policy.MinimumReserveUSD).Scan(&balanceAfter)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, service.ErrInvalidInput
	}
	if err != nil {
		return 0, 0, err
	}
	return roundTrafficPackUSD(balanceBefore), roundTrafficPackUSD(balanceAfter), nil
}

func roundTrafficPackUSD(value float64) float64 {
	return math.Round(value*1e10) / 1e10
}
