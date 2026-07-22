package admin

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
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func TestSanitizeAdminPaymentOrderForResponseAddsCurrency(t *testing.T) {
	now := time.Now()
	order := &dbent.PaymentOrder{
		ID:          1,
		UserID:      2,
		Amount:      100,
		PayAmount:   108,
		FeeRate:     8,
		OutTradeNo:  "sub2_202606250001",
		PaymentType: "stripe",
		OrderType:   "subscription",
		Status:      "COMPLETED",
		ExpiresAt:   now,
		CreatedAt:   now,
		UpdatedAt:   now,
		ProviderSnapshot: map[string]any{
			"schema_version": 2,
			"currency":       "USD",
		},
	}

	got := sanitizeAdminPaymentOrderForResponse(order)
	if got == nil {
		t.Fatal("expected sanitized order")
	}
	if got.Currency != "USD" {
		t.Fatalf("expected currency USD, got %q", got.Currency)
	}

	body, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal sanitized order: %v", err)
	}
	if strings.Contains(string(body), "provider_snapshot") {
		t.Fatalf("expected provider_snapshot to be omitted, got %s", string(body))
	}
}

func TestAdminPaymentHandlerListPlansNormalizesPublicCodexPlanDisplay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newAdminPaymentConfigTestClient(t)
	ctx := context.Background()
	group, err := client.Group.Create().
		SetName("codex-pool-19-usd").
		SetSubscriptionType(service.SubscriptionTypeSubscription).
		SetDailyLimitUsd(15).
		SetWeeklyLimitUsd(58).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.SubscriptionPlan.Create().
		SetGroupID(group.ID).
		SetName("29 元订阅池").
		SetDescription("月度订阅-时间 30天，日限额 15刀，24点刷新").
		SetPrice(29).
		SetValidityDays(30).
		SetValidityUnit("month").
		SetFeatures("每日 15 USD").
		SetProductName("月度订阅 30 天").
		SetForSale(true).
		SetSortOrder(29).
		Save(ctx)
	require.NoError(t, err)

	handler := NewPaymentHandler(nil, service.NewPaymentConfigService(client, nil, nil))
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/payment/plans", nil)

	handler.ListPlans(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Code int `json:"code"`
		Data []struct {
			Name         string `json:"name"`
			Description  string `json:"description"`
			Features     string `json:"features"`
			ProductName  string `json:"product_name"`
			ValidityDays int    `json:"validity_days"`
			ValidityUnit string `json:"validity_unit"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data, 1)
	plan := resp.Data[0]
	require.Equal(t, "29 元订阅池", plan.Name)
	require.Equal(t, "28 天订阅，每 7 天刷新 58 USD 周额度，购买时间起滚动计算", plan.Description)
	require.Equal(t, "周额度 58 USD\n28 天有效期\n购买时间起每 7 天刷新", plan.Features)
	require.Equal(t, "29 元订阅池", plan.ProductName)
	require.Equal(t, 28, plan.ValidityDays)
	require.Equal(t, "day", plan.ValidityUnit)
}

func newAdminPaymentConfigTestClient(t *testing.T) *dbent.Client {
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
	return client
}
