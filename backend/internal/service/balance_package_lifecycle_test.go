package service

import (
	"context"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
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
