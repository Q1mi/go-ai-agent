package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Role 表示对话消息角色。
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message 是 Provider 无关的消息结构。
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ChatRequest 是一次模型请求。
type ChatRequest struct {
	Model    string    `json:"model,omitempty"`
	Messages []Message `json:"messages"`
	Tools    []ToolDef `json:"tools,omitempty"`
}

// ToolDef 是模型可见的工具声明。
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ToolCall 是模型返回的工具调用意图。
type ToolCall struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// ChatResponse 是模型响应。
type ChatResponse struct {
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// Capability 描述 Provider 能力。
type Capability struct {
	Tools bool
}

// Provider 是 Agent 依赖的大模型接口。
type Provider interface {
	Name() string
	Capabilities() Capability
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

// ValidateRequest 校验上层请求。
func ValidateRequest(req ChatRequest) error {
	if len(req.Messages) == 0 {
		return errors.New("messages 不能为空")
	}
	for i, msg := range req.Messages {
		switch msg.Role {
		case RoleSystem, RoleUser, RoleAssistant, RoleTool:
		default:
			return fmt.Errorf("messages[%d] 使用未知 role %q", i, msg.Role)
		}
		if strings.TrimSpace(msg.Content) == "" && len(msg.ToolCalls) == 0 {
			return fmt.Errorf("messages[%d] 缺少 content 或 tool_calls", i)
		}
		if msg.Role == RoleTool && strings.TrimSpace(msg.ToolCallID) == "" {
			return fmt.Errorf("messages[%d] 是 tool role，但缺少 tool_call_id", i)
		}
	}
	for i, item := range req.Tools {
		if strings.TrimSpace(item.Name) == "" {
			return fmt.Errorf("tools[%d].name 不能为空", i)
		}
		if len(item.Parameters) > 0 && !json.Valid(item.Parameters) {
			return fmt.Errorf("tools[%d].parameters 不是合法 JSON", i)
		}
	}
	return nil
}
