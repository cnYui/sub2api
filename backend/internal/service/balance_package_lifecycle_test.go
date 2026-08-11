package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/ent/userbalancepackage"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func TestListUserPackagesOnlyReturnsCurrentValidPackage(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	account, err := client.User.Create().SetEmail("package-list@example.com").SetPasswordHash("hash").Save(ctx)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	plan, err := client.BalancePackagePlan.Create().
		SetCode("package-list-plan").SetName("套餐").SetPriceCny(29).
		SetWeeklyCreditUsd(76).SetValidityDays(28).SetRefreshCount(4).
		SetRefreshIntervalDays(7).SetForSale(true).Save(ctx)
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	now := time.Now().UTC()
	_, err = client.UserBalancePackage.Create().
		SetUserID(account.ID).SetPlanID(plan.ID).SetPaymentOrderID(1).
		SetWeeklyCreditUsd(76).SetCreditedCount(2).SetRefreshCount(4).
		SetRefreshIntervalDays(7).SetStartsAt(now.Add(-7 * 24 * time.Hour)).
		SetExpiresAt(now.Add(21 * 24 * time.Hour)).SetStatus(balancePackageStatusActive).Save(ctx)
	if err != nil {
		t.Fatalf("create active package: %v", err)
	}
	_, err = client.UserBalancePackage.Create().
		SetUserID(account.ID).SetPlanID(plan.ID).SetPaymentOrderID(2).
		SetWeeklyCreditUsd(76).SetCreditedCount(4).SetRefreshCount(4).
		SetRefreshIntervalDays(7).SetStartsAt(now.Add(-30 * 24 * time.Hour)).
		SetExpiresAt(now.Add(30 * 24 * time.Hour)).SetStatus("refunded").Save(ctx)
	if err != nil {
		t.Fatalf("create refunded package: %v", err)
	}
	_, err = client.UserBalancePackage.Create().
		SetUserID(account.ID).SetPlanID(plan.ID).SetPaymentOrderID(3).
		SetWeeklyCreditUsd(76).SetCreditedCount(4).SetRefreshCount(4).
		SetRefreshIntervalDays(7).SetStartsAt(now.Add(-40 * 24 * time.Hour)).
		SetExpiresAt(now.Add(-12 * time.Hour)).SetStatus(balancePackageStatusExpired).Save(ctx)
	if err != nil {
		t.Fatalf("create expired package: %v", err)
	}

	packages, err := NewBalancePackageService(client).ListUserPackages(ctx, account.ID)
	if err != nil {
		t.Fatalf("list packages: %v", err)
	}
	if len(packages) != 1 || packages[0].Status != balancePackageStatusActive || packages[0].RefreshCount != 4 {
		t.Fatalf("unexpected visible packages: %#v", packages)
	}
}

func TestValidateUserPurchaseRejectsDifferentPlan(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	account, err := client.User.Create().SetEmail("package-purchase@example.com").SetPasswordHash("hash").Save(ctx)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	planA := createLifecyclePlan(t, client, "package-plan-a", 29)
	planB := createLifecyclePlan(t, client, "package-plan-b", 39)
	now := time.Now().UTC()
	if _, err := client.UserBalancePackage.Create().SetUserID(account.ID).SetPlanID(planA.ID).SetPaymentOrderID(11).
		SetWeeklyCreditUsd(planA.WeeklyCreditUsd).SetCreditedCount(1).SetRefreshCount(4).SetRefreshIntervalDays(7).
		SetStartsAt(now).SetExpiresAt(now.Add(28 * 24 * time.Hour)).SetStatus(balancePackageStatusActive).Save(ctx); err != nil {
		t.Fatalf("create current package: %v", err)
	}

	service := NewBalancePackageService(client)
	if err := service.ValidateUserPurchase(ctx, account.ID, planA.ID); err != nil {
		t.Fatalf("same plan should be allowed: %v", err)
	}
	if err := service.ValidateUserPurchase(ctx, account.ID, planB.ID); infraerrors.Reason(err) != "BALANCE_PACKAGE_ACTIVE" {
		t.Fatalf("different plan error reason = %q, want BALANCE_PACKAGE_ACTIVE", infraerrors.Reason(err))
	}
}

func TestCreditInitialBalanceRenewsSamePlanWithoutChangingRefreshProgress(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	account, err := client.User.Create().SetEmail("package-renew@example.com").SetPasswordHash("hash").Save(ctx)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	plan := createLifecyclePlan(t, client, "package-renew-plan", 29)
	now := time.Now().UTC()
	current, err := client.UserBalancePackage.Create().SetUserID(account.ID).SetPlanID(plan.ID).SetPaymentOrderID(21).
		SetWeeklyCreditUsd(plan.WeeklyCreditUsd).SetCreditedCount(2).SetRefreshCount(4).SetRefreshIntervalDays(7).
		SetStartsAt(now.Add(-7 * 24 * time.Hour)).SetNextCreditAt(now.Add(24 * time.Hour)).
		SetExpiresAt(now.Add(21 * 24 * time.Hour)).SetStatus(balancePackageStatusActive).Save(ctx)
	if err != nil {
		t.Fatalf("create current package: %v", err)
	}
	planID, weeklyCredit, refreshCount, refreshInterval, validity := plan.ID, plan.WeeklyCreditUsd, plan.RefreshCount, plan.RefreshIntervalDays, plan.ValidityDays
	order := &dbent.PaymentOrder{
		ID:                                22,
		UserID:                            account.ID,
		PaymentType:                       string(payment.TypeAlipay),
		BalancePackagePlanID:              &planID,
		BalancePackageWeeklyCreditUsd:     &weeklyCredit,
		BalancePackageRefreshCount:        &refreshCount,
		BalancePackageRefreshIntervalDays: &refreshInterval,
		BalancePackageValidityDays:        &validity,
	}

	if err := NewBalancePackageService(client).CreditInitialBalance(ctx, order); err != nil {
		t.Fatalf("renew package: %v", err)
	}
	updated, err := client.UserBalancePackage.Get(ctx, current.ID)
	if err != nil {
		t.Fatalf("get renewed package: %v", err)
	}
	if updated.PaymentOrderID != order.ID || updated.CreditedCount != current.CreditedCount || updated.RefreshCount != current.RefreshCount || !updated.ExpiresAt.Equal(current.ExpiresAt.Add(28*24*time.Hour)) {
		t.Fatalf("unexpected renewed package: %#v", updated)
	}
	count, err := client.UserBalancePackage.Query().Where().Count(ctx)
	if err != nil {
		t.Fatalf("count packages: %v", err)
	}
	if count != 1 {
		t.Fatalf("renewal created %d package rows, want 1", count)
	}
}

func TestCreditInitialBalanceRepaysDebtBeforePackageRemaining(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	account, err := client.User.Create().SetEmail("package-initial-debt@example.com").SetPasswordHash("hash").SetBalance(-40).Save(ctx)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	plan := createLifecyclePlan(t, client, "package-initial-debt-plan", 100)
	planID, weeklyCredit, refreshCount := plan.ID, plan.WeeklyCreditUsd, plan.RefreshCount
	refreshInterval, validity := plan.RefreshIntervalDays, plan.ValidityDays
	order := &dbent.PaymentOrder{
		ID:                                23,
		UserID:                            account.ID,
		PaymentType:                       string(payment.TypeAlipay),
		BalancePackagePlanID:              &planID,
		BalancePackageWeeklyCreditUsd:     &weeklyCredit,
		BalancePackageRefreshCount:        &refreshCount,
		BalancePackageRefreshIntervalDays: &refreshInterval,
		BalancePackageValidityDays:        &validity,
	}

	if err := NewBalancePackageService(client).CreditInitialBalance(ctx, order); err != nil {
		t.Fatalf("credit initial package: %v", err)
	}
	updatedUser, err := client.User.Get(ctx, account.ID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if updatedUser.Balance != 60 {
		t.Fatalf("balance = %f, want 60", updatedUser.Balance)
	}
	updatedPackage, err := client.UserBalancePackage.Query().Where(userbalancepackage.UserIDEQ(account.ID)).Only(ctx)
	if err != nil {
		t.Fatalf("get package: %v", err)
	}
	if updatedPackage.RemainingUsd != 60 {
		t.Fatalf("remaining = %f, want 60", updatedPackage.RemainingUsd)
	}
}

func TestCreditInitialBalancePausesFutureCreditsWhenFirstWeekCannotClearDebt(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	account, err := client.User.Create().SetEmail("package-initial-debt-paused@example.com").SetPasswordHash("hash").SetBalance(-150).Save(ctx)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	plan := createLifecyclePlan(t, client, "package-initial-debt-paused-plan", 100)
	planID, weeklyCredit, refreshCount := plan.ID, plan.WeeklyCreditUsd, plan.RefreshCount
	refreshInterval, validity := plan.RefreshIntervalDays, plan.ValidityDays
	order := &dbent.PaymentOrder{
		ID:                                230,
		UserID:                            account.ID,
		PaymentType:                       string(payment.TypeAlipay),
		BalancePackagePlanID:              &planID,
		BalancePackageWeeklyCreditUsd:     &weeklyCredit,
		BalancePackageRefreshCount:        &refreshCount,
		BalancePackageRefreshIntervalDays: &refreshInterval,
		BalancePackageValidityDays:        &validity,
	}

	if err := NewBalancePackageService(client).CreditInitialBalance(ctx, order); err != nil {
		t.Fatalf("credit initial package: %v", err)
	}
	updatedUser, _ := client.User.Get(ctx, account.ID)
	updatedPackage, err := client.UserBalancePackage.Query().Where(userbalancepackage.UserIDEQ(account.ID)).Only(ctx)
	if err != nil {
		t.Fatalf("get package: %v", err)
	}
	if updatedUser.Balance != -50 || updatedPackage.RemainingUsd != 0 || updatedPackage.Status != balancePackageStatusDebtPaused || updatedPackage.NextCreditAt == nil {
		t.Fatalf("unexpected debt-paused package: user=%#v package=%#v", updatedUser, updatedPackage)
	}
}

func TestCreditInitialBalanceKeepsDebtPausedWhenSingleWeekCannotClearDebt(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	account, err := client.User.Create().SetEmail("package-single-week-debt-paused@example.com").SetPasswordHash("hash").SetBalance(-150).Save(ctx)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	plan, err := client.BalancePackagePlan.Create().SetCode("package-single-week-debt-plan").SetName("单周欠费套餐").SetPriceCny(29).
		SetWeeklyCreditUsd(100).SetValidityDays(7).SetRefreshCount(1).SetRefreshIntervalDays(7).SetForSale(true).Save(ctx)
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	planID, weeklyCredit, refreshCount := plan.ID, plan.WeeklyCreditUsd, plan.RefreshCount
	refreshInterval, validity := plan.RefreshIntervalDays, plan.ValidityDays
	order := &dbent.PaymentOrder{
		ID:                                231,
		UserID:                            account.ID,
		PaymentType:                       string(payment.TypeAlipay),
		BalancePackagePlanID:              &planID,
		BalancePackageWeeklyCreditUsd:     &weeklyCredit,
		BalancePackageRefreshCount:        &refreshCount,
		BalancePackageRefreshIntervalDays: &refreshInterval,
		BalancePackageValidityDays:        &validity,
	}

	if err := NewBalancePackageService(client).CreditInitialBalance(ctx, order); err != nil {
		t.Fatalf("credit initial package: %v", err)
	}
	updated, err := client.UserBalancePackage.Query().Where(userbalancepackage.UserIDEQ(account.ID)).Only(ctx)
	if err != nil {
		t.Fatalf("get package: %v", err)
	}
	if updated.Status != balancePackageStatusDebtPaused || updated.RemainingUsd != 0 || updated.NextCreditAt != nil {
		t.Fatalf("unexpected single-week debt-paused package: %#v", updated)
	}
}

func TestCreditDueBalancesReplacesPreviousWeeklyCredit(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	account, err := client.User.Create().SetEmail("package-refresh@example.com").SetPasswordHash("hash").SetBalance(150).Save(ctx)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	plan := createLifecyclePlan(t, client, "package-refresh-plan", 100)
	now := time.Now().UTC().Truncate(time.Second)
	nextCreditAt := now.Add(-time.Minute)
	pkg, err := client.UserBalancePackage.Create().
		SetUserID(account.ID).SetPlanID(plan.ID).SetPaymentOrderID(31).
		SetWeeklyCreditUsd(100).SetRemainingUsd(60).SetCreditedCount(1).SetRefreshCount(4).
		SetRefreshIntervalDays(7).SetStartsAt(now.Add(-7 * 24 * time.Hour)).SetNextCreditAt(nextCreditAt).
		SetExpiresAt(now.Add(21 * 24 * time.Hour)).SetStatus(balancePackageStatusActive).Save(ctx)
	if err != nil {
		t.Fatalf("create package: %v", err)
	}

	credited, err := NewBalancePackageService(client).CreditDueBalances(ctx, now)
	if err != nil {
		t.Fatalf("refresh package: %v", err)
	}
	if credited != 1 {
		t.Fatalf("credited = %d, want 1", credited)
	}
	updatedPackage, err := client.UserBalancePackage.Get(ctx, pkg.ID)
	if err != nil {
		t.Fatalf("get package: %v", err)
	}
	if updatedPackage.RemainingUsd != 100 || updatedPackage.CreditedCount != 2 {
		t.Fatalf("unexpected refreshed package: %#v", updatedPackage)
	}
	updatedUser, err := client.User.Get(ctx, account.ID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if updatedUser.Balance != 190 {
		t.Fatalf("balance = %f, want 190", updatedUser.Balance)
	}
}

func TestCreditDueBalancesPausesDebtBeforeDueWeeklyCredit(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	account, err := client.User.Create().SetEmail("package-debt-refresh@example.com").SetPasswordHash("hash").SetBalance(-40).Save(ctx)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	plan := createLifecyclePlan(t, client, "package-debt-refresh-plan", 100)
	now := time.Now().UTC().Truncate(time.Second)
	pkg, err := client.UserBalancePackage.Create().
		SetUserID(account.ID).SetPlanID(plan.ID).SetPaymentOrderID(32).
		SetWeeklyCreditUsd(100).SetRemainingUsd(0).SetCreditedCount(1).SetRefreshCount(4).
		SetRefreshIntervalDays(7).SetStartsAt(now.Add(-7 * 24 * time.Hour)).SetNextCreditAt(now.Add(-time.Minute)).
		SetExpiresAt(now.Add(21 * 24 * time.Hour)).SetStatus(balancePackageStatusActive).Save(ctx)
	if err != nil {
		t.Fatalf("create package: %v", err)
	}

	credited, err := NewBalancePackageService(client).CreditDueBalances(ctx, now)
	if err != nil {
		t.Fatalf("refresh package: %v", err)
	}
	if credited != 0 {
		t.Fatalf("credited = %d, want 0", credited)
	}
	updatedPackage, err := client.UserBalancePackage.Get(ctx, pkg.ID)
	if err != nil {
		t.Fatalf("get package: %v", err)
	}
	if updatedPackage.Status != balancePackageStatusDebtPaused || updatedPackage.RemainingUsd != 0 || updatedPackage.CreditedCount != 1 {
		t.Fatalf("unexpected debt-paused package: %#v", updatedPackage)
	}
	updatedUser, err := client.User.Get(ctx, account.ID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if updatedUser.Balance != -40 {
		t.Fatalf("balance = %f, want -40", updatedUser.Balance)
	}
}

func TestCreditDueBalancesPausesActiveDebtWithoutConsumingAnotherWeek(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	account, err := client.User.Create().SetEmail("package-runtime-debt-paused@example.com").SetPasswordHash("hash").SetBalance(-40).Save(ctx)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	plan := createLifecyclePlan(t, client, "package-runtime-debt-paused-plan", 100)
	now := time.Now().UTC().Truncate(time.Second)
	pkg, err := client.UserBalancePackage.Create().
		SetUserID(account.ID).SetPlanID(plan.ID).SetPaymentOrderID(320).
		SetWeeklyCreditUsd(100).SetRemainingUsd(0).SetCreditedCount(1).SetRefreshCount(4).
		SetRefreshIntervalDays(7).SetStartsAt(now.Add(-7 * 24 * time.Hour)).SetNextCreditAt(now.Add(3 * 24 * time.Hour)).
		SetExpiresAt(now.Add(21 * 24 * time.Hour)).SetStatus(balancePackageStatusActive).Save(ctx)
	if err != nil {
		t.Fatalf("create package: %v", err)
	}

	credited, err := NewBalancePackageService(client).CreditDueBalances(ctx, now)
	if err != nil {
		t.Fatalf("scan package: %v", err)
	}
	updatedPackage, _ := client.UserBalancePackage.Get(ctx, pkg.ID)
	updatedUser, _ := client.User.Get(ctx, account.ID)
	if credited != 0 || updatedPackage.Status != balancePackageStatusDebtPaused || updatedPackage.CreditedCount != 1 || updatedUser.Balance != -40 {
		t.Fatalf("unexpected runtime pause result: credited=%d user=%#v package=%#v", credited, updatedUser, updatedPackage)
	}
}

func TestDebtPausedPackageRemainsVisibleAndBlocksPurchase(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	account, _ := client.User.Create().SetEmail("package-debt-paused-visible@example.com").SetPasswordHash("hash").SetBalance(-1).Save(ctx)
	plan := createLifecyclePlan(t, client, "package-debt-paused-visible-plan", 100)
	now := time.Now().UTC()
	_, err := client.UserBalancePackage.Create().
		SetUserID(account.ID).SetPlanID(plan.ID).SetPaymentOrderID(321).
		SetWeeklyCreditUsd(100).SetRemainingUsd(0).SetCreditedCount(1).SetRefreshCount(4).
		SetRefreshIntervalDays(7).SetStartsAt(now).SetNextCreditAt(now.Add(7 * 24 * time.Hour)).
		SetExpiresAt(now.Add(28 * 24 * time.Hour)).SetStatus(balancePackageStatusDebtPaused).Save(ctx)
	if err != nil {
		t.Fatalf("create package: %v", err)
	}

	svc := NewBalancePackageService(client)
	packages, err := svc.ListUserPackages(ctx, account.ID)
	if err != nil || len(packages) != 1 || packages[0].Status != balancePackageStatusDebtPaused {
		t.Fatalf("unexpected visible packages: %#v err=%v", packages, err)
	}
	if err := svc.ValidateUserPurchase(ctx, account.ID, plan.ID); infraerrors.Reason(err) != "BALANCE_PACKAGE_DEBT_PAUSED" {
		t.Fatalf("purchase error reason = %q, want BALANCE_PACKAGE_DEBT_PAUSED", infraerrors.Reason(err))
	}
}

func TestResumeDebtPausedPackageRequiresNonNegativeBalance(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	account, _ := client.User.Create().SetEmail("package-resume-debt@example.com").SetPasswordHash("hash").SetBalance(-1).Save(ctx)
	plan := createLifecyclePlan(t, client, "package-resume-debt-plan", 100)
	now := time.Now().UTC().Truncate(time.Second)
	pkg, _ := client.UserBalancePackage.Create().
		SetUserID(account.ID).SetPlanID(plan.ID).SetPaymentOrderID(322).
		SetWeeklyCreditUsd(100).SetRemainingUsd(0).SetCreditedCount(1).SetRefreshCount(4).
		SetRefreshIntervalDays(7).SetStartsAt(now).SetNextCreditAt(now.Add(7 * 24 * time.Hour)).
		SetExpiresAt(now.Add(28 * 24 * time.Hour)).SetStatus(balancePackageStatusDebtPaused).Save(ctx)

	svc := NewBalancePackageService(client)
	if err := svc.ResumeDebtPausedPackage(ctx, pkg.ID, 99, now); infraerrors.Reason(err) != "BALANCE_DEBT_OUTSTANDING" {
		t.Fatalf("resume debt error reason = %q", infraerrors.Reason(err))
	}
	if _, err := client.User.UpdateOneID(account.ID).SetBalance(0).Save(ctx); err != nil {
		t.Fatalf("clear debt: %v", err)
	}
	if err := svc.ResumeDebtPausedPackage(ctx, pkg.ID, 99, now); err != nil {
		t.Fatalf("resume package: %v", err)
	}
	updated, _ := client.UserBalancePackage.Get(ctx, pkg.ID)
	if updated.Status != balancePackageStatusActive || updated.NextCreditAt == nil || !updated.NextCreditAt.Equal(now) {
		t.Fatalf("unexpected resumed package: %#v", updated)
	}
}

func TestCancelPackageByOrderStopsFutureCreditsWithoutRefundOrBalanceAdjustment(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	account, err := client.User.Create().SetEmail("package-cancel@example.com").SetPasswordHash("hash").SetBalance(155).Save(ctx)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	plan := createLifecyclePlan(t, client, "package-cancel-plan", 100)
	now := time.Now().UTC().Truncate(time.Second)
	order, err := client.PaymentOrder.Create().
		SetUserID(account.ID).
		SetUserEmail(account.Email).
		SetUserName("cancel target").
		SetAmount(29).
		SetPayAmount(29).
		SetRechargeCode("cancel-package-order").
		SetPaymentType(string(payment.TypeAlipay)).
		SetPaymentTradeNo("trade-cancel").
		SetOrderType(payment.OrderTypeBalanceSubscription).
		SetStatus(OrderStatusRefundFailed).
		SetClientIP("127.0.0.1").
		SetSrcHost("test.local").
		SetExpiresAt(now.Add(24 * time.Hour)).
		Save(ctx)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	pkg, err := client.UserBalancePackage.Create().
		SetUserID(account.ID).SetPlanID(plan.ID).SetPaymentOrderID(order.ID).
		SetWeeklyCreditUsd(100).SetRemainingUsd(55).SetCreditedCount(2).SetRefreshCount(4).
		SetRefreshIntervalDays(7).SetStartsAt(now.Add(-7 * 24 * time.Hour)).SetNextCreditAt(now.Add(7 * 24 * time.Hour)).
		SetExpiresAt(now.Add(21 * 24 * time.Hour)).SetStatus(balancePackageStatusActive).Save(ctx)
	if err != nil {
		t.Fatalf("create package: %v", err)
	}

	svc := NewBalancePackageService(client)
	if err := svc.CancelPackageByOrder(ctx, order.ID, 99, now); err != nil {
		t.Fatalf("cancel package: %v", err)
	}
	updatedPackage, err := client.UserBalancePackage.Get(ctx, pkg.ID)
	if err != nil {
		t.Fatalf("get cancelled package: %v", err)
	}
	if updatedPackage.Status != balancePackageStatusCancelled || updatedPackage.RemainingUsd != 0 || updatedPackage.NextCreditAt != nil {
		t.Fatalf("unexpected cancelled package: %#v", updatedPackage)
	}
	updatedUser, err := client.User.Get(ctx, account.ID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if updatedUser.Balance != account.Balance {
		t.Fatalf("balance changed from %v to %v", account.Balance, updatedUser.Balance)
	}
	updatedOrder, err := client.PaymentOrder.Get(ctx, order.ID)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if updatedOrder.Status != OrderStatusRefundFailed {
		t.Fatalf("order status = %q, want %q", updatedOrder.Status, OrderStatusRefundFailed)
	}
	auditCount, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(fmt.Sprintf("%d", order.ID)), paymentauditlog.ActionEQ(balancePackageManualCancelAudit)).
		Count(ctx)
	if err != nil {
		t.Fatalf("count cancellation audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("cancellation audit count = %d, want 1", auditCount)
	}
	if err := svc.CancelPackageByOrder(ctx, order.ID, 99, now); infraerrors.Reason(err) != "BALANCE_PACKAGE_NOT_CANCELLABLE" {
		t.Fatalf("second cancellation error = %q, want BALANCE_PACKAGE_NOT_CANCELLABLE", infraerrors.Reason(err))
	}
}

func TestBalancePackageRemainingAfterDebtSupportsMultipleWeeks(t *testing.T) {
	cases := []struct {
		name         string
		baseBalance  float64
		weeklyCredit float64
		wantRemain   float64
	}{
		{name: "debt exceeds credit", baseBalance: -250, weeklyCredit: 100, wantRemain: 0},
		{name: "credit clears debt", baseBalance: -40, weeklyCredit: 100, wantRemain: 60},
		{name: "ordinary balance keeps full weekly credit", baseBalance: 20, weeklyCredit: 100, wantRemain: 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := balancePackageRemainingAfterDebt(tc.baseBalance, tc.weeklyCredit); got != tc.wantRemain {
				t.Fatalf("remaining = %f, want %f", got, tc.wantRemain)
			}
		})
	}
}

func TestCreditDueBalancesClearsExpiredWeeklyCredit(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	account, err := client.User.Create().SetEmail("package-expiry@example.com").SetPasswordHash("hash").SetBalance(150).Save(ctx)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	plan := createLifecyclePlan(t, client, "package-expiry-plan", 100)
	now := time.Now().UTC().Truncate(time.Second)
	pkg, err := client.UserBalancePackage.Create().
		SetUserID(account.ID).SetPlanID(plan.ID).SetPaymentOrderID(41).
		SetWeeklyCreditUsd(100).SetRemainingUsd(60).SetCreditedCount(4).SetRefreshCount(4).
		SetRefreshIntervalDays(7).SetStartsAt(now.Add(-28 * 24 * time.Hour)).
		SetExpiresAt(now.Add(-time.Minute)).SetStatus(balancePackageStatusCompleted).Save(ctx)
	if err != nil {
		t.Fatalf("create package: %v", err)
	}

	credited, err := NewBalancePackageService(client).CreditDueBalances(ctx, now)
	if err != nil {
		t.Fatalf("expire package: %v", err)
	}
	if credited != 0 {
		t.Fatalf("credited = %d, want 0", credited)
	}
	updatedPackage, err := client.UserBalancePackage.Get(ctx, pkg.ID)
	if err != nil {
		t.Fatalf("get package: %v", err)
	}
	if updatedPackage.Status != balancePackageStatusExpired || updatedPackage.RemainingUsd != 0 {
		t.Fatalf("unexpected expired package: %#v", updatedPackage)
	}
	updatedUser, err := client.User.Get(ctx, account.ID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if updatedUser.Balance != 90 {
		t.Fatalf("balance = %f, want 90", updatedUser.Balance)
	}
}

func createLifecyclePlan(t *testing.T, client *dbent.Client, code string, price float64) *dbent.BalancePackagePlan {
	t.Helper()
	plan, err := client.BalancePackagePlan.Create().SetCode(code).SetName(code).SetPriceCny(price).
		SetWeeklyCreditUsd(price).SetValidityDays(28).SetRefreshCount(4).SetRefreshIntervalDays(7).
		SetForSale(true).Save(context.Background())
	if err != nil {
		t.Fatalf("create plan %s: %v", code, err)
	}
	return plan
}
