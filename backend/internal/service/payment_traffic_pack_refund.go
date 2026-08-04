package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// GetTrafficPackRefundQuote 按流量卡已用额度与已过时间的较大值计算退款报价。
func (s *PaymentService) GetTrafficPackRefundQuote(ctx context.Context, orderID, userID int64) (*BalancePackageRefundQuote, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, orderID)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.UserID != userID {
		return nil, infraerrors.Forbidden("FORBIDDEN", "no permission")
	}
	if o.PaymentType == payment.PaymentTypeAdminGrant {
		return nil, infraerrors.Forbidden("ADMIN_GRANTED_ORDER", "admin granted packages cannot be refunded")
	}
	return s.calculateTrafficPackRefundQuote(ctx, o)
}

func (s *PaymentService) calculateTrafficPackRefundQuote(ctx context.Context, o *dbent.PaymentOrder) (*BalancePackageRefundQuote, error) {
	if o == nil || o.OrderType != payment.OrderTypeTrafficPack {
		return nil, infraerrors.BadRequest("INVALID_ORDER_TYPE", "only traffic pack orders have a refund quote")
	}
	quote := &BalancePackageRefundQuote{
		PurchaseBaseAmount: o.Amount,
		NonRefundableFee:   math.Max(o.PayAmount-o.Amount, 0),
		CalculatedAt:       time.Now().UTC(),
	}
	if s.trafficPackService == nil {
		quote.ManualReviewRequired = true
		return quote, nil
	}
	credit, err := s.trafficPackService.GetCreditByOrderID(ctx, o.ID)
	if err != nil || credit == nil {
		if err == nil || err == ErrUserNotFound {
			quote.ManualReviewRequired = true
			return quote, nil
		}
		return nil, fmt.Errorf("find traffic credit: %w", err)
	}
	if !IsTrafficPackPlatform(TrafficPackPlatformOpenAI) || credit.ExpiresAt.Before(credit.CreditedAt) || credit.ExpiresAt.Equal(credit.CreditedAt) {
		quote.ManualReviewRequired = true
		return quote, nil
	}
	quote.PeriodStartsAt = credit.CreditedAt
	quote.PeriodExpiresAt = credit.ExpiresAt
	quote.PeriodTotalQuotaUSD = math.Max(credit.InitialUSD, 0)
	quote.UsedQuotaUSD = math.Max(credit.InitialUSD-credit.RemainingUSD, 0)
	quote.TimeRatio = refundTimeRatio(credit.CreditedAt, credit.ExpiresAt, quote.CalculatedAt)
	quote.UsageRatio, quote.ConsumptionRatio, quote.EstimatedRefundAmount = calculateBalancePackageRefundAmounts(quote.PurchaseBaseAmount, quote.TimeRatio, quote.UsedQuotaUSD, quote.PeriodTotalQuotaUSD)
	quote.Eligible = quote.EstimatedRefundAmount > 0
	return quote, nil
}

func (s *PaymentService) requireTrafficPackRefundQuote(ctx context.Context, o *dbent.PaymentOrder) (*BalancePackageRefundQuote, error) {
	quote, err := s.calculateTrafficPackRefundQuote(ctx, o)
	if err != nil {
		return nil, err
	}
	if quote.ManualReviewRequired {
		return nil, errBalancePackageRefundManualReview
	}
	if !quote.Eligible {
		return nil, infraerrors.BadRequest("NO_REFUNDABLE_QUOTA", "traffic pack has been fully consumed")
	}
	return quote, nil
}

func (s *PaymentService) requestTrafficPackRefund(ctx context.Context, o *dbent.PaymentOrder, userID int64, reason string) error {
	quote, err := s.requireTrafficPackRefundQuote(ctx, o)
	if err != nil {
		return err
	}
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		trimmedReason = fmt.Sprintf("refund order:%d", o.ID)
	}
	updated, err := s.entClient.PaymentOrder.Update().
		Where(paymentorder.IDEQ(o.ID), paymentorder.UserIDEQ(userID), paymentorder.StatusIn(OrderStatusCompleted, OrderStatusRefundFailed), paymentorder.OrderTypeEQ(payment.OrderTypeTrafficPack)).
		SetStatus(OrderStatusRefundRequested).
		SetRefundRequestedAt(time.Now().UTC()).
		SetRefundRequestReason(trimmedReason).
		SetRefundRequestedBy(fmt.Sprintf("%d", userID)).
		SetRefundAmount(quote.EstimatedRefundAmount).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("lock traffic pack refund: %w", err)
	}
	if updated == 0 {
		return infraerrors.Conflict("CONFLICT", "order status changed")
	}
	locked := *o
	locked.Status = OrderStatusRefundRequested
	locked.RefundAmount = quote.EstimatedRefundAmount
	locked.RefundRequestReason = &trimmedReason
	plan := &RefundPlan{
		OrderID: locked.ID, Order: &locked, RefundAmount: quote.EstimatedRefundAmount,
		GatewayAmount: calculateGatewayRefundAmount(locked.Amount, locked.PayAmount, quote.EstimatedRefundAmount, PaymentOrderCurrency(&locked)),
		Reason:        trimmedReason, DeductionType: payment.DeductionTypeNone, Operator: "user",
	}
	_, err = s.ExecuteRefund(ctx, plan)
	return err
}
