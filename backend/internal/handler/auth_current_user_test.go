//go:build unit

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAuthHandlerGetCurrentUserReturnsProfileCompatibilityFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	verifiedAt := time.Date(2026, 4, 20, 8, 30, 0, 0, time.UTC)
	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:           31,
			Email:        "me@example.com",
			Username:     "linuxdo-handle",
			Role:         service.RoleUser,
			Status:       service.StatusActive,
			AvatarURL:    "https://cdn.example.com/linuxdo.png",
			AvatarSource: "remote_url",
		},
		identities: []service.UserAuthIdentityRecord{
			{
				ProviderType:    "linuxdo",
				ProviderKey:     "linuxdo",
				ProviderSubject: "linuxdo-subject-31",
				VerifiedAt:      &verifiedAt,
				Metadata: map[string]any{
					"username":   "linuxdo-handle",
					"avatar_url": "https://cdn.example.com/linuxdo.png",
				},
			},
		},
	}

	handler := &AuthHandler{
		userService: service.NewUserService(repo, nil, nil, nil),
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 31})

	handler.GetCurrentUser(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Equal(t, true, resp.Data["email_bound"])
	require.Equal(t, true, resp.Data["linuxdo_bound"])
	require.Equal(t, "https://cdn.example.com/linuxdo.png", resp.Data["avatar_url"])

	authBindings, ok := resp.Data["auth_bindings"].(map[string]any)
	require.True(t, ok)
	linuxdoBinding, ok := authBindings["linuxdo"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, linuxdoBinding["bound"])

	avatarSource, ok := resp.Data["avatar_source"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "linuxdo", avatarSource["provider"])
	require.Equal(t, "linuxdo", avatarSource["source"])

	profileSources, ok := resp.Data["profile_sources"].(map[string]any)
	require.True(t, ok)
	usernameSource, ok := profileSources["username"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "linuxdo", usernameSource["provider"])
	require.Equal(t, "linuxdo", usernameSource["source"])
}

func TestAuthHandlerGetCurrentUserIncludesTrafficCreditExhaustionNotice(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &userHandlerRepoStub{
		user: &service.User{
			ID:       31,
			Email:    "notice@example.com",
			Username: "notice-user",
			Role:     service.RoleUser,
			Status:   service.StatusActive,
		},
	}
	handler := &AuthHandler{
		userService:                 service.NewUserService(repo, nil, nil, nil),
		trafficCreditExhaustionRepo: &trafficCreditExhaustionRepoStub{pending: []int64{7, 9}},
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 31})

	handler.GetCurrentUser(c)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Notice *struct {
				EventIDs []int64 `json:"event_ids"`
			} `json:"traffic_credit_exhaustion_notice"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.NotNil(t, resp.Data.Notice)
	require.Equal(t, []int64{7, 9}, resp.Data.Notice.EventIDs)
}

func TestAuthHandlerGetCurrentUserOmitsTrafficCreditExhaustionNoticeWhenEmptyOrUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		repo service.TrafficCreditExhaustionRepository
	}{
		{name: "empty", repo: &trafficCreditExhaustionRepoStub{}},
		{name: "query error", repo: &trafficCreditExhaustionRepoStub{listErr: errTrafficCreditExhaustionRepoUnavailable}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := &userHandlerRepoStub{
				user: &service.User{
					ID:       31,
					Email:    "notice-empty@example.com",
					Username: "notice-empty-user",
					Role:     service.RoleUser,
					Status:   service.StatusActive,
				},
			}
			handler := &AuthHandler{
				userService:                 service.NewUserService(userRepo, nil, nil, nil),
				trafficCreditExhaustionRepo: tt.repo,
			}

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
			c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 31})

			handler.GetCurrentUser(c)

			require.Equal(t, http.StatusOK, recorder.Code)

			var resp struct {
				Code int            `json:"code"`
				Data map[string]any `json:"data"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
			require.Equal(t, 0, resp.Code)
			_, exists := resp.Data["traffic_credit_exhaustion_notice"]
			require.False(t, exists)
		})
	}
}
