package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/subscriptionplan"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// validatePlanRequired checks that all required fields for a plan are provided.
func validatePlanRequired(name string, groupID int64, price float64, validityDays int, validityUnit string, originalPrice *float64) error {
	if strings.TrimSpace(name) == "" {
		return infraerrors.BadRequest("PLAN_NAME_REQUIRED", "plan name is required")
	}
	if groupID <= 0 {
		return infraerrors.BadRequest("PLAN_GROUP_REQUIRED", "group is required")
	}
	if price <= 0 {
		return infraerrors.BadRequest("PLAN_PRICE_INVALID", "price must be > 0")
	}
	if validityDays <= 0 {
		return infraerrors.BadRequest("PLAN_VALIDITY_REQUIRED", "validity days must be > 0")
	}
	if strings.TrimSpace(validityUnit) == "" {
		return infraerrors.BadRequest("PLAN_VALIDITY_UNIT_REQUIRED", "validity unit is required")
	}
	if originalPrice != nil && *originalPrice < 0 {
		return infraerrors.BadRequest("PLAN_ORIGINAL_PRICE_INVALID", "original price must be >= 0")
	}
	return nil
}

// validatePlanPatch validates only the non-nil fields in a patch update.
func validatePlanPatch(req UpdatePlanRequest) error {
	if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
		return infraerrors.BadRequest("PLAN_NAME_REQUIRED", "plan name is required")
	}
	if req.GroupID != nil && *req.GroupID <= 0 {
		return infraerrors.BadRequest("PLAN_GROUP_REQUIRED", "group is required")
	}
	if req.Price != nil && *req.Price <= 0 {
		return infraerrors.BadRequest("PLAN_PRICE_INVALID", "price must be > 0")
	}
	if req.ValidityDays != nil && *req.ValidityDays <= 0 {
		return infraerrors.BadRequest("PLAN_VALIDITY_REQUIRED", "validity days must be > 0")
	}
	if req.ValidityUnit != nil && strings.TrimSpace(*req.ValidityUnit) == "" {
		return infraerrors.BadRequest("PLAN_VALIDITY_UNIT_REQUIRED", "validity unit is required")
	}
	if req.OriginalPrice != nil && *req.OriginalPrice < 0 {
		return infraerrors.BadRequest("PLAN_ORIGINAL_PRICE_INVALID", "original price must be >= 0")
	}
	return nil
}

// --- Plan CRUD ---

// PlanGroupInfo holds the group details needed for subscription plan display.
type PlanGroupInfo struct {
	Platform        string   `json:"platform"`
	Name            string   `json:"name"`
	RateMultiplier  float64  `json:"rate_multiplier"`
	DailyLimitUSD   *float64 `json:"daily_limit_usd"`
	WeeklyLimitUSD  *float64 `json:"weekly_limit_usd"`
	MonthlyLimitUSD *float64 `json:"monthly_limit_usd"`
	ModelScopes     []string `json:"supported_model_scopes"`
}

// PlanQuotaSnapshot 是购买页和下单快照共用的套餐额度窗口描述。
type PlanQuotaSnapshot struct {
	DailyLimitUSD         *float64 `json:"daily_limit_usd,omitempty"`
	WeeklyLimitUSD        *float64 `json:"weekly_limit_usd,omitempty"`
	MonthlyLimitUSD       *float64 `json:"monthly_limit_usd,omitempty"`
	PeriodTotalQuotaUSD   *float64 `json:"period_total_quota_usd,omitempty"`
	QuotaWindowUnit       string   `json:"quota_window_unit"`
	QuotaWindowDays       int      `json:"quota_window_days"`
	EffectiveValidityDays int      `json:"effective_validity_days"`
}

// PlanDisplaySnapshot 是公共购买页、结账页和管理端共享的套餐可见文案。
type PlanDisplaySnapshot struct {
	Name        string
	Description string
	Features    string
	ProductName string
}

func BuildPlanQuotaSnapshot(groupName string, dailyLimit, weeklyLimit, monthlyLimit *float64, validityDays int, validityUnit string) PlanQuotaSnapshot {
	effectiveDays := psComputeValidityDays(validityDays, validityUnit)
	snapshot := PlanQuotaSnapshot{
		DailyLimitUSD:         cloneOptionalFloat64(dailyLimit),
		WeeklyLimitUSD:        cloneOptionalFloat64(weeklyLimit),
		MonthlyLimitUSD:       cloneOptionalFloat64(monthlyLimit),
		QuotaWindowUnit:       "day",
		QuotaWindowDays:       1,
		EffectiveValidityDays: effectiveDays,
	}
	if limit, ok := PublicCodexSubscriptionWeeklyLimitUSD(groupName); ok {
		weekly := limit
		total := limit * 4
		snapshot.DailyLimitUSD = nil
		snapshot.WeeklyLimitUSD = &weekly
		snapshot.MonthlyLimitUSD = nil
		snapshot.PeriodTotalQuotaUSD = &total
		snapshot.QuotaWindowUnit = "week"
		snapshot.QuotaWindowDays = subscriptionWeeklyWindowDays
		snapshot.EffectiveValidityDays = publicCodexSubscriptionValidityDays
		return snapshot
	}
	if snapshot.WeeklyLimitUSD != nil && *snapshot.WeeklyLimitUSD > 0 {
		snapshot.QuotaWindowUnit = "week"
		snapshot.QuotaWindowDays = subscriptionWeeklyWindowDays
		return snapshot
	}
	if snapshot.MonthlyLimitUSD != nil && *snapshot.MonthlyLimitUSD > 0 {
		snapshot.QuotaWindowUnit = "month"
		snapshot.QuotaWindowDays = 30
	}
	return snapshot
}

func BuildPlanDisplaySnapshot(planName, description, features, productName string, price float64, groupName string, quota PlanQuotaSnapshot) PlanDisplaySnapshot {
	display := PlanDisplaySnapshot{
		Name:        planName,
		Description: description,
		Features:    features,
		ProductName: productName,
	}
	weeklyLimit, ok := PublicCodexSubscriptionWeeklyLimitUSD(groupName)
	if !ok {
		return display
	}
	if quota.WeeklyLimitUSD != nil {
		weeklyLimit = *quota.WeeklyLimitUSD
	}
	weeklyLimitText := strconv.FormatFloat(weeklyLimit, 'f', -1, 64)
	display.Description = fmt.Sprintf("28 天订阅，每 7 天刷新 %s USD 周额度，购买时间起滚动计算", weeklyLimitText)
	display.Features = strings.Join([]string{
		fmt.Sprintf("周额度 %s USD", weeklyLimitText),
		"28 天有效期",
		"购买时间起每 7 天刷新",
	}, "\n")

	name := strings.TrimSpace(display.Name)
	product := strings.TrimSpace(display.ProductName)
	if name == "" || containsLegacySubscriptionQuotaCopy(name) {
		switch {
		case product != "" && !containsLegacySubscriptionQuotaCopy(product):
			name = product
		case price > 0:
			name = fmt.Sprintf("%s 元订阅池", strconv.FormatFloat(price, 'f', -1, 64))
		default:
			name = groupName
		}
	}
	display.Name = name
	if product == "" || containsLegacySubscriptionQuotaCopy(product) {
		display.ProductName = name
	}
	return display
}

// NormalizeSubscriptionPlanForDisplay 将订阅套餐实体覆盖为当前应展示的公开文案。
func NormalizeSubscriptionPlanForDisplay(plan *dbent.SubscriptionPlan, groupInfo PlanGroupInfo) {
	if plan == nil {
		return
	}
	quota := BuildPlanQuotaSnapshot(groupInfo.Name, groupInfo.DailyLimitUSD, groupInfo.WeeklyLimitUSD, groupInfo.MonthlyLimitUSD, plan.ValidityDays, plan.ValidityUnit)
	display := BuildPlanDisplaySnapshot(plan.Name, plan.Description, plan.Features, plan.ProductName, plan.Price, groupInfo.Name, quota)
	plan.Name = display.Name
	plan.Description = display.Description
	plan.Features = display.Features
	plan.ProductName = display.ProductName
	if _, ok := PublicCodexSubscriptionWeeklyLimitUSD(groupInfo.Name); ok {
		plan.ValidityDays = publicCodexSubscriptionValidityDays
		plan.ValidityUnit = "day"
	}
}

// NormalizeSubscriptionPlansForDisplay 批量规范化订阅套餐实体。
func NormalizeSubscriptionPlansForDisplay(plans []*dbent.SubscriptionPlan, groupInfoMap map[int64]PlanGroupInfo) {
	for _, plan := range plans {
		if plan == nil {
			continue
		}
		groupInfo, ok := groupInfoMap[plan.GroupID]
		if !ok {
			continue
		}
		NormalizeSubscriptionPlanForDisplay(plan, groupInfo)
	}
}

func containsLegacySubscriptionQuotaCopy(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, token := range []string{"月度", "30天", "30 天", "日限额", "每日", "24点", "24 点"} {
		if strings.Contains(value, token) {
			return true
		}
	}
	return false
}

// GetGroupPlatformMap returns a map of group_id → platform for the given plans.
func (s *PaymentConfigService) GetGroupPlatformMap(ctx context.Context, plans []*dbent.SubscriptionPlan) map[int64]string {
	info := s.GetGroupInfoMap(ctx, plans)
	m := make(map[int64]string, len(info))
	for id, gi := range info {
		m[id] = gi.Platform
	}
	return m
}

// GetGroupInfoMap returns a map of group_id → PlanGroupInfo for the given plans.
func (s *PaymentConfigService) GetGroupInfoMap(ctx context.Context, plans []*dbent.SubscriptionPlan) map[int64]PlanGroupInfo {
	ids := make([]int64, 0, len(plans))
	seen := make(map[int64]bool)
	for _, p := range plans {
		if !seen[p.GroupID] {
			seen[p.GroupID] = true
			ids = append(ids, p.GroupID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	groups, err := s.entClient.Group.Query().Where(group.IDIn(ids...)).All(ctx)
	if err != nil {
		return nil
	}
	m := make(map[int64]PlanGroupInfo, len(groups))
	for _, g := range groups {
		quota := BuildPlanQuotaSnapshot(g.Name, g.DailyLimitUsd, g.WeeklyLimitUsd, g.MonthlyLimitUsd, 0, "day")
		m[int64(g.ID)] = PlanGroupInfo{
			Platform:        g.Platform,
			Name:            g.Name,
			RateMultiplier:  g.RateMultiplier,
			DailyLimitUSD:   quota.DailyLimitUSD,
			WeeklyLimitUSD:  quota.WeeklyLimitUSD,
			MonthlyLimitUSD: quota.MonthlyLimitUSD,
			ModelScopes:     g.SupportedModelScopes,
		}
	}
	return m
}

func (s *PaymentConfigService) ListPlans(ctx context.Context) ([]*dbent.SubscriptionPlan, error) {
	return s.entClient.SubscriptionPlan.Query().Order(subscriptionplan.BySortOrder()).All(ctx)
}

func (s *PaymentConfigService) ListPlansForSale(ctx context.Context) ([]*dbent.SubscriptionPlan, error) {
	return s.entClient.SubscriptionPlan.Query().Where(subscriptionplan.ForSaleEQ(true)).Order(subscriptionplan.BySortOrder()).All(ctx)
}

func (s *PaymentConfigService) CreatePlan(ctx context.Context, req CreatePlanRequest) (*dbent.SubscriptionPlan, error) {
	if err := validatePlanRequired(req.Name, req.GroupID, req.Price, req.ValidityDays, req.ValidityUnit, req.OriginalPrice); err != nil {
		return nil, err
	}
	display, isPublicCodex, err := s.planDisplayForGroupID(ctx, req.GroupID, req.Name, req.Description, req.Features, req.ProductName, req.Price, req.ValidityDays, req.ValidityUnit)
	if err != nil {
		return nil, err
	}
	validityDays := req.ValidityDays
	validityUnit := req.ValidityUnit
	if isPublicCodex {
		validityDays = publicCodexSubscriptionValidityDays
		validityUnit = "day"
	}
	b := s.entClient.SubscriptionPlan.Create().
		SetGroupID(req.GroupID).SetName(display.Name).SetDescription(display.Description).
		SetPrice(req.Price).SetValidityDays(validityDays).SetValidityUnit(validityUnit).
		SetFeatures(display.Features).SetProductName(display.ProductName).
		SetForSale(req.ForSale).SetSortOrder(req.SortOrder)
	if req.OriginalPrice != nil {
		b.SetOriginalPrice(*req.OriginalPrice)
	}
	return b.Save(ctx)
}

// UpdatePlan updates a subscription plan by ID (patch semantics).
// NOTE: This function exceeds 30 lines due to per-field nil-check patch update boilerplate
// plus a validation guard for non-nil fields.
func (s *PaymentConfigService) UpdatePlan(ctx context.Context, id int64, req UpdatePlanRequest) (*dbent.SubscriptionPlan, error) {
	if err := validatePlanPatch(req); err != nil {
		return nil, err
	}
	existing, err := s.entClient.SubscriptionPlan.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	targetGroupID := existing.GroupID
	if req.GroupID != nil {
		targetGroupID = *req.GroupID
	}
	name := existing.Name
	if req.Name != nil {
		name = *req.Name
	}
	description := existing.Description
	if req.Description != nil {
		description = *req.Description
	}
	features := existing.Features
	if req.Features != nil {
		features = *req.Features
	}
	productName := existing.ProductName
	if req.ProductName != nil {
		productName = *req.ProductName
	}
	price := existing.Price
	if req.Price != nil {
		price = *req.Price
	}
	validityDays := existing.ValidityDays
	if req.ValidityDays != nil {
		validityDays = *req.ValidityDays
	}
	validityUnit := existing.ValidityUnit
	if req.ValidityUnit != nil {
		validityUnit = *req.ValidityUnit
	}
	display, forcePublicCodexValidity, err := s.planDisplayForGroupID(ctx, targetGroupID, name, description, features, productName, price, validityDays, validityUnit)
	if err != nil {
		return nil, err
	}
	u := s.entClient.SubscriptionPlan.UpdateOneID(id)
	if req.GroupID != nil {
		u.SetGroupID(*req.GroupID)
	}
	if req.Name != nil {
		u.SetName(*req.Name)
	}
	if req.Description != nil {
		u.SetDescription(*req.Description)
	}
	if req.Price != nil {
		u.SetPrice(*req.Price)
	}
	if req.OriginalPrice != nil {
		u.SetOriginalPrice(*req.OriginalPrice)
	}
	if forcePublicCodexValidity {
		u.SetName(display.Name)
		u.SetDescription(display.Description)
		u.SetFeatures(display.Features)
		u.SetProductName(display.ProductName)
		u.SetValidityDays(publicCodexSubscriptionValidityDays)
		u.SetValidityUnit("day")
	} else {
		if req.ValidityDays != nil {
			u.SetValidityDays(*req.ValidityDays)
		}
		if req.ValidityUnit != nil {
			u.SetValidityUnit(*req.ValidityUnit)
		}
	}
	if req.Features != nil {
		u.SetFeatures(*req.Features)
	}
	if req.ProductName != nil {
		u.SetProductName(*req.ProductName)
	}
	if req.ForSale != nil {
		u.SetForSale(*req.ForSale)
	}
	if req.SortOrder != nil {
		u.SetSortOrder(*req.SortOrder)
	}
	return u.Save(ctx)
}

func (s *PaymentConfigService) planDisplayForGroupID(ctx context.Context, groupID int64, name, description, features, productName string, price float64, validityDays int, validityUnit string) (PlanDisplaySnapshot, bool, error) {
	display := PlanDisplaySnapshot{Name: name, Description: description, Features: features, ProductName: productName}
	groupEntity, err := s.entClient.Group.Query().Where(group.IDEQ(groupID)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return display, false, nil
		}
		return display, false, fmt.Errorf("load group for public codex plan display normalization: %w", err)
	}
	quota := BuildPlanQuotaSnapshot(groupEntity.Name, groupEntity.DailyLimitUsd, groupEntity.WeeklyLimitUsd, groupEntity.MonthlyLimitUsd, validityDays, validityUnit)
	isPublicCodex := groupEntity.SubscriptionType == SubscriptionTypeSubscription
	if isPublicCodex {
		_, isPublicCodex = PublicCodexSubscriptionWeeklyLimitUSD(groupEntity.Name)
	}
	if !isPublicCodex {
		return display, false, nil
	}
	return BuildPlanDisplaySnapshot(name, description, features, productName, price, groupEntity.Name, quota), true, nil
}

func (s *PaymentConfigService) DeletePlan(ctx context.Context, id int64) error {
	count, err := s.countPendingOrdersByPlan(ctx, id)
	if err != nil {
		return fmt.Errorf("check pending orders: %w", err)
	}
	if count > 0 {
		return infraerrors.Conflict("PENDING_ORDERS",
			fmt.Sprintf("this plan has %d in-progress orders and cannot be deleted — wait for orders to complete first", count))
	}
	return s.entClient.SubscriptionPlan.DeleteOneID(id).Exec(ctx)
}

// GetPlan returns a subscription plan by ID.
func (s *PaymentConfigService) GetPlan(ctx context.Context, id int64) (*dbent.SubscriptionPlan, error) {
	plan, err := s.entClient.SubscriptionPlan.Get(ctx, id)
	if err != nil {
		return nil, infraerrors.NotFound("PLAN_NOT_FOUND", "subscription plan not found")
	}
	return plan, nil
}
