package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageLogAccountSummaryOmitsCredentials(t *testing.T) {
	account := &service.Account{ID: 9, Name: "channel", Credentials: map[string]any{"api_key": "secret", "base_url": "https://example.test"}}

	got := usageLogAccountSummary(account)

	require.NotNil(t, got)
	require.Nil(t, got.Credentials)
	require.Equal(t, "channel", got.Name)
	require.Equal(t, "secret", account.Credentials["api_key"])
}
