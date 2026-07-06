package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Role 表示模型消息角色。
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message 是本练习使用的最小对话消息。
type Message struct {
	Role    Role   `json:"role"`
	Name    string `json:"name,omitempty"`
	Content string `json:"content"`
}

// Usage 记录 token 用量。
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Total 返回总 token。
func (usage Usage) Total() int {
	return usage.InputTokens + usage.OutputTokens
}

// ChatRequest 是一次模型调用。
type ChatRequest struct {
	Model       string    `json:"model,omitempty"`
	Messages    []Message `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
}

// ChatResponse 是模型响应。
type ChatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Content string `json:"content"`
	Usage   Usage  `json:"usage"`
}

// Provider 是 Agent 依赖的大模型接口。
type Provider interface {
	Name() string
	DefaultModel() string
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}

// ValidateRequest 校验一次模型请求。
func ValidateRequest(req ChatRequest) error {
	if len(req.Messages) == 0 {
		return fmt.Errorf("messages 不能为空")
	}
	for i, msg := range req.Messages {
		switch msg.Role {
		case RoleSystem, RoleUser, RoleAssistant:
		case RoleTool:
			return fmt.Errorf("messages[%d] 使用 role=tool，但本练习的 OpenAI 兼容请求没有 tool_call_id，请把工具结果包装成普通上下文消息", i)
		default:
			return fmt.Errorf("messages[%d] 使用未知 role %q", i, msg.Role)
		}
		if strings.TrimSpace(msg.Content) == "" {
			return fmt.Errorf("messages[%d].content 不能为空", i)
		}
	}
	if req.Temperature != nil && (*req.Temperature < 0 || *req.Temperature > 2) {
		return fmt.Errorf("temperature 必须在 [0, 2] 范围内")
	}
	if req.MaxTokens != nil && *req.MaxTokens <= 0 {
		return fmt.Errorf("max_tokens 必须大于 0")
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
	return "", fmt.Errorf("请求和 Provider 均未配置模型")
}

// Ptr 返回值的指针，便于设置可选参数。
func Ptr[T any](v T) *T { return &v }

// EstimateTokens 是演示用低成本 token 估算。
func EstimateTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	ascii, cjk := 0, 0
	for _, r := range text {
		if r < 128 {
			ascii++
			continue
		}
		cjk++
	}
	return ascii/4 + cjk*2/3 + 1
}

// CountMessages 估算消息列表 token。
func CountMessages(messages []Message) int {
	total := 0
	for _, msg := range messages {
		total += EstimateTokens(string(msg.Role)) + EstimateTokens(msg.Name) + EstimateTokens(msg.Content)
	}
	return total
}

// FormatMessages 把消息列表格式化为调试文本。
func FormatMessages(messages []Message) string {
	var builder strings.Builder
	for _, msg := range messages {
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}
		builder.WriteString(string(msg.Role))
		if msg.Name != "" {
			builder.WriteString("(")
			builder.WriteString(msg.Name)
			builder.WriteString(")")
		}
		builder.WriteString(": ")
		builder.WriteString(msg.Content)
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

// ParseJSON 从模型文本中解析 JSON。
func ParseJSON[T any](text string) (T, error) {
	var zero T
	text = strings.TrimSpace(text)
	if text == "" {
		return zero, fmt.Errorf("empty json text")
	}
	if err := json.Unmarshal([]byte(text), &zero); err == nil {
		return zero, nil
	}
	re := regexp.MustCompile(`(?s)\{.*\}`)
	m := re.FindString(text)
	if m == "" {
		return zero, fmt.Errorf("json object not found: %s", text)
	}
	if err := json.Unmarshal([]byte(m), &zero); err != nil {
		return zero, err
	}
	return zero, nil
}
