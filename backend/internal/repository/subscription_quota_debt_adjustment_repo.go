package repository

import (
	"context"
	"fmt"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionquotadebtadjustment"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type subscriptionQuotaDebtAdjustmentRepository struct {
	client *dbent.Client
}

func NewSubscriptionQuotaDebtAdjustmentRepository(client *dbent.Client) service.SubscriptionQuotaDebtAdjustmentRepository {
	return &subscriptionQuotaDebtAdjustmentRepository{client: client}
}

func (r *subscriptionQuotaDebtAdjustmentRepository) GetBySourceKey(
	ctx context.Context,
	sourceKey string,
) (*service.SubscriptionQuotaDebtAdjustment, error) {
	client := clientFromContext(ctx, r.client)
	adjustment, err := client.SubscriptionQuotaDebtAdjustment.Query().
		Where(subscriptionquotadebtadjustment.SourceKeyEQ(sourceKey)).
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrSubscriptionQuotaDebtAdjustmentNotFound, nil)
	}
	return subscriptionQuotaDebtAdjustmentEntityToService(adjustment), nil
}

func (r *subscriptionQuotaDebtAdjustmentRepository) Create(
	ctx context.Context,
	adjustment *service.SubscriptionQuotaDebtAdjustment,
) error {
	if adjustment == nil {
		return service.ErrSubscriptionQuotaDebtAdjustmentNilInput
	}
	client := clientFromContext(ctx, r.client)
	status := adjustment.ApplicationStatus
	if status == "" {
		status = service.SubscriptionQuotaDebtStatusPending
	}
	created, err := client.SubscriptionQuotaDebtAdjustment.Create().
		SetSubscriptionID(adjustment.SubscriptionID).
		SetUserID(adjustment.UserID).
		SetGroupID(adjustment.GroupID).
		SetSourceKey(adjustment.SourceKey).
		SetOverageUsd(adjustment.OverageUSD).
		SetWeeklyLimitUsd(adjustment.WeeklyLimitUSD).
		SetDailyEquivalentUsd(adjustment.DailyEquivalentUSD).
		SetRawDeductionDays(adjustment.RawDeductionDays).
		SetDeductedDays(adjustment.DeductedDays).
		SetOriginalExpiresAt(adjustment.OriginalExpiresAt).
		SetNewExpiresAt(adjustment.NewExpiresAt).
		SetApplicationStatus(status).
		SetNillableAppliedAt(adjustment.AppliedAt).
		SetNotes(adjustment.Notes).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, nil, service.ErrSubscriptionQuotaDebtAdjustmentSourceExists)
	}
	*adjustment = *subscriptionQuotaDebtAdjustmentEntityToService(created)
	return nil
}

func subscriptionQuotaDebtAdjustmentEntityToService(
	adjustment *dbent.SubscriptionQuotaDebtAdjustment,
) *service.SubscriptionQuotaDebtAdjustment {
	if adjustment == nil {
		return nil
	}
	return &service.SubscriptionQuotaDebtAdjustment{
		ID:                 adjustment.ID,
		SubscriptionID:     adjustment.SubscriptionID,
		UserID:             adjustment.UserID,
		GroupID:            adjustment.GroupID,
		SourceKey:          adjustment.SourceKey,
		OverageUSD:         adjustment.OverageUsd,
		WeeklyLimitUSD:     adjustment.WeeklyLimitUsd,
		DailyEquivalentUSD: adjustment.DailyEquivalentUsd,
		RawDeductionDays:   adjustment.RawDeductionDays,
		DeductedDays:       adjustment.DeductedDays,
		OriginalExpiresAt:  adjustment.OriginalExpiresAt,
		NewExpiresAt:       adjustment.NewExpiresAt,
		ApplicationStatus:  adjustment.ApplicationStatus,
		AppliedAt:          cloneTime(adjustment.AppliedAt),
		Notes:              adjustment.Notes,
		CreatedAt:          adjustment.CreatedAt,
		UpdatedAt:          adjustment.UpdatedAt,
	}
}

func subscriptionQuotaDebtAdjustmentSourceKey(subscriptionID int64) string {
	return fmt.Sprintf("weekly_quota_cutover_overage:%d", subscriptionID)
}
