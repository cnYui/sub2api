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

// balancePackageGrantOrder 描述一笔零金额、非支付的余额套餐发放订单。
// 后台手动发放和兑换码发放共用它，保证两条路径写出的订单快照完全一致。
type balancePackageGrantOrder struct {
	Plan               *dbent.BalancePackagePlan
	User               *dbent.User
	PaymentType        string
	RechargeCodePrefix string
	Origin             string
}

// createBalancePackageGrantOrder 在调用方事务内创建零金额的已完成订单。
// recharge_code 依赖自增 ID，因此先建后补，与历史实现保持一致。
func createBalancePackageGrantOrder(ctx context.Context, client *dbent.Client, in balancePackageGrantOrder) (*dbent.PaymentOrder, error) {
	now := time.Now().UTC()
	order, err := client.PaymentOrder.Create().
		SetUserID(in.User.ID).
		SetUserEmail(in.User.Email).
		SetUserName(in.User.Username).
		SetNillableUserNotes(psNilIfEmpty(in.User.Notes)).
		SetAmount(0).
		SetPayAmount(0).
		SetFeeRate(0).
		SetRechargeCode("").
		SetOutTradeNo("").
		SetPaymentType(in.PaymentType).
		SetPaymentTradeNo("").
		SetOrderType(payment.OrderTypeBalanceSubscription).
		SetBalancePackagePlanID(in.Plan.ID).
		SetBalancePackageWeeklyCreditUsd(in.Plan.WeeklyCreditUsd).
		SetBalancePackageRefreshCount(in.Plan.RefreshCount).
		SetBalancePackageRefreshIntervalDays(in.Plan.RefreshIntervalDays).
		SetBalancePackageValidityDays(in.Plan.ValidityDays).
		SetStatus(OrderStatusCompleted).
		SetExpiresAt(now).
		SetPaidAt(now).
		SetCompletedAt(now).
		SetClientIP(in.Origin).
		SetSrcHost(in.Origin).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create balance package grant order: %w", err)
	}
	order, err = client.PaymentOrder.UpdateOneID(order.ID).
		SetRechargeCode(in.RechargeCodePrefix + strconv.FormatInt(order.ID, 10)).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("set balance package grant code: %w", err)
	}
	return order, nil
}

// loadGrantableUser 读取并校验发放目标用户；禁用账号不发放。
func loadGrantableUser(ctx context.Context, client *dbent.Client, userID int64) (*dbent.User, error) {
	grantUser, err := client.User.Query().Where(user.IDEQ(userID)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get grant user: %w", err)
	}
	if grantUser.Status != payment.EntityStatusActive {
		return nil, infraerrors.Forbidden("USER_INACTIVE", "user account is disabled")
	}
	return grantUser, nil
}

// GrantByRedeemCode 在调用方事务内按档位发放余额套餐，供兑换码流程使用。
// 返回建出来的订单，调用方在事务提交后用它发到账通知邮件。
//
// 与后台手动发放的两点差异：
//   - 不要求档位仍在售：兑换码发出时就已经是对用户的承诺，之后下架档位不应让码作废；
//   - 订单 payment_type 记为 redeem_code，既能在订单列表里区分来源，也让退款白名单天然拒绝它。
//
// 用户已有有效套餐时由 creditInitialBalance 直接拒绝（不续费、不改绑订单），
// 事务回滚会把兑换码恢复为未使用，用户可以在本期套餐失效后再兑换。
func (s *BalancePackageService) GrantByRedeemCode(ctx context.Context, client *dbent.Client, userID, planID int64, code string) (*dbent.PaymentOrder, error) {
	if s == nil {
		return nil, infraerrors.ServiceUnavailable("BALANCE_PACKAGE_UNAVAILABLE", "balance package service is unavailable")
	}
	if client == nil {
		return nil, fmt.Errorf("balance package transaction client is unavailable")
	}
	if userID <= 0 || planID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_GRANT_INPUT", "user and plan are required")
	}

	plan, err := client.BalancePackagePlan.Query().Where(balancepackageplan.IDEQ(planID)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, infraerrors.NotFound("BALANCE_PACKAGE_NOT_AVAILABLE", "balance package is not available")
		}
		return nil, fmt.Errorf("get balance package plan: %w", err)
	}
	if err := validateBalancePackagePlan(plan); err != nil {
		return nil, err
	}

	grantUser, err := loadGrantableUser(ctx, client, userID)
	if err != nil {
		return nil, err
	}

	order, err := createBalancePackageGrantOrder(ctx, client, balancePackageGrantOrder{
		Plan:               plan,
		User:               grantUser,
		PaymentType:        payment.PaymentTypeRedeemCode,
		RechargeCodePrefix: "REDEEM-",
		Origin:             "redeem",
	})
	if err != nil {
		return nil, err
	}

	pkg, err := s.creditInitialBalance(ctx, client, order)
	if err != nil {
		return nil, err
	}
	if pkg == nil {
		return nil, infraerrors.Conflict("BALANCE_PACKAGE_ALREADY_GRANTED", "balance package grant already exists")
	}

	detail, _ := json.Marshal(map[string]any{
		"redeem_code":  code,
		"plan_id":      plan.ID,
		"payment_type": payment.PaymentTypeRedeemCode,
	})
	if _, err := client.PaymentAuditLog.Create().
		SetOrderID(strconv.FormatInt(order.ID, 10)).
		SetAction("REDEEM_BALANCE_PACKAGE_GRANTED").
		SetDetail(string(detail)).
		SetOperator(fmt.Sprintf("user:%d", userID)).
		Save(ctx); err != nil {
		return nil, fmt.Errorf("record redeem balance package grant audit: %w", err)
	}

	return order, nil
}
