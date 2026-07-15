package service

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestCalculateHybridFunding_UsesBalanceBeforeAlipay(t *testing.T) {
	got, err := calculateHybridFunding(
		decimal.RequireFromString("79.00"),
		decimal.RequireFromString("0.79"),
		decimal.RequireFromString("6.32"),
	)

	require.NoError(t, err)
	require.Equal(t, "79.79", got.PayAmount.StringFixed(2))
	require.Equal(t, "6.32", got.BalanceAmount.StringFixed(2))
	require.Equal(t, "73.47", got.GatewayAmount.StringFixed(2))
	require.Equal(t, "6.32", got.BalancePrincipal.StringFixed(2))
	require.Equal(t, "72.68", got.GatewayPrincipal.StringFixed(2))
}

func TestCalculateHybridFunding_RejectsNonCentClientExpectation(t *testing.T) {
	_, err := validateHybridCheckoutExpectation(
		"79.79",
		"6.321",
		decimal.RequireFromString("79.79"),
		decimal.RequireFromString("6.32"),
	)

	require.ErrorIs(t, err, errCheckoutChanged)
}
