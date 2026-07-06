package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Role 表示 Chat Completions 消息角色。
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message 是 Provider 无关的消息结构。
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// Usage 记录一次模型调用的 token 消耗。
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Add 累加 token 消耗。
func (usage Usage) Add(other Usage) Usage {
	return Usage{
		PromptTokens:     usage.PromptTokens + other.PromptTokens,
		CompletionTokens: usage.CompletionTokens + other.CompletionTokens,
		TotalTokens:      usage.TotalTokens + other.TotalTokens,
	}
}

// ChatRequest 是一次模型请求。
type ChatRequest struct {
	Model    string    `json:"model,omitempty"`
	Messages []Message `json:"messages"`
}

// ChatResponse 是模型响应。
type ChatResponse struct {
	Content string `json:"content"`
	Usage   Usage  `json:"usage"`
}

// Provider 是多智能体系统依赖的大模型接口。
type Provider interface {
	Name() string
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

// ValidateRequest 检查调用请求。
func ValidateRequest(req ChatRequest) error {
	if len(req.Messages) == 0 {
		return errors.New("messages 不能为空")
	}
	for i, msg := range req.Messages {
		switch msg.Role {
		case RoleSystem, RoleUser, RoleAssistant:
		default:
			return fmt.Errorf("messages[%d] 使用未知 role %q", i, msg.Role)
		}
		if strings.TrimSpace(msg.Content) == "" {
			return fmt.Errorf("messages[%d].content 不能为空", i)
		}
	}
	return nil
}

// EstimateTokens 用字符数估算 token，供离线 provider 和缺少 usage 的 API 使用。
func EstimateTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	runes := len([]rune(text))
	tokens := runes / 2
	if tokens == 0 {
		tokens = 1
	}
	return tokens
}

// EstimatePromptTokens 估算一组消息的输入 token。
func EstimatePromptTokens(messages []Message) int {
	total := 0
	for _, msg := range messages {
		total += EstimateTokens(string(msg.Role)) + EstimateTokens(msg.Content)
	}
	return total
}

// Meter 包装 Provider，统计调用次数和 token 消耗。
type Meter struct {
	provider Provider
	mu       sync.Mutex
	calls    int
	usage    Usage
}

// NewMeter 创建带统计能力的 Provider。
func NewMeter(provider Provider) *Meter {
	return &Meter{provider: provider}
}

// Name 返回内部 Provider 名称。
func (meter *Meter) Name() string {
	if meter == nil || meter.provider == nil {
		return "nil"
	}
	return meter.provider.Name()
}

// Chat 调用内部 Provider 并记录消耗。
func (meter *Meter) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if meter == nil || meter.provider == nil {
		return nil, errors.New("meter 内部 provider 为空")
	}
	resp, err := meter.provider.Chat(ctx, req)
	if err != nil {
		return nil, err
	}
	usage := resp.Usage
	if usage.TotalTokens == 0 {
		usage.PromptTokens = EstimatePromptTokens(req.Messages)
		usage.CompletionTokens = EstimateTokens(resp.Content)
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
		resp.Usage = usage
	}
	meter.mu.Lock()
	meter.calls++
	meter.usage = meter.usage.Add(usage)
	meter.mu.Unlock()
	return resp, nil
}

// Snapshot 返回当前统计快照。
func (meter *Meter) Snapshot() (int, Usage) {
	if meter == nil {
		return 0, Usage{}
	}
	meter.mu.Lock()
	defer meter.mu.Unlock()
	return meter.calls, meter.usage
}
