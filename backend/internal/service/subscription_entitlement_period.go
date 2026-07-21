package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"entgo.io/ent/dialect"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/user"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	SubscriptionEntitlementPeriodStatusActive  = "active"
	SubscriptionEntitlementPeriodStatusRevoked = "revoked"

	subscriptionEntitlementSourceTypePaymentOrder    = "payment_order"
	subscriptionEntitlementSourceTypeRedeemCode      = "redeem_code"
	subscriptionEntitlementSourceTypeSignupDefault   = "signup_default"
	subscriptionEntitlementSourceTypeProviderDefault = "provider_default"
	subscriptionEntitlementSourceTypeAdminAssignment = "admin_assignment"
	subscriptionEntitlementSourceTypeAdminAdjustment = "admin_adjustment"
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
	ID                  int64
	UserID              int64
	SubscriptionID      int64
	GroupID             int64
	Source              SubscriptionEntitlementSource
	StartsAt            time.Time
	ExpiresAt           time.Time
	PeriodDays          int
	DailyLimitUSD       *float64
	WeeklyLimitUSD      *float64
	PeriodTotalQuotaUSD *float64
	QuotaWindowUnit     string
	QuotaWindowDays     int
	Status              string
	RevokedAt           *time.Time
	RevokedReason       string
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

func paymentOrderSubscriptionEntitlementSource(orderID int64) SubscriptionEntitlementSource {
	return SubscriptionEntitlementSource{
		Type: subscriptionEntitlementSourceTypePaymentOrder,
		ID:   strconv.FormatInt(orderID, 10),
	}
}

func redeemCodeSubscriptionEntitlementSource(codeID int64) SubscriptionEntitlementSource {
	return SubscriptionEntitlementSource{
		Type: subscriptionEntitlementSourceTypeRedeemCode,
		ID:   strconv.FormatInt(codeID, 10),
	}
}

func signupDefaultSubscriptionEntitlementSource(userID, groupID int64) SubscriptionEntitlementSource {
	return SubscriptionEntitlementSource{
		Type: subscriptionEntitlementSourceTypeSignupDefault,
		ID:   fmt.Sprintf("%d:%d", userID, groupID),
	}
}

func providerDefaultSubscriptionEntitlementSource(grantID, groupID int64) SubscriptionEntitlementSource {
	return SubscriptionEntitlementSource{
		Type: subscriptionEntitlementSourceTypeProviderDefault,
		ID:   fmt.Sprintf("%d:%d", grantID, groupID),
	}
}

func adminAssignmentSubscriptionEntitlementSource(userID, groupID int64, validityDays int, notes string) SubscriptionEntitlementSource {
	digest := sha256.Sum256([]byte(strings.TrimSpace(notes)))
	return SubscriptionEntitlementSource{
		Type: subscriptionEntitlementSourceTypeAdminAssignment,
		ID:   fmt.Sprintf("%d:%d:%d:%x", userID, groupID, normalizeAssignValidityDays(validityDays), digest),
	}
}

func adminAdjustmentSubscriptionEntitlementSource(subscriptionID int64, newExpiresAt time.Time) SubscriptionEntitlementSource {
	return SubscriptionEntitlementSource{
		Type: subscriptionEntitlementSourceTypeAdminAdjustment,
		ID:   fmt.Sprintf("%d:%s", subscriptionID, newExpiresAt.UTC().Format(time.RFC3339Nano)),
	}
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

	group, err := s.resolveEntitlementGroup(ctx, input)
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
	validityDays := normalizeAssignValidityDays(input.ValidityDays)
	if group.UsesRollingWeeklyQuota() {
		validityDays = 28
	}
	expiresAt := startsAt.AddDate(0, 0, validityDays)
	if expiresAt.After(MaxExpiresAt) {
		expiresAt = MaxExpiresAt
	}
	periodDays, err := subscriptionEntitlementPeriodDays(startsAt, expiresAt)
	if err != nil {
		return nil, err
	}

	var subscription *UserSubscription
	if existingSub == nil {
		subscription, err = s.createSubscriptionWithTerm(ctx, input, group, startsAt, expiresAt)
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
	if group.UsesRollingWeeklyQuota() && subscription.WeeklyAnchorAt == nil {
		anchor := subscription.StartsAt
		subscription.WeeklyAnchorAt = &anchor
		if subscription.WeeklyWindowStart == nil {
			subscription.WeeklyWindowStart = &anchor
		}
		if err := s.userSubRepo.Update(ctx, subscription); err != nil {
			return nil, fmt.Errorf("set rolling weekly anchor: %w", err)
		}
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
	if group.UsesRollingWeeklyQuota() {
		period.WeeklyLimitUSD = cloneOptionalFloat64(input.WeeklyLimitUSD)
		if period.WeeklyLimitUSD == nil {
			period.WeeklyLimitUSD = cloneOptionalFloat64(group.EffectiveWeeklyLimitUSD())
		}
		period.PeriodTotalQuotaUSD = cloneOptionalFloat64(input.PeriodTotalQuotaUSD)
		if period.PeriodTotalQuotaUSD == nil && period.WeeklyLimitUSD != nil {
			total := *period.WeeklyLimitUSD * 4
			period.PeriodTotalQuotaUSD = &total
		}
		period.QuotaWindowUnit = input.QuotaWindowUnit
		if period.QuotaWindowUnit == "" {
			period.QuotaWindowUnit = "week"
		}
		period.QuotaWindowDays = input.QuotaWindowDays
		if period.QuotaWindowDays <= 0 {
			period.QuotaWindowDays = subscriptionWeeklyWindowDays
		}
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

func (s *SubscriptionService) resolveEntitlementGroup(ctx context.Context, input *AssignSubscriptionInput) (*Group, error) {
	if input != nil && input.GroupSnapshot != nil {
		group := *input.GroupSnapshot
		if group.ID == 0 {
			group.ID = input.GroupID
		}
		group.Hydrated = true
		return &group, nil
	}
	return s.groupRepo.GetByID(ctx, input.GroupID)
}

func (s *SubscriptionService) lockSubscriptionEntitlementUser(ctx context.Context, userID int64) error {
	client := s.entClient
	if tx := dbent.TxFromContext(ctx); tx != nil {
		client = tx.Client()
	}
	if client == nil {
		return nil
	}
	query := client.User.Query().Where(user.IDEQ(userID))
	if client.Driver() != nil && client.Driver().Dialect() == dialect.Postgres {
		query = query.ForUpdate()
	}
	_, err := query.Only(ctx)
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
	group *Group,
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
	if group != nil && group.UsesRollingWeeklyQuota() {
		anchor := startsAt
		subscription.WeeklyAnchorAt = &anchor
		subscription.WeeklyWindowStart = &anchor
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
