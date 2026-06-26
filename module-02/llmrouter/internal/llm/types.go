package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	// Model 为空时由每个 Provider 使用自己的默认模型，便于跨 Provider 故障转移。
	Model       string    `json:"model,omitempty"`
	Messages    []Message `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

type Option func(*ChatRequest)

func NewChatRequest(model string, messages []Message, opts ...Option) ChatRequest {
	req := ChatRequest{
		Model:    model,
		Messages: append([]Message(nil), messages...),
	}
	for _, opt := range opts {
		opt(&req)
	}
	return req
}

func WithTemperature(temperature float64) Option {
	return func(req *ChatRequest) { req.Temperature = &temperature }
}

func WithMaxTokens(maxTokens int) Option {
	return func(req *ChatRequest) { req.MaxTokens = &maxTokens }
}

type Usage struct {
	InputTokens  int
	OutputTokens int
}

func (u Usage) TotalTokens() int {
	return u.InputTokens + u.OutputTokens
}

type ChatResponse struct {
	Content      string
	Model        string
	FinishReason string
	Usage        Usage
}

type StreamChunk struct {
	Content string
	Usage   *Usage
	Done    bool
	Err     error
}

type Capability struct {
	Streaming bool
	Thinking  bool
	Tools     bool
}

type Provider interface {
	Name() string
	DefaultModel() string
	Capabilities() Capability
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
}

func ValidateRequest(req ChatRequest) error {
	if len(req.Messages) == 0 {
		return errors.New("messages 不能为空")
	}
	for i, message := range req.Messages {
		switch message.Role {
		case RoleSystem, RoleUser, RoleAssistant, RoleTool:
		default:
			return fmt.Errorf("messages[%d] 使用了未知 role %q", i, message.Role)
		}
		if strings.TrimSpace(message.Content) == "" {
			return fmt.Errorf("messages[%d].content 不能为空", i)
		}
	}
	if req.Temperature != nil && (*req.Temperature < 0 || *req.Temperature > 2) {
		return fmt.Errorf("temperature 必须在 [0, 2] 范围内")
	}
	if req.MaxTokens != nil && *req.MaxTokens <= 0 {
		return errors.New("max_tokens 必须大于 0")
	}
	return nil
}

func EffectiveModel(requestModel, defaultModel string) (string, error) {
	if model := strings.TrimSpace(requestModel); model != "" {
		return model, nil
	}
	if model := strings.TrimSpace(defaultModel); model != "" {
		return model, nil
	}
	return "", errors.New("请求和 Provider 均未配置模型")
}
