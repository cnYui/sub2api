package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"entgo.io/ent/dialect"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/usersubscription"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	RefundGatewayNotStarted  = "NOT_STARTED"
	RefundGatewayNotRequired = "NOT_REQUIRED"
	RefundGatewayPending     = "PENDING"
	RefundGatewaySucceeded   = "SUCCEEDED"
	RefundGatewayFailed      = "FAILED"
	RefundGatewayUnknown     = "UNKNOWN"

	RefundEntitlementNotStarted = "NOT_STARTED"
	RefundEntitlementSucceeded  = "SUCCEEDED"
	RefundEntitlementFailed     = "FAILED"
	RefundEntitlementManual     = "MANUAL_REVIEW"
)

func paymentOrderRefundRetryable(order *dbent.PaymentOrder) bool {
	if order == nil || order.Status != OrderStatusRefundFailed {
		return false
	}
	return order.RefundGatewayStatus == RefundGatewayFailed ||
		order.RefundGatewayStatus == RefundGatewaySucceeded && order.RefundEntitlementStatus == RefundEntitlementFailed
}

func PaymentOrderRefundRetryable(order *dbent.PaymentOrder) bool {
	return paymentOrderRefundRetryable(order)
}

func paymentOrderRefundContinuable(order *dbent.PaymentOrder) bool {
	return paymentOrderRefundRetryable(order) ||
		order != nil && order.Status == OrderStatusRefunding && order.RefundGatewayStatus == RefundGatewaySucceeded
}

func (s *PaymentService) validateExclusiveRefundSubscription(ctx context.Context, order *dbent.PaymentOrder, sub *UserSubscription) error {
	return s.validateExclusiveRefundSubscriptionWithClient(ctx, s.entClient, order, sub)
}

func (s *PaymentService) validateExclusiveRefundSubscriptionWithClient(ctx context.Context, client *dbent.Client, order *dbent.PaymentOrder, sub *UserSubscription) error {
	if order == nil || order.SubscriptionID == nil {
		return infraerrors.BadRequest("SUBSCRIPTION_LINK_REQUIRED", "refund requires an exact subscription link")
	}
	if sub == nil {
		return infraerrors.BadRequest("SUBSCRIPTION_NOT_FOUND", "linked subscription not found")
	}
	if order.SubscriptionGroupID == nil || order.SubscriptionDays == nil || sub.ID != *order.SubscriptionID || sub.UserID != order.UserID || sub.GroupID != *order.SubscriptionGroupID {
		return infraerrors.BadRequest("SUBSCRIPTION_MISMATCH", "linked subscription does not match order")
	}
	linkedOrders, err := client.PaymentOrder.Query().
		Where(
			paymentorder.SubscriptionIDEQ(sub.ID),
			paymentorder.OrderTypeEQ(payment.OrderTypeSubscription),
		).
		Count(ctx)
	if err != nil {
		return err
	}
	if linkedOrders != 1 {
		return infraerrors.Conflict("SHARED_SUBSCRIPTION_REFUND_REQUIRES_MANUAL", "shared subscription entitlement requires manual refund review")
	}

	// TODO: 引入订单级权益区间后移除此限制；当前共享订阅行无法安全拆分后续续期、赠送或人工调整的权益。
	expectedExpiresAt := sub.StartsAt.AddDate(0, 0, *order.SubscriptionDays)
	delta := sub.ExpiresAt.Sub(expectedExpiresAt)
	if delta < 0 {
		delta = -delta
	}
	if delta > time.Second {
		return infraerrors.Conflict("SUBSCRIPTION_TERM_CHANGED_REFUND_REQUIRES_MANUAL", "subscription term changed after purchase")
	}
	return nil
}

func (s *PaymentService) lockAndLoadRefundSubscription(ctx context.Context, client *dbent.Client, order *dbent.PaymentOrder) (*UserSubscription, error) {
	if order == nil || order.SubscriptionID == nil || s.subscriptionSvc == nil {
		return nil, infraerrors.BadRequest("SUBSCRIPTION_LINK_REQUIRED", "refund requires an exact subscription link")
	}
	if client != nil && client.Driver() != nil && client.Driver().Dialect() == dialect.Postgres {
		_, err := client.UserSubscription.Query().
			Where(usersubscription.IDEQ(*order.SubscriptionID)).
			ForUpdate().
			Only(ctx)
		if err != nil {
			if dbent.IsNotFound(err) {
				return nil, ErrSubscriptionNotFound
			}
			return nil, err
		}
	}
	return s.subscriptionSvc.GetByID(ctx, *order.SubscriptionID)
}

func refundEntitlementRequiresManualReview(err error) bool {
	switch infraerrors.Reason(err) {
	case "SUBSCRIPTION_LINK_REQUIRED", "SUBSCRIPTION_MISMATCH", "SHARED_SUBSCRIPTION_REFUND_REQUIRES_MANUAL", "SUBSCRIPTION_TERM_CHANGED_REFUND_REQUIRES_MANUAL":
		return true
	default:
		return false
	}
}

func (s *PaymentService) lockRefundOrder(ctx context.Context, client *dbent.Client, orderID int64) (*dbent.PaymentOrder, error) {
	query := client.PaymentOrder.Query().Where(paymentorder.IDEQ(orderID))
	if client.Driver() != nil && client.Driver().Dialect() == dialect.Postgres {
		query = query.ForUpdate()
	}
	return query.Only(ctx)
}

func (s *PaymentService) revokeRefundSubscriptionInTransaction(ctx context.Context, sub *UserSubscription) error {
	if sub == nil || s.subscriptionSvc == nil || s.subscriptionSvc.userSubRepo == nil {
		return infraerrors.BadRequest("SUBSCRIPTION_NOT_FOUND", "linked subscription not found")
	}
	if err := s.subscriptionSvc.userSubRepo.UpdateStatus(ctx, sub.ID, SubscriptionStatusExpired); err != nil {
		return err
	}
	return s.subscriptionSvc.userSubRepo.Delete(ctx, sub.ID)
}

func (s *PaymentService) invalidateRefundSubscriptionCaches(sub *UserSubscription) {
	if sub == nil || s.subscriptionSvc == nil {
		return
	}
	s.subscriptionSvc.InvalidateSubCache(sub.UserID, sub.GroupID)
	if s.billingCacheService != nil {
		_ = s.billingCacheService.InvalidateSubscription(context.Background(), sub.UserID, sub.GroupID)
	}
}

func (s *PaymentService) markGatewayRefundEntitlementFailure(ctx context.Context, orderID int64, cause error, operator string, refundAmount float64) {
	now := time.Now()
	_, _ = s.entClient.PaymentOrder.UpdateOneID(orderID).
		SetStatus(OrderStatusRefundFailed).
		SetRefundEntitlementStatus(RefundEntitlementFailed).
		SetFailedAt(now).
		SetFailedReason(psErrMsg(cause)).
		Save(ctx)
	s.writeAuditLog(ctx, orderID, "REFUND_REVOKE_FAILED", operator, map[string]any{
		"detail": psErrMsg(cause), "refundAmount": refundAmount,
	})
}

func (s *PaymentService) completeGatewaySubscriptionRefundTransaction(
	ctx context.Context,
	orderID int64,
	fallbackReason string,
	operator string,
	force bool,
) (*RefundResult, error) {
	if s == nil || s.entClient == nil || s.subscriptionSvc == nil {
		return nil, infraerrors.InternalServer("SUBSCRIPTION_SERVICE_UNAVAILABLE", "subscription service is unavailable")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin gateway refund finalization transaction: %w", err)
	}
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	rollback := func(cause error) error {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("%w; rollback gateway refund finalization: %v", cause, rollbackErr)
		}
		return cause
	}

	order, err := s.lockRefundOrder(txCtx, client, orderID)
	if err != nil {
		return nil, rollback(fmt.Errorf("lock refund order: %w", err))
	}
	if order.OrderType != payment.OrderTypeSubscription || order.RefundGatewayStatus != RefundGatewaySucceeded {
		return nil, rollback(infraerrors.Conflict("INVALID_REFUND_STATE", "gateway refund is not ready for entitlement finalization"))
	}
	if order.Status != OrderStatusRefunding && order.Status != OrderStatusRefundFailed {
		return nil, rollback(infraerrors.Conflict("INVALID_REFUND_STATE", "refund order is not in a continuable state"))
	}
	if order.RefundAmount <= 0 || psStringValue(order.RefundRequestID) == "" {
		return nil, rollback(infraerrors.BadRequest("INVALID_REFUND_STATE", "refund facts are incomplete"))
	}

	reason := strings.TrimSpace(psStringValue(order.RefundRequestReason))
	if reason == "" {
		reason = strings.TrimSpace(psStringValue(order.RefundReason))
	}
	if reason == "" {
		reason = strings.TrimSpace(fallbackReason)
	}
	if reason == "" {
		reason = fmt.Sprintf("refund order:%d", order.ID)
	}

	sub, subErr := s.lockAndLoadRefundSubscription(txCtx, client, order)
	if subErr != nil && !errors.Is(subErr, ErrSubscriptionNotFound) {
		failure := rollback(fmt.Errorf("load refund subscription: %w", subErr))
		s.markGatewayRefundEntitlementFailure(ctx, order.ID, failure, operator, order.RefundAmount)
		return nil, failure
	}
	if sub != nil {
		if err := s.validateExclusiveRefundSubscriptionWithClient(txCtx, client, order, sub); err != nil {
			if refundEntitlementRequiresManualReview(err) {
				now := time.Now()
				if _, updateErr := client.PaymentOrder.UpdateOneID(order.ID).
					SetStatus(OrderStatusRefundFailed).
					SetRefundEntitlementStatus(RefundEntitlementManual).
					SetFailedAt(now).
					SetFailedReason(psErrMsg(err)).
					Save(txCtx); updateErr != nil {
					return nil, rollback(fmt.Errorf("mark refund for manual review: %w", updateErr))
				}
				if auditErr := s.createAuditLogIfAbsentWithClient(txCtx, client, order.ID, "REFUND_MANUAL_REVIEW_REQUIRED", operator, map[string]any{
					"detail": psErrMsg(err), "refundAmount": order.RefundAmount,
				}); auditErr != nil {
					return nil, rollback(fmt.Errorf("write manual refund review audit: %w", auditErr))
				}
				if commitErr := tx.Commit(); commitErr != nil {
					return nil, fmt.Errorf("commit manual refund review state: %w", commitErr)
				}
				return nil, err
			}
			failure := rollback(fmt.Errorf("validate refund subscription: %w", err))
			s.markGatewayRefundEntitlementFailure(ctx, order.ID, failure, operator, order.RefundAmount)
			return nil, failure
		}
		if err := s.revokeRefundSubscriptionInTransaction(txCtx, sub); err != nil && !errors.Is(err, ErrSubscriptionNotFound) {
			failure := rollback(fmt.Errorf("revoke subscription after gateway refund: %w", err))
			s.markGatewayRefundEntitlementFailure(ctx, order.ID, failure, operator, order.RefundAmount)
			return nil, failure
		}
	}

	finalStatus := OrderStatusRefunded
	if order.RefundAmount < order.Amount {
		finalStatus = OrderStatusPartiallyRefunded
	}
	now := time.Now()
	if _, err := client.PaymentOrder.UpdateOneID(order.ID).
		SetStatus(finalStatus).
		SetRefundAmount(order.RefundAmount).
		SetRefundReason(reason).
		SetRefundAt(now).
		SetForceRefund(force).
		SetRefundEntitlementStatus(RefundEntitlementSucceeded).
		ClearFailedAt().
		ClearFailedReason().
		Save(txCtx); err != nil {
		failure := rollback(fmt.Errorf("finalize gateway refund order: %w", err))
		s.markGatewayRefundEntitlementFailure(ctx, order.ID, failure, operator, order.RefundAmount)
		return nil, failure
	}
	if err := s.createAuditLogIfAbsentWithClient(txCtx, client, order.ID, "REFUND_SUCCESS", operator, map[string]any{
		"refundAmount": order.RefundAmount, "reason": reason, "force": force,
	}); err != nil {
		failure := rollback(fmt.Errorf("write gateway refund success audit: %w", err))
		s.markGatewayRefundEntitlementFailure(ctx, order.ID, failure, operator, order.RefundAmount)
		return nil, failure
	}
	if err := tx.Commit(); err != nil {
		failure := fmt.Errorf("commit gateway refund finalization transaction: %w", err)
		s.markGatewayRefundEntitlementFailure(ctx, order.ID, failure, operator, order.RefundAmount)
		return nil, failure
	}
	s.invalidateRefundSubscriptionCaches(sub)
	subDays := 0
	if order.SubscriptionDays != nil {
		subDays = *order.SubscriptionDays
	}
	return &RefundResult{Success: true, SubDaysDeducted: subDays}, nil
}
