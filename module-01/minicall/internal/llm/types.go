package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream"`
}

type ChatResponse struct {
	Content      string
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

type StreamChunk struct {
	Content string
	Err     error
}

// Provider 是模型供应商的统一抽象。本练习只使用 Chat，后续课程会实现 ChatStream。
type Provider interface {
	Name() string
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
}

type chatCompletionResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// DecodeChatResponse 解析 OpenAI 兼容的 Chat Completions 响应，并转成统一结构。
func DecodeChatResponse(r io.Reader) (*ChatResponse, error) {
	if r == nil {
		return nil, errors.New("响应体不能为空")
	}

	var wire chatCompletionResponse
	if err := json.NewDecoder(r).Decode(&wire); err != nil {
		return nil, fmt.Errorf("解析模型响应: %w", err)
	}
	if len(wire.Choices) == 0 {
		return nil, errors.New("模型响应中没有 choices")
	}

	return &ChatResponse{
		Content:      wire.Choices[0].Message.Content,
		InputTokens:  wire.Usage.PromptTokens,
		OutputTokens: wire.Usage.CompletionTokens,
		TotalTokens:  wire.Usage.TotalTokens,
	}, nil
}
