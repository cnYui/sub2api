package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupJSONOmitsLegacyImagePricingFields(t *testing.T) {
	raw, err := json.Marshal(Group{AllowImageGeneration: true})
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Equal(t, true, payload["allow_image_generation"])
	for _, field := range []string{
		"image_rate_independent",
		"image_rate_multiplier",
		"image_price_1k",
		"image_price_2k",
		"image_price_4k",
	} {
		require.NotContains(t, payload, field)
	}
}
