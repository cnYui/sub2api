//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInternalUsageEventService_SkipsUncorrelatedCLIProxySuccessEvent(t *testing.T) {
	service := &InternalUsageEventService{}
	event := CLIProxyUsageEvent{
		RequestID:  "cliproxy-request-1",
		APIKeyHash: "key-hash",
		Model:      "gpt-5.6-sol",
		Success:    true,
	}

	result, err := service.RecordCLIProxyUsageEvent(context.Background(), event, []byte(`{"request_id":"cliproxy-request-1"}`))

	require.NoError(t, err)
	require.True(t, result.Skipped)
	require.Equal(t, event.RequestID, result.RequestID)
}
