package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/balancepackageplan"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// GrantBalancePackageInput 是管理员手动发放余额套餐的服务端输入。
type GrantBalancePackageInput struct {
	UserID               int64
	BalancePackagePlanID int64
	AdminUserID          int64
}

// BalancePackageGrant 是后台发放的可审计结果。
type BalancePackageGrant struct {
	OrderID          int64 `json:"order_id"`
	BalancePackageID int64 `json:"balance_package_id"`
}

// GrantBalancePackage 为管理员创建一笔零金额的非支付订单，并原子发放首期套餐余额。
func (s *PaymentService) GrantBalancePackage(ctx context.Context, input GrantBalancePackageInput) (*BalancePackageGrant, error) {
	if input.UserID <= 0 || input.BalancePackagePlanID <= 0 || input.AdminUserID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_GRANT_INPUT", "user, plan and admin are required")
	}
	if s == nil || s.entClient == nil || s.balancePackageService == nil {
		return nil, infraerrors.ServiceUnavailable("BALANCE_PACKAGE_UNAVAILABLE", "balance package service is unavailable")
	}
	if err := s.balancePackageService.ensureDefaultPlans(ctx); err != nil {
		return nil, err
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin admin balance package grant: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	plan, err := client.BalancePackagePlan.Query().
		Where(balancepackageplan.IDEQ(input.BalancePackagePlanID), balancepackageplan.ForSaleEQ(true)).
		Only(txCtx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, infraerrors.NotFound("BALANCE_PACKAGE_NOT_AVAILABLE", "balance package is not available")
		}
		return nil, fmt.Errorf("get balance package plan: %w", err)
	}
	if err := validateBalancePackagePlan(plan); err != nil {
		return nil, err
	}

	grantUser, err := loadGrantableUser(txCtx, client, input.UserID)
	if err != nil {
		return nil, err
	}

	order, err := createBalancePackageGrantOrder(txCtx, client, balancePackageGrantOrder{
		Plan:               plan,
		User:               grantUser,
		PaymentType:        payment.PaymentTypeAdminGrant,
		RechargeCodePrefix: "ADMIN-GRANT-",
		Origin:             "admin",
	})
	if err != nil {
		return nil, err
	}

	pkg, err := s.balancePackageService.creditInitialBalance(txCtx, client, order)
	if err != nil {
		return nil, err
	}
	if pkg == nil {
		return nil, infraerrors.Conflict("BALANCE_PACKAGE_ALREADY_GRANTED", "balance package grant already exists")
	}
	detail, _ := json.Marshal(map[string]any{
		"admin_user_id": input.AdminUserID,
		"plan_id":       plan.ID,
		"payment_type":  payment.PaymentTypeAdminGrant,
	})
	if _, err := client.PaymentAuditLog.Create().
		SetOrderID(strconv.FormatInt(order.ID, 10)).
		SetAction("ADMIN_BALANCE_PACKAGE_GRANTED").
		SetDetail(string(detail)).
		SetOperator(fmt.Sprintf("admin:%d", input.AdminUserID)).
		Save(txCtx); err != nil {
		return nil, fmt.Errorf("record admin balance package grant audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit admin balance package grant: %w", err)
	}
	s.balancePackageService.invalidateBalanceCache(ctx, input.UserID)
	if s.purchaseNotifyService != nil {
		s.purchaseNotifyService.NotifyBalancePackage(order)
	}
	return &BalancePackageGrant{OrderID: order.ID, BalancePackageID: pkg.ID}, nil
}
