package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	SubscriptionQuotaDebtStatusPending        = "pending"
	SubscriptionQuotaDebtStatusApplied        = "applied"
	SubscriptionQuotaDebtStatusAlreadyApplied = "already_applied"
	SubscriptionQuotaDebtStatusManualReview   = "manual_review"
)

var (
	ErrSubscriptionQuotaDebtAdjustmentNotFound = infraerrors.NotFound(
		"SUBSCRIPTION_QUOTA_DEBT_ADJUSTMENT_NOT_FOUND",
		"subscription quota debt adjustment not found",
	)
	ErrSubscriptionQuotaDebtAdjustmentNilInput = infraerrors.BadRequest(
		"SUBSCRIPTION_QUOTA_DEBT_ADJUSTMENT_NIL_INPUT",
		"subscription quota debt adjustment cannot be nil",
	)
	ErrSubscriptionQuotaDebtAdjustmentSourceExists = infraerrors.Conflict(
		"SUBSCRIPTION_QUOTA_DEBT_ADJUSTMENT_SOURCE_EXISTS",
		"subscription quota debt adjustment source already exists",
	)
)

// SubscriptionQuotaDebtAdjustment 是额度切换或人工审计产生的不可变抵扣事实。
type SubscriptionQuotaDebtAdjustment struct {
	ID                 int64
	SubscriptionID     int64
	UserID             int64
	GroupID            int64
	SourceKey          string
	OverageUSD         float64
	WeeklyLimitUSD     float64
	DailyEquivalentUSD float64
	RawDeductionDays   float64
	DeductedDays       int
	OriginalExpiresAt  time.Time
	NewExpiresAt       time.Time
	ApplicationStatus  string
	AppliedAt          *time.Time
	Notes              string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// SubscriptionQuotaDebtAdjustmentRepository 按 source_key 提供幂等审计事实持久化。
type SubscriptionQuotaDebtAdjustmentRepository interface {
	GetBySourceKey(ctx context.Context, sourceKey string) (*SubscriptionQuotaDebtAdjustment, error)
	Create(ctx context.Context, adjustment *SubscriptionQuotaDebtAdjustment) error
}
