package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIFailoverExhaustedPreservesUpstreamServiceUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	handler := &OpenAIGatewayHandler{}
	handler.handleFailoverExhausted(context, &service.UpstreamFailoverError{StatusCode: http.StatusServiceUnavailable}, false)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Equal(t, "No upstream service is currently available. Please retry later.", gjson.GetBytes(recorder.Body.Bytes(), "error.message").String())
	require.Equal(t, "S2A-5001", gjson.GetBytes(recorder.Body.Bytes(), "error.error_id").String())
	require.Equal(t, "NO_AVAILABLE_UPSTREAM", gjson.GetBytes(recorder.Body.Bytes(), "error.sub2api_code").String())
	require.Equal(t, "S2A-5001", recorder.Header().Get("X-Sub2API-Error-ID"))
	require.Equal(t, "NO_AVAILABLE_UPSTREAM", recorder.Header().Get("X-Sub2API-Error-Code"))
}

func TestOpenAIFailoverExhaustedPreservesUpstreamRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	responseHeaders := make(http.Header)
	responseHeaders.Set("X-CLIProxy-Error-Class", "all_accounts_rate_limited")
	responseHeaders.Set("Retry-After", "17")

	handler := &OpenAIGatewayHandler{}
	handler.handleFailoverExhausted(context, &service.UpstreamFailoverError{
		StatusCode:      http.StatusServiceUnavailable,
		ResponseHeaders: responseHeaders,
		ResponseBody:    bytes.TrimSpace([]byte(`{"error":{"code":"all_accounts_rate_limited"}}`)),
	}, false)

	require.Equal(t, http.StatusTooManyRequests, recorder.Code)
	require.Equal(t, "rate_limit_error", gjson.GetBytes(recorder.Body.Bytes(), "error.type").String())
	require.Equal(t, "S2A-5004", gjson.GetBytes(recorder.Body.Bytes(), "error.error_id").String())
	require.Equal(t, "UPSTREAM_RATE_LIMITED", gjson.GetBytes(recorder.Body.Bytes(), "error.sub2api_code").String())
	require.Equal(t, "17", recorder.Header().Get("Retry-After"))
	require.Equal(t, int64(17), gjson.GetBytes(recorder.Body.Bytes(), "error.retry_after").Int())
}

func TestAnthropicFailoverExhaustedPublishesErrorContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	handler := &OpenAIGatewayHandler{}
	handler.handleAnthropicFailoverExhausted(context, &service.UpstreamFailoverError{StatusCode: http.StatusServiceUnavailable}, false)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Equal(t, "error", gjson.GetBytes(recorder.Body.Bytes(), "type").String())
	require.Equal(t, "S2A-5001", gjson.GetBytes(recorder.Body.Bytes(), "error.error_id").String())
	require.Equal(t, "NO_AVAILABLE_UPSTREAM", gjson.GetBytes(recorder.Body.Bytes(), "error.sub2api_code").String())
	require.Equal(t, "S2A-5001", recorder.Header().Get("X-Sub2API-Error-ID"))
}

func TestGeminiFailoverExhaustedPublishesErrorContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini:generateContent", nil)

	handler := &GatewayHandler{}
	handler.handleGeminiFailoverExhausted(context, &service.UpstreamFailoverError{StatusCode: http.StatusServiceUnavailable})

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Equal(t, "No upstream service is currently available. Please retry later.", gjson.GetBytes(recorder.Body.Bytes(), "error.message").String())
	require.Equal(t, "NO_AVAILABLE_UPSTREAM", gjson.GetBytes(recorder.Body.Bytes(), "error.details.0.reason").String())
	require.Equal(t, "S2A-5001", gjson.GetBytes(recorder.Body.Bytes(), "error.details.0.metadata.error_id").String())
	require.Equal(t, "S2A-5001", recorder.Header().Get("X-Sub2API-Error-ID"))
}
