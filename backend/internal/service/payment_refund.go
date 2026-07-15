package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/paymentproviderinstance"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// --- Refund Flow ---

// getOrderProviderInstance looks up the provider instance that processed this order.
// For legacy orders without provider_instance_id, it resolves only when the
// historical instance is uniquely identifiable from the stored order fields.
func (s *PaymentService) getOrderProviderInstance(ctx context.Context, o *dbent.PaymentOrder) (*dbent.PaymentProviderInstance, error) {
	if s == nil || s.entClient == nil || o == nil {
		return nil, nil
	}

	if snapshot := psOrderProviderSnapshot(o); snapshot != nil {
		return s.resolveSnapshotOrderProviderInstance(ctx, o, snapshot)
	}

	instIDStr := strings.TrimSpace(psStringValue(o.ProviderInstanceID))
	if instIDStr == "" {
		return s.resolveUniqueLegacyOrderProviderInstance(ctx, o)
	}

	instID, err := strconv.ParseInt(instIDStr, 10, 64)
	if err != nil {
		return nil, nil
	}
	return s.entClient.PaymentProviderInstance.Get(ctx, instID)
}

// getRefundOrderProviderInstance resolves the provider instance for refund paths.
// Refunds must be pinned to an explicit historical binding, so legacy
// "best-effort" provider guessing is intentionally not allowed here.
func (s *PaymentService) getRefundOrderProviderInstance(ctx context.Context, o *dbent.PaymentOrder) (*dbent.PaymentProviderInstance, error) {
	if s == nil || s.entClient == nil || o == nil {
		return nil, nil
	}

	if snapshot := psOrderProviderSnapshot(o); snapshot != nil {
		return s.resolveSnapshotOrderProviderInstance(ctx, o, snapshot)
	}

	instIDStr := strings.TrimSpace(psStringValue(o.ProviderInstanceID))
	if instIDStr == "" {
		return nil, nil
	}

	instID, err := strconv.ParseInt(instIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("order %d refund provider instance id is invalid: %s", o.ID, instIDStr)
	}
	inst, err := s.entClient.PaymentProviderInstance.Get(ctx, instID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, fmt.Errorf("order %d refund provider instance %s is missing", o.ID, instIDStr)
		}
		return nil, err
	}
	return inst, nil
}

func (s *PaymentService) resolveUniqueLegacyOrderProviderInstance(ctx context.Context, o *dbent.PaymentOrder) (*dbent.PaymentProviderInstance, error) {
	paymentType := payment.GetBasePaymentType(strings.TrimSpace(o.PaymentType))
	providerKey := strings.TrimSpace(psStringValue(o.ProviderKey))
	if providerKey != "" {
		instances, err := s.entClient.PaymentProviderInstance.Query().
			Where(paymentproviderinstance.ProviderKeyEQ(providerKey)).
			All(ctx)
		if err != nil {
			return nil, err
		}
		matched := psFilterLegacyOrderProviderInstances(paymentType, instances)
		if len(matched) == 1 {
			return matched[0], nil
		}
		return nil, nil
	}

	if paymentType == "" {
		return nil, nil
	}

	instances, err := s.entClient.PaymentProviderInstance.Query().
		All(ctx)
	if err != nil {
		return nil, err
	}

	matched := psFilterLegacyOrderProviderInstances(paymentType, instances)
	if len(matched) == 1 {
		return matched[0], nil
	}
	return nil, nil
}

func psFilterLegacyOrderProviderInstances(orderPaymentType string, instances []*dbent.PaymentProviderInstance) []*dbent.PaymentProviderInstance {
	if len(instances) == 0 {
		return nil
	}
	if strings.TrimSpace(orderPaymentType) == "" {
		return instances
	}
	var matched []*dbent.PaymentProviderInstance
	for _, inst := range instances {
		if psLegacyOrderMatchesInstance(orderPaymentType, inst) {
			matched = append(matched, inst)
		}
	}
	return matched
}

func psLegacyOrderMatchesInstance(orderPaymentType string, inst *dbent.PaymentProviderInstance) bool {
	if inst == nil {
		return false
	}

	baseType := payment.GetBasePaymentType(strings.TrimSpace(orderPaymentType))
	instanceProviderKey := strings.TrimSpace(inst.ProviderKey)
	if baseType == "" {
		return false
	}

	if baseType == payment.TypeStripe {
		return instanceProviderKey == payment.TypeStripe
	}
	if instanceProviderKey == payment.TypeStripe {
		return false
	}
	if instanceProviderKey == baseType {
		return true
	}
	return payment.InstanceSupportsType(inst.SupportedTypes, baseType)
}

func (s *PaymentService) RequestRefund(ctx context.Context, oid, uid int64, reason string) error {
	o, err := s.validateUserAutoRefundRequest(ctx, oid, uid)
	if err != nil {
		return err
	}
	return s.executeUserAutoRefund(ctx, o, uid, reason)
}

func (s *PaymentService) validateUserAutoRefundRequest(ctx context.Context, oid, uid int64) (*dbent.PaymentOrder, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	if o.UserID != uid {
		return nil, infraerrors.Forbidden("FORBIDDEN", "no permission")
	}
	if o.Status != OrderStatusCompleted && !paymentOrderRefundContinuable(o) {
		return nil, infraerrors.BadRequest("INVALID_STATUS", "order status does not allow refund")
	}
	if o.OrderType != payment.OrderTypeSubscription {
		return nil, infraerrors.BadRequest("INVALID_ORDER_TYPE", "only subscription orders can request refund")
	}
	if o.SubscriptionGroupID == nil || o.SubscriptionDays == nil || o.SubscriptionID == nil {
		return nil, infraerrors.BadRequest("INVALID_STATUS", "missing subscription info")
	}
	if o.PaymentType == payment.TypeBalance {
		return o, nil
	}
	if payment.GetBasePaymentType(o.PaymentType) != payment.TypeAlipay {
		return nil, infraerrors.BadRequest("INVALID_PAYMENT_TYPE", "only alipay or balance subscription orders can request refund")
	}
	if o.RefundGatewayStatus == RefundGatewaySucceeded {
		return o, nil
	}
	inst, err := s.getRefundOrderProviderInstance(ctx, o)
	if err != nil || inst == nil {
		return nil, infraerrors.Forbidden("USER_REFUND_DISABLED", "refund is not available for this order")
	}
	if !inst.RefundEnabled || !inst.AllowUserRefund {
		return nil, infraerrors.Forbidden("USER_REFUND_DISABLED", "user refund is not enabled for this provider")
	}
	return o, nil
}

func (s *PaymentService) executeUserAutoRefund(ctx context.Context, o *dbent.PaymentOrder, uid int64, reason string) error {
	if s == nil || s.subscriptionSvc == nil {
		return infraerrors.InternalServer("SUBSCRIPTION_SERVICE_UNAVAILABLE", "subscription service is unavailable")
	}
	sub, err := s.subscriptionSvc.GetByID(ctx, *o.SubscriptionID)
	missingAfterGatewaySuccess := o.RefundGatewayStatus == RefundGatewaySucceeded && errors.Is(err, ErrSubscriptionNotFound)
	if err != nil && !missingAfterGatewaySuccess {
		return infraerrors.BadRequest("SUBSCRIPTION_NOT_FOUND", "linked subscription not found")
	}
	if sub != nil && o.RefundGatewayStatus != RefundGatewaySucceeded {
		if err := s.validateExclusiveRefundSubscription(ctx, o, sub); err != nil {
			return err
		}
	}
	now := time.Now()
	isRetry := o.Status == OrderStatusRefundFailed
	isContinuation := o.Status == OrderStatusRefunding && o.RefundGatewayStatus == RefundGatewaySucceeded
	isExistingRefund := isRetry || isContinuation
	refundAmount := o.RefundAmount
	nr := psStringValue(o.RefundRequestReason)
	requestID := psStringValue(o.RefundRequestID)
	by := fmt.Sprintf("%d", uid)
	lock := s.entClient.PaymentOrder.Update().Where(paymentorder.IDEQ(o.ID), paymentorder.UserIDEQ(uid))
	if isExistingRefund {
		if refundAmount <= 0 || requestID == "" {
			return infraerrors.BadRequest("INVALID_REFUND_STATE", "refund facts are incomplete")
		}
		lock = lock.Where(paymentorder.StatusEQ(o.Status))
	} else {
		if sub == nil || sub.Status != SubscriptionStatusActive || !sub.ExpiresAt.After(now) {
			return infraerrors.BadRequest("SUBSCRIPTION_NOT_FOUND", "active linked subscription not found")
		}
		refundAmount = calculateSubscriptionRefundAmount(o.Amount, *o.SubscriptionDays, sub.StartsAt, now)
		if refundAmount <= 0 {
			return infraerrors.BadRequest("NO_REFUNDABLE_DAYS", "no refundable subscription days remaining")
		}
		nr = strings.TrimSpace(reason)
		if nr == "" {
			nr = fmt.Sprintf("refund order:%d", o.ID)
		}
		requestID = fmt.Sprintf("refund-%d", o.ID)
		if o.PaymentType == payment.TypeBalance {
			return s.executeBalanceSubscriptionRefundTransaction(ctx, o, sub, refundAmount, nr, by, "user:"+by, requestID, false, OrderStatusCompleted, now)
		}
		lock = lock.
			Where(paymentorder.StatusEQ(OrderStatusCompleted)).
			SetRefundRequestedAt(now).
			SetRefundRequestReason(nr).
			SetRefundRequestedBy(by).
			SetRefundAmount(refundAmount).
			SetRefundRequestID(requestID).
			SetRefundGatewayStatus(RefundGatewayNotStarted).
			SetRefundEntitlementStatus(RefundEntitlementNotStarted)
	}
	c, err := lock.SetStatus(OrderStatusRefunding).ClearFailedAt().ClearFailedReason().Save(ctx)
	if err != nil {
		return fmt.Errorf("lock refund order: %w", err)
	}
	if c == 0 {
		return infraerrors.Conflict("CONFLICT", "order status changed")
	}
	locked, err := s.entClient.PaymentOrder.Get(ctx, o.ID)
	if err != nil {
		return err
	}
	auditAction := "REFUND_REQUESTED"
	if isRetry {
		auditAction = "REFUND_RETRY_REQUESTED"
	} else if isContinuation {
		auditAction = "REFUND_CONTINUED"
	}
	s.writeAuditLog(ctx, o.ID, auditAction, "user:"+by, map[string]any{"amount": refundAmount, "reason": nr, "auto": true, "retry": isRetry, "continuation": isContinuation, "requestID": requestID})

	return s.executeUserGatewaySubscriptionRefund(ctx, locked, refundAmount, nr, "user:"+by)
}

func (s *PaymentService) executeUserGatewaySubscriptionRefund(ctx context.Context, o *dbent.PaymentOrder, refundAmount float64, reason, operator string) error {
	if o.RefundGatewayStatus == RefundGatewaySucceeded {
		_, err := s.completeGatewaySubscriptionRefundTransaction(ctx, o.ID, reason, operator, false)
		return err
	}
	if o.RefundGatewayStatus != RefundGatewayNotStarted && o.RefundGatewayStatus != RefundGatewayFailed {
		return infraerrors.Conflict("REFUND_NOT_RETRYABLE", "gateway refund status requires manual reconciliation")
	}
	p := &RefundPlan{
		OrderID:       o.ID,
		Order:         o,
		RefundAmount:  refundAmount,
		GatewayAmount: refundAmount,
		Reason:        reason,
	}
	if _, err := s.entClient.PaymentOrder.UpdateOneID(o.ID).SetRefundGatewayStatus(RefundGatewayUnknown).Save(ctx); err != nil {
		return fmt.Errorf("mark gateway refund in progress: %w", err)
	}
	resp, err := s.gwRefund(ctx, p)
	if err != nil {
		now := time.Now()
		gatewayStatus := RefundGatewayUnknown
		if payment.IsRefundRejected(err) {
			gatewayStatus = RefundGatewayFailed
		}
		_, _ = s.entClient.PaymentOrder.UpdateOneID(o.ID).
			SetStatus(OrderStatusRefundFailed).
			SetRefundGatewayStatus(gatewayStatus).
			SetFailedAt(now).
			SetFailedReason(psErrMsg(err)).
			Save(ctx)
		s.writeAuditLog(ctx, o.ID, "REFUND_FAILED", "user", map[string]any{"detail": psErrMsg(err), "auto": true})
		return infraerrors.InternalServer("REFUND_FAILED", psErrMsg(err))
	}
	if resp.Status == payment.ProviderStatusPending {
		_, err := s.entClient.PaymentOrder.UpdateOneID(o.ID).
			SetRefundGatewayStatus(RefundGatewayPending).
			SetNillableRefundProviderRef(psNilIfEmpty(resp.RefundID)).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("mark gateway refund pending: %w", err)
		}
		s.writeAuditLog(ctx, o.ID, "REFUND_GATEWAY_PENDING", "user", map[string]any{"providerRef": resp.RefundID, "auto": true})
		return nil
	}
	if _, err := s.entClient.PaymentOrder.UpdateOneID(o.ID).
		SetRefundGatewayStatus(RefundGatewaySucceeded).
		SetNillableRefundProviderRef(psNilIfEmpty(resp.RefundID)).
		Save(ctx); err != nil {
		return fmt.Errorf("persist gateway refund success: %w", err)
	}
	o.RefundGatewayStatus = RefundGatewaySucceeded
	o.RefundProviderRef = psNilIfEmpty(resp.RefundID)
	_, err = s.completeGatewaySubscriptionRefundTransaction(ctx, o.ID, reason, operator, false)
	return err
}

func (s *PaymentService) executeBalanceSubscriptionRefundTransaction(
	ctx context.Context,
	o *dbent.PaymentOrder,
	_ *UserSubscription,
	refundAmount float64,
	reason string,
	requestedBy string,
	operator string,
	requestID string,
	force bool,
	expectedStatus string,
	now time.Time,
) error {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin balance refund transaction: %w", err)
	}
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	rollback := func(cause error) error {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("%w; rollback balance refund: %v", cause, rollbackErr)
		}
		return cause
	}

	c, err := client.PaymentOrder.Update().
		Where(paymentorder.IDEQ(o.ID), paymentorder.UserIDEQ(o.UserID), paymentorder.StatusEQ(expectedStatus)).
		SetStatus(OrderStatusRefunding).
		SetRefundRequestedAt(now).
		SetRefundRequestReason(reason).
		SetRefundRequestedBy(requestedBy).
		SetRefundAmount(refundAmount).
		SetRefundRequestID(requestID).
		SetRefundGatewayStatus(RefundGatewayNotRequired).
		SetRefundEntitlementStatus(RefundEntitlementNotStarted).
		ClearFailedAt().
		ClearFailedReason().
		Save(txCtx)
	if err != nil {
		return rollback(fmt.Errorf("lock balance refund order: %w", err))
	}
	if c == 0 {
		return rollback(infraerrors.Conflict("CONFLICT", "order status changed"))
	}
	lockedSub, err := s.lockAndLoadRefundSubscription(txCtx, client, o)
	if err != nil {
		return rollback(infraerrors.BadRequest("SUBSCRIPTION_NOT_FOUND", "linked subscription not found"))
	}
	if err := s.validateExclusiveRefundSubscriptionWithClient(txCtx, client, o, lockedSub); err != nil {
		return rollback(err)
	}
	n, err := client.User.Update().Where(user.IDEQ(o.UserID)).AddBalance(refundAmount).Save(txCtx)
	if err != nil {
		return rollback(fmt.Errorf("refund balance: %w", err))
	}
	if n == 0 {
		return rollback(infraerrors.NotFound("USER_NOT_FOUND", "user not found"))
	}
	if err := s.revokeRefundSubscriptionInTransaction(txCtx, lockedSub); err != nil && !errors.Is(err, ErrSubscriptionNotFound) {
		return rollback(fmt.Errorf("revoke subscription after balance refund: %w", err))
	}
	finalStatus := OrderStatusRefunded
	if refundAmount < o.Amount {
		finalStatus = OrderStatusPartiallyRefunded
	}
	_, err = client.PaymentOrder.UpdateOneID(o.ID).
		SetStatus(finalStatus).
		SetRefundAmount(refundAmount).
		SetRefundReason(reason).
		SetRefundAt(now).
		SetForceRefund(force).
		SetRefundGatewayStatus(RefundGatewayNotRequired).
		SetRefundEntitlementStatus(RefundEntitlementSucceeded).
		ClearFailedAt().
		ClearFailedReason().
		Save(txCtx)
	if err != nil {
		return rollback(fmt.Errorf("mark balance refund success: %w", err))
	}
	if err := s.createAuditLogIfAbsentWithClient(txCtx, client, o.ID, "REFUND_REQUESTED", operator, map[string]any{
		"amount": refundAmount, "reason": reason, "auto": true, "retry": false, "requestID": requestID,
	}); err != nil {
		return rollback(fmt.Errorf("write balance refund request audit: %w", err))
	}
	if err := s.createAuditLogIfAbsentWithClient(txCtx, client, o.ID, "REFUND_SUCCESS", operator, map[string]any{
		"refundAmount": refundAmount, "reason": reason, "balanceRefunded": refundAmount, "auto": true,
	}); err != nil {
		return rollback(fmt.Errorf("write balance refund success audit: %w", err))
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit balance refund transaction: %w", err)
	}

	s.invalidateRefundSubscriptionCaches(lockedSub)
	if s.billingCacheService != nil {
		_ = s.billingCacheService.InvalidateUserBalance(context.Background(), o.UserID)
	}
	return nil
}

func (s *PaymentService) PrepareRefund(ctx context.Context, oid int64, amt float64, reason string, force, deduct bool) (*RefundPlan, *RefundResult, error) {
	o, err := s.entClient.PaymentOrder.Get(ctx, oid)
	if err != nil {
		return nil, nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	isRetry := o.Status == OrderStatusRefundFailed
	isContinuation := o.Status == OrderStatusRefunding && o.RefundGatewayStatus == RefundGatewaySucceeded
	isExistingRefund := isRetry || isContinuation
	if (o.Status == OrderStatusRefundFailed || o.Status == OrderStatusRefunding) && !paymentOrderRefundContinuable(o) {
		return nil, nil, infraerrors.Conflict("REFUND_RECONCILIATION_REQUIRED", "refund status requires manual reconciliation")
	}
	if o.Status != OrderStatusCompleted && o.Status != OrderStatusRefundRequested && !isExistingRefund {
		return nil, nil, infraerrors.BadRequest("INVALID_STATUS", "order status does not allow refund")
	}
	if o.OrderType == payment.OrderTypeSubscription {
		if o.SubscriptionID == nil || s.subscriptionSvc == nil {
			return nil, nil, infraerrors.BadRequest("SUBSCRIPTION_LINK_REQUIRED", "refund requires an exact subscription link")
		}
		sub, subErr := s.subscriptionSvc.GetByID(ctx, *o.SubscriptionID)
		missingAfterGatewaySuccess := o.RefundGatewayStatus == RefundGatewaySucceeded && errors.Is(subErr, ErrSubscriptionNotFound)
		if subErr != nil && !missingAfterGatewaySuccess {
			return nil, nil, infraerrors.BadRequest("SUBSCRIPTION_NOT_FOUND", "linked subscription not found")
		}
		if sub != nil && o.RefundGatewayStatus != RefundGatewaySucceeded {
			if err := s.validateExclusiveRefundSubscription(ctx, o, sub); err != nil {
				return nil, nil, err
			}
		}
	}
	if o.PaymentType != payment.TypeBalance && o.RefundGatewayStatus != RefundGatewaySucceeded {
		inst, instErr := s.getRefundOrderProviderInstance(ctx, o)
		if instErr != nil {
			slog.Warn("refund: provider instance lookup failed", "orderID", oid, "error", instErr)
			return nil, nil, infraerrors.InternalServer("PROVIDER_LOOKUP_FAILED", "failed to look up payment provider for this order")
		}
		if inst == nil {
			return nil, nil, infraerrors.Forbidden("REFUND_DISABLED", "refund is not available for this order")
		}
		if !inst.RefundEnabled {
			return nil, nil, infraerrors.Forbidden("REFUND_DISABLED", "refund is not enabled for this provider")
		}
	}
	if math.IsNaN(amt) || math.IsInf(amt, 0) {
		return nil, nil, infraerrors.BadRequest("INVALID_AMOUNT", "invalid refund amount")
	}
	if isExistingRefund {
		amt = o.RefundAmount
		if amt <= 0 || psStringValue(o.RefundRequestID) == "" {
			return nil, nil, infraerrors.BadRequest("INVALID_REFUND_STATE", "refund facts are incomplete")
		}
	} else if amt <= 0 {
		amt = o.Amount
	}
	orderCurrency := PaymentOrderCurrency(o)
	if amt-o.Amount > paymentAmountToleranceForCurrency(orderCurrency) {
		return nil, nil, infraerrors.BadRequest("REFUND_AMOUNT_EXCEEDED", "refund amount exceeds recharge")
	}
	ga := amt
	if o.OrderType != payment.OrderTypeSubscription {
		ga = calculateGatewayRefundAmount(o.Amount, o.PayAmount, amt, orderCurrency)
	}
	rr := strings.TrimSpace(reason)
	if isExistingRefund && o.RefundRequestReason != nil {
		rr = *o.RefundRequestReason
	} else if rr == "" && o.RefundRequestReason != nil {
		rr = *o.RefundRequestReason
	}
	if rr == "" {
		rr = fmt.Sprintf("refund order:%d", o.ID)
	}
	p := &RefundPlan{OrderID: oid, Order: o, RefundAmount: amt, GatewayAmount: ga, Reason: rr, Force: force, DeductBalance: deduct, DeductionType: payment.DeductionTypeNone}
	if o.OrderType == payment.OrderTypeSubscription || deduct {
		if er := s.prepDeduct(ctx, o, p, force); er != nil {
			return nil, er, nil
		}
	}
	return p, nil, nil
}

func (s *PaymentService) prepDeduct(ctx context.Context, o *dbent.PaymentOrder, p *RefundPlan, force bool) *RefundResult {
	if o.OrderType == payment.OrderTypeSubscription {
		p.DeductionType = payment.DeductionTypeSubscription
		if o.SubscriptionID != nil && o.SubscriptionDays != nil {
			p.SubDaysToDeduct = *o.SubscriptionDays
			sub, err := s.subscriptionSvc.GetByID(ctx, *o.SubscriptionID)
			if err == nil && sub != nil {
				p.SubscriptionID = sub.ID
			} else if o.RefundGatewayStatus == RefundGatewaySucceeded && errors.Is(err, ErrSubscriptionNotFound) {
				p.SubscriptionID = *o.SubscriptionID
			} else if !force {
				return &RefundResult{Success: false, Warning: "cannot find active subscription for deduction, use force", RequireForce: true}
			}
		}
		return nil
	}
	u, err := s.userRepo.GetByID(ctx, o.UserID)
	if err != nil {
		if !force {
			return &RefundResult{Success: false, Warning: "cannot fetch user balance, use force", RequireForce: true}
		}
		return nil
	}
	p.DeductionType = payment.DeductionTypeBalance
	p.BalanceToDeduct = math.Min(p.RefundAmount, u.Balance)
	return nil
}

func (s *PaymentService) ExecuteRefund(ctx context.Context, p *RefundPlan) (*RefundResult, error) {
	if p == nil {
		return nil, infraerrors.BadRequest("INVALID_REFUND_PLAN", "refund plan is required")
	}
	o, err := s.entClient.PaymentOrder.Get(ctx, p.OrderID)
	if err != nil {
		return nil, infraerrors.NotFound("NOT_FOUND", "order not found")
	}
	isRetry := o.Status == OrderStatusRefundFailed
	isContinuation := o.Status == OrderStatusRefunding && o.RefundGatewayStatus == RefundGatewaySucceeded
	isExistingRefund := isRetry || isContinuation
	if (o.Status == OrderStatusRefundFailed || o.Status == OrderStatusRefunding) && !paymentOrderRefundContinuable(o) {
		return nil, infraerrors.Conflict("REFUND_RECONCILIATION_REQUIRED", "refund status requires manual reconciliation")
	}
	if o.Status != OrderStatusCompleted && o.Status != OrderStatusRefundRequested && !isExistingRefund {
		return nil, infraerrors.BadRequest("INVALID_STATUS", "order status does not allow refund")
	}
	var linkedSubscription *UserSubscription
	if o.OrderType == payment.OrderTypeSubscription {
		if o.SubscriptionID == nil || s.subscriptionSvc == nil {
			return nil, infraerrors.BadRequest("SUBSCRIPTION_LINK_REQUIRED", "refund requires an exact subscription link")
		}
		linkedSubscription, err = s.subscriptionSvc.GetByID(ctx, *o.SubscriptionID)
		missingAfterGatewaySuccess := o.RefundGatewayStatus == RefundGatewaySucceeded && errors.Is(err, ErrSubscriptionNotFound)
		if err != nil && !missingAfterGatewaySuccess {
			return nil, infraerrors.BadRequest("SUBSCRIPTION_NOT_FOUND", "linked subscription not found")
		}
		if linkedSubscription != nil && o.RefundGatewayStatus != RefundGatewaySucceeded {
			if err := s.validateExclusiveRefundSubscription(ctx, o, linkedSubscription); err != nil {
				return nil, err
			}
		}
		p.DeductionType = payment.DeductionTypeSubscription
		p.SubscriptionID = *o.SubscriptionID
		if o.SubscriptionDays != nil {
			p.SubDaysToDeduct = *o.SubscriptionDays
		}
	}
	if o.PaymentType == payment.TypeBalance && o.OrderType == payment.OrderTypeSubscription {
		if linkedSubscription == nil {
			return nil, infraerrors.BadRequest("SUBSCRIPTION_NOT_FOUND", "linked subscription not found")
		}
		requestID := psStringValue(o.RefundRequestID)
		if requestID == "" {
			requestID = fmt.Sprintf("refund-%d", o.ID)
		}
		if err := s.executeBalanceSubscriptionRefundTransaction(ctx, o, linkedSubscription, p.RefundAmount, p.Reason, "admin", "admin", requestID, p.Force, o.Status, time.Now()); err != nil {
			return nil, err
		}
		return &RefundResult{Success: true, BalanceDeducted: -p.RefundAmount, SubDaysDeducted: p.SubDaysToDeduct}, nil
	}

	lock := s.entClient.PaymentOrder.Update().Where(paymentorder.IDEQ(p.OrderID), paymentorder.StatusEQ(o.Status))
	if isExistingRefund {
		p.RefundAmount = o.RefundAmount
		p.Reason = psStringValue(o.RefundRequestReason)
		if p.Reason == "" {
			p.Reason = psStringValue(o.RefundReason)
		}
		p.GatewayAmount = p.RefundAmount
		if o.OrderType != payment.OrderTypeSubscription {
			p.GatewayAmount = calculateGatewayRefundAmount(o.Amount, o.PayAmount, p.RefundAmount, PaymentOrderCurrency(o))
		}
	} else {
		requestID := psStringValue(o.RefundRequestID)
		if requestID == "" {
			requestID = fmt.Sprintf("refund-%d", o.ID)
		}
		gatewayStatus := RefundGatewayNotStarted
		if o.PaymentType == payment.TypeBalance {
			gatewayStatus = RefundGatewayNotRequired
		}
		lock = lock.
			SetRefundAmount(p.RefundAmount).
			SetRefundRequestID(requestID).
			SetRefundRequestReason(p.Reason).
			SetRefundGatewayStatus(gatewayStatus).
			SetRefundEntitlementStatus(RefundEntitlementNotStarted)
	}
	c, err := lock.SetStatus(OrderStatusRefunding).ClearFailedAt().ClearFailedReason().Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("lock: %w", err)
	}
	if c == 0 {
		return nil, infraerrors.Conflict("CONFLICT", "order status changed")
	}
	locked, err := s.entClient.PaymentOrder.Get(ctx, p.OrderID)
	if err != nil {
		return nil, err
	}
	p.Order = locked
	if locked.RefundGatewayStatus == RefundGatewaySucceeded {
		return s.executeAdminRefundEntitlement(ctx, p)
	}
	if locked.RefundGatewayStatus == RefundGatewayNotRequired {
		return s.executeAdminRefundEntitlement(ctx, p)
	}
	if locked.RefundGatewayStatus != RefundGatewayNotStarted && locked.RefundGatewayStatus != RefundGatewayFailed {
		return nil, infraerrors.Conflict("REFUND_RECONCILIATION_REQUIRED", "refund status requires manual reconciliation")
	}
	if _, err := s.entClient.PaymentOrder.UpdateOneID(p.OrderID).SetRefundGatewayStatus(RefundGatewayUnknown).Save(ctx); err != nil {
		return nil, fmt.Errorf("mark gateway refund in progress: %w", err)
	}
	resp, err := s.gwRefund(ctx, p)
	if err != nil {
		gatewayStatus := RefundGatewayUnknown
		if payment.IsRefundRejected(err) {
			gatewayStatus = RefundGatewayFailed
		}
		now := time.Now()
		_, _ = s.entClient.PaymentOrder.UpdateOneID(p.OrderID).
			SetStatus(OrderStatusRefundFailed).
			SetRefundGatewayStatus(gatewayStatus).
			SetFailedAt(now).
			SetFailedReason(psErrMsg(err)).
			Save(ctx)
		return nil, infraerrors.InternalServer("REFUND_FAILED", psErrMsg(err))
	}
	if resp.Status == payment.ProviderStatusPending {
		_, err := s.entClient.PaymentOrder.UpdateOneID(p.OrderID).
			SetRefundGatewayStatus(RefundGatewayPending).
			SetNillableRefundProviderRef(psNilIfEmpty(resp.RefundID)).
			Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("mark gateway refund pending: %w", err)
		}
		return &RefundResult{Success: false, Warning: "gateway refund pending"}, nil
	}
	if _, err := s.entClient.PaymentOrder.UpdateOneID(p.OrderID).
		SetRefundGatewayStatus(RefundGatewaySucceeded).
		SetNillableRefundProviderRef(psNilIfEmpty(resp.RefundID)).
		Save(ctx); err != nil {
		return nil, fmt.Errorf("persist gateway refund success: %w", err)
	}
	p.Order.RefundGatewayStatus = RefundGatewaySucceeded
	return s.executeAdminRefundEntitlement(ctx, p)
}

func (s *PaymentService) executeAdminRefundEntitlement(ctx context.Context, p *RefundPlan) (*RefundResult, error) {
	o := p.Order
	if o == nil {
		return nil, infraerrors.BadRequest("INVALID_REFUND_PLAN", "refund order is required")
	}
	if o.OrderType == payment.OrderTypeSubscription && o.RefundGatewayStatus == RefundGatewaySucceeded {
		return s.completeGatewaySubscriptionRefundTransaction(ctx, o.ID, p.Reason, "admin", p.Force)
	}
	if o.RefundEntitlementStatus != RefundEntitlementSucceeded {
		deductionType := p.DeductionType
		if o.RefundEntitlementStatus == RefundEntitlementFailed && o.OrderType == payment.OrderTypeSubscription && o.SubscriptionID != nil {
			deductionType = payment.DeductionTypeSubscription
			p.SubscriptionID = *o.SubscriptionID
		}
		var err error
		switch deductionType {
		case payment.DeductionTypeBalance:
			if p.BalanceToDeduct > 0 {
				if s.userRepo == nil {
					err = fmt.Errorf("user repository is unavailable")
				} else {
					err = s.userRepo.DeductBalance(ctx, o.UserID, p.BalanceToDeduct)
				}
			}
		case payment.DeductionTypeSubscription:
			subscriptionID := p.SubscriptionID
			if subscriptionID == 0 && o.SubscriptionID != nil {
				subscriptionID = *o.SubscriptionID
			}
			if subscriptionID == 0 {
				err = fmt.Errorf("refund subscription link is missing")
			} else if s.subscriptionSvc == nil {
				err = fmt.Errorf("subscription service is unavailable")
			} else if revokeErr := s.subscriptionSvc.RevokeSubscription(ctx, subscriptionID); revokeErr != nil && !errors.Is(revokeErr, ErrSubscriptionNotFound) {
				err = revokeErr
			}
		}
		if err != nil {
			now := time.Now()
			_, _ = s.entClient.PaymentOrder.UpdateOneID(p.OrderID).
				SetStatus(OrderStatusRefundFailed).
				SetRefundEntitlementStatus(RefundEntitlementFailed).
				SetFailedAt(now).
				SetFailedReason(psErrMsg(err)).
				Save(ctx)
			return nil, fmt.Errorf("apply refund entitlement: %w", err)
		}
		if _, err := s.entClient.PaymentOrder.UpdateOneID(p.OrderID).SetRefundEntitlementStatus(RefundEntitlementSucceeded).Save(ctx); err != nil {
			return nil, fmt.Errorf("mark refund entitlement success: %w", err)
		}
	}
	return s.markRefundOk(ctx, p)
}

func (s *PaymentService) gwRefund(ctx context.Context, p *RefundPlan) (*payment.RefundResponse, error) {
	if strings.TrimSpace(p.Order.PaymentTradeNo) == "" && strings.TrimSpace(p.Order.OutTradeNo) == "" {
		return nil, fmt.Errorf("payment refund missing trade identifier")
	}

	// Use the exact provider instance that created this order, not a random one
	// from the registry. Each instance has its own merchant credentials.
	prov, err := s.getRefundProvider(ctx, p.Order)
	if err != nil {
		return nil, fmt.Errorf("get refund provider: %w", err)
	}
	if err := validateProviderSnapshotMetadata(p.Order, prov.ProviderKey(), providerMerchantIdentityMetadata(prov)); err != nil {
		s.writeAuditLog(ctx, p.Order.ID, "REFUND_PROVIDER_METADATA_MISMATCH", "admin", map[string]any{
			"detail": err.Error(),
		})
		return nil, err
	}
	resp, err := prov.Refund(ctx, payment.RefundRequest{
		TradeNo:   p.Order.PaymentTradeNo,
		OrderID:   p.Order.OutTradeNo,
		Amount:    formatGatewayRefundAmount(p.GatewayAmount, p.Order),
		Reason:    p.Reason,
		RequestID: psStringValue(p.Order.RefundRequestID),
	})
	if err != nil {
		return resp, err
	}
	if err := validateRefundProviderResponse(resp); err != nil {
		return resp, err
	}
	return resp, nil
}

func formatGatewayRefundAmount(amount float64, order *dbent.PaymentOrder) string {
	return payment.FormatAmountForCurrency(amount, PaymentOrderCurrency(order))
}

func validateRefundProviderResponse(resp *payment.RefundResponse) error {
	if resp == nil {
		return fmt.Errorf("payment refund response missing")
	}
	status := strings.TrimSpace(resp.Status)
	switch status {
	case payment.ProviderStatusSuccess, payment.ProviderStatusRefunded, payment.ProviderStatusPending:
		return nil
	case payment.ProviderStatusFailed:
		return &payment.RefundRejectedError{Err: fmt.Errorf("payment refund failed: status %s", status)}
	default:
		return fmt.Errorf("payment refund returned unknown status: %s", status)
	}
}

// getRefundProvider creates a provider using the order's original instance config.
// Delegates to getOrderProvider which handles instance lookup and fallback.
func (s *PaymentService) getRefundProvider(ctx context.Context, o *dbent.PaymentOrder) (payment.Provider, error) {
	if s != nil && s.refundProvider != nil {
		return s.refundProvider, nil
	}
	inst, err := s.getRefundOrderProviderInstance(ctx, o)
	if err != nil {
		return nil, err
	}
	if inst == nil {
		return nil, fmt.Errorf("refund provider instance is unavailable for order %d", o.ID)
	}
	return s.createProviderFromInstance(ctx, inst)
}

func (s *PaymentService) markRefundOk(ctx context.Context, p *RefundPlan) (*RefundResult, error) {
	fs := OrderStatusRefunded
	if p.RefundAmount < p.Order.Amount {
		fs = OrderStatusPartiallyRefunded
	}
	now := time.Now()
	_, err := s.entClient.PaymentOrder.UpdateOneID(p.OrderID).
		SetStatus(fs).
		SetRefundAmount(p.RefundAmount).
		SetRefundReason(p.Reason).
		SetRefundAt(now).
		SetForceRefund(p.Force).
		ClearFailedAt().
		ClearFailedReason().
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("mark refund: %w", err)
	}
	s.writeAuditLog(ctx, p.OrderID, "REFUND_SUCCESS", "admin", map[string]any{"refundAmount": p.RefundAmount, "reason": p.Reason, "balanceDeducted": p.BalanceToDeduct, "force": p.Force})
	return &RefundResult{Success: true, BalanceDeducted: p.BalanceToDeduct, SubDaysDeducted: p.SubDaysToDeduct}, nil
}
