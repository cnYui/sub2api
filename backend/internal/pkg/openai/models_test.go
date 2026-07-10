//go:build unit

package openai

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultModelsIncludeGPT56CompleteNamesOnly(t *testing.T) {
	ids := DefaultModelIDs()

	require.Contains(t, ids, "gpt-5.6-sol")
	require.Contains(t, ids, "gpt-5.6-terra")
	require.Contains(t, ids, "gpt-5.6-luna")
	require.NotContains(t, ids, "gpt-5.6")
}

func TestDefaultModelsGPT56DisplayNames(t *testing.T) {
	byID := make(map[string]Model, len(DefaultModels))
	for _, model := range DefaultModels {
		byID[model.ID] = model
	}

	require.Equal(t, "GPT-5.6 Sol", byID["gpt-5.6-sol"].DisplayName)
	require.Equal(t, "GPT-5.6 Terra", byID["gpt-5.6-terra"].DisplayName)
	require.Equal(t, "GPT-5.6 Luna", byID["gpt-5.6-luna"].DisplayName)
	require.Equal(t, "openai", byID["gpt-5.6-sol"].OwnedBy)
	require.Equal(t, "model", byID["gpt-5.6-sol"].Object)
}
