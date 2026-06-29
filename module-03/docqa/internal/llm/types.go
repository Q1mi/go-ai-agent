package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Role 表示一条消息在对话协议中的角色。
//
// M03 会用它组织 system、user 和 assistant 消息，再交给从 M02 搬入的网关。
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message 是跨 Provider 的统一消息结构。
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// ChatRequest 是跨 Provider 的统一请求结构。
//
// Model 为空时，网关中的具体 Provider 会使用自己的 DefaultModel。
type ChatRequest struct {
	Model       string    `json:"model,omitempty"`
	Messages    []Message `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// Option 是 ChatRequest 的函数式配置项。
type Option func(*ChatRequest)

// NewChatRequest 创建一次模型请求，并复制 messages，避免调用方后续修改影响请求内容。
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

// WithTemperature 设置采样温度。
//
// 指针字段可以区分“没有设置”和“显式设置为 0”。
func WithTemperature(temperature float64) Option {
	return func(req *ChatRequest) { req.Temperature = &temperature }
}

// WithMaxTokens 设置最大输出 token。
func WithMaxTokens(maxTokens int) Option {
	return func(req *ChatRequest) { req.MaxTokens = &maxTokens }
}

// Usage 表示一次模型调用的 token 用量。
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// TotalTokens 返回输入和输出 token 总量。
func (usage Usage) TotalTokens() int {
	return usage.InputTokens + usage.OutputTokens
}

// ChatResponse 是跨 Provider 的统一响应结构。
type ChatResponse struct {
	Content      string
	Model        string
	FinishReason string
	Usage        Usage
}

// StreamChunk 是流式输出的统一增量结构。
//
// M03 文档问答命令当前使用非流式 Chat，但保留该类型可以保持 M02 Provider
// 接口的一致性。
type StreamChunk struct {
	Content string
	Usage   *Usage
	Done    bool
	Err     error
}

// Capability 描述 Provider 能力。
type Capability struct {
	Streaming bool
}

// Provider 是 M02 建立的大模型供应商统一抽象。
//
// M03 只依赖这个接口和 gateway.Gateway，让 prompt/context 代码避开厂商协议细节。
type Provider interface {
	Name() string
	DefaultModel() string
	Capabilities() Capability
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
}

// FormatMessages 把消息列表格式化为适合调试和 token 估算的文本。
func FormatMessages(messages []Message) string {
	var builder strings.Builder
	for _, message := range messages {
		if strings.TrimSpace(message.Content) == "" {
			continue
		}
		builder.WriteString(string(message.Role))
		builder.WriteString(": ")
		builder.WriteString(message.Content)
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

// ValidateRequest 校验模型请求中上层必须保证的字段。
func ValidateRequest(req ChatRequest) error {
	if len(req.Messages) == 0 {
		return errors.New("messages 不能为空")
	}
	for i, message := range req.Messages {
		switch message.Role {
		case RoleSystem, RoleUser, RoleAssistant, RoleTool:
		default:
			return fmt.Errorf("messages[%d] 使用未知 role %q", i, message.Role)
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

// EffectiveModel 返回请求模型名和 Provider 默认模型名中的有效值。
func EffectiveModel(requestModel, defaultModel string) (string, error) {
	if model := strings.TrimSpace(requestModel); model != "" {
		return model, nil
	}
	if model := strings.TrimSpace(defaultModel); model != "" {
		return model, nil
	}
	return "", errors.New("请求和 Provider 均未配置模型")
}
