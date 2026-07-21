package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type usageBillingRepository struct {
	db     *sql.DB
	policy service.TrafficCreditPolicy
}

func NewUsageBillingRepository(_ *dbent.Client, sqlDB *sql.DB, policies ...service.TrafficCreditPolicy) service.UsageBillingRepository {
	return &usageBillingRepository{db: sqlDB, policy: firstTrafficCreditPolicy(policies)}
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
	var id int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO usage_billing_dedup (request_id, api_key_id, request_fingerprint)
		VALUES ($1, $2, $3)
		ON CONFLICT (request_id, api_key_id) DO NOTHING
		RETURNING id
	`, cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		var existingFingerprint string
		if err := tx.QueryRowContext(ctx, `
			SELECT request_fingerprint
			FROM usage_billing_dedup
			WHERE request_id = $1 AND api_key_id = $2
		`, cmd.RequestID, cmd.APIKeyID).Scan(&existingFingerprint); err != nil {
			return false, err
		}
		if strings.TrimSpace(existingFingerprint) != strings.TrimSpace(cmd.RequestFingerprint) {
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
	`, cmd.RequestID, cmd.APIKeyID).Scan(&archivedFingerprint)
	if err == nil {
		if strings.TrimSpace(archivedFingerprint) != strings.TrimSpace(cmd.RequestFingerprint) {
			return false, service.ErrUsageBillingRequestConflict
		}
		return false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	return true, nil
}

func (r *usageBillingRepository) applyUsageBillingEffects(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, result *service.UsageBillingApplyResult) error {
	if cmd.SubscriptionCost > 0 && cmd.SubscriptionID != nil {
		if err := incrementUsageBillingSubscription(ctx, tx, *cmd.SubscriptionID, cmd.EntitlementPeriodID, cmd.SubscriptionCost, cmd.CompletedAt); err != nil {
			return err
		}
	}

	if cmd.BalanceCost > 0 {
		newBalance, sufficient, err := deductUsageBillingBalance(ctx, tx, cmd.UserID, cmd.BalanceCost)
		if err != nil {
			return err
		}
		result.NewBalance = &newBalance
		result.BalanceOverdrafted = !sufficient
	}

	if cmd.TrafficPackCost > 0 {
		if cmd.TrafficCreditReservationID != nil {
			debtUSD, err := settleUsageBillingTrafficCreditReservation(ctx, tx, cmd, r.policy)
			if err != nil {
				return err
			}
			result.TrafficCreditDebtUSD = debtUSD
		} else {
			covered, err := deductUsageBillingTrafficPack(ctx, tx, cmd.UserID, cmd.APIKeyID, cmd.TrafficPackCost, cmd.RequestID, r.policy)
			if err != nil {
				return err
			}
			if !covered {
				return service.ErrInsufficientBalance
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

type usageBillingTrafficCreditReservation struct {
	ID                 int64
	RequestID          string
	APIKeyID           int64
	UserID             int64
	Platform           string
	RequestFingerprint string
	ReservedUSD        float64
	DebtUSD            float64
	Status             string
}

type usageBillingTrafficCreditReservationItem struct {
	CreditID    int64
	ReservedUSD float64
	SettledUSD  float64
}

func settleUsageBillingTrafficCreditReservation(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, policy service.TrafficCreditPolicy) (float64, error) {
	if cmd == nil || cmd.TrafficCreditReservationID == nil || *cmd.TrafficCreditReservationID <= 0 {
		return 0, service.ErrInvalidInput
	}
	amountUSD := roundTrafficPackUSD(cmd.TrafficPackCost)
	if amountUSD <= 0 {
		return 0, nil
	}
	reservation, err := loadUsageBillingTrafficCreditReservation(ctx, tx, *cmd.TrafficCreditReservationID)
	if err != nil {
		return 0, err
	}
	if reservation.RequestID != strings.TrimSpace(cmd.RequestID) ||
		reservation.APIKeyID != cmd.APIKeyID ||
		reservation.UserID != cmd.UserID ||
		strings.TrimSpace(reservation.RequestFingerprint) != strings.TrimSpace(cmd.RequestFingerprint) {
		return 0, service.ErrUsageBillingRequestConflict
	}
	switch reservation.Status {
	case string(service.TrafficCreditReservationReserved), string(service.TrafficCreditReservationDispatched), string(service.TrafficCreditReservationUnknown):
	case string(service.TrafficCreditReservationSettled), string(service.TrafficCreditReservationDebt):
		return roundTrafficPackUSD(reservation.DebtUSD), nil
	default:
		return 0, service.ErrInvalidInput
	}

	items, err := listUsageBillingTrafficCreditReservationItems(ctx, tx, reservation.ID)
	if err != nil {
		return 0, err
	}
	remaining := amountUSD
	covered := 0.0
	for _, item := range items {
		reservedAvailable := roundTrafficPackUSD(item.ReservedUSD - item.SettledUSD)
		if reservedAvailable <= 0 {
			continue
		}
		settleUSD := roundTrafficPackUSD(minFloat64(remaining, reservedAvailable))
		remaining = roundTrafficPackUSD(remaining - settleUSD)
		covered = roundTrafficPackUSD(covered + settleUSD)
		if err := settleUsageBillingTrafficCreditReservationItem(ctx, tx, policy, cmd.UserID, cmd.APIKeyID, cmd.RequestID, item.CreditID, reservation.ID, reservedAvailable, settleUSD); err != nil {
			return 0, err
		}
	}
	if remaining > 0 {
		extra, err := deductUsageBillingTrafficPackPartial(ctx, tx, cmd.UserID, cmd.APIKeyID, remaining, cmd.RequestID, policy)
		if err != nil {
			return 0, err
		}
		covered = roundTrafficPackUSD(covered + extra)
		remaining = roundTrafficPackUSD(remaining - extra)
	}
	debtUSD := roundTrafficPackUSD(amountUSD - covered)
	if debtUSD < 0 {
		debtUSD = 0
	}
	status := string(service.TrafficCreditReservationSettled)
	lastError := ""
	if debtUSD > 0 {
		status = string(service.TrafficCreditReservationDebt)
		lastError = service.ErrInsufficientBalance.Error()
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE traffic_credit_reservations
		SET settled_usd = $2,
			debt_usd = $3,
			status = $4,
			last_error = $5,
			updated_at = NOW()
		WHERE id = $1
	`, reservation.ID, covered, debtUSD, status, lastError); err != nil {
		return 0, err
	}
	return debtUSD, nil
}

func loadUsageBillingTrafficCreditReservation(ctx context.Context, tx *sql.Tx, reservationID int64) (*usageBillingTrafficCreditReservation, error) {
	var reservation usageBillingTrafficCreditReservation
	err := tx.QueryRowContext(ctx, `
		SELECT id, request_id, api_key_id, user_id, platform, request_fingerprint,
			reserved_usd, debt_usd, status
		FROM traffic_credit_reservations
		WHERE id = $1
		FOR UPDATE
	`, reservationID).Scan(
		&reservation.ID,
		&reservation.RequestID,
		&reservation.APIKeyID,
		&reservation.UserID,
		&reservation.Platform,
		&reservation.RequestFingerprint,
		&reservation.ReservedUSD,
		&reservation.DebtUSD,
		&reservation.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrInvalidInput
	}
	if err != nil {
		return nil, err
	}
	if reservation.Platform != service.TrafficPackPlatformOpenAI {
		return nil, service.ErrInvalidInput
	}
	return &reservation, nil
}

func listUsageBillingTrafficCreditReservationItems(ctx context.Context, tx *sql.Tx, reservationID int64) ([]usageBillingTrafficCreditReservationItem, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT i.credit_id, i.reserved_usd, i.settled_usd
		FROM traffic_credit_reservation_items i
		JOIN user_traffic_credits c ON c.id = i.credit_id
		WHERE i.reservation_id = $1
		ORDER BY c.expires_at ASC, c.credited_at ASC, c.id ASC
		FOR UPDATE OF i, c
	`, reservationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := []usageBillingTrafficCreditReservationItem{}
	for rows.Next() {
		var item usageBillingTrafficCreditReservationItem
		if err := rows.Scan(&item.CreditID, &item.ReservedUSD, &item.SettledUSD); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func settleUsageBillingTrafficCreditReservationItem(
	ctx context.Context,
	tx *sql.Tx,
	policy service.TrafficCreditPolicy,
	userID int64,
	apiKeyID int64,
	requestID string,
	creditID int64,
	reservationID int64,
	releaseUSD float64,
	settleUSD float64,
) error {
	var balanceBefore float64
	if err := tx.QueryRowContext(ctx, `
		SELECT remaining_usd
		FROM user_traffic_credits
		WHERE id = $1
	`, creditID).Scan(&balanceBefore); err != nil {
		return err
	}
	var balanceAfter float64
	err := tx.QueryRowContext(ctx, `
		UPDATE user_traffic_credits
		SET remaining_usd = CASE
				WHEN remaining_usd - $1 < 0.0000000001 THEN 0
				ELSE remaining_usd - $1
			END,
			reserved_usd = CASE
				WHEN reserved_usd - $2 < 0.0000000001 THEN 0
				ELSE reserved_usd - $2
			END,
			updated_at = NOW()
		WHERE id = $3
			AND remaining_usd + 0.0000000001 >= $1
			AND reserved_usd + 0.0000000001 >= $2
		RETURNING remaining_usd
	`, settleUSD, releaseUSD, creditID).Scan(&balanceAfter)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrInvalidInput
	}
	if err != nil {
		return err
	}
	if err := recordTrafficCreditExhaustion(ctx, tx, policy, userID, creditID, requestID, trafficCreditExhaustionBatchKey(requestID, apiKeyID), balanceBefore, balanceAfter); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE traffic_credit_reservation_items
		SET settled_usd = settled_usd + $3
		WHERE reservation_id = $1 AND credit_id = $2
	`, reservationID, creditID, settleUSD); err != nil {
		return err
	}
	if settleUSD <= 0 {
		return nil
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO traffic_credit_ledger (
			user_id, credit_id, order_id, request_id, entry_type, amount_usd, balance_after_usd, created_at
		)
		VALUES ($1, $2, NULL, $3, $4, $5, $6, NOW())
	`, userID, creditID, requestID, service.TrafficCreditLedgerTypeDeduction, settleUSD, balanceAfter)
	return err
}

func deductUsageBillingTrafficPack(ctx context.Context, tx *sql.Tx, userID int64, apiKeyID int64, amountUSD float64, requestID string, policy service.TrafficCreditPolicy) (bool, error) {
	batches, err := listUsageBillingTrafficCredits(ctx, tx, userID, policy)
	if err != nil {
		return false, err
	}
	deductions, covered := service.PlanTrafficCreditDeductions(batches, amountUSD, policy)
	if !covered {
		return false, nil
	}
	nowExpr := "NOW()"
	for _, deduction := range deductions {
		var balanceBefore float64
		if err := tx.QueryRowContext(ctx, `
			SELECT remaining_usd
			FROM user_traffic_credits
			WHERE id = $1
		`, deduction.CreditID).Scan(&balanceBefore); err != nil {
			return false, err
		}
		var balanceAfter float64
		err := tx.QueryRowContext(ctx, `
			UPDATE user_traffic_credits
			SET remaining_usd = remaining_usd - $1,
				updated_at = `+nowExpr+`
			WHERE id = $2
				AND remaining_usd > $3
				AND remaining_usd + 0.0000000001 >= $1
			RETURNING remaining_usd
		`, deduction.AmountUSD, deduction.CreditID, policy.MinimumReserveUSD).Scan(&balanceAfter)
		if errors.Is(err, sql.ErrNoRows) {
			return false, service.ErrInvalidInput
		}
		if err != nil {
			return false, err
		}
		if err := recordTrafficCreditExhaustion(ctx, tx, policy, userID, deduction.CreditID, requestID, trafficCreditExhaustionBatchKey(requestID, apiKeyID), balanceBefore, balanceAfter); err != nil {
			return false, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO traffic_credit_ledger (
				user_id, credit_id, order_id, request_id, entry_type, amount_usd, balance_after_usd, created_at
			)
			VALUES ($1, $2, NULL, $3, $4, $5, $6, `+nowExpr+`)
		`, userID, deduction.CreditID, requestID, service.TrafficCreditLedgerTypeDeduction, deduction.AmountUSD, balanceAfter); err != nil {
			return false, err
		}
	}
	return true, nil
}

func deductUsageBillingTrafficPackPartial(ctx context.Context, tx *sql.Tx, userID int64, apiKeyID int64, amountUSD float64, requestID string, policy service.TrafficCreditPolicy) (float64, error) {
	batches, err := listUsageBillingTrafficCredits(ctx, tx, userID, policy)
	if err != nil {
		return 0, err
	}
	deductions, _ := service.PlanTrafficCreditDeductions(batches, amountUSD, policy)
	deducted := 0.0
	for _, deduction := range deductions {
		var balanceBefore float64
		if err := tx.QueryRowContext(ctx, `
			SELECT remaining_usd
			FROM user_traffic_credits
			WHERE id = $1
		`, deduction.CreditID).Scan(&balanceBefore); err != nil {
			return deducted, err
		}
		var balanceAfter float64
		err := tx.QueryRowContext(ctx, `
			UPDATE user_traffic_credits
			SET remaining_usd = CASE
					WHEN remaining_usd - $1 < 0.0000000001 THEN 0
					ELSE remaining_usd - $1
				END,
				updated_at = NOW()
			WHERE id = $2
				AND remaining_usd > $3
				AND remaining_usd - reserved_usd + 0.0000000001 >= $1
			RETURNING remaining_usd
		`, deduction.AmountUSD, deduction.CreditID, policy.MinimumReserveUSD).Scan(&balanceAfter)
		if errors.Is(err, sql.ErrNoRows) {
			return deducted, service.ErrInvalidInput
		}
		if err != nil {
			return deducted, err
		}
		if err := recordTrafficCreditExhaustion(ctx, tx, policy, userID, deduction.CreditID, requestID, trafficCreditExhaustionBatchKey(requestID, apiKeyID), balanceBefore, balanceAfter); err != nil {
			return deducted, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO traffic_credit_ledger (
				user_id, credit_id, order_id, request_id, entry_type, amount_usd, balance_after_usd, created_at
			)
			VALUES ($1, $2, NULL, $3, $4, $5, $6, NOW())
		`, userID, deduction.CreditID, requestID, service.TrafficCreditLedgerTypeDeduction, deduction.AmountUSD, balanceAfter); err != nil {
			return deducted, err
		}
		deducted = roundTrafficPackUSD(deducted + deduction.AmountUSD)
	}
	return deducted, nil
}

func listUsageBillingTrafficCredits(ctx context.Context, tx *sql.Tx, userID int64, policy service.TrafficCreditPolicy) ([]service.TrafficCreditBatch, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, user_id, order_id, pack_id, initial_usd, remaining_usd, reserved_usd, credited_at, expires_at
		FROM user_traffic_credits
		WHERE user_id = $1 AND platform = $2
			AND remaining_usd > $3
			AND remaining_usd - reserved_usd > 0
			AND expires_at > NOW()
		ORDER BY expires_at ASC, credited_at ASC, id ASC
		FOR UPDATE
	`, userID, service.TrafficPackPlatformOpenAI, policy.MinimumReserveUSD)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	batches := []service.TrafficCreditBatch{}
	for rows.Next() {
		var batch service.TrafficCreditBatch
		var orderID sql.NullInt64
		var packID sql.NullInt64
		var remainingUSD float64
		if err := rows.Scan(&batch.ID, &batch.UserID, &orderID, &packID, &batch.InitialUSD, &remainingUSD, &batch.ReservedUSD, &batch.CreditedAt, &batch.ExpiresAt); err != nil {
			return nil, err
		}
		batch.RemainingUSD = roundTrafficPackUSD(remainingUSD - batch.ReservedUSD)
		if orderID.Valid {
			batch.OrderID = &orderID.Int64
		}
		if packID.Valid {
			batch.PackID = &packID.Int64
		}
		batches = append(batches, batch)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return batches, nil
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func incrementUsageBillingSubscription(ctx context.Context, tx *sql.Tx, subscriptionID int64, entitlementPeriodID *int64, costUSD float64, completedAt time.Time) error {
	if completedAt.IsZero() {
		completedAt = time.Now()
	}
	completedAt = completedAt.In(timezone.Location())
	rolling, err := isUsageBillingRollingWeeklySubscription(ctx, tx, subscriptionID, completedAt)
	if err != nil {
		return err
	}
	if rolling {
		return incrementUsageBillingRollingWeeklySubscription(ctx, tx, subscriptionID, entitlementPeriodID, costUSD, completedAt)
	}
	dailyStart := timezone.StartOfDay(completedAt)
	weeklyStart := timezone.StartOfWeek(completedAt)

	const updateSQL = `
		UPDATE user_subscriptions us
		SET
			daily_usage_usd = CASE
				WHEN us.daily_window_start IS NULL THEN $1
				WHEN us.daily_window_start < $3 THEN
					$1 + CASE
						WHEN g.daily_limit_usd IS NOT NULL AND g.daily_limit_usd > 0 THEN
							GREATEST(
								us.daily_usage_usd - (
									g.daily_limit_usd * GREATEST(FLOOR(EXTRACT(EPOCH FROM ($3 - us.daily_window_start)) / 86400), 1)
								),
								0
							)
						ELSE 0
					END
				ELSE us.daily_usage_usd + $1
			END,
			daily_window_start = CASE
				WHEN us.daily_window_start IS NULL OR us.daily_window_start < $3 THEN $3
				ELSE us.daily_window_start
			END,
			weekly_usage_usd = CASE
				WHEN us.weekly_window_start IS NULL OR us.weekly_window_start < $4 THEN $1
				ELSE us.weekly_usage_usd + $1
			END,
			weekly_window_start = CASE
				WHEN us.weekly_window_start IS NULL OR us.weekly_window_start < $4 THEN $4
				ELSE us.weekly_window_start
			END,
			monthly_usage_usd = CASE
				WHEN us.monthly_window_start IS NULL OR us.monthly_window_start + INTERVAL '30 days' <= $5 THEN $1
				ELSE us.monthly_usage_usd + $1
			END,
			monthly_window_start = CASE
				WHEN us.monthly_window_start IS NULL OR us.monthly_window_start + INTERVAL '30 days' <= $5 THEN $5
				ELSE us.monthly_window_start
			END,
			updated_at = $5
		FROM groups g
		WHERE us.id = $2
			AND us.deleted_at IS NULL
			AND us.group_id = g.id
			AND g.deleted_at IS NULL
	`
	res, err := tx.ExecContext(ctx, updateSQL, costUSD, subscriptionID, dailyStart, weeklyStart, completedAt)
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

func isUsageBillingRollingWeeklySubscription(ctx context.Context, tx *sql.Tx, subscriptionID int64, completedAt time.Time) (bool, error) {
	var groupName, subscriptionType string
	var weeklyLimit sql.NullFloat64
	var hasWeeklyEntitlement bool
	err := tx.QueryRowContext(ctx, `
		SELECT g.name, g.subscription_type, g.weekly_limit_usd,
			EXISTS (
				SELECT 1
				FROM subscription_entitlement_periods sep
				WHERE sep.subscription_id = us.id
					AND sep.status = 'active'
					AND sep.weekly_limit_usd IS NOT NULL
					AND sep.starts_at <= $2
					AND sep.expires_at > $2
			)
		FROM user_subscriptions us
		JOIN groups g ON g.id = us.group_id AND g.deleted_at IS NULL
		WHERE us.id = $1 AND us.deleted_at IS NULL
	`, subscriptionID, completedAt).Scan(&groupName, &subscriptionType, &weeklyLimit, &hasWeeklyEntitlement)
	if errors.Is(err, sql.ErrNoRows) {
		return false, service.ErrSubscriptionNotFound
	}
	if err != nil {
		return false, err
	}
	group := &service.Group{Name: groupName, SubscriptionType: subscriptionType}
	if weeklyLimit.Valid {
		group.WeeklyLimitUSD = &weeklyLimit.Float64
	}
	return group.UsesRollingWeeklyQuota() && hasWeeklyEntitlement, nil
}

func incrementUsageBillingRollingWeeklySubscription(ctx context.Context, tx *sql.Tx, subscriptionID int64, entitlementPeriodID *int64, costUSD float64, completedAt time.Time) error {
	type rollingSubscriptionRow struct {
		ID                int64
		StartsAt          time.Time
		ExpiresAt         time.Time
		WeeklyAnchorAt    sql.NullTime
		WeeklyWindowStart sql.NullTime
	}
	var row rollingSubscriptionRow
	err := tx.QueryRowContext(ctx, `
		SELECT id, starts_at, expires_at, weekly_anchor_at, weekly_window_start
		FROM user_subscriptions
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, subscriptionID).Scan(&row.ID, &row.StartsAt, &row.ExpiresAt, &row.WeeklyAnchorAt, &row.WeeklyWindowStart)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrSubscriptionNotFound
	}
	if err != nil {
		return err
	}

	periodID, weeklyLimitUSD, entitlementExpiresAt, err := loadUsageBillingRollingWeeklyEntitlement(ctx, tx, subscriptionID, entitlementPeriodID, completedAt)
	if err != nil {
		return err
	}
	if periodID <= 0 || weeklyLimitUSD <= 0 {
		return service.ErrSubscriptionNotFound
	}

	anchor := row.StartsAt
	if row.WeeklyAnchorAt.Valid {
		anchor = row.WeeklyAnchorAt.Time
	}
	var windowStart *time.Time
	if row.WeeklyWindowStart.Valid {
		windowStart = &row.WeeklyWindowStart.Time
	}
	window := service.CalculateSubscriptionWeeklyWindow(anchor, windowStart, entitlementExpiresAt, completedAt, weeklyLimitUSD)
	if window.Expired {
		return service.ErrSubscriptionNotFound
	}

	res, err := tx.ExecContext(ctx, `
		UPDATE user_subscriptions
		SET weekly_usage_usd = CASE
				WHEN weekly_window_start IS NULL OR weekly_window_start <> $2 THEN $1
				ELSE weekly_usage_usd + $1
			END,
			weekly_window_start = $2,
			updated_at = $3
		WHERE id = $4 AND deleted_at IS NULL
	`, costUSD, window.Start, completedAt, subscriptionID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrSubscriptionNotFound
	}
	return nil
}

func loadUsageBillingRollingWeeklyEntitlement(ctx context.Context, tx *sql.Tx, subscriptionID int64, entitlementPeriodID *int64, completedAt time.Time) (int64, float64, time.Time, error) {
	var (
		id          int64
		expiresAt   time.Time
		weeklyLimit sql.NullFloat64
	)
	if entitlementPeriodID != nil && *entitlementPeriodID > 0 {
		err := tx.QueryRowContext(ctx, `
			SELECT id, weekly_limit_usd, expires_at
			FROM subscription_entitlement_periods
			WHERE id = $1
				AND subscription_id = $2
				AND status = 'active'
				AND starts_at <= $3
				AND expires_at > $3
			FOR SHARE
		`, *entitlementPeriodID, subscriptionID, completedAt).Scan(&id, &weeklyLimit, &expiresAt)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, time.Time{}, service.ErrSubscriptionNotFound
		}
		if err != nil {
			return 0, 0, time.Time{}, err
		}
		if !weeklyLimit.Valid {
			return 0, 0, time.Time{}, service.ErrSubscriptionNotFound
		}
		return id, weeklyLimit.Float64, expiresAt, nil
	}

	err := tx.QueryRowContext(ctx, `
		SELECT id, weekly_limit_usd, expires_at
		FROM subscription_entitlement_periods
		WHERE subscription_id = $1
			AND status = 'active'
			AND starts_at <= $2
			AND expires_at > $2
			AND weekly_limit_usd IS NOT NULL
		ORDER BY starts_at DESC, id DESC
		LIMIT 1
		FOR SHARE
	`, subscriptionID, completedAt).Scan(&id, &weeklyLimit, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, time.Time{}, service.ErrSubscriptionNotFound
	}
	if err != nil {
		return 0, 0, time.Time{}, err
	}
	return id, weeklyLimit.Float64, expiresAt, nil
}

func deductUsageBillingBalance(ctx context.Context, tx *sql.Tx, userID int64, amount float64) (float64, bool, error) {
	var newBalance float64
	err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance - $1,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL AND balance >= $1
		RETURNING balance
	`, amount, userID).Scan(&newBalance)
	if err == nil {
		return newBalance, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}

	err = tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance - $1,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING balance
	`, amount, userID).Scan(&newBalance)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, service.ErrUserNotFound
	}
	if err != nil {
		return 0, false, err
	}
	return newBalance, false, nil
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
