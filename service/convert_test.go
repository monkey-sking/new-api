package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestResponseOpenAI2ClaudePreservesCacheUsage(t *testing.T) {
	t.Parallel()

	resp := &dto.OpenAITextResponse{
		Id:    "resp_123",
		Model: "gpt-4.1",
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message: dto.Message{
					Role:    "assistant",
					Content: "hello",
				},
				FinishReason: "stop",
			},
		},
		Usage: dto.Usage{
			PromptTokens:     120,
			CompletionTokens: 48,
			PromptTokensDetails: dto.InputTokenDetails{
				CachedTokens:         17,
				CachedCreationTokens: 9,
			},
		},
	}

	claudeResp := ResponseOpenAI2Claude(resp, nil)
	require.NotNil(t, claudeResp)
	require.NotNil(t, claudeResp.Usage)
	require.Equal(t, 120, claudeResp.Usage.InputTokens)
	require.Equal(t, 48, claudeResp.Usage.OutputTokens)
	require.Equal(t, 17, claudeResp.Usage.CacheReadInputTokens)
	require.Equal(t, 9, claudeResp.Usage.CacheCreationInputTokens)
}
