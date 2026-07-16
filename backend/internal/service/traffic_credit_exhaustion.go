package service

import (
	"context"
	"time"
)

type TrafficCreditExhaustionNotice struct {
	EventIDs []int64 `json:"event_ids"`
}

type TrafficCreditExhaustionRepository interface {
	ListPendingEventIDs(ctx context.Context, userID int64) ([]int64, error)
	AcknowledgeEvents(ctx context.Context, userID int64, eventIDs []int64, now time.Time) error
}
