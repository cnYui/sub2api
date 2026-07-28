package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var ErrBillingAuthorizationDebtOutstanding = errors.New("billing authorization debt is outstanding")
var ErrInvalidBillingAuthorizationTransition = errors.New("invalid billing authorization transition")

type BillingAuthorizationStatus string

const (
	BillingAuthorizationReserved   BillingAuthorizationStatus = "reserved"
	BillingAuthorizationDispatched BillingAuthorizationStatus = "dispatched"
	BillingAuthorizationUnknown    BillingAuthorizationStatus = "unknown"
	BillingAuthorizationSettled    BillingAuthorizationStatus = "settled"
	BillingAuthorizationReleased   BillingAuthorizationStatus = "released"
	BillingAuthorizationDebt       BillingAuthorizationStatus = "debt"
	BillingAuthorizationSuspense   BillingAuthorizationStatus = "suspense"
)

type BillingAuthorization struct {
	ID                  int64
	RequestID           string
	APIKeyID            int64
	UserID              int64
	Source              BillingSource
	SubscriptionID      *int64
	EntitlementPeriodID *int64
	RequestFingerprint  string
	ReservedUSD         float64
	SettledUSD          float64
	DebtUSD             float64
	SuspenseUSD         float64
	PricingSnapshot     json.RawMessage
	EstimateBreakdown   json.RawMessage
	EstimatorVersion    string
	Status              BillingAuthorizationStatus
	EffectiveBody       []byte
	EffectiveImageCount int
	ExpiresAt           time.Time
	DispatchedAt        *time.Time
	SettledAt           *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type BillingAuthorizationReserveMode string

const (
	BillingAuthorizationReserveFull BillingAuthorizationReserveMode = "full"
	BillingAuthorizationReserveFit  BillingAuthorizationReserveMode = "fit"
)

type BillingAuthorizationReserveInput struct {
	RequestID           string
	APIKeyID            int64
	UserID              int64
	Platform            string
	Model               string
	RequestFingerprint  string
	Source              BillingSource
	SubscriptionID      *int64
	EntitlementPeriodID *int64
	BudgetPlan          OpenAIBillingBudgetPlan
	Mode                BillingAuthorizationReserveMode
	ExpiresAt           time.Time
}

type BillingAuthorizationRepository interface {
	HasOutstandingDebt(context.Context, int64, string) (bool, error)
	TryReserve(context.Context, BillingAuthorizationReserveInput) (*BillingAuthorization, bool, error)
	MarkDispatched(context.Context, int64, time.Time) error
	MarkUnknown(context.Context, int64, string, time.Time) error
	Release(context.Context, int64, string, time.Time) error
	ClaimUnknown(context.Context, time.Time, int) ([]BillingAuthorization, error)
	MoveUnknownToSuspense(context.Context, int64, string, time.Time) error
}

func ValidateBillingAuthorizationTransition(from, to BillingAuthorizationStatus) error {
	if from == to {
		return nil
	}
	allowed := map[BillingAuthorizationStatus]map[BillingAuthorizationStatus]bool{
		BillingAuthorizationReserved: {
			BillingAuthorizationReleased:   true,
			BillingAuthorizationDispatched: true,
		},
		BillingAuthorizationDispatched: {
			BillingAuthorizationSettled:  true,
			BillingAuthorizationReleased: true,
			BillingAuthorizationUnknown:  true,
			BillingAuthorizationDebt:     true,
		},
		BillingAuthorizationUnknown: {
			BillingAuthorizationSettled:  true,
			BillingAuthorizationReleased: true,
			BillingAuthorizationDebt:     true,
			BillingAuthorizationSuspense: true,
		},
	}
	if allowed[from][to] {
		return nil
	}
	return ErrInvalidBillingAuthorizationTransition
}
