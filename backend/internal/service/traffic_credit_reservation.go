package service

import (
	"context"
	"encoding/json"
	"math"
	"time"
)

type TrafficCreditReservationStatus string

const (
	TrafficCreditReservationReserved   TrafficCreditReservationStatus = "reserved"
	TrafficCreditReservationDispatched TrafficCreditReservationStatus = "dispatched"
	TrafficCreditReservationUnknown    TrafficCreditReservationStatus = "unknown"
	TrafficCreditReservationSettled    TrafficCreditReservationStatus = "settled"
	TrafficCreditReservationReleased   TrafficCreditReservationStatus = "released"
	TrafficCreditReservationDebt       TrafficCreditReservationStatus = "debt"
)

type TrafficCreditReservationInput struct {
	RequestID          string
	APIKeyID           int64
	UserID             int64
	Platform           string
	Model              string
	RequestFingerprint string
	PricingSnapshot    json.RawMessage
	ReserveUSD         float64
	ExpiresAt          time.Time
}

type TrafficCreditReservation struct {
	ID int64
	TrafficCreditReservationInput
	SettledUSD float64
	DebtUSD    float64
	Status     TrafficCreditReservationStatus
	LastError  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Items      []TrafficCreditReservationItem
}

type TrafficCreditReservationItem struct {
	CreditID    int64
	ReservedUSD float64
	SettledUSD  float64
}

type TrafficCreditReservationRepository interface {
	GetAvailableUSD(ctx context.Context, userID int64, platform string, now time.Time) (float64, error)
	Reserve(ctx context.Context, input TrafficCreditReservationInput) (*TrafficCreditReservation, bool, error)
	MarkDispatched(ctx context.Context, reservationID int64) error
	MarkUnknown(ctx context.Context, reservationID int64, reason string) error
	Release(ctx context.Context, reservationID int64, now time.Time) error
	ReleaseExpiredReserved(ctx context.Context, now time.Time, limit int) (int, error)
	HasOutstandingDebt(ctx context.Context, userID int64, platform string) (bool, error)
}

func PlanTrafficCreditReservations(batches []TrafficCreditBatch, amountUSD float64, policies ...TrafficCreditPolicy) ([]TrafficCreditReservationItem, bool) {
	if amountUSD <= 0 {
		return nil, true
	}
	policy := resolveTrafficCreditPolicy(policies)
	remaining := roundTrafficCreditAmount(amountUSD)
	plan := make([]TrafficCreditReservationItem, 0, len(batches))
	for _, batch := range orderedTrafficCreditBatches(batches) {
		if remaining <= 0 {
			break
		}
		available := policy.AvailableUSD(batch.RemainingUSD, batch.ReservedUSD)
		if available <= 0 {
			continue
		}
		amount := roundTrafficCreditAmount(math.Min(available, remaining))
		if amount <= 0 {
			continue
		}
		plan = append(plan, TrafficCreditReservationItem{
			CreditID:    batch.ID,
			ReservedUSD: amount,
		})
		remaining = roundTrafficCreditAmount(remaining - amount)
	}
	return plan, remaining <= 0
}
