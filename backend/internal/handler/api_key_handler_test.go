package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAutomaticCreateAPIKeyRequestIgnoresGroupID(t *testing.T) {
	groupID := int64(9)
	quota := 5.0
	rateLimit := 1.5
	req := CreateAPIKeyRequest{
		Name:        "auto-key",
		GroupID:     &groupID,
		Quota:       &quota,
		RateLimit5h: &rateLimit,
	}

	got := automaticCreateAPIKeyServiceRequest(req)

	require.Equal(t, "auto-key", got.Name)
	require.Nil(t, got.GroupID)
	require.Equal(t, quota, got.Quota)
	require.Equal(t, rateLimit, got.RateLimit5h)
}

func TestAutomaticUpdateAPIKeyRequestIgnoresGroupID(t *testing.T) {
	groupID := int64(9)
	quota := 5.0
	req := UpdateAPIKeyRequest{
		GroupID: &groupID,
		Quota:   &quota,
	}

	got := automaticUpdateAPIKeyServiceRequest(req)

	require.Nil(t, got.GroupID)
	require.Equal(t, &quota, got.Quota)
}
