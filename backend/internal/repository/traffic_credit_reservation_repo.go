package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type trafficCreditReservationRepository struct {
	db     *sql.DB
	policy service.TrafficCreditPolicy
}

func NewTrafficCreditReservationRepository(db *sql.DB, policies ...service.TrafficCreditPolicy) service.TrafficCreditReservationRepository {
	return &trafficCreditReservationRepository{db: db, policy: firstTrafficCreditPolicy(policies)}
}

func (r *trafficCreditReservationRepository) GetAvailableUSD(ctx context.Context, userID int64, platform string, now time.Time) (float64, error) {
	var available float64
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(GREATEST(remaining_usd - reserved_usd, 0)), 0)
		FROM user_traffic_credits
		WHERE user_id = $1 AND platform = $2 AND remaining_usd > $4 AND expires_at > $3
	`, userID, platform, now, r.policy.MinimumReserveUSD).Scan(&available)
	return roundTrafficPackUSD(available), err
}

func (r *trafficCreditReservationRepository) Reserve(ctx context.Context, input service.TrafficCreditReservationInput) (_ *service.TrafficCreditReservation, created bool, err error) {
	if r == nil || r.db == nil {
		return nil, false, errors.New("traffic credit reservation repository db is nil")
	}
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.RequestFingerprint = strings.TrimSpace(input.RequestFingerprint)
	input.Platform = strings.TrimSpace(input.Platform)
	input.Model = strings.TrimSpace(input.Model)
	input.ReserveUSD = roundTrafficPackUSD(input.ReserveUSD)
	if input.RequestID == "" || input.APIKeyID <= 0 || input.UserID <= 0 || input.Platform == "" || input.ReserveUSD <= 0 || input.ExpiresAt.IsZero() || !json.Valid(input.PricingSnapshot) {
		return nil, false, service.ErrInvalidInput
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback() }()

	var reservationID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO traffic_credit_reservations (
			request_id, api_key_id, user_id, platform, model,
			request_fingerprint, pricing_snapshot, reserved_usd, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (request_id, api_key_id) DO NOTHING
		RETURNING id
	`, input.RequestID, input.APIKeyID, input.UserID, input.Platform, input.Model,
		input.RequestFingerprint, []byte(input.PricingSnapshot), input.ReserveUSD, input.ExpiresAt).Scan(&reservationID)
	if errors.Is(err, sql.ErrNoRows) {
		existing, loadErr := loadTrafficCreditReservation(ctx, tx, input.RequestID, input.APIKeyID)
		if loadErr != nil {
			return nil, false, loadErr
		}
		if existing.RequestFingerprint != input.RequestFingerprint {
			return nil, false, service.ErrUsageBillingRequestConflict
		}
		if err := tx.Commit(); err != nil {
			return nil, false, err
		}
		return existing, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	batches, err := lockAvailableTrafficCredits(ctx, tx, input.UserID, input.Platform, time.Now(), r.policy)
	if err != nil {
		return nil, false, err
	}
	items, covered := service.PlanTrafficCreditReservations(batches, input.ReserveUSD, r.policy)
	if !covered {
		return nil, false, service.ErrInsufficientBalance
	}
	for _, item := range items {
		var reservedAfter float64
		err := tx.QueryRowContext(ctx, `
			UPDATE user_traffic_credits
			SET reserved_usd = reserved_usd + $1, updated_at = NOW()
			WHERE id = $2
				AND remaining_usd > $3
				AND remaining_usd - reserved_usd + 0.0000000001 >= $1
			RETURNING reserved_usd
		`, item.ReservedUSD, item.CreditID, r.policy.MinimumReserveUSD).Scan(&reservedAfter)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, service.ErrInsufficientBalance
		}
		if err != nil {
			return nil, false, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO traffic_credit_reservation_items (reservation_id, credit_id, reserved_usd)
			VALUES ($1, $2, $3)
		`, reservationID, item.CreditID, item.ReservedUSD); err != nil {
			return nil, false, err
		}
	}
	reservation, err := loadTrafficCreditReservationByID(ctx, tx, reservationID)
	if err != nil {
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	return reservation, true, nil
}

func (r *trafficCreditReservationRepository) MarkDispatched(ctx context.Context, reservationID int64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE traffic_credit_reservations
		SET status = 'dispatched', updated_at = NOW()
		WHERE id = $1 AND status = 'reserved'
	`, reservationID)
	return err
}

func (r *trafficCreditReservationRepository) MarkUnknown(ctx context.Context, reservationID int64, reason string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE traffic_credit_reservations
		SET status = 'unknown', last_error = $2, updated_at = NOW()
		WHERE id = $1 AND status IN ('reserved', 'dispatched')
	`, reservationID, reason)
	return err
}

func (r *trafficCreditReservationRepository) Release(ctx context.Context, reservationID int64, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var status string
	if err := tx.QueryRowContext(ctx, `
		SELECT status
		FROM traffic_credit_reservations
		WHERE id = $1
		FOR UPDATE
	`, reservationID).Scan(&status); err != nil {
		return err
	}
	if status == string(service.TrafficCreditReservationReleased) {
		return tx.Commit()
	}
	if status != string(service.TrafficCreditReservationReserved) {
		return service.ErrInvalidInput
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT credit_id, reserved_usd
		FROM traffic_credit_reservation_items
		WHERE reservation_id = $1
		ORDER BY credit_id
		FOR UPDATE
	`, reservationID)
	if err != nil {
		return err
	}
	type releaseItem struct {
		creditID    int64
		reservedUSD float64
	}
	items := make([]releaseItem, 0)
	for rows.Next() {
		var item releaseItem
		if err := rows.Scan(&item.creditID, &item.reservedUSD); err != nil {
			_ = rows.Close()
			return err
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range items {
		result, err := tx.ExecContext(ctx, `
			UPDATE user_traffic_credits
			SET reserved_usd = reserved_usd - $1, updated_at = $2
			WHERE id = $3 AND reserved_usd + 0.0000000001 >= $1
		`, item.reservedUSD, now, item.creditID)
		if err != nil {
			return err
		}
		if affected, err := result.RowsAffected(); err != nil || affected != 1 {
			if err != nil {
				return err
			}
			return service.ErrInvalidInput
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE traffic_credit_reservations
		SET status = 'released', updated_at = $2
		WHERE id = $1
	`, reservationID, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *trafficCreditReservationRepository) ReleaseExpiredReserved(ctx context.Context, now time.Time, limit int) (int, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("traffic credit reservation repository db is nil")
	}
	if limit <= 0 {
		limit = 100
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT id
		FROM traffic_credit_reservations
		WHERE status = 'reserved' AND expires_at <= $1
		ORDER BY expires_at ASC, id ASC
		FOR UPDATE SKIP LOCKED
		LIMIT $2
	`, now, limit)
	if err != nil {
		return 0, err
	}
	reservationIDs := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, err
		}
		reservationIDs = append(reservationIDs, id)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, reservationID := range reservationIDs {
		if err := releaseTrafficCreditReservationLocked(ctx, tx, reservationID, now); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(reservationIDs), nil
}

func (r *trafficCreditReservationRepository) HasOutstandingDebt(ctx context.Context, userID int64, platform string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM traffic_credit_reservations
			WHERE user_id = $1 AND platform = $2 AND status = 'debt' AND debt_usd > 0
		)
	`, userID, platform).Scan(&exists)
	return exists, err
}

func releaseTrafficCreditReservationLocked(ctx context.Context, tx *sql.Tx, reservationID int64, now time.Time) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT credit_id, reserved_usd
		FROM traffic_credit_reservation_items
		WHERE reservation_id = $1
		ORDER BY credit_id
		FOR UPDATE
	`, reservationID)
	if err != nil {
		return err
	}
	type releaseItem struct {
		creditID    int64
		reservedUSD float64
	}
	items := make([]releaseItem, 0)
	for rows.Next() {
		var item releaseItem
		if err := rows.Scan(&item.creditID, &item.reservedUSD); err != nil {
			_ = rows.Close()
			return err
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, item := range items {
		result, err := tx.ExecContext(ctx, `
			UPDATE user_traffic_credits
			SET reserved_usd = CASE
					WHEN reserved_usd - $1 < 0.0000000001 THEN 0
					ELSE reserved_usd - $1
				END,
				updated_at = $2
			WHERE id = $3 AND reserved_usd + 0.0000000001 >= $1
		`, item.reservedUSD, now, item.creditID)
		if err != nil {
			return err
		}
		if affected, err := result.RowsAffected(); err != nil || affected != 1 {
			if err != nil {
				return err
			}
			return service.ErrInvalidInput
		}
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE traffic_credit_reservations
		SET status = 'released',
			last_error = 'expired before dispatch',
			updated_at = $2
		WHERE id = $1 AND status = 'reserved'
	`, reservationID, now)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		if err != nil {
			return err
		}
		return service.ErrInvalidInput
	}
	return nil
}

func lockAvailableTrafficCredits(ctx context.Context, tx *sql.Tx, userID int64, platform string, now time.Time, policy service.TrafficCreditPolicy) ([]service.TrafficCreditBatch, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, user_id, order_id, pack_id, initial_usd, remaining_usd, reserved_usd, credited_at, expires_at
		FROM user_traffic_credits
		WHERE user_id = $1 AND platform = $2
			AND remaining_usd > $4
			AND remaining_usd - reserved_usd > 0
			AND expires_at > $3
		ORDER BY expires_at, credited_at, id
		FOR UPDATE
	`, userID, platform, now, policy.MinimumReserveUSD)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	batches := make([]service.TrafficCreditBatch, 0)
	for rows.Next() {
		var batch service.TrafficCreditBatch
		var orderID sql.NullInt64
		var packID sql.NullInt64
		if err := rows.Scan(
			&batch.ID,
			&batch.UserID,
			&orderID,
			&packID,
			&batch.InitialUSD,
			&batch.RemainingUSD,
			&batch.ReservedUSD,
			&batch.CreditedAt,
			&batch.ExpiresAt,
		); err != nil {
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

type trafficCreditReservationScanner interface {
	Scan(dest ...any) error
}

func loadTrafficCreditReservation(ctx context.Context, tx *sql.Tx, requestID string, apiKeyID int64) (*service.TrafficCreditReservation, error) {
	return scanTrafficCreditReservation(tx.QueryRowContext(ctx, `
		SELECT id, request_id, api_key_id, user_id, platform, model, request_fingerprint,
			pricing_snapshot, reserved_usd, settled_usd, debt_usd, status, last_error,
			expires_at, created_at, updated_at
		FROM traffic_credit_reservations
		WHERE request_id = $1 AND api_key_id = $2
	`, requestID, apiKeyID), tx, ctx)
}

func loadTrafficCreditReservationByID(ctx context.Context, tx *sql.Tx, id int64) (*service.TrafficCreditReservation, error) {
	return scanTrafficCreditReservation(tx.QueryRowContext(ctx, `
		SELECT id, request_id, api_key_id, user_id, platform, model, request_fingerprint,
			pricing_snapshot, reserved_usd, settled_usd, debt_usd, status, last_error,
			expires_at, created_at, updated_at
		FROM traffic_credit_reservations
		WHERE id = $1
	`, id), tx, ctx)
}

func scanTrafficCreditReservation(scanner trafficCreditReservationScanner, tx *sql.Tx, ctx context.Context) (*service.TrafficCreditReservation, error) {
	var reservation service.TrafficCreditReservation
	var pricingSnapshot []byte
	if err := scanner.Scan(
		&reservation.ID,
		&reservation.RequestID,
		&reservation.APIKeyID,
		&reservation.UserID,
		&reservation.Platform,
		&reservation.Model,
		&reservation.RequestFingerprint,
		&pricingSnapshot,
		&reservation.ReserveUSD,
		&reservation.SettledUSD,
		&reservation.DebtUSD,
		&reservation.Status,
		&reservation.LastError,
		&reservation.ExpiresAt,
		&reservation.CreatedAt,
		&reservation.UpdatedAt,
	); err != nil {
		return nil, err
	}
	reservation.PricingSnapshot = append(json.RawMessage(nil), pricingSnapshot...)
	rows, err := tx.QueryContext(ctx, `
		SELECT credit_id, reserved_usd, settled_usd
		FROM traffic_credit_reservation_items
		WHERE reservation_id = $1
		ORDER BY credit_id
	`, reservation.ID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var item service.TrafficCreditReservationItem
		if err := rows.Scan(&item.CreditID, &item.ReservedUSD, &item.SettledUSD); err != nil {
			return nil, err
		}
		reservation.Items = append(reservation.Items, item)
	}
	return &reservation, rows.Err()
}
