package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/q1mi/llmrouter/internal/llm"
)

var errStreamDone = errors.New("stream done")

func parseStreamEvent(
	data []byte,
	usage *llm.Usage,
) (llm.StreamChunk, bool, error) {
	var envelope struct {
		Type    string `json:"type"`
		Message *struct {
			Usage struct {
				InputTokens int `json:"input_tokens"`
			} `json:"usage"`
		} `json:"message"`
		Delta struct {
			Type       string `json:"type"`
			Text       string `json:"text"`
			StopReason string `json:"stop_reason"`
		} `json:"delta"`
		Usage *struct {
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return llm.StreamChunk{}, false, fmt.Errorf("解析 Claude 流事件: %w", err)
	}

	switch envelope.Type {
	case "message_start":
		if envelope.Message != nil {
			usage.InputTokens = envelope.Message.Usage.InputTokens
		}
	case "content_block_delta":
		if envelope.Delta.Type == "text_delta" {
			return llm.StreamChunk{Content: envelope.Delta.Text}, false, nil
		}
	case "message_delta":
		if envelope.Usage != nil {
			usage.OutputTokens = envelope.Usage.OutputTokens
		}
	case "message_stop":
		return llm.StreamChunk{}, true, nil
	case "error":
		message := "未知错误"
		if envelope.Error != nil && envelope.Error.Message != "" {
			message = envelope.Error.Message
		}
		return llm.StreamChunk{}, false, fmt.Errorf("Claude 流事件返回错误: %s", message)
	}
	return llm.StreamChunk{}, false, nil
}

func sendChunk(ctx context.Context, output chan<- llm.StreamChunk, chunk llm.StreamChunk) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case output <- chunk:
		return nil
	}
}
