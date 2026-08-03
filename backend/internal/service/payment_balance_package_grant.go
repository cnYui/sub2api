package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/balancepackageplan"
	"github.com/Wei-Shaw/sub2api/ent/user"
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

	grantUser, err := client.User.Query().Where(user.IDEQ(input.UserID)).Only(txCtx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get grant user: %w", err)
	}
	if grantUser.Status != payment.EntityStatusActive {
		return nil, infraerrors.Forbidden("USER_INACTIVE", "user account is disabled")
	}

	now := time.Now().UTC()
	order, err := client.PaymentOrder.Create().
		SetUserID(grantUser.ID).
		SetUserEmail(grantUser.Email).
		SetUserName(grantUser.Username).
		SetNillableUserNotes(psNilIfEmpty(grantUser.Notes)).
		SetAmount(0).
		SetPayAmount(0).
		SetFeeRate(0).
		SetRechargeCode("").
		SetOutTradeNo("").
		SetPaymentType(payment.PaymentTypeAdminGrant).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeBalanceSubscription).
		SetBalancePackagePlanID(plan.ID).
		SetBalancePackageWeeklyCreditUsd(plan.WeeklyCreditUsd).
		SetBalancePackageRefreshCount(plan.RefreshCount).
		SetBalancePackageRefreshIntervalDays(plan.RefreshIntervalDays).
		SetBalancePackageValidityDays(plan.ValidityDays).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(now).
		SetPaidAt(now).
		SetCompletedAt(now).
		SetClientIP("admin").
		SetSrcHost("admin").
		Save(txCtx)
	if err != nil {
		return nil, fmt.Errorf("create admin balance package grant order: %w", err)
	}
	order, err = client.PaymentOrder.UpdateOneID(order.ID).
		SetRechargeCode("ADMIN-GRANT-" + strconv.FormatInt(order.ID, 10)).
		Save(txCtx)
	if err != nil {
		return nil, fmt.Errorf("set admin balance package grant code: %w", err)
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
	return &BalancePackageGrant{OrderID: order.ID, BalancePackageID: pkg.ID}, nil
}
