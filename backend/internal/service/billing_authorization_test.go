package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateBillingAuthorizationTransition(t *testing.T) {
	require.NoError(t, ValidateBillingAuthorizationTransition(BillingAuthorizationReserved, BillingAuthorizationDispatched))
	require.NoError(t, ValidateBillingAuthorizationTransition(BillingAuthorizationDispatched, BillingAuthorizationUnknown))
	require.NoError(t, ValidateBillingAuthorizationTransition(BillingAuthorizationUnknown, BillingAuthorizationSuspense))
	require.ErrorIs(t, ValidateBillingAuthorizationTransition(BillingAuthorizationSettled, BillingAuthorizationDispatched), ErrInvalidBillingAuthorizationTransition)
}
