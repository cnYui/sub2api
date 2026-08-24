package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func TestGrantBalancePackageCreatesAuditablePackageLifecycle(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	target, err := client.User.Create().
		SetEmail("grant-target@example.com").
		SetPasswordHash("hash").
		Save(ctx)
	if err != nil {
		t.Fatalf("create target user: %v", err)
	}
	plan, err := client.BalancePackagePlan.Create().
		SetCode("grant-test").
		SetName("Grant Test").
		SetPriceCny(29).
		SetWeeklyCreditUsd(76).
		SetValidityDays(28).
		SetRefreshCount(4).
		SetRefreshIntervalDays(7).
		SetForSale(true).
		Save(ctx)
	if err != nil {
		t.Fatalf("create balance package plan: %v", err)
	}
	svc := &PaymentService{
		entClient:             client,
		balancePackageService: NewBalancePackageService(client),
	}

	grant, err := svc.GrantBalancePackage(ctx, GrantBalancePackageInput{
		UserID:               target.ID,
		BalancePackagePlanID: plan.ID,
		AdminUserID:          99,
	})
	if err != nil {
		t.Fatalf("grant balance package: %v", err)
	}
	if grant.OrderID <= 0 || grant.BalancePackageID <= 0 {
		t.Fatalf("invalid grant result: %#v", grant)
	}

	order, err := client.PaymentOrder.Get(ctx, grant.OrderID)
	if err != nil {
		t.Fatalf("get grant order: %v", err)
	}
	if order.PaymentType != payment.PaymentTypeAdminGrant || order.OrderType != payment.OrderTypeBalanceSubscription || order.Status != OrderStatusCompleted {
		t.Fatalf("unexpected grant order: payment_type=%q order_type=%q status=%q", order.PaymentType, order.OrderType, order.Status)
	}
	if order.Amount != 0 || order.PayAmount != 0 || order.BalancePackagePlanID == nil || *order.BalancePackagePlanID != plan.ID {
		t.Fatalf("unexpected grant order financial snapshot: %#v", order)
	}

	pkg, err := client.UserBalancePackage.Get(ctx, grant.BalancePackageID)
	if err != nil {
		t.Fatalf("get user balance package: %v", err)
	}
	if pkg.PaymentOrderID != order.ID || pkg.CreditedCount != 1 || pkg.WeeklyCreditUsd != plan.WeeklyCreditUsd {
		t.Fatalf("unexpected balance package state: %#v", pkg)
	}
	creditedUser, err := client.User.Get(ctx, target.ID)
	if err != nil {
		t.Fatalf("get credited user: %v", err)
	}
	if creditedUser.Balance != plan.WeeklyCreditUsd || creditedUser.TotalRecharged != plan.WeeklyCreditUsd {
		t.Fatalf("unexpected user credits: balance=%v total_recharged=%v", creditedUser.Balance, creditedUser.TotalRecharged)
	}
	grantAuditCount, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(order.RechargeCode[len("ADMIN-GRANT-"):]), paymentauditlog.ActionEQ("ADMIN_BALANCE_PACKAGE_GRANTED")).
		Count(ctx)
	if err != nil {
		t.Fatalf("count grant audit logs: %v", err)
	}
	if grantAuditCount != 1 {
		t.Fatalf("grant audit count = %d, want 1", grantAuditCount)
	}

	if _, err := svc.PrepareRefund(ctx, order.ID, "test"); infraerrors.Reason(err) != "ADMIN_GRANTED_ORDER" {
		t.Fatalf("admin grant refund error reason = %q, want ADMIN_GRANTED_ORDER (err=%v)", infraerrors.Reason(err), err)
	}
}
