package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/balancepackageplan"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/ent/userbalancepackage"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	balancePackageStatusActive    = "active"
	balancePackageStatusCompleted = "completed"
	balancePackageCreditBatchSize = 100
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
	ValidityDays        int        `json:"validity_days"`
	RefreshCount        int        `json:"refresh_count"`
	RefreshIntervalDays int        `json:"refresh_interval_days"`
	CreditedCount       int        `json:"credited_count"`
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
	entClient *dbent.Client
}

func NewBalancePackageService(entClient *dbent.Client) *BalancePackageService {
	return &BalancePackageService{entClient: entClient}
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

	packages, err := s.entClient.UserBalancePackage.Query().
		Where(userbalancepackage.UserIDEQ(userID)).
		Order(userbalancepackage.ByCreatedAt(sql.OrderDesc())).
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

	now := time.Now()
	result := make([]UserBalancePackageView, 0, len(packages))
	for _, item := range packages {
		plan := planByID[item.PlanID]
		view := UserBalancePackageView{
			ID:                  item.ID,
			PlanID:              item.PlanID,
			WeeklyCreditUSD:     item.WeeklyCreditUsd,
			ValidityDays:        int(item.ExpiresAt.Sub(item.StartsAt) / (24 * time.Hour)),
			RefreshCount:        item.RefreshCount,
			RefreshIntervalDays: item.RefreshIntervalDays,
			CreditedCount:       item.CreditedCount,
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
			view.ValidityDays = plan.ValidityDays
		}
		if view.Name == "" {
			view.Name = fmt.Sprintf("余额套餐 #%d", item.PlanID)
		}
		if view.ValidityDays <= 0 {
			view.ValidityDays = int(item.ExpiresAt.Sub(item.StartsAt) / (24 * time.Hour))
		}
		if !item.ExpiresAt.After(now) && view.Status != "refunded" {
			view.Status = "expired"
		}
		result = append(result, view)
	}
	return result, nil
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

	startsAt := time.Now().UTC()
	if order.PaidAt != nil {
		startsAt = order.PaidAt.UTC()
	}
	weeklyCredit := *order.BalancePackageWeeklyCreditUsd
	refreshCount := *order.BalancePackageRefreshCount
	refreshIntervalDays := *order.BalancePackageRefreshIntervalDays
	validityDays := *order.BalancePackageValidityDays
	status := balancePackageStatusActive
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
	return pkg, nil
}

// CreditDueBalances 发放全部到期周额度。每条记录先条件更新，再增加余额，两个步骤在同一事务内。
func (s *BalancePackageService) CreditDueBalances(ctx context.Context, now time.Time) (int, error) {
	if s == nil || s.entClient == nil {
		return 0, fmt.Errorf("balance package service is unavailable")
	}
	due, err := s.entClient.UserBalancePackage.Query().
		Where(userbalancepackage.StatusEQ(balancePackageStatusActive), userbalancepackage.NextCreditAtLTE(now.UTC())).
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

func (s *BalancePackageService) creditDueBalance(ctx context.Context, item *dbent.UserBalancePackage, now time.Time) (bool, error) {
	if item == nil || item.NextCreditAt == nil || item.CreditedCount >= item.RefreshCount {
		return false, nil
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return false, fmt.Errorf("begin scheduled balance package credit: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	newCount := item.CreditedCount + 1
	update := client.UserBalancePackage.UpdateOneID(item.ID).
		Where(userbalancepackage.StatusEQ(balancePackageStatusActive), userbalancepackage.CreditedCountEQ(item.CreditedCount), userbalancepackage.NextCreditAtLTE(now)).
		SetCreditedCount(newCount)
	if newCount >= item.RefreshCount {
		update.SetStatus(balancePackageStatusCompleted).ClearNextCreditAt()
	} else {
		update.SetNextCreditAt(item.NextCreditAt.AddDate(0, 0, item.RefreshIntervalDays))
	}
	if _, err := update.Save(txCtx); err != nil {
		if dbent.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("claim scheduled balance package credit: %w", err)
	}

	updated, err := client.User.Update().Where(user.IDEQ(item.UserID)).AddBalance(item.WeeklyCreditUsd).AddTotalRecharged(item.WeeklyCreditUsd).Save(txCtx)
	if err != nil {
		return false, fmt.Errorf("credit scheduled balance package balance: %w", err)
	}
	if updated == 0 {
		return false, ErrUserNotFound
	}
	if err := createBalancePackageCreditAudit(txCtx, client, item.PaymentOrderID, "BALANCE_PACKAGE_WEEKLY_CREDIT", item.WeeklyCreditUsd, newCount); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit scheduled balance package credit: %w", err)
	}
	return true, nil
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
	if _, err := client.PaymentAuditLog.Create().SetOrderID(fmt.Sprintf("%d", orderID)).SetAction(action).SetDetail(string(detail)).SetOperator("system").Save(ctx); err != nil {
		return fmt.Errorf("record balance package credit audit: %w", err)
	}
	return nil
}
