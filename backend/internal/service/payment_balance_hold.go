package service

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentbalancehold"
	"github.com/Wei-Shaw/sub2api/ent/user"
)

const (
	balanceHoldStatusReserved = "RESERVED"
	balanceHoldStatusCaptured = "CAPTURED"
	balanceHoldStatusReleased = "RELEASED"
)

func (s *PaymentService) reserveBalanceForHybridOrder(ctx context.Context, orderID, userID int64, amount float64, expiresAt time.Time) error {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin balance hold transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := reserveBalanceForHybridOrderTx(ctx, tx, orderID, userID, amount, expiresAt); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit balance hold transaction: %w", err)
	}
	return nil
}

func reserveBalanceForHybridOrderTx(ctx context.Context, tx *dbent.Tx, orderID, userID int64, amount float64, expiresAt time.Time) error {
	updated, err := tx.User.Update().
		Where(user.IDEQ(userID), user.BalanceGTE(amount)).
		AddBalance(-amount).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("reserve user balance: %w", err)
	}
	if updated != 1 {
		return errCheckoutChanged
	}

	if _, err := tx.PaymentBalanceHold.Create().
		SetOrderID(orderID).
		SetUserID(userID).
		SetAmount(amount).
		SetStatus(balanceHoldStatusReserved).
		SetExpiresAt(expiresAt).
		Save(ctx); err != nil {
		return fmt.Errorf("create balance hold: %w", err)
	}
	return nil
}

func (s *PaymentService) capturePaymentBalanceHold(ctx context.Context, orderID int64) (int, error) {
	updated, err := s.entClient.PaymentBalanceHold.Update().
		Where(
			paymentbalancehold.OrderIDEQ(orderID),
			paymentbalancehold.StatusEQ(balanceHoldStatusReserved),
		).
		SetStatus(balanceHoldStatusCaptured).
		SetCapturedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("capture balance hold: %w", err)
	}
	return updated, nil
}

func (s *PaymentService) releasePaymentBalanceHold(ctx context.Context, orderID int64, reason string) (int, error) {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin balance release transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	hold, err := tx.PaymentBalanceHold.Query().
		Where(
			paymentbalancehold.OrderIDEQ(orderID),
			paymentbalancehold.StatusEQ(balanceHoldStatusReserved),
		).
		Only(ctx)
	if dbent.IsNotFound(err) {
		return 0, tx.Commit()
	}
	if err != nil {
		return 0, fmt.Errorf("query reserved balance hold: %w", err)
	}

	now := time.Now()
	updated, err := tx.PaymentBalanceHold.Update().
		Where(
			paymentbalancehold.IDEQ(hold.ID),
			paymentbalancehold.StatusEQ(balanceHoldStatusReserved),
		).
		SetStatus(balanceHoldStatusReleased).
		SetReleasedAt(now).
		SetReleaseReason(reason).
		Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("release balance hold: %w", err)
	}
	if updated == 0 {
		if err := tx.Commit(); err != nil {
			return 0, fmt.Errorf("commit skipped balance release: %w", err)
		}
		return 0, nil
	}
	if _, err := tx.User.UpdateOneID(hold.UserID).AddBalance(hold.Amount).Save(ctx); err != nil {
		return 0, fmt.Errorf("restore released balance: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit balance release transaction: %w", err)
	}
	return updated, nil
}
