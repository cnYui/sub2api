package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newGatewayRoutesTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	RegisterGatewayRoutes(
		router,
		&handler.Handlers{
			Gateway:       &handler.GatewayHandler{},
			OpenAIGateway: &handler.OpenAIGatewayHandler{},
		},
		servermiddleware.APIKeyAuthMiddleware(func(c *gin.Context) {
			groupID := int64(1)
			c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{
				GroupID: &groupID,
				Group:   &service.Group{Platform: service.PlatformOpenAI},
			})
			c.Next()
		}),
		nil,
		nil,
		nil,
		nil,
		nil,
		&config.Config{},
	)

	return router
}

func TestGatewayRoutesRejectBareOpenAICompatiblePaths(t *testing.T) {
	router := newGatewayRoutesTestRouter()

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/models?client_version=0.142.5"},
		{http.MethodPost, "/responses"},
		{http.MethodPost, "/responses/compact"},
		{http.MethodGet, "/responses"},
		{http.MethodPost, "/chat/completions"},
		{http.MethodPost, "/embeddings"},
		{http.MethodPost, "/images/generations"},
		{http.MethodPost, "/images/edits"},
		{http.MethodPost, "/backend-api/codex/responses"},
		{http.MethodPost, "/backend-api/codex/responses/compact"},
		{http.MethodGet, "/backend-api/codex/responses"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"model":"gpt-5"}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Contains(t, w.Body.String(), `"code":"INVALID_BASE_URL"`)
			require.Contains(t, w.Body.String(), `https://api.aaccx.pw/v1`)
		})
	}
}

func TestGatewayRoutesKeepFormalV1OpenAIPathsRegistered(t *testing.T) {
	router := newGatewayRoutesTestRouter()

	routes := make(map[string]bool)
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = true
	}

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/v1/models"},
		{http.MethodPost, "/v1/responses"},
		{http.MethodPost, "/v1/responses/*subpath"},
		{http.MethodPost, "/v1/chat/completions"},
		{http.MethodPost, "/v1/embeddings"},
		{http.MethodPost, "/v1/images/generations"},
		{http.MethodPost, "/v1/images/edits"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			require.True(t, routes[tc.method+" "+tc.path], "formal route must remain registered")
		})
	}
}

func TestOpenAIOnlyRejectsNonOpenAIPlatform(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		groupID := int64(2)
		c.Set(string(servermiddleware.ContextKeyAPIKey), &service.APIKey{
			GroupID: &groupID,
			Group:   &service.Group{Platform: service.PlatformAnthropic},
		})
		c.Next()
	})
	called := false
	router.POST("/embeddings", openAIOnly("Embeddings", func(c *gin.Context) {
		called = true
		c.Status(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/embeddings", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.False(t, called)
	require.Contains(t, w.Body.String(), "Embeddings API is not supported for this platform")
}
