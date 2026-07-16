package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"
)

var ErrTrafficCreditDebtOutstanding = errors.New("traffic credit debt is outstanding")

type BillingSource string

const (
	BillingSourceSubscription  BillingSource = "subscription"
	BillingSourceBalance       BillingSource = "balance"
	BillingSourceTrafficCredit BillingSource = "traffic_credit"
)

type OpenAIBillingAuthorizationInput struct {
	RequestID          string
	RequestFingerprint string
	APIKeyID           int64
	UserID             int64
	Platform           string
	Model              string
	Group              *Group
	Subscription       *UserSubscription
	BalanceEligible    bool
	ServiceTier        string
	RateMultiplier     float64
	Body               []byte
	OutputLimitField   string
}

type OpenAIBillingAuthorization struct {
	Source             BillingSource
	ReservationID      *int64
	RequestFingerprint string
	ReserveUSD         float64
	PricingSnapshot    json.RawMessage
	EffectiveBody      []byte
	Enforced           bool
}

type OpenAIBillingAuthorizer interface {
	Authorize(ctx context.Context, input OpenAIBillingAuthorizationInput) (*OpenAIBillingAuthorization, error)
	MarkDispatched(ctx context.Context, reservationID int64) error
	MarkUnknown(ctx context.Context, reservationID int64, reason string) error
	Release(ctx context.Context, reservationID int64) error
}

type OpenAITrafficBudgetEstimator interface {
	Estimate(ctx context.Context, input OpenAITrafficBudgetInput) (*OpenAITrafficCreditBudget, error)
}

type OpenAIBillingAuthorizationService struct {
	reservationRepo    TrafficCreditReservationRepository
	estimator          OpenAITrafficBudgetEstimator
	reservationTimeout time.Duration
	enabled            bool
	shadow             bool
}

func NewOpenAIBillingAuthorizationService(
	reservationRepo TrafficCreditReservationRepository,
	estimator OpenAITrafficBudgetEstimator,
	reservationTimeout time.Duration,
	enabled bool,
	shadow bool,
) *OpenAIBillingAuthorizationService {
	return &OpenAIBillingAuthorizationService{
		reservationRepo:    reservationRepo,
		estimator:          estimator,
		reservationTimeout: reservationTimeout,
		enabled:            enabled,
		shadow:             shadow,
	}
}

func (s *OpenAIBillingAuthorizationService) Authorize(
	ctx context.Context,
	input OpenAIBillingAuthorizationInput,
) (*OpenAIBillingAuthorization, error) {
	if s == nil || s.estimator == nil {
		return nil, ErrBillingPreauthUnavailable
	}

	if input.Subscription != nil && input.Group != nil {
		budget, err := s.estimate(ctx, input, math.MaxFloat64)
		if err != nil {
			return nil, err
		}
		daily, weekly, monthly := input.Subscription.CheckAllLimits(input.Group, budget.ReserveUSD)
		if daily && weekly && monthly {
			return &OpenAIBillingAuthorization{
				Source:             BillingSourceSubscription,
				RequestFingerprint: input.RequestFingerprint,
				ReserveUSD:         budget.ReserveUSD,
				PricingSnapshot:    budget.PricingSnapshot,
				EffectiveBody:      budget.Body,
				Enforced:           s.enabled,
			}, nil
		}
	} else if input.BalanceEligible {
		return &OpenAIBillingAuthorization{
			Source:             BillingSourceBalance,
			RequestFingerprint: input.RequestFingerprint,
			EffectiveBody:      append([]byte(nil), input.Body...),
			Enforced:           s.enabled,
		}, nil
	}

	if s.reservationRepo == nil || !IsTrafficPackPlatform(input.Platform) {
		recordTrafficCreditPreauthorizationRejected(ErrTrafficCreditInsufficient)
		return nil, ErrTrafficCreditInsufficient
	}
	hasDebt, err := s.reservationRepo.HasOutstandingDebt(ctx, input.UserID, input.Platform)
	if err != nil {
		recordTrafficCreditPreauthorizationRejected(err)
		return nil, err
	}
	if hasDebt {
		recordTrafficCreditPreauthorizationRejected(ErrTrafficCreditDebtOutstanding)
		return nil, ErrTrafficCreditDebtOutstanding
	}
	availableUSD, err := s.reservationRepo.GetAvailableUSD(ctx, input.UserID, input.Platform, time.Now())
	if err != nil {
		recordTrafficCreditPreauthorizationRejected(err)
		return nil, err
	}
	budget, err := s.estimate(ctx, input, availableUSD)
	if err != nil {
		recordTrafficCreditPreauthorizationRejected(err)
		return nil, err
	}
	authorization := &OpenAIBillingAuthorization{
		Source:             BillingSourceTrafficCredit,
		RequestFingerprint: input.RequestFingerprint,
		ReserveUSD:         budget.ReserveUSD,
		PricingSnapshot:    budget.PricingSnapshot,
		EffectiveBody:      budget.Body,
		Enforced:           s.enabled,
	}
	if !s.enabled {
		if s.shadow {
			recordTrafficCreditPreauthorizationSuccess()
			return authorization, nil
		}
		recordTrafficCreditPreauthorizationRejected(ErrBillingPreauthUnavailable)
		return nil, ErrBillingPreauthUnavailable
	}
	reservation, _, err := s.reservationRepo.Reserve(ctx, TrafficCreditReservationInput{
		RequestID:          input.RequestID,
		APIKeyID:           input.APIKeyID,
		UserID:             input.UserID,
		Platform:           input.Platform,
		Model:              input.Model,
		RequestFingerprint: input.RequestFingerprint,
		PricingSnapshot:    budget.PricingSnapshot,
		ReserveUSD:         budget.ReserveUSD,
		ExpiresAt:          time.Now().Add(s.reservationTimeout),
	})
	if err != nil {
		recordTrafficCreditPreauthorizationRejected(err)
		return nil, err
	}
	authorization.ReservationID = &reservation.ID
	recordTrafficCreditPreauthorizationSuccess()
	return authorization, nil
}

func (s *OpenAIBillingAuthorizationService) MarkDispatched(ctx context.Context, reservationID int64) error {
	if s == nil || s.reservationRepo == nil || reservationID <= 0 {
		return ErrBillingPreauthUnavailable
	}
	return s.reservationRepo.MarkDispatched(ctx, reservationID)
}

func (s *OpenAIBillingAuthorizationService) MarkUnknown(ctx context.Context, reservationID int64, reason string) error {
	if s == nil || s.reservationRepo == nil || reservationID <= 0 {
		return ErrBillingPreauthUnavailable
	}
	err := s.reservationRepo.MarkUnknown(ctx, reservationID, reason)
	if err == nil {
		recordTrafficCreditReservationUnknown()
	}
	return err
}

func (s *OpenAIBillingAuthorizationService) Release(ctx context.Context, reservationID int64) error {
	if s == nil || s.reservationRepo == nil || reservationID <= 0 {
		return ErrBillingPreauthUnavailable
	}
	return s.reservationRepo.Release(ctx, reservationID, time.Now())
}

func (s *OpenAIBillingAuthorizationService) estimate(
	ctx context.Context,
	input OpenAIBillingAuthorizationInput,
	availableUSD float64,
) (*OpenAITrafficCreditBudget, error) {
	var groupID *int64
	if input.Group != nil {
		groupID = &input.Group.ID
	}
	return s.estimator.Estimate(ctx, OpenAITrafficBudgetInput{
		Model:            input.Model,
		GroupID:          groupID,
		ServiceTier:      input.ServiceTier,
		RateMultiplier:   input.RateMultiplier,
		Body:             input.Body,
		AvailableUSD:     availableUSD,
		OutputLimitField: input.OutputLimitField,
	})
}
