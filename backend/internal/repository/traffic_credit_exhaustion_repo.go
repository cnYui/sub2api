package repository

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type trafficCreditExhaustionRepository struct {
	db *sql.DB
}

func NewTrafficCreditExhaustionRepository(db *sql.DB) service.TrafficCreditExhaustionRepository {
	return &trafficCreditExhaustionRepository{db: db}
}

func (r *trafficCreditExhaustionRepository) ListPendingEventIDs(ctx context.Context, userID int64) ([]int64, error) {
	if r == nil || r.db == nil || userID <= 0 {
		return nil, service.ErrInvalidInput
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id
		FROM traffic_credit_exhaustion_events
		WHERE user_id = $1 AND acknowledged_at IS NULL
		ORDER BY created_at ASC, id ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *trafficCreditExhaustionRepository) AcknowledgeEvents(ctx context.Context, userID int64, eventIDs []int64, now time.Time) error {
	if r == nil || r.db == nil || userID <= 0 || len(eventIDs) == 0 {
		return service.ErrInvalidInput
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := acknowledgeTrafficCreditExhaustionEvents(ctx, tx, userID, eventIDs, now); err != nil {
		return err
	}
	return tx.Commit()
}

func recordTrafficCreditExhaustion(
	ctx context.Context,
	exec sqlExecutor,
	policy service.TrafficCreditPolicy,
	userID int64,
	creditID int64,
	requestID string,
	batchKey string,
	beforeUSD float64,
	afterUSD float64,
) error {
	if exec == nil || userID <= 0 || creditID <= 0 {
		return service.ErrInvalidInput
	}
	if policy.IsDepleted(beforeUSD) || !policy.IsDepleted(afterUSD) {
		return nil
	}
	requestID = strings.TrimSpace(requestID)
	batchKey = strings.TrimSpace(batchKey)
	if requestID == "" || batchKey == "" {
		return service.ErrInvalidInput
	}
	_, err := exec.ExecContext(ctx, `
		INSERT INTO traffic_credit_exhaustion_events (user_id, credit_id, request_id, batch_key)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, credit_id) DO NOTHING
	`, userID, creditID, requestID, batchKey)
	return err
}

func acknowledgeAllTrafficCreditExhaustionEvents(ctx context.Context, exec sqlExecutor, userID int64, now time.Time) error {
	if exec == nil || userID <= 0 {
		return service.ErrInvalidInput
	}
	_, err := exec.ExecContext(ctx, `
		UPDATE traffic_credit_exhaustion_events
		SET acknowledged_at = $2
		WHERE user_id = $1 AND acknowledged_at IS NULL
	`, userID, now)
	return err
}

func acknowledgeTrafficCreditExhaustionEvents(ctx context.Context, exec sqlExecutor, userID int64, eventIDs []int64, now time.Time) error {
	if exec == nil || userID <= 0 || len(eventIDs) == 0 {
		return service.ErrInvalidInput
	}
	seen := make(map[int64]struct{}, len(eventIDs))
	uniqueIDs := make([]int64, 0, len(eventIDs))
	for _, id := range eventIDs {
		if id <= 0 {
			return service.ErrInvalidInput
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}
	for _, id := range uniqueIDs {
		var ownerID int64
		err := scanSingleRow(ctx, exec, `
			SELECT user_id
			FROM traffic_credit_exhaustion_events
			WHERE id = $1
		`, []any{id}, &ownerID)
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrInvalidInput
		}
		if err != nil {
			return err
		}
		if ownerID != userID {
			return service.ErrInvalidInput
		}
	}
	for _, id := range uniqueIDs {
		if _, err := exec.ExecContext(ctx, `
			UPDATE traffic_credit_exhaustion_events
			SET acknowledged_at = COALESCE(acknowledged_at, $3)
			WHERE user_id = $1 AND id = $2
		`, userID, id, now); err != nil {
			return err
		}
	}
	return nil
}

func trafficCreditExhaustionBatchKey(requestID string, apiKeyID int64) string {
	return strings.TrimSpace(requestID) + ":" + strconv.FormatInt(apiKeyID, 10)
}
