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

	dbName := "file:" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name()) + "?mode=memory&cache=shared"
	db, err := sql.Open("sqlite", dbName)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	return service.NewPaymentConfigService(client, checkoutSettingRepoStub{}, nil)
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
