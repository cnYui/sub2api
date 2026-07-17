package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type dashboardUsageRepoStub struct {
	service.UsageLogRepository
	stats       *usagestats.UserDashboardStats
	quota       *usagestats.UserDashboardQuota
	statsUserID int64
	quotaUserID int64
}

func (s *dashboardUsageRepoStub) GetUserDashboardStats(ctx context.Context, userID int64) (*usagestats.UserDashboardStats, error) {
	s.statsUserID = userID
	return s.stats, nil
}

func (s *dashboardUsageRepoStub) GetUserDashboardQuota(ctx context.Context, userID int64) (*usagestats.UserDashboardQuota, error) {
	s.quotaUserID = userID
	return s.quota, nil
}

func newDashboardUsageTestRouter(repo *dashboardUsageRepoStub, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	usageSvc := service.NewUsageService(repo, nil, nil, nil)
	handler := NewUsageHandler(usageSvc, nil, nil, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
		c.Next()
	})
	router.GET("/usage/dashboard/stats", handler.DashboardStats)
	router.GET("/usage/dashboard/quota", handler.DashboardQuota)
	return router
}

func TestDashboardStatsIncludesQuota(t *testing.T) {
	quota := &usagestats.UserDashboardQuota{
		PeriodMode:     usagestats.UserDashboardQuotaModeEntitlementPeriod,
		TodayUsageUSD:  1.23,
		TodayLimitUSD:  19,
		PeriodUsageUSD: 5.67,
		PeriodLimitUSD: 570,
		PeriodDays:     30,
	}
	repo := &dashboardUsageRepoStub{
		stats: &usagestats.UserDashboardStats{
			TotalAPIKeys: 1,
			Quota:        quota,
		},
		quota: quota,
	}
	router := newDashboardUsageTestRouter(repo, 42)

	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/stats", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(42), repo.statsUserID)
	var got struct {
		Data usagestats.UserDashboardStats `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.NotNil(t, got.Data.Quota)
	require.Equal(t, usagestats.UserDashboardQuotaModeEntitlementPeriod, got.Data.Quota.PeriodMode)
	require.InDelta(t, 1.23, got.Data.Quota.TodayUsageUSD, 0.0000001)
}

func TestDashboardQuotaReturnsCurrentUserQuota(t *testing.T) {
	quota := &usagestats.UserDashboardQuota{
		PeriodMode:     usagestats.UserDashboardQuotaModeRolling30Legacy,
		TodayUsageUSD:  0.5,
		TodayLimitUSD:  11,
		PeriodUsageUSD: 2.5,
		PeriodLimitUSD: 330,
		PeriodDays:     30,
	}
	repo := &dashboardUsageRepoStub{quota: quota}
	router := newDashboardUsageTestRouter(repo, 99)

	req := httptest.NewRequest(http.MethodGet, "/usage/dashboard/quota", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(99), repo.quotaUserID)
	var got struct {
		Data usagestats.UserDashboardQuota `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, usagestats.UserDashboardQuotaModeRolling30Legacy, got.Data.PeriodMode)
	require.InDelta(t, 2.5, got.Data.PeriodUsageUSD, 0.0000001)
}
