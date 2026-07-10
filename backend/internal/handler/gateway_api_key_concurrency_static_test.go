package handler

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGatewayEntrypointsDoNotUseUserSlotForCallerConcurrency(t *testing.T) {
	files := []string{
		"gateway_handler.go",
		"gateway_handler_responses.go",
		"gateway_handler_chat_completions.go",
		"gemini_v1beta_handler.go",
		"openai_gateway_handler.go",
		"openai_chat_completions.go",
		"openai_embeddings.go",
		"openai_images.go",
	}
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			data, err := os.ReadFile(file)
			require.NoError(t, err)
			content := string(data)
			require.NotContains(t, content, "AcquireUserSlotWithWait(c, subject.UserID")
			require.NotContains(t, content, "AcquireUserSlotWithWait(c, authSubject.UserID")
			require.NotContains(t, content, "AcquireUserSlotWithWait(c, userID, userConcurrency")
			require.NotContains(t, content, "TryAcquireUserSlot(ctx, subject.UserID")
			require.NotContains(t, content, "acquireResponsesUserSlot(c, subject.UserID")
		})
	}
}
