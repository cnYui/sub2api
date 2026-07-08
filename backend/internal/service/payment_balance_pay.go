package service

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

func (s *PaymentService) BalancePayOrder(ctx context.Context, req BalancePayOrderRequest) (*BalancePayOrderResponse, error) {
	orderType, ok := payment.NormalizeOrderType(req.OrderType)
	if !ok || orderType == payment.OrderTypeBalance {
		return nil, infraerrors.BadRequest("INVALID_ORDER_TYPE", "balance payment only supports product orders")
	}
	req.OrderType = orderType

	cfg, err := s.configService.GetPaymentConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("get payment config: %w", err)
	}
	if !cfg.Enabled {
		return nil, infraerrors.Forbidden("PAYMENT_DISABLED", "payment system is disabled")
	}

	userEntity, err := s.entClient.User.Get(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if userEntity.Status != payment.EntityStatusActive {
		return nil, infraerrors.Forbidden("USER_INACTIVE", "user account is disabled")
	}

	orderAmount, payAmount, feeRate, plan, trafficPack, err := s.resolveBalancePayProduct(ctx, req, cfg)
	if err != nil {
		return nil, err
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin balance pay tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	if err := s.deductBalanceForPurchaseInTx(txCtx, tx, req.UserID, payAmount); err != nil {
		return nil, err
	}

	now := time.Now()
	outTradeNo, err := s.allocateOutTradeNo(txCtx, tx)
	if err != nil {
		return nil, err
	}
	orderBuilder := tx.PaymentOrder.Create().
		SetUserID(req.UserID).
		SetUserEmail(userEntity.Email).
		SetUserName(userEntity.Username).
		SetNillableUserNotes(psNilIfEmpty(userEntity.Notes)).
		SetAmount(orderAmount).
		SetPayAmount(payAmount).
		SetFeeRate(feeRate).
		SetRechargeCode(fmt.Sprintf("PAY-BALANCE-%d", now.UnixNano())).
		SetOutTradeNo(outTradeNo).
		SetPaymentType(payment.TypeBalance).
		SetPaymentTradeNo("balance").
		SetOrderType(req.OrderType).
		SetStatus(OrderStatusRecharging).
		SetExpiresAt(now).
		SetPaidAt(now).
		SetClientIP(req.ClientIP).
		SetSrcHost(req.SrcHost)
	if req.SrcURL != "" {
		orderBuilder.SetSrcURL(req.SrcURL)
	}
	if plan != nil {
		orderBuilder.SetPlanID(plan.ID).
			SetSubscriptionGroupID(plan.GroupID).
			SetSubscriptionDays(psComputeValidityDays(plan.ValidityDays, plan.ValidityUnit))
	}
	if trafficPack != nil {
		orderBuilder.SetProviderSnapshot(buildPaymentOrderProviderSnapshot(nil, CreateOrderRequest{}, trafficPack))
	}

	order, err := orderBuilder.Save(txCtx)
	if err != nil {
		return nil, fmt.Errorf("create balance payment order: %w", err)
	}

	switch req.OrderType {
	case payment.OrderTypeSubscription:
		err = s.fulfillSubscriptionOrderInTx(txCtx, tx.Client(), order, false)
	case payment.OrderTypeTrafficPack:
		err = s.fulfillTrafficPackOrderInTx(txCtx, tx.Client(), order)
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit balance pay tx: %w", err)
	}
	if s.billingCacheService != nil {
		_ = s.billingCacheService.InvalidateUserBalance(context.Background(), req.UserID)
	}
	return &BalancePayOrderResponse{
		OrderID:     order.ID,
		Amount:      orderAmount,
		PayAmount:   payAmount,
		FeeRate:     feeRate,
		Status:      OrderStatusCompleted,
		PaymentType: payment.TypeBalance,
		OrderType:   req.OrderType,
		OutTradeNo:  outTradeNo,
		Currency:    payment.DefaultPaymentCurrency,
	}, nil
}

func (s *PaymentService) resolveBalancePayProduct(ctx context.Context, req BalancePayOrderRequest, cfg *PaymentConfig) (float64, float64, float64, *dbent.SubscriptionPlan, *TrafficPack, error) {
	feeRate := cfg.RechargeFeeRate
	switch req.OrderType {
	case payment.OrderTypeSubscription:
		plan, err := s.validateSubOrder(ctx, CreateOrderRequest{UserID: req.UserID, OrderType: req.OrderType, PlanID: req.PlanID})
		if err != nil {
			return 0, 0, 0, nil, nil, err
		}
		_, payAmount, err := calculateCreateOrderPayAmountForOrder(req.OrderType, plan.Price, feeRate, cfg.BalanceRechargeMultiplier, payment.DefaultPaymentCurrency)
		if err != nil {
			return 0, 0, 0, nil, nil, err
		}
		return plan.Price, payAmount, feeRate, plan, nil, nil
	case payment.OrderTypeTrafficPack:
		pack, err := s.validateTrafficPackOrder(ctx, CreateOrderRequest{OrderType: req.OrderType, TrafficPackID: req.TrafficPackID})
		if err != nil {
			return 0, 0, 0, nil, nil, err
		}
		_, payAmount, err := calculateCreateOrderPayAmountForOrder(req.OrderType, pack.Price, feeRate, cfg.BalanceRechargeMultiplier, payment.DefaultPaymentCurrency)
		if err != nil {
			return 0, 0, 0, nil, nil, err
		}
		return pack.Price, payAmount, feeRate, nil, pack, nil
	default:
		return 0, 0, 0, nil, nil, infraerrors.BadRequest("INVALID_ORDER_TYPE", "balance payment only supports product orders")
	}
}

func (s *PaymentService) deductBalanceForPurchaseInTx(ctx context.Context, tx *dbent.Tx, userID int64, amount float64) error {
	n, err := tx.User.Update().
		Where(user.IDEQ(userID), user.BalanceGTE(amount)).
		AddBalance(-amount).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("deduct balance: %w", err)
	}
	if n == 0 {
		return infraerrors.BadRequest("BALANCE_INSUFFICIENT", "balance is insufficient")
	}
	reloaded, err := tx.User.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("reload deducted balance: %w", err)
	}
	rounded := decimal.NewFromFloat(reloaded.Balance).Round(2).InexactFloat64()
	if rounded != reloaded.Balance {
		if _, err := tx.User.Update().Where(user.IDEQ(userID)).SetBalance(rounded).Save(ctx); err != nil {
			return fmt.Errorf("round deducted balance: %w", err)
		}
	}
	return nil
}
