//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestTrafficCreditReservationRepository_ConcurrentReserveCannotOversell(t *testing.T) {
	ctx := context.Background()
	userID, creditID := createTrafficCreditReservationFixture(t, 1)
	repo := NewTrafficCreditReservationRepository(integrationDB)

	inputs := []service.TrafficCreditReservationInput{
		newTrafficCreditReservationInput(userID, "req-"+uuid.NewString(), "fingerprint-a", 0.75),
		newTrafficCreditReservationInput(userID, "req-"+uuid.NewString(), "fingerprint-b", 0.75),
	}
	type result struct {
		reservation *service.TrafficCreditReservation
		err         error
	}
	results := make(chan result, len(inputs))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, input := range inputs {
		input := input
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			reservation, _, err := repo.Reserve(ctx, input)
			results <- result{reservation: reservation, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	successes := 0
	insufficient := 0
	for got := range results {
		switch {
		case got.err == nil:
			successes++
			require.NotNil(t, got.reservation)
		case got.err == service.ErrInsufficientBalance:
			insufficient++
		default:
			require.NoError(t, got.err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, insufficient)

	var reservedUSD float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT reserved_usd FROM user_traffic_credits WHERE id = $1", creditID).Scan(&reservedUSD))
	require.InDelta(t, 0.75, reservedUSD, 1e-10)
}

func TestTrafficCreditReservationRepository_IdempotentByRequestAndKey(t *testing.T) {
	ctx := context.Background()
	userID, _ := createTrafficCreditReservationFixture(t, 2)
	repo := NewTrafficCreditReservationRepository(integrationDB)
	input := newTrafficCreditReservationInput(userID, "req-"+uuid.NewString(), "same-fingerprint", 0.75)

	first, created, err := repo.Reserve(ctx, input)
	require.NoError(t, err)
	require.True(t, created)

	again, created, err := repo.Reserve(ctx, input)
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, first.ID, again.ID)

	conflict := input
	conflict.RequestFingerprint = "different-fingerprint"
	_, _, err = repo.Reserve(ctx, conflict)
	require.ErrorIs(t, err, service.ErrUsageBillingRequestConflict)
}

func TestTrafficCreditReservationRepository_ReleaseRestoresAvailableAmount(t *testing.T) {
	ctx := context.Background()
	userID, creditID := createTrafficCreditReservationFixture(t, 1)
	repo := NewTrafficCreditReservationRepository(integrationDB)
	reservation, _, err := repo.Reserve(ctx, newTrafficCreditReservationInput(userID, "req-"+uuid.NewString(), "release", 0.6))
	require.NoError(t, err)

	require.NoError(t, repo.Release(ctx, reservation.ID, time.Now().UTC()))

	var remainingUSD float64
	var reservedUSD float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT remaining_usd, reserved_usd
		FROM user_traffic_credits
		WHERE id = $1
	`, creditID).Scan(&remainingUSD, &reservedUSD))
	require.InDelta(t, 1, remainingUSD, 1e-10)
	require.Zero(t, reservedUSD)

	var status string
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT status FROM traffic_credit_reservations WHERE id = $1", reservation.ID).Scan(&status))
	require.Equal(t, string(service.TrafficCreditReservationReleased), status)
}

func TestTrafficCreditReservationRepository_ThresholdResidualAvailability(t *testing.T) {
	ctx := context.Background()
	policy := service.TrafficCreditPolicy{MinimumReserveUSD: 0.01}
	repo := NewTrafficCreditReservationRepository(integrationDB, policy)

	depletedUserID, _ := createTrafficCreditReservationFixture(t, 0.01)
	availableUSD, err := repo.GetAvailableUSD(ctx, depletedUserID, service.TrafficPackPlatformOpenAI, time.Now().UTC())
	require.NoError(t, err)
	require.Zero(t, availableUSD)
	_, _, err = repo.Reserve(ctx, newTrafficCreditReservationInput(depletedUserID, "req-"+uuid.NewString(), "threshold-depleted", 0.001))
	require.ErrorIs(t, err, service.ErrInsufficientBalance)

	availableUserID, availableCreditID := createTrafficCreditReservationFixture(t, 0.0100000001)
	availableUSD, err = repo.GetAvailableUSD(ctx, availableUserID, service.TrafficPackPlatformOpenAI, time.Now().UTC())
	require.NoError(t, err)
	require.InDelta(t, 0.0100000001, availableUSD, 1e-10)
	reservation, created, err := repo.Reserve(ctx, newTrafficCreditReservationInput(availableUserID, "req-"+uuid.NewString(), "threshold-available", 0.0100000001))
	require.NoError(t, err)
	require.True(t, created)
	require.NotNil(t, reservation)

	var reservedUSD float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT reserved_usd FROM user_traffic_credits WHERE id = $1", availableCreditID).Scan(&reservedUSD))
	require.InDelta(t, 0.0100000001, reservedUSD, 1e-10)
}

func TestTrafficCreditReservationRepository_ReleasesUndispatchedExpiredReservation(t *testing.T) {
	ctx := context.Background()
	userID, creditID := createTrafficCreditReservationFixture(t, 1)
	repo := NewTrafficCreditReservationRepository(integrationDB)
	reservation, _, err := repo.Reserve(ctx, newTrafficCreditReservationInput(userID, "req-"+uuid.NewString(), "expired-reserved", 0.6))
	require.NoError(t, err)
	now := time.Now().UTC()
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		UPDATE traffic_credit_reservations
		SET expires_at = $2
		WHERE id = $1
		RETURNING id
	`, reservation.ID, now.Add(-time.Minute)).Scan(&reservation.ID))

	released, err := repo.ReleaseExpiredReserved(ctx, now, 100)

	require.NoError(t, err)
	require.Equal(t, 1, released)

	var remainingUSD float64
	var reservedUSD float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT remaining_usd, reserved_usd
		FROM user_traffic_credits
		WHERE id = $1
	`, creditID).Scan(&remainingUSD, &reservedUSD))
	require.InDelta(t, 1, remainingUSD, 1e-10)
	require.Zero(t, reservedUSD)

	var status string
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT status FROM traffic_credit_reservations WHERE id = $1", reservation.ID).Scan(&status))
	require.Equal(t, string(service.TrafficCreditReservationReleased), status)
}

func TestTrafficCreditReservationRepository_DoesNotReleaseDispatchedExpiredReservation(t *testing.T) {
	ctx := context.Background()
	userID, creditID := createTrafficCreditReservationFixture(t, 1)
	repo := NewTrafficCreditReservationRepository(integrationDB)
	reservation, _, err := repo.Reserve(ctx, newTrafficCreditReservationInput(userID, "req-"+uuid.NewString(), "expired-dispatched", 0.6))
	require.NoError(t, err)
	require.NoError(t, repo.MarkDispatched(ctx, reservation.ID))
	now := time.Now().UTC()
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		UPDATE traffic_credit_reservations
		SET expires_at = $2
		WHERE id = $1
		RETURNING id
	`, reservation.ID, now.Add(-time.Minute)).Scan(&reservation.ID))

	released, err := repo.ReleaseExpiredReserved(ctx, now, 100)

	require.NoError(t, err)
	require.Zero(t, released)

	var reservedUSD float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT reserved_usd
		FROM user_traffic_credits
		WHERE id = $1
	`, creditID).Scan(&reservedUSD))
	require.InDelta(t, 0.6, reservedUSD, 1e-10)

	var status string
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT status FROM traffic_credit_reservations WHERE id = $1", reservation.ID).Scan(&status))
	require.Equal(t, string(service.TrafficCreditReservationDispatched), status)
}

func createTrafficCreditReservationFixture(t *testing.T, remainingUSD float64) (int64, int64) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	user := mustCreateUser(t, testEntClient(t), &service.User{
		Email:        fmt.Sprintf("traffic-reservation-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
	})
	order, err := integrationEntClient.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName("traffic-reservation-test").
		SetAmount(1).
		SetPayAmount(1).
		SetFeeRate(0).
		SetRechargeCode("TRAFFIC-RESERVATION-" + uuid.NewString()).
		SetOutTradeNo("traffic_reservation_" + uuid.NewString()).
		SetPaymentType(payment.TypeBalance).
		SetPaymentTradeNo("balance").
		SetOrderType(payment.OrderTypeTrafficPack).
		SetStatus(payment.OrderStatusCompleted).
		SetExpiresAt(now.Add(time.Hour)).
		SetPaidAt(now).
		SetClientIP("127.0.0.1").
		SetSrcHost("integration.test").
		Save(ctx)
	require.NoError(t, err)
	var creditID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO user_traffic_credits (
			user_id, order_id, pack_id, platform, initial_usd, remaining_usd, reserved_usd,
			credited_at, expires_at, created_at, updated_at
		)
		VALUES ($1, $2, 2, $3, $4, $4, 0, $5, $6, $5, $5)
		RETURNING id
	`, user.ID, order.ID, service.TrafficPackPlatformOpenAI, remainingUSD, now, now.Add(24*time.Hour)).Scan(&creditID))
	return user.ID, creditID
}

func newTrafficCreditReservationInput(userID int64, requestID, fingerprint string, reserveUSD float64) service.TrafficCreditReservationInput {
	return service.TrafficCreditReservationInput{
		RequestID:          requestID,
		APIKeyID:           9,
		UserID:             userID,
		Platform:           service.TrafficPackPlatformOpenAI,
		Model:              "gpt-5.1",
		RequestFingerprint: fingerprint,
		PricingSnapshot:    json.RawMessage(`{"model":"gpt-5.1"}`),
		ReserveUSD:         reserveUSD,
		ExpiresAt:          time.Now().UTC().Add(5 * time.Minute),
	}
}
