//go:build unit

package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

var errTrafficCreditExhaustionRepoUnavailable = errors.New("traffic credit exhaustion repo unavailable")

type trafficCreditExhaustionRepoStub struct {
	pending []int64
	listErr error
	ackErr  error
	calls   [][]int64
	userIDs []int64
}

func (s *trafficCreditExhaustionRepoStub) ListPendingEventIDs(context.Context, int64) ([]int64, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	out := make([]int64, len(s.pending))
	copy(out, s.pending)
	return out, nil
}

func (s *trafficCreditExhaustionRepoStub) AcknowledgeEvents(_ context.Context, userID int64, eventIDs []int64, _ time.Time) error {
	copied := append([]int64(nil), eventIDs...)
	s.calls = append(s.calls, copied)
	s.userIDs = append(s.userIDs, userID)
	return s.ackErr
}

func TestTrafficCreditExhaustionHandlerRejectsInvalidAckPayloadWithoutRepositoryCall(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		body string
	}{
		{name: "empty list", body: `{"event_ids":[]}`},
		{name: "negative id", body: `{"event_ids":[1,-2]}`},
		{name: "zero id", body: `{"event_ids":[0]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &trafficCreditExhaustionRepoStub{}
			handler := &UserHandler{trafficCreditExhaustionRepo: repo}

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/user/traffic-credit-exhaustion-events/ack", bytes.NewBufferString(tt.body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 31})

			handler.AcknowledgeTrafficCreditExhaustionEvents(c)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.Empty(t, repo.calls)
		})
	}
}

func TestTrafficCreditExhaustionHandlerRejectsForeignEventIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &trafficCreditExhaustionRepoStub{ackErr: service.ErrInvalidInput}
	handler := &UserHandler{trafficCreditExhaustionRepo: repo}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/user/traffic-credit-exhaustion-events/ack", bytes.NewBufferString(`{"event_ids":[7,9]}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 31})

	handler.AcknowledgeTrafficCreditExhaustionEvents(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, [][]int64{{7, 9}}, repo.calls)
	require.Equal(t, []int64{31}, repo.userIDs)
}

func TestTrafficCreditExhaustionHandlerAcknowledgeEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &trafficCreditExhaustionRepoStub{}
	handler := &UserHandler{trafficCreditExhaustionRepo: repo}

	for range 2 {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/user/traffic-credit-exhaustion-events/ack", bytes.NewBufferString(`{"event_ids":[7,9]}`))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 31})

		handler.AcknowledgeTrafficCreditExhaustionEvents(c)

		require.Equal(t, http.StatusNoContent, recorder.Code)
	}

	require.Equal(t, [][]int64{{7, 9}, {7, 9}}, repo.calls)
	require.Equal(t, []int64{31, 31}, repo.userIDs)
}
