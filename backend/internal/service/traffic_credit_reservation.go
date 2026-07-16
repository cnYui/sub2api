package service

import (
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

func PlanTrafficCreditReservations(batches []TrafficCreditBatch, amountUSD float64) ([]TrafficCreditReservationItem, bool) {
	if amountUSD <= 0 {
		return nil, true
	}
	remaining := roundTrafficCreditAmount(amountUSD)
	plan := make([]TrafficCreditReservationItem, 0, len(batches))
	for _, batch := range orderedTrafficCreditBatches(batches) {
		if remaining <= 0 {
			break
		}
		available := roundTrafficCreditAmount(math.Max(batch.RemainingUSD-batch.ReservedUSD, 0))
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
