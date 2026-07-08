package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type effectiveGroupResolverStub struct {
	result *service.EffectiveGroupResult
	err    error
}

func (s *effectiveGroupResolverStub) ResolveEffectiveGroupForRequest(ctx context.Context, userID int64, path string, forcePlatform string) (*service.EffectiveGroupResult, error) {
	return s.result, s.err
}

func TestResolveEffectiveGroupMiddlewareWritesRequestScopedAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	original := &service.APIKey{ID: 1, UserID: 62, Key: "sk-test", GroupID: nil, User: &service.User{ID: 62, Status: service.StatusActive}}
	group := &service.Group{ID: 77, Name: service.TrafficPackOpenAIGroupName, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), original)
		c.Next()
	})
	router.Use(ResolveEffectiveGroup(&effectiveGroupResolverStub{
		result: &service.EffectiveGroupResult{Group: group, Source: service.EffectiveGroupSourceTrafficPack},
	}, AnthropicErrorWriter))
	router.GET("/v1/responses", func(c *gin.Context) {
		got, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.NotSame(t, original, got)
		require.Nil(t, original.GroupID)
		require.Nil(t, original.Group)
		require.NotNil(t, got.GroupID)
		require.Equal(t, int64(77), *got.GroupID)
		require.Equal(t, group, got.Group)

		ctxGroup, ok := c.Request.Context().Value(ctxkey.Group).(*service.Group)
		require.True(t, ok)
		require.Equal(t, group.ID, ctxGroup.ID)
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestResolveEffectiveGroupMiddlewareSkipsFixedGroupKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(2)
	original := &service.APIKey{ID: 1, UserID: 7, GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI}}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), original)
		c.Next()
	})
	router.Use(ResolveEffectiveGroup(&effectiveGroupResolverStub{}, AnthropicErrorWriter))
	router.GET("/v1/responses", func(c *gin.Context) {
		got, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.Same(t, original, got)
		c.Status(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/responses", nil))
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestResolveEffectiveGroupMiddlewareRejectsUnsupportedAutomaticKeyEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	original := &service.APIKey{ID: 1, UserID: 62, Key: "auto-key", GroupID: nil, User: &service.User{ID: 62, Status: service.StatusActive}}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), original)
		c.Next()
	})
	router.Use(ResolveEffectiveGroupForSupportedEndpoints(&effectiveGroupResolverStub{
		result: &service.EffectiveGroupResult{
			Group:  &service.Group{ID: 77, Platform: service.PlatformOpenAI},
			Source: service.EffectiveGroupSourceTrafficPack,
		},
	}, AnthropicErrorWriter))
	router.GET("/v1beta/models", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1beta/models", nil))

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "AUTO_KEY_UNSUPPORTED_ENDPOINT")
}

func TestResolveEffectiveGroupMiddlewareWritesGoogleErrorForUnsupportedAutomaticKeyEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	original := &service.APIKey{ID: 1, UserID: 62, Key: "auto-key", GroupID: nil, User: &service.User{ID: 62, Status: service.StatusActive}}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), original)
		c.Next()
	})
	router.Use(ResolveEffectiveGroupForSupportedEndpoints(&effectiveGroupResolverStub{}, GoogleErrorWriter))
	router.GET("/v1beta/models", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1beta/models", nil))

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "AUTO_KEY_UNSUPPORTED_ENDPOINT")
	require.Contains(t, rec.Body.String(), "PERMISSION_DENIED")
}

func TestDefaultAutomaticKeyEndpointPolicyAllowsUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/usage", nil)

	platform, supported := DefaultAutomaticKeyEndpointPolicy(c)

	require.True(t, supported)
	require.Equal(t, service.PlatformOpenAI, platform)
}
