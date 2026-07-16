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

const usageFactClaimLease = 30 * time.Second

type usageFactRepository struct {
	db *sql.DB
}

type usageFactScanner interface {
	Scan(dest ...any) error
}

func NewUsageFactRepository(db *sql.DB) service.UsageFactRepository {
	return &usageFactRepository{db: db}
}

func (r *usageFactRepository) CreatePending(ctx context.Context, fact *service.UsageFact) (*service.UsageFact, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, errors.New("usage fact repository db is nil")
	}
	if fact == nil {
		return nil, false, service.ErrInvalidInput
	}
	fact.RequestID = strings.TrimSpace(fact.RequestID)
	if fact.RequestID == "" {
		return nil, false, service.ErrUsageFactRequestIDRequired
	}
	if fact.PayloadVersion <= 0 {
		fact.PayloadVersion = service.UsageFactPayloadVersion1
	}
	if fact.CompletedAt.IsZero() {
		fact.CompletedAt = time.Now()
	}
	if fact.NextAttemptAt.IsZero() {
		fact.NextAttemptAt = fact.CompletedAt
	}

	created, err := scanUsageFact(r.db.QueryRowContext(ctx, `
		INSERT INTO usage_facts (
			request_id, api_key_id, user_id, account_id, request_fingerprint,
			payload_version, payload, billing_status, next_attempt_at, completed_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending', $8, $9)
		ON CONFLICT (request_id, api_key_id) DO NOTHING
		RETURNING id, request_id, api_key_id, user_id, account_id,
			request_fingerprint, payload_version, payload, billing_status,
			attempt_count, next_attempt_at, last_error, completed_at,
			settled_at, created_at, updated_at
	`,
		fact.RequestID,
		fact.APIKeyID,
		fact.UserID,
		fact.AccountID,
		strings.TrimSpace(fact.RequestFingerprint),
		fact.PayloadVersion,
		[]byte(fact.Payload),
		fact.NextAttemptAt,
		fact.CompletedAt,
	))
	if err == nil {
		return created, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}

	existing, err := scanUsageFact(r.db.QueryRowContext(ctx, `
		SELECT id, request_id, api_key_id, user_id, account_id,
			request_fingerprint, payload_version, payload, billing_status,
			attempt_count, next_attempt_at, last_error, completed_at,
			settled_at, created_at, updated_at
		FROM usage_facts
		WHERE request_id = $1 AND api_key_id = $2
	`, fact.RequestID, fact.APIKeyID))
	if err != nil {
		return nil, false, err
	}
	if strings.TrimSpace(existing.RequestFingerprint) != strings.TrimSpace(fact.RequestFingerprint) {
		return nil, false, service.ErrUsageBillingRequestConflict
	}
	return existing, false, nil
}

func (r *usageFactRepository) ClaimPending(ctx context.Context, limit int, now time.Time) ([]service.UsageFact, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("usage fact repository db is nil")
	}
	if limit <= 0 {
		return []service.UsageFact{}, nil
	}
	leaseUntil := now.Add(usageFactClaimLease)
	rows, err := r.db.QueryContext(ctx, `
		WITH candidates AS (
			SELECT id
			FROM usage_facts
			WHERE billing_status IN ('pending', 'settling')
				AND next_attempt_at <= $1
			ORDER BY next_attempt_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT $2
		)
		UPDATE usage_facts AS f
		SET billing_status = 'settling',
			attempt_count = attempt_count + 1,
			next_attempt_at = $3,
			updated_at = NOW()
		FROM candidates
		WHERE f.id = candidates.id
		RETURNING f.id, f.request_id, f.api_key_id, f.user_id, f.account_id,
			f.request_fingerprint, f.payload_version, f.payload, f.billing_status,
			f.attempt_count, f.next_attempt_at, f.last_error, f.completed_at,
			f.settled_at, f.created_at, f.updated_at
	`, now, limit, leaseUntil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	facts := make([]service.UsageFact, 0, limit)
	for rows.Next() {
		fact, err := scanUsageFact(rows)
		if err != nil {
			return nil, err
		}
		facts = append(facts, *fact)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return facts, nil
}

func (r *usageFactRepository) MarkSettled(ctx context.Context, id int64, settledAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE usage_facts
		SET billing_status = 'settled', settled_at = $2, last_error = '', updated_at = NOW()
		WHERE id = $1 AND billing_status IN ('pending', 'settling')
	`, id, settledAt)
	return err
}

func (r *usageFactRepository) MarkDebt(ctx context.Context, id int64, reason string, settledAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE usage_facts
		SET billing_status = 'debt', settled_at = $2, last_error = $3, updated_at = NOW()
		WHERE id = $1 AND billing_status IN ('pending', 'settling')
	`, id, settledAt, reason)
	return err
}

func (r *usageFactRepository) MarkRetry(ctx context.Context, id int64, reason string, nextAttemptAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE usage_facts
		SET billing_status = 'pending', next_attempt_at = $2, last_error = $3, updated_at = NOW()
		WHERE id = $1 AND billing_status IN ('pending', 'settling')
	`, id, nextAttemptAt, reason)
	return err
}

func scanUsageFact(scanner usageFactScanner) (*service.UsageFact, error) {
	var (
		fact      service.UsageFact
		payload   []byte
		settledAt sql.NullTime
	)
	if err := scanner.Scan(
		&fact.ID,
		&fact.RequestID,
		&fact.APIKeyID,
		&fact.UserID,
		&fact.AccountID,
		&fact.RequestFingerprint,
		&fact.PayloadVersion,
		&payload,
		&fact.BillingStatus,
		&fact.AttemptCount,
		&fact.NextAttemptAt,
		&fact.LastError,
		&fact.CompletedAt,
		&settledAt,
		&fact.CreatedAt,
		&fact.UpdatedAt,
	); err != nil {
		return nil, err
	}
	fact.Payload = append(json.RawMessage(nil), payload...)
	if settledAt.Valid {
		fact.SettledAt = &settledAt.Time
	}
	return &fact, nil
}
