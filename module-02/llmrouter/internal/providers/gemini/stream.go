package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/q1mi/llmrouter/internal/llm"
)

var errStreamDone = errors.New("stream done")

func parseStreamEvent(data []byte) (llm.StreamChunk, bool, error) {
	var payload generateContentResponse
	if err := json.Unmarshal(data, &payload); err != nil {
		return llm.StreamChunk{}, false, fmt.Errorf("解析 Gemini 流事件: %w", err)
	}
	if payload.Error != nil && payload.Error.Message != "" {
		return llm.StreamChunk{}, false, fmt.Errorf("Gemini 流事件返回错误: %s", payload.Error.Message)
	}

	chunk := llm.StreamChunk{}
	if metadataUsage := usageFromMetadata(payload.UsageMetadata); metadataUsage != (llm.Usage{}) {
		chunk.Usage = &metadataUsage
	}
	if len(payload.Candidates) == 0 {
		if payload.PromptFeedback != nil && payload.PromptFeedback.BlockReason != "" {
			return llm.StreamChunk{}, false, fmt.Errorf(
				"Gemini 请求被拦截: %s",
				payload.PromptFeedback.BlockReason,
			)
		}
		return chunk, false, nil
	}

	candidate := payload.Candidates[0]
	chunk.Content = textFromContent(candidate.Content)
	return chunk, candidate.FinishReason != "", nil
}

func sendChunk(ctx context.Context, output chan<- llm.StreamChunk, chunk llm.StreamChunk) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case output <- chunk:
		return nil
	}
}
