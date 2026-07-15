package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/payment/provider"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	paymentFundingModeGateway = "gateway"
	paymentFundingModeBalance = "balance"
	paymentFundingModeMixed   = "mixed"

	providerInitStatusNotStarted = "NOT_STARTED"
	providerInitStatusCreating   = "CREATING"
	providerInitStatusCreated    = "CREATED"
	providerInitStatusUnknown    = "UNKNOWN"

	paymentResolutionStatusPaid    = "PAID"
	paymentResolutionStatusUnpaid  = "UNPAID"
	paymentResolutionStatusUnknown = "UNKNOWN"
)

func (s *PaymentService) claimProviderInitialization(ctx context.Context, orderID int64, now time.Time, lease time.Duration) (bool, error) {
	if lease <= 0 {
		lease = 2 * time.Minute
	}
	updated, err := s.entClient.PaymentOrder.Update().
		Where(
			paymentorder.IDEQ(orderID),
			paymentorder.Or(
				paymentorder.ProviderInitStatusIn("", providerInitStatusNotStarted, providerInitStatusUnknown),
				paymentorder.And(
					paymentorder.ProviderInitStatusEQ(providerInitStatusCreating),
					paymentorder.ProviderInitLeaseUntilLTE(now),
				),
			),
		).
		SetProviderInitStatus(providerInitStatusCreating).
		SetProviderInitAttemptedAt(now).
		SetProviderInitLeaseUntil(now.Add(lease)).
		Save(ctx)
	if err != nil {
		return false, fmt.Errorf("claim provider initialization: %w", err)
	}
	return updated == 1, nil
}

func (s *PaymentService) newPaymentProvider(providerKey, instanceID string, config map[string]string) (payment.Provider, error) {
	if s != nil && s.createProvider != nil {
		return s.createProvider(providerKey, instanceID, config)
	}
	return provider.CreateProvider(providerKey, instanceID, config)
}

func errProviderInitializationInProgress() error {
	return infraerrors.Conflict("PAYMENT_PROVIDER_INITIALIZING", "payment provider initialization is already in progress")
}
