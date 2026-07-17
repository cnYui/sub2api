//go:build integration

package offlinepaymentbackfill

import (
	"context"
	"database/sql"
	"time"
)

type OfflinePaymentBackfillEntry struct {
	SubscriptionID int64
	UserID         int64
	PaidAt         time.Time
	ExpectedExpiry time.Time
}

type OfflinePaymentBackfillBatch struct {
	Source  string
	PlanID  int64
	GroupID int64
	Entries []OfflinePaymentBackfillEntry
}

func RunOfflinePaymentBackfillBatch(ctx context.Context, db *sql.DB, batch OfflinePaymentBackfillBatch, operator string, execute bool) (OfflinePaymentBackfillResult, error) {
	entries := make([]offlinePaymentBackfillEntry, 0, len(batch.Entries))
	for _, entry := range batch.Entries {
		entries = append(entries, offlinePaymentBackfillEntry{
			SubscriptionID: entry.SubscriptionID,
			UserID:         entry.UserID,
			PaidAt:         entry.PaidAt,
			ExpectedExpiry: entry.ExpectedExpiry,
		})
	}
	return runOfflinePaymentBackfillBatch(ctx, db, offlinePaymentBackfillBatch{
		Source:  batch.Source,
		PlanID:  batch.PlanID,
		GroupID: batch.GroupID,
		Entries: entries,
	}, operator, execute)
}
