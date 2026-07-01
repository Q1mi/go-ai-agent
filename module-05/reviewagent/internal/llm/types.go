package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Role 表示一条对话消息的角色。
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message 是跨 Provider 的统一消息结构。
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// ChatRequest 是一次模型请求。
//
// Model 为空时，Provider 使用自己的默认模型。
type ChatRequest struct {
	Model       string    `json:"model,omitempty"`
	Messages    []Message `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
}

// Option 是 ChatRequest 的函数式配置项。
type Option func(*ChatRequest)

// NewChatRequest 创建请求并复制 messages，避免调用方后续修改影响本次请求。
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
func WithTemperature(temperature float64) Option {
	return func(req *ChatRequest) { req.Temperature = &temperature }
}

// WithMaxTokens 设置最大输出 token。
func WithMaxTokens(maxTokens int) Option {
	return func(req *ChatRequest) { req.MaxTokens = &maxTokens }
}

// Usage 表示一次模型调用的 token 用量。
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// TotalTokens 返回输入与输出 token 总数。
func (usage Usage) TotalTokens() int {
	return usage.InputTokens + usage.OutputTokens
}

// ChatResponse 是 Provider 返回的统一响应。
type ChatResponse struct {
	Content      string `json:"content"`
	Model        string `json:"model,omitempty"`
	FinishReason string `json:"finish_reason,omitempty"`
	Usage        Usage  `json:"usage"`
}

// StreamChunk 保留流式接口的统一类型，M05 CLI 当前使用非流式 Chat。
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

// Provider 是 M05 模式代码依赖的大模型抽象。
type Provider interface {
	Name() string
	DefaultModel() string
	Capabilities() Capability
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
}

// ValidateRequest 校验一次模型请求中上层必须保证的字段。
func ValidateRequest(req ChatRequest) error {
	if len(req.Messages) == 0 {
		return errors.New("messages 不能为空")
	}
	for i, message := range req.Messages {
		switch message.Role {
		case RoleSystem, RoleUser, RoleAssistant:
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

// EffectiveModel 返回请求模型名和默认模型名中的有效值。
func EffectiveModel(requestModel, defaultModel string) (string, error) {
	if model := strings.TrimSpace(requestModel); model != "" {
		return model, nil
	}
	if model := strings.TrimSpace(defaultModel); model != "" {
		return model, nil
	}
	return "", errors.New("请求和 Provider 均未配置模型")
}

// ParseInto 从模型输出中解析 JSON 到指定结构。
//
// 模型偶尔会把 JSON 包进 Markdown 代码块，或者在 JSON 前后加说明文字。
// 这里先抽取第一个 JSON object/array，再交给 encoding/json 解析。
func ParseInto[T any](text string) (T, error) {
	var out T
	raw, err := ExtractJSON(text)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return out, fmt.Errorf("解析 JSON 失败: %w; 原始 JSON=%s", err, raw)
	}
	return out, nil
}

// ExtractJSON 从文本中提取第一个完整 JSON object 或 array。
func ExtractJSON(text string) (string, error) {
	text = strings.TrimSpace(stripCodeFence(text))
	if text == "" {
		return "", errors.New("输出为空，无法解析 JSON")
	}
	start := -1
	for i, r := range text {
		if r == '{' || r == '[' {
			start = i
			break
		}
	}
	if start < 0 {
		return "", fmt.Errorf("未找到 JSON 起始符: %q", text)
	}

	var stack []rune
	inString := false
	escaped := false
	for i, r := range text[start:] {
		pos := start + i
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch r {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch r {
		case '"':
			inString = true
		case '{', '[':
			stack = append(stack, r)
		case '}', ']':
			if len(stack) == 0 {
				return "", fmt.Errorf("JSON 结束符不匹配: %q", text)
			}
			open := stack[len(stack)-1]
			if (open == '{' && r != '}') || (open == '[' && r != ']') {
				return "", fmt.Errorf("JSON 结束符不匹配: %q", text)
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return text[start : pos+1], nil
			}
		}
	}
	return "", fmt.Errorf("JSON 未闭合: %q", text[start:])
}

func stripCodeFence(text string) string {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "```") {
		return text
	}
	lines := strings.Split(text, "\n")
	if len(lines) < 2 {
		return text
	}
	if strings.HasPrefix(strings.TrimSpace(lines[0]), "```") &&
		strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
		return strings.Join(lines[1:len(lines)-1], "\n")
	}
	return text
}
