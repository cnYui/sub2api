//go:build unit

package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUsageFactResponseGate_BuffersNonStreamingUntilRelease(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	gate := newUsageFactResponseGate(c.Writer, false)

	gate.WriteHeader(http.StatusCreated)
	_, err := gate.Write([]byte(`{"id":"resp_1"}`))
	require.NoError(t, err)
	require.Empty(t, recorder.Body.String())
	require.Equal(t, http.StatusOK, recorder.Code)

	require.NoError(t, gate.Release())
	require.Equal(t, http.StatusCreated, recorder.Code)
	require.JSONEq(t, `{"id":"resp_1"}`, recorder.Body.String())
}

func TestUsageFactResponseGate_HoldsSSETerminalEvent(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	gate := newUsageFactResponseGate(c.Writer, true)

	_, err := gate.Write([]byte("data: {\"type\":\"response.output_text.delta\"}\n\n"))
	require.NoError(t, err)
	_, err = gate.Write([]byte("data: {\"type\":\"response.completed\"}\n\n"))
	require.NoError(t, err)
	require.Contains(t, recorder.Body.String(), "response.output_text.delta")
	require.NotContains(t, recorder.Body.String(), "response.completed")

	require.NoError(t, gate.Release())
	require.Contains(t, recorder.Body.String(), "response.completed")
}

func TestUsageFactResponseGate_HoldsSplitSSETerminalEvent(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	gate := newUsageFactResponseGate(c.Writer, true)

	_, err := gate.Write([]byte("data: {\"type\":\"response.comp"))
	require.NoError(t, err)
	_, err = gate.Write([]byte("leted\"}\n\n"))
	require.NoError(t, err)
	require.NotContains(t, recorder.Body.String(), "response.completed")

	require.NoError(t, gate.Release())
	require.Contains(t, recorder.Body.String(), "response.completed")
}

func TestUsageFactResponseGate_DiscardDropsBufferedResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	gate := newUsageFactResponseGate(c.Writer, false)

	_, err := gate.Write([]byte(`{"id":"resp_1"}`))
	require.NoError(t, err)
	gate.Discard()
	require.NoError(t, gate.Release())
	require.Empty(t, recorder.Body.String())
}

func TestFinalizeUsageFactResponse_NonStreamingPersistenceFailureReplacesUpstreamSuccess(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	gate := newUsageFactResponseGate(c.Writer, false)
	c.Writer = gate
	_, err := gate.Write([]byte(`{"id":"resp_1"}`))
	require.NoError(t, err)

	err = finalizeUsageFactResponse(c, gate, func(ctx context.Context) error {
		return errors.New("database unavailable")
	})

	require.ErrorContains(t, err, "database unavailable")
	require.NotContains(t, recorder.Body.String(), "resp_1")
	require.Contains(t, recorder.Body.String(), "billing_persistence_error")
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestFinalizeUsageFactResponse_StreamingPersistenceFailureDropsTerminalEvent(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	gate := newUsageFactResponseGate(c.Writer, true)
	c.Writer = gate
	_, err := gate.Write([]byte("data: {\"type\":\"response.output_text.delta\"}\n\n"))
	require.NoError(t, err)
	_, err = gate.Write([]byte("data: {\"type\":\"response.completed\"}\n\n"))
	require.NoError(t, err)

	err = finalizeUsageFactResponse(c, gate, func(ctx context.Context) error {
		return errors.New("database unavailable")
	})

	require.ErrorContains(t, err, "database unavailable")
	require.Contains(t, recorder.Body.String(), "response.output_text.delta")
	require.NotContains(t, recorder.Body.String(), "response.completed")
	require.Contains(t, recorder.Body.String(), "billing_persistence_error")
}
