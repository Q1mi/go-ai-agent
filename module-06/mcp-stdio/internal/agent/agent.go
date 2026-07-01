package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/q1mi/mcptools/internal/llm"
	"github.com/q1mi/mcptools/internal/tool"
)

const defaultSystemPrompt = "你是一个命令行 AI 助手。需要查询时间、计算或使用外部能力时，请调用可用工具。"

// EventType 表示 Agent 运行事件类型。
type EventType string

const (
	EventThought     EventType = "thought"
	EventToolCall    EventType = "tool_call"
	EventToolResult  EventType = "tool_result"
	EventAnswerDelta EventType = "answer_delta"
	EventError       EventType = "error"
	EventDone        EventType = "done"
)

// Event 是 Agent 对外输出的执行轨迹。
type Event struct {
	Type EventType
	Text string
	Tool string
	Args string
}

// Agent 是最小 Function Calling Agent。
type Agent struct {
	provider     llm.Provider
	model        string
	tools        *tool.Registry
	systemPrompt string
	maxSteps     int
}

// Option 配置 Agent。
type Option func(*Agent)

// WithSystemPrompt 设置系统提示词。
func WithSystemPrompt(prompt string) Option {
	return func(agent *Agent) { agent.systemPrompt = prompt }
}

// WithMaxSteps 设置最大模型调用轮数。
func WithMaxSteps(maxSteps int) Option {
	return func(agent *Agent) { agent.maxSteps = maxSteps }
}

// New 创建 Agent。
func New(provider llm.Provider, model string, registry *tool.Registry, opts ...Option) *Agent {
	agent := &Agent{
		provider:     provider,
		model:        model,
		tools:        registry,
		systemPrompt: defaultSystemPrompt,
		maxSteps:     8,
	}
	for _, opt := range opts {
		opt(agent)
	}
	return agent
}

// Run 执行一次 Agent 任务，返回最终答案。
func (agent *Agent) Run(ctx context.Context, goal string) (string, error) {
	var answer string
	for event := range agent.RunStream(ctx, goal) {
		switch event.Type {
		case EventAnswerDelta:
			answer += event.Text
		case EventError:
			return "", errors.New(event.Text)
		}
	}
	return strings.TrimSpace(answer), nil
}

// RunStream 执行任务并输出事件流。
func (agent *Agent) RunStream(ctx context.Context, goal string) <-chan Event {
	out := make(chan Event, 16)
	go func() {
		defer close(out)
		agent.run(ctx, goal, out)
	}()
	return out
}

func (agent *Agent) run(ctx context.Context, goal string, out chan<- Event) {
	emit := func(event Event) bool {
		select {
		case <-ctx.Done():
			return false
		case out <- event:
			return true
		}
	}
	if agent.provider == nil {
		emit(Event{Type: EventError, Text: "provider 不能为空"})
		return
	}
	if agent.tools == nil {
		agent.tools = tool.NewRegistry()
	}
	if !agent.provider.Capabilities().Tools {
		emit(Event{Type: EventError, Text: "当前 provider 未开启工具调用能力"})
		return
	}
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: agent.systemPrompt},
		{Role: llm.RoleUser, Content: strings.TrimSpace(goal)},
	}
	usage := llm.Usage{}
	for step := 0; step < agent.maxSteps; step++ {
		resp, err := agent.provider.Chat(ctx, llm.ChatRequest{
			Model:    agent.model,
			Messages: messages,
			Tools:    agent.tools.ToolDefs(),
		})
		if err != nil {
			emit(Event{Type: EventError, Text: err.Error()})
			return
		}
		usage.InputTokens += resp.InputTokens
		usage.OutputTokens += resp.OutputTokens

		if len(resp.ToolCalls) == 0 {
			answer := strings.TrimSpace(resp.Content)
			if answer == "" {
				emit(Event{Type: EventError, Text: "模型返回空响应：没有回答内容，也没有工具调用"})
				return
			}
			emit(Event{Type: EventAnswerDelta, Text: answer})
			emit(Event{Type: EventDone})
			return
		}

		if strings.TrimSpace(resp.Content) != "" {
			emit(Event{Type: EventThought, Text: resp.Content})
		}
		messages = append(messages, llm.Message{Role: llm.RoleAssistant, Content: resp.Content, ToolCalls: resp.ToolCalls})
		for _, call := range resp.ToolCalls {
			if err := ctx.Err(); err != nil {
				emit(Event{Type: EventError, Text: err.Error()})
				return
			}
			args := call.Args
			if len(args) == 0 {
				args = json.RawMessage(`{}`)
			}
			emit(Event{Type: EventToolCall, Tool: call.Name, Args: string(args)})
			observation := agent.callTool(ctx, call.Name, args)
			emit(Event{Type: EventToolResult, Tool: call.Name, Text: observation})
			messages = append(messages, llm.Message{Role: llm.RoleTool, ToolCallID: call.ID, Content: observation})
		}
	}
	emit(Event{Type: EventError, Text: fmt.Sprintf("达到最大步骤数 %d，提前终止", agent.maxSteps)})
	_ = usage
}

func (agent *Agent) callTool(ctx context.Context, name string, args json.RawMessage) string {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	item, ok := agent.tools.Get(name)
	if !ok {
		return fmt.Sprintf("错误：工具 %q 不存在", name)
	}
	out, err := item.Call(ctx, args)
	if err != nil {
		return "错误：工具执行失败：" + err.Error() + "\n" + out
	}
	return out
}
