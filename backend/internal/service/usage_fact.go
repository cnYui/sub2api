package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const UsageFactPayloadVersion1 = 1

const (
	UsageFactStatusPending  = "pending"
	UsageFactStatusSettling = "settling"
	UsageFactStatusSettled  = "settled"
	UsageFactStatusDebt     = "debt"
	UsageFactStatusFailed   = "failed"
)

var ErrUsageFactRequestIDRequired = errors.New("usage fact request_id is required")

type UsageFactPayload struct {
	BillingCommand UsageBillingCommand           `json:"billing_command"`
	UsageLog       UsageLog                      `json:"usage_log"`
	Effects        UsageSettlementEffectsPayload `json:"effects"`
	OpenAIBilling  *OpenAIUsageBillingSnapshot   `json:"openai_billing,omitempty"`
}

type UsageSettlementEffectsPayload struct {
	UserID                int64   `json:"user_id"`
	APIKeyID              int64   `json:"api_key_id"`
	AccountID             int64   `json:"account_id"`
	GroupID               *int64  `json:"group_id,omitempty"`
	Platform              string  `json:"platform"`
	ActualCost            float64 `json:"actual_cost"`
	TotalCost             float64 `json:"total_cost"`
	AccountRateMultiplier float64 `json:"account_rate_multiplier"`
	IsSubscription        bool    `json:"is_subscription"`
	IsTrafficCredit       bool    `json:"is_traffic_credit"`
}

type UsageFact struct {
	ID                 int64
	RequestID          string
	APIKeyID           int64
	UserID             int64
	AccountID          int64
	RequestFingerprint string
	ReservationID      *int64
	PayloadVersion     int
	Payload            json.RawMessage
	BillingStatus      string
	AttemptCount       int
	NextAttemptAt      time.Time
	LastError          string
	CompletedAt        time.Time
	SettledAt          *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type UsageFactRepository interface {
	CreatePending(ctx context.Context, fact *UsageFact) (*UsageFact, bool, error)
	FindByRequestID(ctx context.Context, requestID string) ([]UsageFact, error)
	ClaimPending(ctx context.Context, limit int, now, leaseUntil time.Time) ([]UsageFact, error)
	MarkSettled(ctx context.Context, id int64, settledAt time.Time) error
	MarkDebt(ctx context.Context, id int64, reason string, settledAt time.Time) error
	MarkRetry(ctx context.Context, id int64, reason string, nextAttemptAt time.Time) error
}

func EncodeUsageFactPayload(payload UsageFactPayload) (json.RawMessage, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

func DecodeUsageFactPayload(version int, raw json.RawMessage) (UsageFactPayload, error) {
	if version != UsageFactPayloadVersion1 {
		return UsageFactPayload{}, errors.New("unsupported usage fact payload version")
	}
	var payload UsageFactPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return UsageFactPayload{}, err
	}
	return payload, nil
}

func NewUsageFact(payload UsageFactPayload) (*UsageFact, error) {
	payload.BillingCommand.Normalize()
	if strings.TrimSpace(payload.BillingCommand.RequestID) == "" {
		return nil, ErrUsageFactRequestIDRequired
	}
	raw, err := EncodeUsageFactPayload(payload)
	if err != nil {
		return nil, err
	}
	nextAttemptAt := payload.BillingCommand.CompletedAt
	return &UsageFact{
		RequestID:          payload.BillingCommand.RequestID,
		APIKeyID:           payload.BillingCommand.APIKeyID,
		UserID:             payload.BillingCommand.UserID,
		AccountID:          payload.BillingCommand.AccountID,
		RequestFingerprint: payload.BillingCommand.RequestFingerprint,
		ReservationID:      payload.BillingCommand.TrafficCreditReservationID,
		PayloadVersion:     UsageFactPayloadVersion1,
		Payload:            raw,
		BillingStatus:      UsageFactStatusPending,
		NextAttemptAt:      nextAttemptAt,
		CompletedAt:        payload.BillingCommand.CompletedAt,
	}, nil
}
