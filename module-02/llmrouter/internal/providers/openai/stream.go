package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/q1mi/llmrouter/internal/llm"
)

var errStreamDone = errors.New("stream done")

func parseStreamEvent(data []byte) (llm.StreamChunk, bool, error) {
	if string(data) == "[DONE]" {
		return llm.StreamChunk{}, true, nil
	}

	var payload struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
		Usage *usage `json:"usage"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return llm.StreamChunk{}, false, fmt.Errorf("解析 OpenAI 流事件: %w", err)
	}
	if payload.Error != nil {
		return llm.StreamChunk{}, false, fmt.Errorf("OpenAI 流事件返回错误: %s", payload.Error.Message)
	}

	chunk := llm.StreamChunk{}
	if len(payload.Choices) > 0 {
		chunk.Content = payload.Choices[0].Delta.Content
	}
	if payload.Usage != nil {
		chunk.Usage = &llm.Usage{
			InputTokens:  payload.Usage.PromptTokens,
			OutputTokens: payload.Usage.CompletionTokens,
		}
	}
	return chunk, false, nil
}

func sendChunk(ctx context.Context, output chan<- llm.StreamChunk, chunk llm.StreamChunk) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case output <- chunk:
		return nil
	}
}
