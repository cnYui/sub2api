package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func decodeSettingsResponse(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var resp response.Response
	require.NoError(t, json.Unmarshal(body, &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	return data
}

func TestSettingHandler_UpdateSettingsResponseUsesSameRuntimeMappingAsGet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyRegistrationEnabled:      "true",
			service.SettingKeyPromoCodeEnabled:         "true",
			service.SettingKeyOpsMonitoringEnabled:     "true",
			service.SettingKeyWebSearchEmulationConfig: `{"enabled":true,"providers":[{"type":"brave","api_key_configured":true}]}`,
		},
	}
	settingService := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	opsService := service.NewOpsService(nil, repo, &config.Config{Ops: config.OpsConfig{Enabled: false}}, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewSettingHandler(settingService, nil, nil, opsService, nil, nil, nil)

	getRec := httptest.NewRecorder()
	getCtx, _ := gin.CreateTestContext(getRec)
	getCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	handler.GetSettings(getCtx)
	require.Equal(t, http.StatusOK, getRec.Code)
	getData := decodeSettingsResponse(t, getRec.Body.Bytes())
	require.Equal(t, true, getData["web_search_emulation_enabled"])
	require.Equal(t, false, getData["ops_monitoring_enabled"])

	putBody := map[string]any{
		"registration_enabled":   true,
		"promo_code_enabled":     true,
		"ops_monitoring_enabled": true,
	}
	rawBody, err := json.Marshal(putBody)
	require.NoError(t, err)

	putRec := httptest.NewRecorder()
	putCtx, _ := gin.CreateTestContext(putRec)
	putCtx.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	putCtx.Request.Header.Set("Content-Type", "application/json")
	handler.UpdateSettings(putCtx)
	require.Equal(t, http.StatusOK, putRec.Code)
	putData := decodeSettingsResponse(t, putRec.Body.Bytes())

	require.Equal(t, getData["web_search_emulation_enabled"], putData["web_search_emulation_enabled"])
	require.Equal(t, getData["ops_monitoring_enabled"], putData["ops_monitoring_enabled"])
}
