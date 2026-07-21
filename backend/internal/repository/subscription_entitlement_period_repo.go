package repository

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionentitlementperiod"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type subscriptionEntitlementPeriodRepository struct {
	client *dbent.Client
}

func NewSubscriptionEntitlementPeriodRepository(client *dbent.Client) service.SubscriptionEntitlementPeriodRepository {
	return &subscriptionEntitlementPeriodRepository{client: client}
}

func (r *subscriptionEntitlementPeriodRepository) GetBySource(
	ctx context.Context,
	source service.SubscriptionEntitlementSource,
) (*service.SubscriptionEntitlementPeriod, error) {
	client := clientFromContext(ctx, r.client)
	period, err := client.SubscriptionEntitlementPeriod.Query().
		Where(
			subscriptionentitlementperiod.SourceTypeEQ(source.Type),
			subscriptionentitlementperiod.SourceIDEQ(source.ID),
		).
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrSubscriptionEntitlementPeriodNotFound, nil)
	}
	return subscriptionEntitlementPeriodEntityToService(period), nil
}

func (r *subscriptionEntitlementPeriodRepository) Create(
	ctx context.Context,
	period *service.SubscriptionEntitlementPeriod,
) error {
	if period == nil {
		return service.ErrSubscriptionEntitlementPeriodNilInput
	}

	client := clientFromContext(ctx, r.client)
	status := period.Status
	if status == "" {
		status = service.SubscriptionEntitlementPeriodStatusActive
	}
	builder := client.SubscriptionEntitlementPeriod.Create().
		SetUserID(period.UserID).
		SetSubscriptionID(period.SubscriptionID).
		SetGroupID(period.GroupID).
		SetSourceType(period.Source.Type).
		SetSourceID(period.Source.ID).
		SetStartsAt(period.StartsAt).
		SetExpiresAt(period.ExpiresAt).
		SetPeriodDays(period.PeriodDays).
		SetNillableDailyLimitUsd(period.DailyLimitUSD).
		SetNillableWeeklyLimitUsd(period.WeeklyLimitUSD).
		SetNillablePeriodTotalQuotaUsd(period.PeriodTotalQuotaUSD).
		SetStatus(status).
		SetNillableRevokedAt(period.RevokedAt).
		SetRevokedReason(period.RevokedReason)
	if period.QuotaWindowUnit != "" {
		builder.SetQuotaWindowUnit(period.QuotaWindowUnit)
	}
	if period.QuotaWindowDays > 0 {
		builder.SetQuotaWindowDays(period.QuotaWindowDays)
	}
	created, err := builder.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, nil, service.ErrSubscriptionEntitlementPeriodSourceExists)
	}
	period.ID = created.ID
	period.Status = created.Status
	return nil
}

func (r *subscriptionEntitlementPeriodRepository) RevokeUnexpiredBySubscription(
	ctx context.Context,
	subscriptionID int64,
	now time.Time,
	reason string,
) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.SubscriptionEntitlementPeriod.Update().
		Where(
			subscriptionentitlementperiod.SubscriptionIDEQ(subscriptionID),
			subscriptionentitlementperiod.StatusEQ(service.SubscriptionEntitlementPeriodStatusActive),
			subscriptionentitlementperiod.ExpiresAtGT(now),
		).
		SetStatus(service.SubscriptionEntitlementPeriodStatusRevoked).
		SetRevokedAt(now).
		SetRevokedReason(reason).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("revoke subscription entitlement periods: %w", err)
	}
	return nil
}

func (r *subscriptionEntitlementPeriodRepository) RevokeBySource(
	ctx context.Context,
	source service.SubscriptionEntitlementSource,
	now time.Time,
	reason string,
) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.SubscriptionEntitlementPeriod.Update().
		Where(
			subscriptionentitlementperiod.SourceTypeEQ(source.Type),
			subscriptionentitlementperiod.SourceIDEQ(source.ID),
			subscriptionentitlementperiod.StatusEQ(service.SubscriptionEntitlementPeriodStatusActive),
			subscriptionentitlementperiod.ExpiresAtGT(now),
		).
		SetStatus(service.SubscriptionEntitlementPeriodStatusRevoked).
		SetRevokedAt(now).
		SetRevokedReason(reason).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("revoke subscription entitlement period by source: %w", err)
	}
	return nil
}

func subscriptionEntitlementPeriodEntityToService(
	period *dbent.SubscriptionEntitlementPeriod,
) *service.SubscriptionEntitlementPeriod {
	if period == nil {
		return nil
	}
	return &service.SubscriptionEntitlementPeriod{
		ID:             period.ID,
		UserID:         period.UserID,
		SubscriptionID: period.SubscriptionID,
		GroupID:        period.GroupID,
		Source: service.SubscriptionEntitlementSource{
			Type: period.SourceType,
			ID:   period.SourceID,
		},
		StartsAt:            period.StartsAt,
		ExpiresAt:           period.ExpiresAt,
		PeriodDays:          period.PeriodDays,
		DailyLimitUSD:       cloneFloat64(period.DailyLimitUsd),
		WeeklyLimitUSD:      cloneFloat64(period.WeeklyLimitUsd),
		PeriodTotalQuotaUSD: cloneFloat64(period.PeriodTotalQuotaUsd),
		QuotaWindowUnit:     period.QuotaWindowUnit,
		QuotaWindowDays:     period.QuotaWindowDays,
		Status:              period.Status,
		RevokedAt:           cloneTime(period.RevokedAt),
		RevokedReason:       period.RevokedReason,
	}
}

func cloneFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
