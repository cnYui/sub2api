package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/balancepackageplan"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/ent/userbalancepackage"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	balancePackageStatusActive      = "active"
	balancePackageStatusCompleted   = "completed"
	balancePackageStatusDebtPaused  = "debt_paused"
	balancePackageStatusExpired     = "expired"
	balancePackageStatusCancelled   = "cancelled"
	balancePackageCreditBatchSize   = 100
	balancePackageRenewalAudit      = "BALANCE_PACKAGE_RENEWAL"
	balancePackageWeeklyCreditAudit = "BALANCE_PACKAGE_WEEKLY_CREDIT"
	balancePackageDebtPausedAudit   = "BALANCE_PACKAGE_DEBT_PAUSED"
	balancePackageDebtResumedAudit  = "BALANCE_PACKAGE_DEBT_RESUMED"
	balancePackageManualCancelAudit = "BALANCE_PACKAGE_MANUAL_CANCELLATION"
)

type defaultBalancePackagePlan struct {
	code            string
	name            string
	priceCNY        float64
	weeklyCreditUSD float64
	sortOrder       int
}

// UserBalancePackageView 是用户端展示已购余额套餐所需的服务端数据。
// 套餐计划名称和价格来自当前计划记录，到账进度和生命周期来自不可变的用户套餐记录。
type UserBalancePackageView struct {
	ID                  int64      `json:"id"`
	PlanID              int64      `json:"plan_id"`
	Code                string     `json:"code"`
	Name                string     `json:"name"`
	PriceCNY            float64    `json:"price_cny"`
	WeeklyCreditUSD     float64    `json:"weekly_credit_usd"`
	CurrentRemainingUSD float64    `json:"current_remaining_usd"`
	ValidityDays        int        `json:"validity_days"`
	RefreshCount        int        `json:"refresh_count"`
	RefreshIntervalDays int        `json:"refresh_interval_days"`
	CreditedCount       int        `json:"credited_count"`
	RenewalCount        int        `json:"renewal_count"`
	StartsAt            time.Time  `json:"starts_at"`
	NextCreditAt        *time.Time `json:"next_credit_at,omitempty"`
	ExpiresAt           time.Time  `json:"expires_at"`
	Status              string     `json:"status"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

var defaultBalancePackagePlans = []defaultBalancePackagePlan{
	{code: "balance-29", name: "余额套餐 ¥29", priceCNY: 29, weeklyCreditUSD: 76, sortOrder: 10},
	{code: "balance-39", name: "余额套餐 ¥39", priceCNY: 39, weeklyCreditUSD: 102, sortOrder: 20},
	{code: "balance-49", name: "余额套餐 ¥49", priceCNY: 49, weeklyCreditUSD: 128, sortOrder: 30},
	{code: "balance-59", name: "余额套餐 ¥59", priceCNY: 59, weeklyCreditUSD: 154, sortOrder: 40},
	{code: "balance-79", name: "余额套餐 ¥79", priceCNY: 79, weeklyCreditUSD: 206, sortOrder: 50},
	{code: "balance-99", name: "余额套餐 ¥99", priceCNY: 99, weeklyCreditUSD: 258, sortOrder: 60},
	{code: "balance-149", name: "余额套餐 ¥149", priceCNY: 149, weeklyCreditUSD: 389, sortOrder: 70},
	{code: "balance-199", name: "余额套餐 ¥199", priceCNY: 199, weeklyCreditUSD: 520, sortOrder: 80},
	{code: "balance-249", name: "余额套餐 ¥249", priceCNY: 249, weeklyCreditUSD: 651, sortOrder: 90},
	{code: "balance-299", name: "余额套餐 ¥299", priceCNY: 299, weeklyCreditUSD: 781, sortOrder: 100},
}

// BalancePackageService 维护余额套餐及其到账生命周期。
type BalancePackageService struct {
	entClient    *dbent.Client
	billingCache *BillingCacheService
}

func NewBalancePackageService(entClient *dbent.Client) *BalancePackageService {
	return &BalancePackageService{entClient: entClient}
}

func (s *BalancePackageService) SetBillingCache(billingCache *BillingCacheService) {
	if s != nil {
		s.billingCache = billingCache
	}
}

func (s *BalancePackageService) ListPlansForSale(ctx context.Context) ([]*dbent.BalancePackagePlan, error) {
	if err := s.ensureDefaultPlans(ctx); err != nil {
		return nil, err
	}
	return s.entClient.BalancePackagePlan.Query().
		Where(balancepackageplan.ForSaleEQ(true)).
		Order(balancepackageplan.BySortOrder()).
		All(ctx)
}

func (s *BalancePackageService) GetPlanForSale(ctx context.Context, id int64) (*dbent.BalancePackagePlan, error) {
	if id <= 0 {
		return nil, infraerrors.BadRequest("BALANCE_PACKAGE_REQUIRED", "balance package is required")
	}
	plan, err := s.entClient.BalancePackagePlan.Get(ctx, id)
	if err != nil || !plan.ForSale {
		return nil, infraerrors.NotFound("BALANCE_PACKAGE_NOT_AVAILABLE", "balance package is not available")
	}
	if err := validateBalancePackagePlan(plan); err != nil {
		return nil, err
	}
	return plan, nil
}

// ListUserPackages 返回用户已购买或已获发放的余额套餐，供订阅页展示。
func (s *BalancePackageService) ListUserPackages(ctx context.Context, userID int64) ([]UserBalancePackageView, error) {
	if s == nil || s.entClient == nil {
		return nil, fmt.Errorf("balance package service is unavailable")
	}
	if userID <= 0 {
		return []UserBalancePackageView{}, nil
	}

	now := time.Now().UTC()
	packages, err := s.entClient.UserBalancePackage.Query().
		Where(
			userbalancepackage.UserIDEQ(userID),
			userbalancepackage.StatusIn(balancePackageStatusActive, balancePackageStatusCompleted, balancePackageStatusDebtPaused),
			userbalancepackage.ExpiresAtGT(now),
		).
		Order(userbalancepackage.ByCreatedAt(sql.OrderDesc()), userbalancepackage.ByID(sql.OrderDesc())).
		Limit(1).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list user balance packages: %w", err)
	}
	if len(packages) == 0 {
		return []UserBalancePackageView{}, nil
	}

	planIDs := make([]int64, 0, len(packages))
	seenPlanIDs := make(map[int64]struct{}, len(packages))
	for _, item := range packages {
		if _, ok := seenPlanIDs[item.PlanID]; ok {
			continue
		}
		seenPlanIDs[item.PlanID] = struct{}{}
		planIDs = append(planIDs, item.PlanID)
	}
	plans, err := s.entClient.BalancePackagePlan.Query().
		Where(balancepackageplan.IDIn(planIDs...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list balance package plans: %w", err)
	}
	planByID := make(map[int64]*dbent.BalancePackagePlan, len(plans))
	for _, plan := range plans {
		planByID[plan.ID] = plan
	}

	result := make([]UserBalancePackageView, 0, len(packages))
	for _, item := range packages {
		plan := planByID[item.PlanID]
		view := UserBalancePackageView{
			ID:                  item.ID,
			PlanID:              item.PlanID,
			WeeklyCreditUSD:     item.WeeklyCreditUsd,
			CurrentRemainingUSD: item.RemainingUsd,
			ValidityDays:        int(item.ExpiresAt.Sub(item.StartsAt) / (24 * time.Hour)),
			RefreshCount:        item.RefreshCount,
			RefreshIntervalDays: item.RefreshIntervalDays,
			CreditedCount:       item.CreditedCount,
			RenewalCount:        item.RenewalCount,
			StartsAt:            item.StartsAt,
			NextCreditAt:        item.NextCreditAt,
			ExpiresAt:           item.ExpiresAt,
			Status:              item.Status,
			CreatedAt:           item.CreatedAt,
			UpdatedAt:           item.UpdatedAt,
		}
		if plan != nil {
			view.Code = plan.Code
			view.Name = plan.Name
			view.PriceCNY = plan.PriceCny
		}
		if view.Name == "" {
			view.Name = fmt.Sprintf("余额套餐 #%d", item.PlanID)
		}
		if view.ValidityDays <= 0 {
			view.ValidityDays = int(item.ExpiresAt.Sub(item.StartsAt) / (24 * time.Hour))
		}
		if item.ExpiresAt.After(item.StartsAt) {
			view.ValidityDays = int(item.ExpiresAt.Sub(item.StartsAt) / (24 * time.Hour))
		}
		if view.Status == balancePackageStatusActive && item.CreditedCount >= item.RefreshCount {
			view.Status = balancePackageStatusCompleted
		}
		result = append(result, view)
	}
	return result, nil
}

// ValidateUserPurchase 检查用户是否可以购买指定余额套餐。
// 同档套餐允许续费，异档套餐必须先退款，避免同一用户同时持有多个有效套餐。
func (s *BalancePackageService) ValidateUserPurchase(ctx context.Context, userID, planID int64) error {
	if s == nil || s.entClient == nil {
		return fmt.Errorf("balance package service is unavailable")
	}
	current, err := s.currentUserPackage(ctx, s.entClient, userID, time.Now().UTC(), false)
	if err != nil {
		return err
	}
	return validateBalancePackagePlanChange(current, planID)
}

func (s *BalancePackageService) validateUserPurchaseInTx(ctx context.Context, client *dbent.Client, userID, planID int64) error {
	if client == nil {
		return fmt.Errorf("balance package transaction client is unavailable")
	}
	if _, err := lockBalancePackageUser(ctx, client, userID); err != nil {
		if dbent.IsNotFound(err) {
			return ErrUserNotFound
		}
		return fmt.Errorf("lock balance package user: %w", err)
	}
	current, err := s.currentUserPackage(ctx, client, userID, time.Now().UTC(), true)
	if err != nil {
		return err
	}
	return validateBalancePackagePlanChange(current, planID)
}

func (s *BalancePackageService) currentUserPackage(ctx context.Context, client *dbent.Client, userID int64, now time.Time, lock bool) (*dbent.UserBalancePackage, error) {
	if client == nil || userID <= 0 {
		return nil, nil
	}
	query := client.UserBalancePackage.Query().
		Where(
			userbalancepackage.UserIDEQ(userID),
			userbalancepackage.StatusIn(balancePackageStatusActive, balancePackageStatusCompleted, balancePackageStatusDebtPaused),
			userbalancepackage.ExpiresAtGT(now),
		).
		Order(userbalancepackage.ByCreatedAt(sql.OrderDesc()), userbalancepackage.ByID(sql.OrderDesc())).
		Limit(1)
	if lock && client.Driver().Dialect() == dialect.Postgres {
		query = query.ForUpdate()
	}
	item, err := query.First(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("find current balance package: %w", err)
	}
	return item, nil
}

func lockBalancePackageUser(ctx context.Context, client *dbent.Client, userID int64) (*dbent.User, error) {
	query := client.User.Query().Where(user.IDEQ(userID))
	if client.Driver().Dialect() == dialect.Postgres {
		query = query.ForUpdate()
	}
	return query.Only(ctx)
}

func validateBalancePackagePlanChange(current *dbent.UserBalancePackage, planID int64) error {
	if current == nil || current.PlanID == planID {
		return nil
	}
	return balancePackagePlanConflict(current.PlanID, planID)
}

func balancePackagePlanConflict(currentPlanID, requestedPlanID int64) error {
	return infraerrors.Conflict("BALANCE_PACKAGE_ACTIVE", "当前已有有效余额套餐，请先退款后再购买其他套餐").
		WithMetadata(map[string]string{
			"current_plan_id":   fmt.Sprintf("%d", currentPlanID),
			"requested_plan_id": fmt.Sprintf("%d", requestedPlanID),
		})
}

func (s *BalancePackageService) ensureDefaultPlans(ctx context.Context) error {
	if s == nil || s.entClient == nil {
		return fmt.Errorf("balance package service is unavailable")
	}
	count, err := s.entClient.BalancePackagePlan.Query().Count(ctx)
	if err != nil || count > 0 {
		return err
	}
	for _, item := range defaultBalancePackagePlans {
		if err := s.entClient.BalancePackagePlan.Create().
			SetCode(item.code).
			SetName(item.name).
			SetPriceCny(item.priceCNY).
			SetWeeklyCreditUsd(item.weeklyCreditUSD).
			SetValidityDays(28).
			SetRefreshCount(4).
			SetRefreshIntervalDays(7).
			SetForSale(true).
			SetSortOrder(item.sortOrder).
			OnConflictColumns(balancepackageplan.FieldCode).
			Ignore().
			Exec(ctx); err != nil {
			return fmt.Errorf("seed balance package %s: %w", item.code, err)
		}
	}
	return nil
}

func validateBalancePackagePlan(plan *dbent.BalancePackagePlan) error {
	if plan == nil || plan.PriceCny <= 0 || plan.WeeklyCreditUsd <= 0 || plan.ValidityDays <= 0 || plan.RefreshCount <= 0 || plan.RefreshIntervalDays <= 0 {
		return infraerrors.BadRequest("BALANCE_PACKAGE_INVALID", "balance package configuration is invalid")
	}
	return nil
}

// CreditInitialBalance 原子创建用户套餐并到账第一周余额。
func (s *BalancePackageService) CreditInitialBalance(ctx context.Context, order *dbent.PaymentOrder) error {
	if err := validateBalancePackageOrderSnapshot(order); err != nil {
		return err
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin initial balance package credit: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	if _, err := s.creditInitialBalance(txCtx, tx.Client(), order); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit initial balance package credit: %w", err)
	}
	s.invalidateBalanceCache(ctx, order.UserID)
	return nil
}

// creditInitialBalance 在调用方提供的事务内创建套餐和首期到账，供支付回调与后台发放复用。
func (s *BalancePackageService) creditInitialBalance(ctx context.Context, client *dbent.Client, order *dbent.PaymentOrder) (*dbent.UserBalancePackage, error) {
	if client == nil {
		return nil, fmt.Errorf("balance package transaction client is unavailable")
	}
	exists, err := client.UserBalancePackage.Query().Where(userbalancepackage.PaymentOrderIDEQ(order.ID)).Exist(ctx)
	if err != nil {
		return nil, fmt.Errorf("check balance package fulfillment: %w", err)
	}
	if exists {
		return nil, nil
	}
	lockedUser, err := lockBalancePackageUser(ctx, client, order.UserID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("lock balance package user: %w", err)
	}
	current, err := s.currentUserPackage(ctx, client, order.UserID, time.Now().UTC(), true)
	if err != nil {
		return nil, err
	}
	if current != nil {
		if err := validateBalancePackagePlanChange(current, *order.BalancePackagePlanID); err != nil {
			return nil, err
		}
		if order.PaymentType == payment.PaymentTypeAdminGrant {
			return nil, infraerrors.Conflict("BALANCE_PACKAGE_ACTIVE", "用户已有有效余额套餐，不能重复发放")
		}
		return s.renewBalancePackage(ctx, client, order, current, lockedUser)
	}

	startsAt := time.Now().UTC()
	if order.PaidAt != nil {
		startsAt = order.PaidAt.UTC()
	}
	weeklyCredit := *order.BalancePackageWeeklyCreditUsd
	refreshCount := *order.BalancePackageRefreshCount
	refreshIntervalDays := *order.BalancePackageRefreshIntervalDays
	validityDays := *order.BalancePackageValidityDays
	status := balancePackageStatusActive
	remainingCredit := balancePackageRemainingAfterDebt(lockedUser.Balance, weeklyCredit)
	debtRepaid := minFloat(maxFloat(-lockedUser.Balance, 0), weeklyCredit)
	newBalance := lockedUser.Balance + weeklyCredit
	if refreshCount == 1 {
		status = balancePackageStatusCompleted
	}

	updated, err := client.User.Update().Where(user.IDEQ(order.UserID)).AddBalance(weeklyCredit).AddTotalRecharged(weeklyCredit).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("credit initial balance package balance: %w", err)
	}
	if updated == 0 {
		return nil, ErrUserNotFound
	}

	builder := client.UserBalancePackage.Create().
		SetUserID(order.UserID).
		SetPlanID(*order.BalancePackagePlanID).
		SetPaymentOrderID(order.ID).
		SetWeeklyCreditUsd(weeklyCredit).
		SetRemainingUsd(remainingCredit).
		SetCreditedCount(1).
		SetRefreshCount(refreshCount).
		SetRefreshIntervalDays(refreshIntervalDays).
		SetStartsAt(startsAt).
		SetExpiresAt(startsAt.AddDate(0, 0, validityDays)).
		SetStatus(status)
	if status == balancePackageStatusActive {
		builder.SetNextCreditAt(startsAt.AddDate(0, 0, refreshIntervalDays))
	}
	pkg, err := builder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create user balance package: %w", err)
	}
	if err := createBalancePackageCreditAudit(ctx, client, order.ID, "BALANCE_PACKAGE_INITIAL_CREDIT", weeklyCredit, 1); err != nil {
		return nil, err
	}
	if debtRepaid > 0 {
		if err := recordBalanceDebtLedger(ctx, client, order.UserID, "repayment", debtRepaid, lockedUser.Balance, newBalance, "balance_package_initial_credit", fmt.Sprintf("order:%d", order.ID)); err != nil {
			return nil, err
		}
	}
	return pkg, nil
}

// renewBalancePackage 处理同档续费：把现有套餐重置为新一轮到账周期，而不是仅延长有效期。
// 续费即立即发放新周期第 1 期额度、进度回到 1/4、重新计时下一次刷新，并在原到期基础上顺延有效期。
// 若旧周期尚未走完，未发放的期数会顺延进新周期的总期数，保证已付费的额度不丢失。
func (s *BalancePackageService) renewBalancePackage(ctx context.Context, client *dbent.Client, order *dbent.PaymentOrder, current *dbent.UserBalancePackage, lockedUser *dbent.User) (*dbent.UserBalancePackage, error) {
	weeklyCredit := *order.BalancePackageWeeklyCreditUsd
	refreshCount := *order.BalancePackageRefreshCount
	refreshIntervalDays := *order.BalancePackageRefreshIntervalDays
	validityDays := *order.BalancePackageValidityDays
	if weeklyCredit <= 0 || refreshCount <= 0 || refreshIntervalDays <= 0 || validityDays <= 0 {
		return nil, infraerrors.BadRequest("BALANCE_PACKAGE_SNAPSHOT_INVALID", "balance package order snapshot is invalid")
	}

	now := time.Now().UTC()

	// 顺延旧周期尚未发放的期数，避免中途续费时丢失已付费的到账。正常"完成后续费"时为 0。
	carriedPeriods := current.RefreshCount - current.CreditedCount
	if carriedPeriods < 0 {
		carriedPeriods = 0
	}
	newRefreshCount := refreshCount + carriedPeriods

	// 与周额度刷新一致的余额口径：先移除旧窗口剩余，再用本周额度抵扣负余额，剩余进入新窗口。
	baseBalance := lockedUser.Balance - current.RemainingUsd
	newRemaining := balancePackageRemainingAfterDebt(baseBalance, weeklyCredit)
	debtRepaid := minFloat(maxFloat(-baseBalance, 0), weeklyCredit)
	balanceDelta := weeklyCredit - current.RemainingUsd
	newBalance := baseBalance + weeklyCredit

	status := balancePackageStatusActive
	if newRefreshCount <= 1 {
		status = balancePackageStatusCompleted
	}

	updatedUsers, err := client.User.Update().
		Where(user.IDEQ(order.UserID)).
		AddBalance(balanceDelta).
		AddTotalRecharged(weeklyCredit).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("credit renewed balance package balance: %w", err)
	}
	if updatedUsers == 0 {
		return nil, ErrUserNotFound
	}

	builder := client.UserBalancePackage.UpdateOneID(current.ID).
		SetPaymentOrderID(order.ID).
		SetWeeklyCreditUsd(weeklyCredit).
		SetRemainingUsd(newRemaining).
		SetCreditedCount(1).
		SetRefreshCount(newRefreshCount).
		SetRefreshIntervalDays(refreshIntervalDays).
		SetStartsAt(now).
		SetExpiresAt(current.ExpiresAt.AddDate(0, 0, validityDays)).
		SetStatus(status).
		SetRenewalCount(current.RenewalCount + 1).
		SetUpdatedAt(now)
	if status == balancePackageStatusActive {
		builder.SetNextCreditAt(now.AddDate(0, 0, refreshIntervalDays))
	} else {
		builder.ClearNextCreditAt()
	}
	updated, err := builder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("renew user balance package: %w", err)
	}

	if err := createBalancePackageRenewalAudit(ctx, client, order.ID, current.ID, current.ExpiresAt, updated.ExpiresAt, weeklyCredit, newRefreshCount, current.RenewalCount+1, carriedPeriods); err != nil {
		return nil, err
	}
	// 续费订单是新订单 ID，首期到账沿用 INITIAL_CREDIT 审计动作（按订单唯一）。
	if err := createBalancePackageCreditAudit(ctx, client, order.ID, "BALANCE_PACKAGE_INITIAL_CREDIT", weeklyCredit, 1); err != nil {
		return nil, err
	}
	if debtRepaid > 0 {
		if err := recordBalanceDebtLedger(ctx, client, order.UserID, "repayment", debtRepaid, lockedUser.Balance, newBalance, "balance_package_renewal_credit", fmt.Sprintf("order:%d", order.ID)); err != nil {
			return nil, err
		}
	}
	return updated, nil
}

// CreditDueBalances 刷新到期周额度。每条记录只发放当前窗口一次，跳过服务停机期间错过的窗口。
func (s *BalancePackageService) CreditDueBalances(ctx context.Context, now time.Time) (int, error) {
	if s == nil || s.entClient == nil {
		return 0, fmt.Errorf("balance package service is unavailable")
	}
	if err := s.expireDueBalances(ctx, now.UTC()); err != nil {
		return 0, err
	}
	due, err := s.entClient.UserBalancePackage.Query().
		Where(
			userbalancepackage.StatusIn(balancePackageStatusActive, balancePackageStatusDebtPaused),
			userbalancepackage.NextCreditAtLTE(now.UTC()),
			userbalancepackage.ExpiresAtGT(now.UTC()),
		).
		Order(userbalancepackage.ByNextCreditAt()).
		Limit(balancePackageCreditBatchSize).
		All(ctx)
	if err != nil {
		return 0, fmt.Errorf("list due balance package credits: %w", err)
	}

	credited := 0
	for _, item := range due {
		applied, err := s.creditDueBalance(ctx, item, now.UTC())
		if err != nil {
			return credited, err
		}
		if applied {
			credited++
		}
	}
	return credited, nil
}

// PauseDebtPackages 保留旧调度接口；新规则不再因欠费暂停活动套餐。
func (s *BalancePackageService) PauseDebtPackages(ctx context.Context, now time.Time) (int, error) {
	// 保留旧接口以兼容管理员工具；欠费不再暂停套餐，周额度刷新会自动抵消余额欠费。
	return 0, nil
}

func (s *BalancePackageService) pauseDebtPackage(ctx context.Context, candidate *dbent.UserBalancePackage, now time.Time, operator string) (bool, error) {
	if candidate == nil {
		return false, nil
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return false, fmt.Errorf("begin debt package pause: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	lockedUser, err := lockBalancePackageUser(txCtx, client, candidate.UserID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("lock debt package user: %w", err)
	}
	query := client.UserBalancePackage.Query().Where(userbalancepackage.IDEQ(candidate.ID))
	if client.Driver().Dialect() == dialect.Postgres {
		query = query.ForUpdate()
	}
	current, err := query.Only(txCtx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("lock debt package: %w", err)
	}
	if lockedUser.Balance >= 0 || current.Status != balancePackageStatusActive || current.CreditedCount >= current.RefreshCount || !current.ExpiresAt.After(now) {
		return false, nil
	}
	if _, err := client.UserBalancePackage.UpdateOneID(current.ID).
		Where(userbalancepackage.StatusEQ(balancePackageStatusActive)).
		SetStatus(balancePackageStatusDebtPaused).
		Save(txCtx); err != nil {
		if dbent.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("pause debt package: %w", err)
	}
	if err := createBalancePackageDebtAudit(txCtx, client, current.PaymentOrderID, balancePackageDebtPausedAudit, current.CreditedCount, lockedUser.Balance, lockedUser.Balance, current.WeeklyCreditUsd, timeOrZero(current.NextCreditAt), operator); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit debt package pause: %w", err)
	}
	return true, nil
}

// ResumeDebtPausedPackage 兼容历史 debt_paused 数据的管理员恢复接口。
func (s *BalancePackageService) ResumeDebtPausedPackage(ctx context.Context, packageID, adminUserID int64, now time.Time) error {
	if s == nil || s.entClient == nil {
		return infraerrors.ServiceUnavailable("BALANCE_PACKAGE_UNAVAILABLE", "balance package service is unavailable")
	}
	if packageID <= 0 || adminUserID <= 0 {
		return infraerrors.BadRequest("INVALID_RESUME_INPUT", "package and admin are required")
	}
	candidate, err := s.entClient.UserBalancePackage.Get(ctx, packageID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return infraerrors.NotFound("BALANCE_PACKAGE_NOT_FOUND", "balance package not found")
		}
		return err
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin debt package resume: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	lockedUser, err := lockBalancePackageUser(txCtx, client, candidate.UserID)
	if err != nil {
		return fmt.Errorf("lock debt package user: %w", err)
	}
	query := client.UserBalancePackage.Query().Where(userbalancepackage.IDEQ(packageID))
	if client.Driver().Dialect() == dialect.Postgres {
		query = query.ForUpdate()
	}
	current, err := query.Only(txCtx)
	if err != nil {
		return fmt.Errorf("lock debt package: %w", err)
	}
	if current.Status != balancePackageStatusDebtPaused {
		return infraerrors.Conflict("BALANCE_PACKAGE_NOT_DEBT_PAUSED", "balance package is not debt paused")
	}
	if !current.ExpiresAt.After(now) || current.CreditedCount >= current.RefreshCount {
		return infraerrors.Conflict("BALANCE_PACKAGE_EXPIRED", "balance package has expired")
	}
	if lockedUser.Balance < 0 {
		return infraerrors.Conflict("BALANCE_DEBT_OUTSTANDING", "user balance debt must be cleared before resuming package")
	}
	if _, err := client.UserBalancePackage.UpdateOneID(current.ID).
		Where(userbalancepackage.StatusEQ(balancePackageStatusDebtPaused)).
		SetStatus(balancePackageStatusActive).
		SetNextCreditAt(now).
		Save(txCtx); err != nil {
		return fmt.Errorf("resume debt package: %w", err)
	}
	operator := fmt.Sprintf("admin:%d", adminUserID)
	if err := createBalancePackageDebtAudit(txCtx, client, current.PaymentOrderID, balancePackageDebtResumedAudit, current.CreditedCount, lockedUser.Balance, lockedUser.Balance, 0, now, operator); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit debt package resume: %w", err)
	}
	return nil
}

// CancellableOrderIDs 返回仍绑定当前有效余额套餐的订单，用于管理员端展示取消权益入口。
func (s *BalancePackageService) CancellableOrderIDs(ctx context.Context, orderIDs []int64, now time.Time) (map[int64]bool, error) {
	result := make(map[int64]bool)
	if s == nil || s.entClient == nil || len(orderIDs) == 0 {
		return result, nil
	}
	packages, err := s.entClient.UserBalancePackage.Query().
		Where(
			userbalancepackage.PaymentOrderIDIn(orderIDs...),
			userbalancepackage.StatusIn(balancePackageStatusActive, balancePackageStatusCompleted, balancePackageStatusDebtPaused),
			userbalancepackage.ExpiresAtGT(now),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list cancellable balance packages: %w", err)
	}
	for _, item := range packages {
		result[item.PaymentOrderID] = true
	}
	return result, nil
}

// CancelPackageByOrder 仅停止当前套餐权益，不发起退款或改写订单状态。
func (s *BalancePackageService) CancelPackageByOrder(ctx context.Context, orderID, adminUserID int64, now time.Time) error {
	if s == nil || s.entClient == nil {
		return infraerrors.ServiceUnavailable("BALANCE_PACKAGE_UNAVAILABLE", "balance package service is unavailable")
	}
	if orderID <= 0 || adminUserID <= 0 {
		return infraerrors.BadRequest("INVALID_CANCEL_INPUT", "order and admin are required")
	}

	candidate, err := s.entClient.UserBalancePackage.Query().
		Where(userbalancepackage.PaymentOrderIDEQ(orderID)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return infraerrors.NotFound("BALANCE_PACKAGE_NOT_FOUND", "balance package not found")
		}
		return fmt.Errorf("find balance package for cancellation: %w", err)
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin balance package cancellation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	lockedUser, err := lockBalancePackageUser(txCtx, client, candidate.UserID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return ErrUserNotFound
		}
		return fmt.Errorf("lock balance package user for cancellation: %w", err)
	}

	packageQuery := client.UserBalancePackage.Query().Where(userbalancepackage.IDEQ(candidate.ID))
	if client.Driver().Dialect() == dialect.Postgres {
		packageQuery = packageQuery.ForUpdate()
	}
	current, err := packageQuery.Only(txCtx)
	if err != nil {
		return fmt.Errorf("lock balance package for cancellation: %w", err)
	}
	if current.PaymentOrderID != orderID || !isExpirableBalancePackageStatus(current.Status) || !current.ExpiresAt.After(now) {
		return infraerrors.Conflict("BALANCE_PACKAGE_NOT_CANCELLABLE", "balance package is no longer cancellable")
	}

	orderQuery := client.PaymentOrder.Query().
		Where(paymentorder.IDEQ(orderID), paymentorder.UserIDEQ(lockedUser.ID), paymentorder.OrderTypeEQ(payment.OrderTypeBalanceSubscription), paymentorder.StatusIn(OrderStatusCompleted, OrderStatusRefundFailed))
	if client.Driver().Dialect() == dialect.Postgres {
		orderQuery = orderQuery.ForUpdate()
	}
	if _, err := orderQuery.Only(txCtx); err != nil {
		if dbent.IsNotFound(err) {
			return infraerrors.Conflict("BALANCE_PACKAGE_NOT_CANCELLABLE", "balance package order is no longer cancellable")
		}
		return fmt.Errorf("lock balance package order for cancellation: %w", err)
	}

	if _, err := client.UserBalancePackage.UpdateOneID(current.ID).
		Where(userbalancepackage.StatusIn(balancePackageStatusActive, balancePackageStatusCompleted, balancePackageStatusDebtPaused)).
		SetStatus(balancePackageStatusCancelled).
		SetRemainingUsd(0).
		ClearNextCreditAt().
		Save(txCtx); err != nil {
		return fmt.Errorf("cancel balance package: %w", err)
	}

	detail, err := json.Marshal(map[string]any{
		"reason":                  "管理员取消当前余额套餐权益，用户可重新购买",
		"user_id":                 lockedUser.ID,
		"package_id":              current.ID,
		"package_status_before":   current.Status,
		"package_status_after":    balancePackageStatusCancelled,
		"remaining_usd_before":    current.RemainingUsd,
		"remaining_usd_after":     0,
		"credited_count":          current.CreditedCount,
		"next_credit_at_before":   current.NextCreditAt,
		"next_credit_at_after":    nil,
		"balance_unchanged_usd":   lockedUser.Balance,
		"gateway_refund_executed": false,
	})
	if err != nil {
		return fmt.Errorf("encode balance package cancellation audit: %w", err)
	}
	if _, err := client.PaymentAuditLog.Create().
		SetOrderID(fmt.Sprintf("%d", orderID)).
		SetAction(balancePackageManualCancelAudit).
		SetDetail(string(detail)).
		SetOperator(fmt.Sprintf("admin:%d", adminUserID)).
		Save(txCtx); err != nil {
		return fmt.Errorf("record balance package cancellation audit: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit balance package cancellation: %w", err)
	}
	s.invalidateBalanceCache(ctx, current.UserID)
	return nil
}

func (s *BalancePackageService) creditDueBalance(ctx context.Context, item *dbent.UserBalancePackage, now time.Time) (bool, error) {
	if item == nil || item.NextCreditAt == nil || item.CreditedCount >= item.RefreshCount || !item.ExpiresAt.After(now) {
		return false, nil
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return false, fmt.Errorf("begin scheduled balance package credit: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	lockedUser, err := lockBalancePackageUser(txCtx, client, item.UserID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("lock scheduled balance package user: %w", err)
	}
	currentBalance := lockedUser.Balance
	currentQuery := client.UserBalancePackage.Query().Where(userbalancepackage.IDEQ(item.ID))
	if client.Driver().Dialect() == dialect.Postgres {
		currentQuery = currentQuery.ForUpdate()
	}
	current, err := currentQuery.Only(txCtx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("lock scheduled balance package: %w", err)
	}
	if (current.Status != balancePackageStatusActive && current.Status != balancePackageStatusDebtPaused) || current.NextCreditAt == nil ||
		current.CreditedCount >= current.RefreshCount || !current.ExpiresAt.After(now) {
		return false, nil
	}
	periodsDue := balancePackagePeriodsDue(current.NextCreditAt, now, current.RefreshIntervalDays)
	newCount := current.CreditedCount + periodsDue
	if newCount > current.RefreshCount {
		newCount = current.RefreshCount
	}
	if newCount <= current.CreditedCount {
		return false, nil
	}
	newNextCreditAt := current.NextCreditAt.AddDate(0, 0, periodsDue*current.RefreshIntervalDays)
	update := client.UserBalancePackage.UpdateOneID(current.ID).
		Where(
			userbalancepackage.StatusIn(balancePackageStatusActive, balancePackageStatusDebtPaused),
			userbalancepackage.CreditedCountEQ(current.CreditedCount),
			userbalancepackage.NextCreditAtLTE(now),
			userbalancepackage.ExpiresAtGT(now),
		).
		SetCreditedCount(newCount)
	baseBalance := currentBalance - current.RemainingUsd
	newBalance := baseBalance + current.WeeklyCreditUsd
	newRemaining := balancePackageRemainingAfterDebt(baseBalance, current.WeeklyCreditUsd)
	debtRepaid := minFloat(maxFloat(-baseBalance, 0), current.WeeklyCreditUsd)
	update.SetRemainingUsd(newRemaining)
	if newCount >= current.RefreshCount {
		update.SetStatus(balancePackageStatusCompleted).ClearNextCreditAt()
	} else {
		update.SetStatus(balancePackageStatusActive)
		update.SetNextCreditAt(newNextCreditAt)
	}
	if _, err := update.Save(txCtx); err != nil {
		if dbent.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("claim scheduled balance package credit: %w", err)
	}

	// users.balance 还包含普通充值和返利；先移除旧窗口，再用本周额度填补欠费，剩余才进入套餐窗口。
	balanceDelta := current.WeeklyCreditUsd - current.RemainingUsd
	updatedBuilder := client.User.Update().
		Where(user.IDEQ(current.UserID)).
		AddBalance(balanceDelta).
		AddTotalRecharged(current.WeeklyCreditUsd)
	updated, err := updatedBuilder.Save(txCtx)
	if err != nil {
		return false, fmt.Errorf("refresh scheduled balance package balance: %w", err)
	}
	if updated == 0 {
		return false, ErrUserNotFound
	}
	if debtRepaid > 0 {
		if err := recordBalanceDebtLedger(txCtx, client, current.UserID, "repayment", debtRepaid, currentBalance, newBalance, "balance_package_weekly_credit", fmt.Sprintf("package:%d:credit:%d", current.ID, newCount)); err != nil {
			return false, err
		}
	}
	if err := createBalancePackageCreditAudit(txCtx, client, current.PaymentOrderID, balancePackageWeeklyCreditAudit, current.WeeklyCreditUsd, newCount); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit scheduled balance package refresh: %w", err)
	}
	s.invalidateBalanceCache(ctx, current.UserID)
	return true, nil
}

// expireDueBalances 清理已到期套餐本周未用额度，避免最后一周余额留在普通钱包中。
func (s *BalancePackageService) expireDueBalances(ctx context.Context, now time.Time) error {
	items, err := s.entClient.UserBalancePackage.Query().
		Where(
			userbalancepackage.StatusIn(balancePackageStatusActive, balancePackageStatusCompleted, balancePackageStatusDebtPaused),
			userbalancepackage.ExpiresAtLTE(now),
		).
		Order(userbalancepackage.ByExpiresAt()).
		Limit(balancePackageCreditBatchSize).
		All(ctx)
	if err != nil {
		return fmt.Errorf("list expired balance packages: %w", err)
	}
	for _, item := range items {
		if err := s.expireBalancePackage(ctx, item.ID, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *BalancePackageService) expireBalancePackage(ctx context.Context, packageID int64, now time.Time) error {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin balance package expiration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	candidate, err := client.UserBalancePackage.Get(txCtx, packageID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("lock expired balance package: %w", err)
	}
	if candidate.ExpiresAt.After(now) || !isExpirableBalancePackageStatus(candidate.Status) {
		return nil
	}
	if _, err := lockBalancePackageUser(txCtx, client, candidate.UserID); err != nil {
		if dbent.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("lock expired balance package user: %w", err)
	}
	itemQuery := client.UserBalancePackage.Query().Where(userbalancepackage.IDEQ(packageID))
	if client.Driver().Dialect() == dialect.Postgres {
		itemQuery = itemQuery.ForUpdate()
	}
	item, err := itemQuery.Only(txCtx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("lock expired balance package: %w", err)
	}
	if item.ExpiresAt.After(now) || !isExpirableBalancePackageStatus(item.Status) {
		return nil
	}
	if _, err := client.UserBalancePackage.UpdateOneID(item.ID).
		SetRemainingUsd(0).
		SetStatus(balancePackageStatusExpired).
		ClearNextCreditAt().
		Save(txCtx); err != nil {
		return fmt.Errorf("expire balance package: %w", err)
	}
	if item.RemainingUsd > 0 {
		updated, err := client.User.Update().Where(user.IDEQ(item.UserID)).AddBalance(-item.RemainingUsd).Save(txCtx)
		if err != nil {
			return fmt.Errorf("clear expired balance package credit: %w", err)
		}
		if updated == 0 {
			return ErrUserNotFound
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit balance package expiration: %w", err)
	}
	s.invalidateBalanceCache(ctx, item.UserID)
	return nil
}

func isExpirableBalancePackageStatus(status string) bool {
	return status == balancePackageStatusActive || status == balancePackageStatusCompleted || status == balancePackageStatusDebtPaused
}

func balancePackagePeriodsDue(nextCreditAt *time.Time, now time.Time, intervalDays int) int {
	if nextCreditAt == nil || intervalDays <= 0 || now.Before(*nextCreditAt) {
		return 0
	}
	interval := time.Duration(intervalDays) * 24 * time.Hour
	return int(now.Sub(*nextCreditAt)/interval) + 1
}

// balancePackageRemainingAfterDebt 保证周额度先偿还负余额，只有剩余部分才进入本周套餐额度。
func balancePackageRemainingAfterDebt(baseBalance, weeklyCredit float64) float64 {
	if weeklyCredit <= 0 {
		return 0
	}
	if baseBalance >= 0 {
		return weeklyCredit
	}
	return maxFloat(weeklyCredit+baseBalance, 0)
}

func maxFloat(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}

func minFloat(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

func timeOrZero(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}

func (s *BalancePackageService) invalidateBalanceCache(ctx context.Context, userID int64) {
	if s == nil || s.billingCache == nil || userID <= 0 {
		return
	}
	if err := s.billingCache.InvalidateUserBalance(ctx, userID); err != nil {
		slog.Warn("invalidate balance package user cache failed", "user_id", userID, "error", err)
	}
}

// recordBalanceDebtLedger 追加不可变欠费/还款流水，避免套餐退款或刷新覆盖历史欠费事实。
func recordBalanceDebtLedger(ctx context.Context, client *dbent.Client, userID int64, entryType string, amount, balanceBefore, balanceAfter float64, sourceType, sourceRef string) error {
	if client == nil || client.Driver().Dialect() != dialect.Postgres || userID <= 0 || amount <= 0 {
		return nil
	}
	if _, err := client.ExecContext(ctx, `
		INSERT INTO balance_debt_ledger
			(user_id, entry_type, amount_usd, balance_before_usd, balance_after_usd, source_type, source_ref, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
	`, userID, entryType, amount, balanceBefore, balanceAfter, sourceType, sourceRef); err != nil {
		return fmt.Errorf("record balance debt ledger: %w", err)
	}
	return nil
}

func validateBalancePackageOrderSnapshot(order *dbent.PaymentOrder) error {
	if order == nil || order.BalancePackagePlanID == nil || order.BalancePackageWeeklyCreditUsd == nil || order.BalancePackageRefreshCount == nil ||
		order.BalancePackageRefreshIntervalDays == nil || order.BalancePackageValidityDays == nil || *order.BalancePackageWeeklyCreditUsd <= 0 ||
		*order.BalancePackageRefreshCount <= 0 || *order.BalancePackageRefreshIntervalDays <= 0 || *order.BalancePackageValidityDays <= 0 {
		return infraerrors.BadRequest("BALANCE_PACKAGE_SNAPSHOT_INVALID", "balance package order snapshot is invalid")
	}
	return nil
}

func createBalancePackageCreditAudit(ctx context.Context, client *dbent.Client, orderID int64, action string, credit float64, count int) error {
	detail, _ := json.Marshal(map[string]any{"credit_usd": credit, "credited_count": count})
	auditAction := action
	if action == balancePackageWeeklyCreditAudit {
		// payment_audit_logs 对同一订单和 action 唯一；周期到账必须按次数形成独立审计事件。
		auditAction = fmt.Sprintf("%s_%d", action, count)
	}
	if _, err := client.PaymentAuditLog.Create().SetOrderID(fmt.Sprintf("%d", orderID)).SetAction(auditAction).SetDetail(string(detail)).SetOperator("system").Save(ctx); err != nil {
		return fmt.Errorf("record balance package credit audit: %w", err)
	}
	return nil
}

func createBalancePackageDebtAudit(ctx context.Context, client *dbent.Client, orderID int64, action string, creditedCount int, balanceBefore, balanceAfter, weeklyCredit float64, plannedAt time.Time, operator string) error {
	detail, _ := json.Marshal(map[string]any{
		"credited_count":         creditedCount,
		"balance_before_usd":     balanceBefore,
		"balance_after_usd":      balanceAfter,
		"weekly_credit_usd":      weeklyCredit,
		"planned_next_credit_at": plannedAt.UTC().Format(time.RFC3339),
	})
	auditAction := fmt.Sprintf("%s_%d_%d", action, creditedCount, time.Now().UTC().UnixNano())
	if _, err := client.PaymentAuditLog.Create().
		SetOrderID(fmt.Sprintf("%d", orderID)).
		SetAction(auditAction).
		SetDetail(string(detail)).
		SetOperator(operator).
		Save(ctx); err != nil {
		return fmt.Errorf("record balance package debt audit: %w", err)
	}
	return nil
}

func createBalancePackageRenewalAudit(ctx context.Context, client *dbent.Client, orderID, packageID int64, previousExpiresAt, expiresAt time.Time, weeklyCreditUsd float64, refreshCount, renewalCount, carriedPeriods int) error {
	detail, _ := json.Marshal(map[string]any{
		"package_id":          packageID,
		"previous_expires_at": previousExpiresAt.UTC().Format(time.RFC3339),
		"expires_at":          expiresAt.UTC().Format(time.RFC3339),
		"validity_days_added": int(expiresAt.Sub(previousExpiresAt) / (24 * time.Hour)),
		"weekly_credit_usd":   weeklyCreditUsd,
		"refresh_count":       refreshCount,
		"renewal_count":       renewalCount,
		"carried_periods":     carriedPeriods,
		"cycle_reset":         true,
	})
	if _, err := client.PaymentAuditLog.Create().
		SetOrderID(fmt.Sprintf("%d", orderID)).
		SetAction(balancePackageRenewalAudit).
		SetDetail(string(detail)).
		SetOperator("system").
		Save(ctx); err != nil {
		return fmt.Errorf("record balance package renewal audit: %w", err)
	}
	return nil
}
