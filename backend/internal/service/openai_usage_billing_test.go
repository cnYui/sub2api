//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIUsageBillingBuildComponentsSplitsMainAndImageTokens(t *testing.T) {
	usage := OpenAIUsage{
		InputTokens:              130,
		ImageInputTokens:         20,
		OutputTokens:             70,
		CacheCreationInputTokens: 5,
		CacheReadInputTokens:     10,
		ImageOutputTokens:        40,
	}

	components := BuildOpenAIBillingComponents(usage, "gpt-main", "")

	require.Len(t, components, 2)
	require.Equal(t, OpenAIBillingComponent{
		Kind:  "main",
		Model: "gpt-main",
		Tokens: UsageTokens{
			InputTokens:         100,
			OutputTokens:        30,
			CacheCreationTokens: 5,
			CacheReadTokens:     10,
		},
	}, components[0])
	require.Equal(t, OpenAIBillingComponent{
		Kind:  "image",
		Model: "gpt-image-2",
		Tokens: UsageTokens{
			ImageInputTokens:  20,
			ImageOutputTokens: 40,
		},
	}, components[1])
}

func TestOpenAIUsageBillingCostDoesNotMultiplyByImageCount(t *testing.T) {
	usage := OpenAIUsage{
		InputTokens:              130,
		ImageInputTokens:         20,
		OutputTokens:             70,
		CacheCreationInputTokens: 5,
		CacheReadInputTokens:     10,
		ImageOutputTokens:        40,
	}
	components := []OpenAIBillingComponent{
		{
			Kind:  "main",
			Model: "gpt-main",
			Tokens: UsageTokens{
				InputTokens:         100,
				OutputTokens:        30,
				CacheCreationTokens: 5,
				CacheReadTokens:     10,
			},
		},
		{
			Kind:  "image",
			Model: "gpt-image-2",
			Tokens: UsageTokens{
				ImageInputTokens:  20,
				ImageOutputTokens: 40,
			},
		},
	}
	require.Equal(t, components, BuildOpenAIBillingComponents(usage, "gpt-main", "gpt-image-2"))

	mainCost := CostBreakdown{
		InputCost:         100 * 1e-6,
		OutputCost:        30 * 2e-6,
		CacheCreationCost: 5 * 3e-6,
		CacheReadCost:     10 * 4e-6,
	}
	mainCost.TotalCost = mainCost.InputCost + mainCost.OutputCost + mainCost.CacheCreationCost + mainCost.CacheReadCost
	mainCost.ActualCost = mainCost.TotalCost * 1.5

	imageCost := CostBreakdown{
		ImageInputCost:  20 * 8e-6,
		ImageOutputCost: 40 * 30e-6,
	}
	imageCost.TotalCost = imageCost.ImageInputCost + imageCost.ImageOutputCost
	imageCost.ActualCost = imageCost.TotalCost * 1.5

	once := MergeCostBreakdowns(&mainCost, &imageCost)
	forImageCountThree := MergeCostBreakdowns(&mainCost, &imageCost)

	require.InDelta(t, once.TotalCost, forImageCountThree.TotalCost, 1e-12)
	require.InDelta(t, once.ActualCost, forImageCountThree.ActualCost, 1e-12)
	require.InDelta(t, 20*8e-6, once.ImageInputCost, 1e-12)
	require.InDelta(t, 40*30e-6, once.ImageOutputCost, 1e-12)
}
