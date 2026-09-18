package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/ent/userbalancepackage"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const balancePackageStatusRefunded = "refunded"

// BalancePackageRefundQuote 是余额套餐退款报价及其计算依据。
type BalancePackageRefundQuote struct {
	Eligible             bool    `json:"eligible"`
	ManualReviewRequired bool    `json:"manual_review_required"`
	PurchaseBaseAmount   float64 `json:"purchase_base_amount"`
	NonRefundableFee     float64 `json:"non_refundable_fee"`
	PeriodTotalQuotaUSD  float64 `json:"period_total_quota_usd"`
	// UsedQuotaUSD 含 RetainedQuotaUSD：取消套餐时留给用户的额度用户已经拿到，必须按已用计。
	UsedQuotaUSD     float64 `json:"used_quota_usd"`
	RetainedQuotaUSD float64 `json:"retained_quota_usd"`
	// ReclaimQuotaUSD 是退款成功时要从余额里收回的本周未用额度。
	ReclaimQuotaUSD       float64   `json:"reclaim_quota_usd"`
	UsageRatio            float64   `json:"usage_ratio"`
	TimeRatio             float64   `json:"time_ratio"`
	ConsumptionRatio      float64   `json:"consumption_ratio"`
	EstimatedRefundAmount float64   `json:"estimated_refund_amount"`
	CalculatedAt          time.Time `json:"calculated_at"`
	PeriodStartsAt        time.Time `json:"-"`
	PeriodExpiresAt       time.Time `json:"-"`
}

var errBalancePackageRefundManualReview = infraerrors.Conflict("REFUND_MANUAL_REVIEW_REQUIRED", "refund requires manual review")

// GetBalancePackageRefundQuote 只读计算用户可见的退款报价。
func (s *PaymentService) GetBalancePackageRefundQuote(ctx context.Context, orderID, userID int64) (*BalancePackageRefundQuote, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, orderID)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.UserID != userID {
		return nil, infraerrors.Forbidden("FORBIDDEN", "no permission")
	}
	if err := validateRealPaidBalancePackageOrder(o); err != nil {
		return nil, err
	}
	return s.calculateBalancePackageRefundQuote(ctx, o)
}

// GetBalancePackageRefundQuoteForAdmin 返回管理员确认退款前看到的实时报价，与确认后服务端重算的口径一致。
func (s *PaymentService) GetBalancePackageRefundQuoteForAdmin(ctx context.Context, orderID int64) (*BalancePackageRefundQuote, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, orderID)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if err := validateRealPaidBalancePackageOrder(o); err != nil {
		return nil, err
	}
	return s.calculateBalancePackageRefundQuote(ctx, o)
}

func (s *PaymentService) calculateBalancePackageRefundQuote(ctx context.Context, o *dbent.PaymentOrder) (*BalancePackageRefundQuote, error) {
	if err := validateRealPaidBalancePackageOrder(o); err != nil {
		return nil, err
	}
	quote := &BalancePackageRefundQuote{
		PurchaseBaseAmount: o.Amount,
		NonRefundableFee:   math.Max(o.PayAmount-o.Amount, 0),
		CalculatedAt:       time.Now().UTC(),
	}
	pkg, err := s.entClient.UserBalancePackage.Query().Where(userbalancepackage.PaymentOrderIDEQ(o.ID)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			quote.ManualReviewRequired = true
			return quote, nil
		}
		return nil, fmt.Errorf("find balance package: %w", err)
	}
	if pkg.ExpiresAt.Before(pkg.StartsAt) || pkg.ExpiresAt.Equal(pkg.StartsAt) {
		quote.ManualReviewRequired = true
		return quote, nil
	}
	quote.PeriodStartsAt = pkg.StartsAt
	quote.PeriodExpiresAt = pkg.ExpiresAt
	quote.PeriodTotalQuotaUSD = math.Max(pkg.WeeklyCreditUsd*float64(pkg.RefreshCount), 0)
	if quote.PeriodTotalQuotaUSD <= 0 {
		quote.ManualReviewRequired = true
		return quote, nil
	}
	quote.TimeRatio = refundTimeRatio(pkg.StartsAt, pkg.ExpiresAt, quote.CalculatedAt)
	var used float64
	var legacyUnattributed int64
	// 续费会复用同一套餐行并重置 starts_at；账本按套餐 ID 累计跨周期用量，
	// 因此退款报价必须限定在当前周期窗口（created_at >= starts_at），
	// 避免把上一周期（属于已被覆盖的旧订单）的用量算进本次续费订单。
	rows, err := s.entClient.QueryContext(ctx, `
		SELECT
			COALESCE(SUM(amount_usd) FILTER (WHERE source_type <> 'legacy_unattributed'), 0),
			COUNT(*) FILTER (WHERE source_type = 'legacy_unattributed')
		FROM balance_package_usage_ledger
		WHERE balance_package_id = $1
		  AND created_at >= $2
	`, pkg.ID, pkg.StartsAt)
	if err != nil {
		return nil, fmt.Errorf("sum balance package usage ledger: %w", err)
	}
	if !rows.Next() {
		_ = rows.Close()
		return nil, fmt.Errorf("sum balance package usage ledger: %w", sql.ErrNoRows)
	}
	if err := rows.Scan(&used, &legacyUnattributed); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("scan balance package usage ledger: %w", err)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("read balance package usage ledger: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close balance package usage ledger: %w", err)
	}
	if legacyUnattributed > 0 {
		quote.ManualReviewRequired = true
		return quote, nil
	}
	retained, known, err := s.balancePackageRetainedQuota(ctx, o.ID, pkg)
	if err != nil {
		return nil, err
	}
	if !known {
		quote.ManualReviewRequired = true
		return quote, nil
	}
	quote.RetainedQuotaUSD = retained
	quote.UsedQuotaUSD = math.Max(used, 0) + retained
	if pkg.Status != balancePackageStatusRefunded {
		quote.ReclaimQuotaUSD = math.Max(pkg.RemainingUsd, 0)
	}
	quote.UsageRatio, quote.ConsumptionRatio, quote.EstimatedRefundAmount = calculateBalancePackageRefundAmounts(
		quote.PurchaseBaseAmount,
		quote.TimeRatio,
		quote.UsedQuotaUSD,
		quote.PeriodTotalQuotaUSD,
	)
	quote.Eligible = quote.EstimatedRefundAmount > 0
	return quote, nil
}

// balancePackageRetainedQuota 返回管理员取消套餐时留在用户余额里的本周剩余额度。
// 取消只停止权益、不动余额，报价不把这笔算作已用，就会在退款时再为它退一次钱。
// known=false 表示无法确定留下了多少，调用方应转人工审核而不是猜。
func (s *PaymentService) balancePackageRetainedQuota(ctx context.Context, orderID int64, pkg *dbent.UserBalancePackage) (float64, bool, error) {
	if pkg == nil || pkg.Status != balancePackageStatusCancelled {
		return 0, true, nil
	}
	entry, err := s.entClient.PaymentAuditLog.Query().
		Where(
			paymentauditlog.OrderIDEQ(strconv.FormatInt(orderID, 10)),
			paymentauditlog.ActionEQ(balancePackageManualCancelAudit),
		).
		First(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			// 历史上有旧动作名和手工 SQL 取消的套餐，有的收回了额度、有的没有，只能人工判断。
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("find balance package cancellation audit: %w", err)
	}
	var detail struct {
		RemainingUSDBefore float64  `json:"remaining_usd_before"`
		BalanceUSD         *float64 `json:"balance_unchanged_usd"`
	}
	if err := json.Unmarshal([]byte(entry.Detail), &detail); err != nil {
		return 0, false, nil
	}
	retained := math.Max(detail.RemainingUSDBefore, 0)
	if detail.BalanceUSD != nil {
		// 余额比剩余额度还少时，用户实际只留下了余额里的那部分。
		retained = math.Min(retained, math.Max(*detail.BalanceUSD, 0))
	}
	return retained, true, nil
}

func refundTimeRatio(startsAt, expiresAt, now time.Time) float64 {
	d := expiresAt.Sub(startsAt)
	if d <= 0 {
		return 1
	}
	return clampRefundRatio(now.Sub(startsAt).Seconds() / d.Seconds())
}

func clampRefundRatio(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 1
	}
	return math.Min(math.Max(value, 0), 1)
}

func calculateBalancePackageRefundAmounts(purchaseAmount, timeRatio, usedQuotaUSD, totalQuotaUSD float64) (usageRatio, consumptionRatio, refundAmount float64) {
	if totalQuotaUSD <= 0 {
		return 1, 1, 0
	}
	usageRatio = clampRefundRatio(usedQuotaUSD / totalQuotaUSD)
	consumptionRatio = math.Max(clampRefundRatio(timeRatio), usageRatio)
	refundAmount = math.Max(purchaseAmount*(1-consumptionRatio), 0)
	return usageRatio, consumptionRatio, refundAmount
}

func (s *PaymentService) requireBalancePackageRefundQuote(ctx context.Context, o *dbent.PaymentOrder) (*BalancePackageRefundQuote, error) {
	quote, err := s.calculateBalancePackageRefundQuote(ctx, o)
	if err != nil {
		return nil, err
	}
	if quote.ManualReviewRequired {
		return nil, errBalancePackageRefundManualReview
	}
	// 额度已全部用尽时预计退款为 0：仍允许退款，只是走「零额度退款」——撤销套餐、
	// 订单置为 REFUNDED，但不向支付网关退任何钱（见 ExecuteRefund 的零额度分支）。
	// 这样用户在额度耗尽、余额为 0 后也能退掉当前套餐，从而购买其它档位。
	// 仍受前面的准入校验约束：必须是真实支付的余额套餐订单，且未落入人工复核。
	return quote, nil
}

func (s *PaymentService) requestBalancePackageRefund(ctx context.Context, o *dbent.PaymentOrder, userID int64, reason string) error {
	quote, err := s.requireBalancePackageRefundQuote(ctx, o)
	if err != nil {
		return err
	}
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		trimmedReason = fmt.Sprintf("refund order:%d", o.ID)
	}
	requestedBy := fmt.Sprintf("%d", userID)
	updated, err := s.entClient.PaymentOrder.Update().
		Where(paymentorder.IDEQ(o.ID), paymentorder.UserIDEQ(userID), paymentorder.StatusIn(OrderStatusCompleted, OrderStatusRefundFailed), paymentorder.OrderTypeEQ(payment.OrderTypeBalanceSubscription)).
		SetStatus(OrderStatusRefundRequested).
		SetRefundRequestedAt(time.Now().UTC()).
		SetRefundRequestReason(trimmedReason).
		SetRefundRequestedBy(requestedBy).
		SetRefundAmount(quote.EstimatedRefundAmount).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("lock balance package refund: %w", err)
	}
	if updated == 0 {
		return infraerrors.Conflict("CONFLICT", "order status changed")
	}
	locked := *o
	locked.Status = OrderStatusRefundRequested
	locked.RefundAmount = quote.EstimatedRefundAmount
	locked.RefundRequestReason = &trimmedReason
	plan := &RefundPlan{
		OrderID:       locked.ID,
		Order:         &locked,
		RefundAmount:  quote.EstimatedRefundAmount,
		GatewayAmount: calculateGatewayRefundAmount(locked.Amount, locked.PayAmount, quote.EstimatedRefundAmount, PaymentOrderCurrency(&locked)),
		Reason:        trimmedReason,
		Operator:      "user",
	}
	_, err = s.ExecuteRefund(ctx, plan)
	return err
}

// balancePackageRevocation 是退款撤销套餐的结果，PackageID 为 0 表示没有需要撤销的套餐。
type balancePackageRevocation struct {
	PackageID     int64
	UserID        int64
	ReclaimedUSD  float64
	BalanceBefore float64
	BalanceAfter  float64
}

// revokeBalancePackage 撤销订单对应的余额套餐，并像到期回收一样把本周未用额度从余额里扣回：
// 退款报价没把这部分算作已用，留在余额里就等于退了钱又把额度送给用户。
// client 必须由调用方传入，以便和订单状态更新共用同一个事务，保证「撤销套餐」和「订单置为 REFUNDED」原子提交。
func (s *PaymentService) revokeBalancePackage(ctx context.Context, client *dbent.Client, orderID int64) (balancePackageRevocation, error) {
	var result balancePackageRevocation
	candidate, err := client.UserBalancePackage.Query().
		Where(userbalancepackage.PaymentOrderIDEQ(orderID), userbalancepackage.StatusNEQ(balancePackageStatusRefunded)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return result, nil
		}
		return result, fmt.Errorf("find balance package to revoke: %w", err)
	}

	// 与计费、周额度到账相同的加锁顺序：先用户，后套餐。
	// 网关已经退款成功，用户被软删除时只撤销套餐、不收回额度，不能让整笔退款回滚。
	lockedUser, err := lockBalancePackageUser(ctx, client, candidate.UserID)
	if err != nil && !dbent.IsNotFound(err) {
		return result, fmt.Errorf("lock balance package user: %w", err)
	}
	packageQuery := client.UserBalancePackage.Query().Where(userbalancepackage.IDEQ(candidate.ID))
	if client.Driver().Dialect() == dialect.Postgres {
		packageQuery = packageQuery.ForUpdate()
	}
	current, err := packageQuery.Only(ctx)
	if err != nil {
		return result, fmt.Errorf("lock balance package: %w", err)
	}
	if current.Status == balancePackageStatusRefunded || current.PaymentOrderID != orderID {
		return result, nil
	}

	if _, err := client.UserBalancePackage.UpdateOneID(current.ID).
		SetStatus(balancePackageStatusRefunded).
		SetRemainingUsd(0).
		ClearNextCreditAt().
		Save(ctx); err != nil {
		return result, fmt.Errorf("revoke balance package: %w", err)
	}
	result.PackageID = current.ID
	result.UserID = current.UserID
	if lockedUser == nil {
		return result, nil
	}
	result.BalanceBefore = lockedUser.Balance
	result.BalanceAfter = lockedUser.Balance
	if current.RemainingUsd <= 0 {
		return result, nil
	}
	if _, err := client.User.Update().
		Where(user.IDEQ(current.UserID)).
		AddBalance(-current.RemainingUsd).
		Save(ctx); err != nil {
		return result, fmt.Errorf("reclaim balance package remaining credit: %w", err)
	}
	result.ReclaimedUSD = current.RemainingUsd
	result.BalanceAfter = lockedUser.Balance - current.RemainingUsd
	return result, nil
}
