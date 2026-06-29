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
	// RoleSystem 表示系统规则消息。
	RoleSystem Role = "system"
	// RoleUser 表示用户输入消息。
	RoleUser Role = "user"
	// RoleAssistant 表示模型回复消息。
	RoleAssistant Role = "assistant"
	// RoleTool 表示工具执行结果消息。
	RoleTool Role = "tool"
)

// Message 是 Agent 与模型之间传递的统一消息结构。
//
// M04 在 M02 对话消息基础上补充 ToolCalls 和 ToolCallID，用来表达工具调用链路。
type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ChatRequest 是 Agent 发给 Provider 的统一请求结构。
type ChatRequest struct {
	Model    string    `json:"model,omitempty"`
	Messages []Message `json:"messages"`
	Tools    []ToolDef `json:"tools,omitempty"`
	Stop     []string  `json:"stop,omitempty"`
}

// ToolDef 和 ToolCall 对应课件 4.5 新增的工具调用抽象。
// Provider 适配层负责把它们映射成 OpenAI、Claude、Gemini 等具体协议。
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ToolCall 表示模型请求 Agent 执行的一次工具调用。
type ToolCall struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// Usage 表示一次或多次模型调用的 token 用量。
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// TotalTokens 返回输入和输出 token 总量。
func (u Usage) TotalTokens() int {
	return u.InputTokens + u.OutputTokens
}

// ChatResponse 是 Provider 返回给 Agent 的统一响应结构。
//
// Content 用于普通回答或思考文本；ToolCalls 用于 Function Calling 模式下的工具请求。
type ChatResponse struct {
	Content      string     `json:"content,omitempty"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	InputTokens  int        `json:"input_tokens,omitempty"`
	OutputTokens int        `json:"output_tokens,omitempty"`
}

// StreamChunk 是流式响应中的增量片段。
type StreamChunk struct {
	Content string
	Err     error
	Done    bool
}

// Capability 描述 Provider 能力。
//
// Tools 为 true 时 Agent 走 Function Calling；Tools 为 false 时 Agent 走 ReAct。
type Capability struct {
	Streaming bool
	Tools     bool
}

// Provider 是 M04 Agent 核心依赖的大模型抽象。
type Provider interface {
	Name() string
	Capabilities() Capability
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
}

// ValidateRequest 校验一次模型请求的基础结构。
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
	return nil
}

// EstimateTokens 用简单启发式估算文本 token。
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	ascii, nonASCII := 0, 0
	for _, r := range text {
		if r < 128 {
			ascii++
		} else {
			nonASCII++
		}
	}
	return ascii/4 + nonASCII*2/3 + 1
}

// EstimateMessages 估算一组消息的 token 总量。
func EstimateMessages(messages []Message) int {
	total := 0
	for _, msg := range messages {
		total += EstimateTokens(string(msg.Role))
		total += EstimateTokens(msg.Content)
		for _, tc := range msg.ToolCalls {
			total += EstimateTokens(tc.Name)
			total += EstimateTokens(string(tc.Args))
		}
	}
	return total
}
