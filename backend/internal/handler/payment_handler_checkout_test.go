//go:build unit

package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

type checkoutTrafficPackRepoStub struct {
	credits []service.TrafficCredit
	summary *service.TrafficCreditSummary
}

func (s *checkoutTrafficPackRepoStub) ListForSale(context.Context) ([]service.TrafficPack, error) {
	return []service.TrafficPack{}, nil
}
func (s *checkoutTrafficPackRepoStub) GetForSaleByID(context.Context, int64) (*service.TrafficPack, error) {
	return nil, service.ErrInvalidInput
}
func (s *checkoutTrafficPackRepoStub) GetSummary(context.Context, int64, time.Time) (*service.TrafficCreditSummary, error) {
	if s.summary != nil {
		return s.summary, nil
	}
	return &service.TrafficCreditSummary{}, nil
}
func (s *checkoutTrafficPackRepoStub) ListUserCredits(context.Context, int64, time.Time) ([]service.TrafficCredit, error) {
	out := make([]service.TrafficCredit, len(s.credits))
	copy(out, s.credits)
	return out, nil
}
func (s *checkoutTrafficPackRepoStub) HasAvailableCredit(context.Context, int64, time.Time) (bool, error) {
	return false, nil
}
func (s *checkoutTrafficPackRepoStub) CreditPurchase(context.Context, service.CreditTrafficPackInput) error {
	return nil
}
func (s *checkoutTrafficPackRepoStub) Deduct(context.Context, int64, float64, string, time.Time) (bool, []service.TrafficCreditDeduction, error) {
	return false, nil, nil
}

type checkoutSettingRepoStub struct{}

func (s checkoutSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	return nil, nil
}
func (s checkoutSettingRepoStub) GetValue(context.Context, string) (string, error) { return "", nil }
func (s checkoutSettingRepoStub) Set(context.Context, string, string) error        { return nil }
func (s checkoutSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		values[key] = ""
	}
	return values, nil
}
func (s checkoutSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	return nil
}
func (s checkoutSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}
func (s checkoutSettingRepoStub) Delete(context.Context, string) error { return nil }

func newCheckoutPaymentConfigService(t *testing.T) *service.PaymentConfigService {
	t.Helper()

	configSvc, _ := newCheckoutPaymentConfigServiceWithClient(t)
	return configSvc
}

func newCheckoutPaymentConfigServiceWithClient(t *testing.T) (*service.PaymentConfigService, *dbent.Client) {
	t.Helper()

	dbName := "file:" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()) + "?mode=memory&cache=shared"
	db, err := sql.Open("sqlite", dbName)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	return service.NewPaymentConfigService(client, checkoutSettingRepoStub{}, nil), client
}

func TestPaymentHandlerGetCheckoutInfoReturnsTrafficCreditsInRepositoryOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)

	orderID := int64(101)
	packID := int64(7)
	creditedAt := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	trafficRepo := &checkoutTrafficPackRepoStub{
		credits: []service.TrafficCredit{
			{
				ID:           20,
				OrderID:      &orderID,
				PackID:       &packID,
				InitialUSD:   3,
				RemainingUSD: 1,
				ReservedUSD:  1,
				AvailableUSD: 0,
				CreditedAt:   creditedAt,
				ExpiresAt:    creditedAt.AddDate(0, 0, 30),
			},
			{
				ID:           10,
				InitialUSD:   5,
				RemainingUSD: 4.25,
				ReservedUSD:  0.25,
				AvailableUSD: 4,
				CreditedAt:   creditedAt.Add(time.Hour),
				ExpiresAt:    creditedAt.AddDate(0, 0, 60),
			},
		},
	}
	paymentSvc := service.NewPaymentService(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	paymentSvc.SetTrafficPackService(service.NewTrafficPackService(trafficRepo))
	handler := NewPaymentHandler(paymentSvc, newCheckoutPaymentConfigService(t), nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/payment/checkout-info", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 31})

	handler.GetCheckoutInfo(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			TrafficCredits []struct {
				ID           int64   `json:"id"`
				OrderID      *int64  `json:"order_id"`
				PackID       *int64  `json:"pack_id"`
				InitialUSD   float64 `json:"initial_usd"`
				RemainingUSD float64 `json:"remaining_usd"`
				ReservedUSD  float64 `json:"reserved_usd"`
				AvailableUSD float64 `json:"available_usd"`
				CreditedAt   string  `json:"credited_at"`
				ExpiresAt    string  `json:"expires_at"`
			} `json:"traffic_credits"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data.TrafficCredits, 2)
	require.Equal(t, int64(20), resp.Data.TrafficCredits[0].ID)
	require.Equal(t, int64(10), resp.Data.TrafficCredits[1].ID)
	require.Equal(t, 0.0, resp.Data.TrafficCredits[0].AvailableUSD)
	require.Equal(t, 4.0, resp.Data.TrafficCredits[1].AvailableUSD)
	require.Equal(t, &orderID, resp.Data.TrafficCredits[0].OrderID)
	require.Equal(t, &packID, resp.Data.TrafficCredits[0].PackID)
	require.NotEmpty(t, resp.Data.TrafficCredits[0].CreditedAt)
	require.NotEmpty(t, resp.Data.TrafficCredits[0].ExpiresAt)
}

func TestPaymentHandlerGetPlansIncludesGroupLimits(t *testing.T) {
	gin.SetMode(gin.TestMode)

	configSvc, client := newCheckoutPaymentConfigServiceWithClient(t)
	ctx := context.Background()
	dailyLimit := 15.0
	weeklyLimit := 70.0
	group, err := client.Group.Create().
		SetName("codex-pool-19-usd").
		SetPlatform("openai").
		SetRateMultiplier(1.25).
		SetDailyLimitUsd(dailyLimit).
		SetWeeklyLimitUsd(weeklyLimit).
		SetSupportedModelScopes([]string{"responses", "images"}).
		Save(ctx)
	require.NoError(t, err)
	originalPrice := 39.0
	_, err = client.SubscriptionPlan.Create().
		SetGroupID(group.ID).
		SetName("29 元订阅池").
		SetDescription("月度订阅-时间 30天，日限额 15刀，24点刷新").
		SetPrice(29).
		SetOriginalPrice(originalPrice).
		SetValidityDays(30).
		SetValidityUnit("day").
		SetFeatures("每日 15 USD").
		SetProductName("29 元订阅池").
		SetForSale(true).
		SetSortOrder(29).
		Save(ctx)
	require.NoError(t, err)
	handler := NewPaymentHandler(nil, configSvc, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/payment/plans", nil)

	handler.GetPlans(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Code int `json:"code"`
		Data []struct {
			Name                  string   `json:"name"`
			Description           string   `json:"description"`
			Features              string   `json:"features"`
			ProductName           string   `json:"product_name"`
			GroupPlatform         string   `json:"group_platform"`
			GroupName             string   `json:"group_name"`
			RateMultiplier        float64  `json:"rate_multiplier"`
			DailyLimitUSD         *float64 `json:"daily_limit_usd"`
			WeeklyLimitUSD        *float64 `json:"weekly_limit_usd"`
			MonthlyLimitUSD       *float64 `json:"monthly_limit_usd"`
			PeriodTotalQuotaUSD   *float64 `json:"period_total_quota_usd"`
			QuotaWindowUnit       string   `json:"quota_window_unit"`
			QuotaWindowDays       int      `json:"quota_window_days"`
			EffectiveValidityDays int      `json:"effective_validity_days"`
			ValidityDays          int      `json:"validity_days"`
			ValidityUnit          string   `json:"validity_unit"`
			SupportedModelScopes  []string `json:"supported_model_scopes"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data, 1)
	require.Equal(t, "openai", resp.Data[0].GroupPlatform)
	require.Equal(t, "codex-pool-19-usd", resp.Data[0].GroupName)
	require.Equal(t, "29 元订阅池", resp.Data[0].Name)
	require.Equal(t, "28 天订阅，每 7 天刷新 76 USD 周额度，购买时间起滚动计算", resp.Data[0].Description)
	require.Equal(t, "29 元订阅池", resp.Data[0].ProductName)
	require.Equal(t, "周额度 76 USD\n28 天有效期\n购买时间起每 7 天刷新", resp.Data[0].Features)
	require.InDelta(t, 1.25, resp.Data[0].RateMultiplier, 0.0000001)
	require.Nil(t, resp.Data[0].DailyLimitUSD)
	require.NotNil(t, resp.Data[0].WeeklyLimitUSD)
	require.InDelta(t, 76, *resp.Data[0].WeeklyLimitUSD, 0.0000001)
	require.Nil(t, resp.Data[0].MonthlyLimitUSD)
	require.NotNil(t, resp.Data[0].PeriodTotalQuotaUSD)
	require.InDelta(t, 304, *resp.Data[0].PeriodTotalQuotaUSD, 0.0000001)
	require.Equal(t, "week", resp.Data[0].QuotaWindowUnit)
	require.Equal(t, 7, resp.Data[0].QuotaWindowDays)
	require.Equal(t, 28, resp.Data[0].EffectiveValidityDays)
	require.Equal(t, 28, resp.Data[0].ValidityDays)
	require.Equal(t, "day", resp.Data[0].ValidityUnit)
	require.Equal(t, []string{"responses", "images"}, resp.Data[0].SupportedModelScopes)
}

func TestPaymentHandlerGetCheckoutInfoUsesPublicCodexQuotaSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	configSvc, client := newCheckoutPaymentConfigServiceWithClient(t)
	ctx := context.Background()
	group, err := client.Group.Create().
		SetName("codex-pool-179-usd").
		SetPlatform("openai").
		SetRateMultiplier(1.0).
		SetDailyLimitUsd(100).
		SetWeeklyLimitUsd(400).
		SetMonthlyLimitUsd(3000).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.SubscriptionPlan.Create().
		SetGroupID(group.ID).
		SetName("199 元订阅池").
		SetDescription("旧套餐文案").
		SetPrice(199).
		SetValidityDays(30).
		SetValidityUnit("day").
		SetFeatures("每日 100 USD").
		SetProductName("199 元订阅池").
		SetForSale(true).
		SetSortOrder(199).
		Save(ctx)
	require.NoError(t, err)
	handler := NewPaymentHandler(service.NewPaymentService(nil, nil, nil, nil, nil, nil, nil, nil, nil), configSvc, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/payment/checkout-info", nil)

	handler.GetCheckoutInfo(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Plans []struct {
				Name                  string   `json:"name"`
				Description           string   `json:"description"`
				Features              []string `json:"features"`
				ProductName           string   `json:"product_name"`
				GroupName             string   `json:"group_name"`
				DailyLimitUSD         *float64 `json:"daily_limit_usd"`
				WeeklyLimitUSD        *float64 `json:"weekly_limit_usd"`
				MonthlyLimitUSD       *float64 `json:"monthly_limit_usd"`
				PeriodTotalQuotaUSD   *float64 `json:"period_total_quota_usd"`
				QuotaWindowUnit       string   `json:"quota_window_unit"`
				QuotaWindowDays       int      `json:"quota_window_days"`
				EffectiveValidityDays int      `json:"effective_validity_days"`
				ValidityDays          int      `json:"validity_days"`
				ValidityUnit          string   `json:"validity_unit"`
			} `json:"plans"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data.Plans, 1)
	plan := resp.Data.Plans[0]
	require.Equal(t, "codex-pool-179-usd", plan.GroupName)
	require.Equal(t, "199 元订阅池", plan.Name)
	require.Equal(t, "28 天订阅，每 7 天刷新 520 USD 周额度，购买时间起滚动计算", plan.Description)
	require.Equal(t, "199 元订阅池", plan.ProductName)
	require.Equal(t, []string{"周额度 520 USD", "28 天有效期", "购买时间起每 7 天刷新"}, plan.Features)
	require.Nil(t, plan.DailyLimitUSD)
	require.NotNil(t, plan.WeeklyLimitUSD)
	require.InDelta(t, 520, *plan.WeeklyLimitUSD, 0.0000001)
	require.Nil(t, plan.MonthlyLimitUSD)
	require.NotNil(t, plan.PeriodTotalQuotaUSD)
	require.InDelta(t, 2080, *plan.PeriodTotalQuotaUSD, 0.0000001)
	require.Equal(t, "week", plan.QuotaWindowUnit)
	require.Equal(t, 7, plan.QuotaWindowDays)
	require.Equal(t, 28, plan.EffectiveValidityDays)
	require.Equal(t, 28, plan.ValidityDays)
	require.Equal(t, "day", plan.ValidityUnit)
}
