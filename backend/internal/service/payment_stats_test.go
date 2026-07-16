package service

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestBuildMethodDistributionIncludesOfflinePayment(t *testing.T) {
	t.Parallel()

	methods := buildMethodDistribution([]*dbent.PaymentOrder{
		{PaymentType: payment.TypeOffline, PayAmount: 29},
		{PaymentType: payment.TypeOffline, PayAmount: 29},
	})

	require.Equal(t, []PaymentMethodStat{{Type: payment.TypeOffline, Amount: 58, Count: 2}}, methods)
}
