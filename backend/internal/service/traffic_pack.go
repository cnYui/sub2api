package service

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	OrderTypeTrafficPack             = "traffic_pack"
	TrafficCreditLedgerTypePurchase  = "purchase"
	TrafficCreditLedgerTypeDeduction = "deduction"
	TrafficCreditLedgerTypeRefund    = "refund"
	TrafficPackPlatformAll           = "all"
	TrafficPackValidityDays          = 28
)

type TrafficPack struct {
	ID           int64   `json:"id"`
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Price        float64 `json:"price"`
	CreditUSD    float64 `json:"credit_usd"`
	ValidityDays int     `json:"validity_days"`
	Platform     string  `json:"platform"`
	ForSale      bool    `json:"for_sale"`
	SortOrder    int     `json:"sort_order"`
}

type TrafficCreditBatch struct {
	ID           int64
	UserID       int64
	OrderID      *int64
	PackID       *int64
	InitialUSD   float64
	RemainingUSD float64
	CreditedAt   time.Time
	ExpiresAt    time.Time
}

type TrafficCredit struct {
	ID           int64     `json:"id"`
	OrderID      *int64    `json:"order_id"`
	PackID       *int64    `json:"pack_id"`
	InitialUSD   float64   `json:"initial_usd"`
	RemainingUSD float64   `json:"remaining_usd"`
	AvailableUSD float64   `json:"available_usd"`
	CreditedAt   time.Time `json:"credited_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type TrafficCreditDeduction struct {
	CreditID  int64
	AmountUSD float64
}

type TrafficCreditSummary struct {
	TotalInitialUSD   float64    `json:"total_initial_usd"`
	TotalRemainingUSD float64    `json:"total_remaining_usd"`
	TrafficDebtUSD    float64    `json:"traffic_debt_usd"`
	NextExpiringUSD   float64    `json:"next_expiring_usd"`
	NextExpiresAt     *time.Time `json:"next_expires_at,omitempty"`
}

type CreditTrafficPackInput struct {
	UserID       int64
	OrderID      int64
	PackID       int64
	CreditUSD    float64
	ValidityDays int
	CreditedAt   time.Time
}

type TrafficPackRepository interface {
	ListForSale(context.Context) ([]TrafficPack, error)
	GetForSaleByID(context.Context, int64) (*TrafficPack, error)
	GetSummary(context.Context, int64, time.Time) (*TrafficCreditSummary, error)
	ListUserCredits(context.Context, int64, time.Time) ([]TrafficCredit, error)
	GetCreditByOrderID(context.Context, int64) (*TrafficCredit, error)
	HasAvailableCredit(context.Context, int64, time.Time) (bool, error)
	CreditPurchase(context.Context, CreditTrafficPackInput) error
	Deduct(context.Context, int64, float64, string, time.Time) (bool, []TrafficCreditDeduction, error)
	RevokePurchase(context.Context, int64, time.Time) error
}

type TrafficPackService struct{ repo TrafficPackRepository }

func NewTrafficPackService(repo TrafficPackRepository) *TrafficPackService {
	return &TrafficPackService{repo: repo}
}

func IsTrafficPackPlatform(platform string) bool {
	return strings.TrimSpace(platform) != ""
}

func (s *TrafficPackService) ListForSale(ctx context.Context) ([]TrafficPack, error) {
	if s == nil || s.repo == nil {
		return []TrafficPack{}, nil
	}
	return s.repo.ListForSale(ctx)
}
func (s *TrafficPackService) GetForSaleByID(ctx context.Context, id int64) (*TrafficPack, error) {
	if s == nil || s.repo == nil {
		return nil, ErrUserNotFound
	}
	return s.repo.GetForSaleByID(ctx, id)
}
func (s *TrafficPackService) GetSummary(ctx context.Context, userID int64, now time.Time) (*TrafficCreditSummary, error) {
	if s == nil || s.repo == nil {
		return &TrafficCreditSummary{}, nil
	}
	return s.repo.GetSummary(ctx, userID, now)
}
func (s *TrafficPackService) ListUserCredits(ctx context.Context, userID int64, now time.Time) ([]TrafficCredit, error) {
	if s == nil || s.repo == nil {
		return []TrafficCredit{}, nil
	}
	return s.repo.ListUserCredits(ctx, userID, now)
}
func (s *TrafficPackService) GetCreditByOrderID(ctx context.Context, orderID int64) (*TrafficCredit, error) {
	if s == nil || s.repo == nil {
		return nil, ErrUserNotFound
	}
	return s.repo.GetCreditByOrderID(ctx, orderID)
}
func (s *TrafficPackService) HasAvailableCredit(ctx context.Context, userID int64, now time.Time) (bool, error) {
	if s == nil || s.repo == nil {
		return false, nil
	}
	return s.repo.HasAvailableCredit(ctx, userID, now)
}
func (s *TrafficPackService) CreditPurchase(ctx context.Context, input CreditTrafficPackInput) error {
	if s == nil || s.repo == nil {
		return ErrUserNotFound
	}
	return s.repo.CreditPurchase(ctx, input)
}
func (s *TrafficPackService) Deduct(ctx context.Context, userID int64, amountUSD float64, requestID string, now time.Time) (bool, []TrafficCreditDeduction, error) {
	if s == nil || s.repo == nil {
		return false, nil, nil
	}
	return s.repo.Deduct(ctx, userID, amountUSD, requestID, now)
}
func (s *TrafficPackService) RevokePurchase(ctx context.Context, orderID int64, now time.Time) error {
	if s == nil || s.repo == nil {
		return ErrUserNotFound
	}
	return s.repo.RevokePurchase(ctx, orderID, now)
}

func PlanTrafficCreditDeductions(batches []TrafficCreditBatch, amountUSD float64) ([]TrafficCreditDeduction, bool) {
	amountUSD = roundTrafficCreditAmount(amountUSD)
	if amountUSD <= 0 {
		return nil, true
	}
	ordered := append([]TrafficCreditBatch(nil), batches...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if !ordered[i].ExpiresAt.Equal(ordered[j].ExpiresAt) {
			return ordered[i].ExpiresAt.Before(ordered[j].ExpiresAt)
		}
		if !ordered[i].CreditedAt.Equal(ordered[j].CreditedAt) {
			return ordered[i].CreditedAt.Before(ordered[j].CreditedAt)
		}
		return ordered[i].ID < ordered[j].ID
	})
	remaining := amountUSD
	plan := make([]TrafficCreditDeduction, 0, len(ordered))
	for _, batch := range ordered {
		if remaining <= 0 || batch.RemainingUSD <= 0 {
			break
		}
		amount := roundTrafficCreditAmount(math.Min(batch.RemainingUSD, remaining))
		if amount <= 0 {
			continue
		}
		plan = append(plan, TrafficCreditDeduction{CreditID: batch.ID, AmountUSD: amount})
		remaining = roundTrafficCreditAmount(remaining - amount)
	}
	return plan, remaining <= 0.0000000001
}

func roundTrafficCreditAmount(value float64) float64 { return math.Round(value*1e10) / 1e10 }
