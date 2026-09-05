package service

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/userbalancepackage"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// BalancePackageRefundQuote 是余额套餐退款报价及其计算依据。
type BalancePackageRefundQuote struct {
	Eligible              bool      `json:"eligible"`
	ManualReviewRequired  bool      `json:"manual_review_required"`
	PurchaseBaseAmount    float64   `json:"purchase_base_amount"`
	NonRefundableFee      float64   `json:"non_refundable_fee"`
	PeriodTotalQuotaUSD   float64   `json:"period_total_quota_usd"`
	UsedQuotaUSD          float64   `json:"used_quota_usd"`
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
	quote.UsedQuotaUSD = math.Max(used, 0)
	quote.UsageRatio, quote.ConsumptionRatio, quote.EstimatedRefundAmount = calculateBalancePackageRefundAmounts(
		quote.PurchaseBaseAmount,
		quote.TimeRatio,
		quote.UsedQuotaUSD,
		quote.PeriodTotalQuotaUSD,
	)
	quote.Eligible = quote.EstimatedRefundAmount > 0
	return quote, nil
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
	if !quote.Eligible {
		return nil, infraerrors.BadRequest("NO_REFUNDABLE_QUOTA", "balance package has been fully consumed")
	}
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

// revokeBalancePackage 撤销订单对应的余额套餐。client 必须由调用方传入，
// 以便和订单状态更新共用同一个事务，保证「撤销套餐」和「订单置为 REFUNDED」原子提交。
func (s *PaymentService) revokeBalancePackage(ctx context.Context, client *dbent.Client, orderID int64) error {
	_, err := client.UserBalancePackage.Update().
		Where(userbalancepackage.PaymentOrderIDEQ(orderID), userbalancepackage.StatusNEQ("refunded")).
		SetStatus("refunded").
		SetRemainingUsd(0).
		ClearNextCreditAt().
		Save(ctx)
	return err
}
