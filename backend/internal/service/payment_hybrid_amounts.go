package service

import (
	"errors"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

var (
	errNotHybridFunding = errors.New("not hybrid funding")
	errCheckoutChanged  = infraerrors.BadRequest("CHECKOUT_CHANGED", "checkout amount has changed")
)

type hybridFunding struct {
	PayAmount        decimal.Decimal
	BalanceAmount    decimal.Decimal
	GatewayAmount    decimal.Decimal
	BalancePrincipal decimal.Decimal
	GatewayPrincipal decimal.Decimal
}

func calculateHybridFunding(principal, fee, available decimal.Decimal) (hybridFunding, error) {
	payAmount := principal.Add(fee).Round(2)
	balanceAmount := decimalMin(available.Round(2), payAmount)
	if !balanceAmount.GreaterThan(decimal.Zero) || !balanceAmount.LessThan(payAmount) {
		return hybridFunding{}, errNotHybridFunding
	}
	balancePrincipal := decimalMin(balanceAmount, principal)
	return hybridFunding{
		PayAmount:        payAmount,
		BalanceAmount:    balanceAmount,
		GatewayAmount:    payAmount.Sub(balanceAmount).Round(2),
		BalancePrincipal: balancePrincipal,
		GatewayPrincipal: principal.Sub(balancePrincipal).Round(2),
	}, nil
}

func validateHybridCheckoutExpectation(expectedPayAmount, expectedBalanceAmount string, payAmount, balanceAmount decimal.Decimal) (hybridFunding, error) {
	expectedPay, err := parseCentAmount(expectedPayAmount)
	if err != nil {
		return hybridFunding{}, errCheckoutChanged
	}
	expectedBalance, err := parseCentAmount(expectedBalanceAmount)
	if err != nil {
		return hybridFunding{}, errCheckoutChanged
	}
	payAmount = payAmount.Round(2)
	balanceAmount = balanceAmount.Round(2)
	if !expectedPay.Equal(payAmount) || !expectedBalance.Equal(balanceAmount) {
		return hybridFunding{}, errCheckoutChanged
	}
	return hybridFunding{PayAmount: payAmount, BalanceAmount: balanceAmount, GatewayAmount: payAmount.Sub(balanceAmount).Round(2)}, nil
}

func parseCentAmount(raw string) (decimal.Decimal, error) {
	value, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Zero, err
	}
	if !value.Equal(value.Round(2)) {
		return decimal.Zero, errCheckoutChanged
	}
	return value.Round(2), nil
}

func decimalMin(a, b decimal.Decimal) decimal.Decimal {
	if a.LessThan(b) {
		return a
	}
	return b
}
