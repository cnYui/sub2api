package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/user"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	SubscriptionEntitlementPeriodStatusActive  = "active"
	SubscriptionEntitlementPeriodStatusRevoked = "revoked"
)

var (
	ErrSubscriptionEntitlementPeriodNotFound = infraerrors.NotFound(
		"SUBSCRIPTION_ENTITLEMENT_PERIOD_NOT_FOUND",
		"subscription entitlement period not found",
	)
	ErrSubscriptionEntitlementPeriodNilInput = infraerrors.BadRequest(
		"SUBSCRIPTION_ENTITLEMENT_PERIOD_NIL_INPUT",
		"subscription entitlement period cannot be nil",
	)
	ErrSubscriptionEntitlementPeriodSourceExists = infraerrors.Conflict(
		"SUBSCRIPTION_ENTITLEMENT_PERIOD_SOURCE_EXISTS",
		"subscription entitlement period source already exists",
	)
	ErrInvalidSubscriptionEntitlementSource = infraerrors.BadRequest(
		"INVALID_SUBSCRIPTION_ENTITLEMENT_SOURCE",
		"subscription entitlement source type and id are required",
	)
	ErrSubscriptionEntitlementPeriodInvalidTerm = infraerrors.BadRequest(
		"INVALID_SUBSCRIPTION_ENTITLEMENT_PERIOD_TERM",
		"subscription entitlement period must contain at least one full day",
	)
	ErrSubscriptionEntitlementPeriodRepositoryUnavailable = infraerrors.InternalServer(
		"SUBSCRIPTION_ENTITLEMENT_PERIOD_REPOSITORY_UNAVAILABLE",
		"subscription entitlement period repository is unavailable",
	)
)

// SubscriptionEntitlementSource 标识一次不可重复的权益发放来源。
type SubscriptionEntitlementSource struct {
	Type string
	ID   string
}

// SubscriptionEntitlementPeriod 保存发放时不可变的套餐额度与期限快照。
type SubscriptionEntitlementPeriod struct {
	ID             int64
	UserID         int64
	SubscriptionID int64
	GroupID        int64
	Source         SubscriptionEntitlementSource
	StartsAt       time.Time
	ExpiresAt      time.Time
	PeriodDays     int
	DailyLimitUSD  *float64
	Status         string
	RevokedAt      *time.Time
	RevokedReason  string
}

// GrantResult 返回订阅与对应权益周期；Replayed 表示未重复发放。
type GrantResult struct {
	Subscription *UserSubscription
	Period       *SubscriptionEntitlementPeriod
	Replayed     bool
	Extended     bool
}

// SubscriptionEntitlementGrantResult 兼容计划中的旧命名。
type SubscriptionEntitlementGrantResult = GrantResult

func (source SubscriptionEntitlementSource) isZero() bool {
	return source.Type == "" && source.ID == ""
}

func (source SubscriptionEntitlementSource) validate() error {
	if strings.TrimSpace(source.Type) == "" || strings.TrimSpace(source.ID) == "" {
		return ErrInvalidSubscriptionEntitlementSource
	}
	if len(source.Type) > 40 || len(source.ID) > 128 {
		return ErrInvalidSubscriptionEntitlementSource
	}
	return nil
}

func hasSubscriptionEntitlementSource(input *AssignSubscriptionInput) bool {
	return input != nil && !input.EntitlementSource.isZero()
}

// GrantSubscriptionEntitlement 在同一事务内更新订阅和权益周期。
func (s *SubscriptionService) GrantSubscriptionEntitlement(ctx context.Context, input *AssignSubscriptionInput) (*GrantResult, error) {
	if input == nil {
		return nil, ErrSubscriptionNilInput
	}
	if err := input.EntitlementSource.validate(); err != nil {
		return nil, err
	}
	if s.entitlementPeriodRepo == nil {
		return nil, ErrSubscriptionEntitlementPeriodRepositoryUnavailable
	}

	callerOwnsTransaction := dbent.TxFromContext(ctx) != nil
	result, err := s.grantSubscriptionEntitlement(ctx, input)
	if err != nil && !callerOwnsTransaction && errors.Is(err, ErrSubscriptionEntitlementPeriodSourceExists) {
		result, err = s.replaySubscriptionEntitlement(ctx, input.EntitlementSource)
	}
	if err != nil {
		return nil, err
	}
	if !result.Replayed {
		s.invalidateSubscriptionCachesAfterCommit(ctx, input.UserID, input.GroupID)
	}
	return result, nil
}

func (s *SubscriptionService) grantSubscriptionEntitlement(ctx context.Context, input *AssignSubscriptionInput) (*GrantResult, error) {
	var result *GrantResult
	err := s.withSubscriptionUpdateTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = s.grantSubscriptionEntitlementInTx(txCtx, input)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *SubscriptionService) grantSubscriptionEntitlementInTx(ctx context.Context, input *AssignSubscriptionInput) (*GrantResult, error) {
	existingPeriod, err := s.entitlementPeriodRepo.GetBySource(ctx, input.EntitlementSource)
	if err == nil {
		return s.grantResultForExistingPeriod(ctx, existingPeriod)
	}
	if !errors.Is(err, ErrSubscriptionEntitlementPeriodNotFound) {
		return nil, fmt.Errorf("get subscription entitlement period by source: %w", err)
	}
	if err := s.lockSubscriptionEntitlementUser(ctx, input.UserID); err != nil {
		return nil, err
	}

	// 用户行锁会串行化同一用户的权益发放，锁后必须复查来源以避免等待期间的重复续期。
	existingPeriod, err = s.entitlementPeriodRepo.GetBySource(ctx, input.EntitlementSource)
	if err == nil {
		return s.grantResultForExistingPeriod(ctx, existingPeriod)
	}
	if !errors.Is(err, ErrSubscriptionEntitlementPeriodNotFound) {
		return nil, fmt.Errorf("recheck subscription entitlement period by source: %w", err)
	}

	group, err := s.groupRepo.GetByID(ctx, input.GroupID)
	if err != nil {
		return nil, fmt.Errorf("get subscription group: %w", err)
	}
	if !group.IsSubscriptionType() {
		return nil, ErrGroupNotSubscriptionType
	}

	now := s.currentTime()
	existingSub, err := s.userSubRepo.GetByUserIDAndGroupID(ctx, input.UserID, input.GroupID)
	if err != nil && !errors.Is(err, ErrSubscriptionNotFound) {
		return nil, fmt.Errorf("get subscription for entitlement: %w", err)
	}
	if errors.Is(err, ErrSubscriptionNotFound) {
		existingSub = nil
	}

	startsAt := now
	if existingSub != nil && existingSub.ExpiresAt.After(now) {
		startsAt = existingSub.ExpiresAt
	}
	expiresAt := startsAt.AddDate(0, 0, normalizeAssignValidityDays(input.ValidityDays))
	if expiresAt.After(MaxExpiresAt) {
		expiresAt = MaxExpiresAt
	}
	periodDays, err := subscriptionEntitlementPeriodDays(startsAt, expiresAt)
	if err != nil {
		return nil, err
	}

	var subscription *UserSubscription
	if existingSub == nil {
		subscription, err = s.createSubscriptionWithTerm(ctx, input, startsAt, expiresAt)
	} else if existingSub.ExpiresAt.After(now) {
		err = s.userSubRepo.ExtendExpiry(ctx, existingSub.ID, expiresAt)
		if err == nil && existingSub.Status != SubscriptionStatusActive {
			err = s.userSubRepo.UpdateStatus(ctx, existingSub.ID, SubscriptionStatusActive)
		}
		if err == nil && input.Notes != "" {
			err = s.userSubRepo.UpdateNotes(ctx, existingSub.ID, appendSubscriptionNotes(existingSub.Notes, input.Notes))
		}
		if err == nil {
			subscription, err = s.userSubRepo.GetByID(ctx, existingSub.ID)
		}
	} else {
		renewed := renewedSubscriptionTerm(existingSub, input.Notes, startsAt, expiresAt)
		if err = s.userSubRepo.Update(ctx, renewed); err == nil {
			subscription, err = s.userSubRepo.GetByID(ctx, existingSub.ID)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("apply subscription entitlement term: %w", err)
	}

	period := &SubscriptionEntitlementPeriod{
		UserID:         input.UserID,
		SubscriptionID: subscription.ID,
		GroupID:        input.GroupID,
		Source:         input.EntitlementSource,
		StartsAt:       startsAt,
		ExpiresAt:      expiresAt,
		PeriodDays:     periodDays,
		DailyLimitUSD:  cloneOptionalFloat64(group.DailyLimitUSD),
		Status:         SubscriptionEntitlementPeriodStatusActive,
	}
	if err := s.entitlementPeriodRepo.Create(ctx, period); err != nil {
		return nil, fmt.Errorf("create subscription entitlement period: %w", err)
	}
	return &GrantResult{
		Subscription: subscription,
		Period:       period,
		Extended:     existingSub != nil,
	}, nil
}

func (s *SubscriptionService) replaySubscriptionEntitlement(ctx context.Context, source SubscriptionEntitlementSource) (*GrantResult, error) {
	period, err := s.entitlementPeriodRepo.GetBySource(ctx, source)
	if err != nil {
		return nil, fmt.Errorf("reload subscription entitlement period: %w", err)
	}
	return s.grantResultForExistingPeriod(ctx, period)
}

func (s *SubscriptionService) grantResultForExistingPeriod(ctx context.Context, period *SubscriptionEntitlementPeriod) (*GrantResult, error) {
	subscription, err := s.userSubRepo.GetByID(ctx, period.SubscriptionID)
	if err != nil {
		return nil, fmt.Errorf("get replayed subscription: %w", err)
	}
	return &GrantResult{
		Subscription: subscription,
		Period:       period,
		Replayed:     true,
	}, nil
}

func (s *SubscriptionService) lockSubscriptionEntitlementUser(ctx context.Context, userID int64) error {
	client := s.entClient
	if tx := dbent.TxFromContext(ctx); tx != nil {
		client = tx.Client()
	}
	if client == nil {
		return nil
	}
	_, err := client.User.Query().Where(user.IDEQ(userID)).ForUpdate().Only(ctx)
	if dbent.IsNotFound(err) {
		return ErrUserNotFound.WithCause(err)
	}
	if err != nil {
		return fmt.Errorf("lock subscription entitlement user: %w", err)
	}
	return nil
}

func (s *SubscriptionService) createSubscriptionWithTerm(
	ctx context.Context,
	input *AssignSubscriptionInput,
	startsAt time.Time,
	expiresAt time.Time,
) (*UserSubscription, error) {
	subscription := &UserSubscription{
		UserID:     input.UserID,
		GroupID:    input.GroupID,
		StartsAt:   startsAt,
		ExpiresAt:  expiresAt,
		Status:     SubscriptionStatusActive,
		AssignedAt: startsAt,
		Notes:      input.Notes,
		CreatedAt:  startsAt,
		UpdatedAt:  startsAt,
	}
	if input.AssignedBy > 0 {
		subscription.AssignedBy = &input.AssignedBy
	}
	if err := s.userSubRepo.Create(ctx, subscription); err != nil {
		return nil, err
	}
	return s.userSubRepo.GetByID(ctx, subscription.ID)
}

func (s *SubscriptionService) currentTime() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func subscriptionEntitlementPeriodDays(startsAt, expiresAt time.Time) (int, error) {
	if !expiresAt.After(startsAt) {
		return 0, ErrSubscriptionEntitlementPeriodInvalidTerm
	}
	days := int(expiresAt.Sub(startsAt) / (24 * time.Hour))
	if days <= 0 {
		return 0, ErrSubscriptionEntitlementPeriodInvalidTerm
	}
	return days, nil
}

func cloneOptionalFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
