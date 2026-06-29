package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/q1mi/assistant/internal/llm"
	"github.com/q1mi/assistant/internal/tool"
)

const defaultSystemPrompt = "你是一个命令行 AI 助手。需要真实计算或查询当前时间时，请调用工具。"

// Agent 对应课件 4.2 的 Agent 结构。
// provider 是模型出口，tools 是可用工具集合，budget 和 store 分别对应 4.6 与 4.10。
type Agent struct {
	provider     llm.Provider
	model        string
	tools        *tool.Registry
	systemPrompt string
	budget       Budget
	store        Store
	sessionID    string
	memory       *State
}

// Option 是 Agent 的函数式配置项。
type Option func(*Agent)

// WithSystemPrompt 设置 Agent 初始 System Prompt。
func WithSystemPrompt(prompt string) Option {
	return func(agent *Agent) {
		agent.systemPrompt = prompt
	}
}

// WithBudget 设置 Agent 单轮运行预算。
func WithBudget(budget Budget) Option {
	return func(agent *Agent) {
		agent.budget = budget
	}
}

// WithStore 设置会话持久化存储和会话 ID。
func WithStore(store Store, sessionID string) Option {
	return func(agent *Agent) {
		agent.store = store
		agent.sessionID = strings.TrimSpace(sessionID)
	}
}

// New 对应课件 4.2 的构造函数，沿用函数式选项模式。
func New(provider llm.Provider, model string, registry *tool.Registry, opts ...Option) *Agent {
	agent := &Agent{
		provider:     provider,
		model:        model,
		tools:        registry,
		systemPrompt: defaultSystemPrompt,
		budget:       DefaultBudget(),
	}
	for _, opt := range opts {
		opt(agent)
	}
	return agent
}

// RunStream 对应课件 4.8。
// 它把 Agent 内部过程转换成事件流，调用方只需要 range channel 即可展示执行过程。
func (agent *Agent) RunStream(ctx context.Context, goal string) <-chan AgentEvent {
	out := make(chan AgentEvent, 16)
	go func() {
		defer close(out)
		agent.run(ctx, goal, out)
	}()
	return out
}

// run 根据 Provider 能力选择 Function Calling 或 ReAct 执行路径。
func (agent *Agent) run(ctx context.Context, goal string, out chan<- AgentEvent) {
	emit := func(event AgentEvent) bool {
		select {
		case <-ctx.Done():
			return false
		case out <- event:
			return true
		}
	}

	if agent.provider == nil {
		emit(AgentEvent{Type: EventError, Text: "provider 不能为空"})
		return
	}
	if agent.tools == nil {
		agent.tools = tool.NewRegistry()
	}
	state, err := agent.initialState(ctx, goal)
	if err != nil {
		emit(AgentEvent{Type: EventError, Text: err.Error()})
		return
	}

	if agent.provider.Capabilities().Tools {
		agent.runFunctionCalling(ctx, state, emit)
		return
	}
	agent.runReAct(ctx, state, emit)
}

// initialState 对应课件 4.2 和 4.10。
// 有 session 时先尝试加载历史；没有历史时从 system + user 两条消息开始。
func (agent *Agent) initialState(ctx context.Context, goal string) (*State, error) {
	now := time.Now()
	if agent.store != nil && agent.sessionID != "" {
		state, err := agent.store.Load(ctx, agent.sessionID)
		if err == nil {
			state.Goal = strings.TrimSpace(goal)
			state.Phase = PhaseThinking
			state.UpdatedAt = now
			state.Messages = dropEmptyAssistantMessages(state.Messages)
			state.Messages = append(state.Messages, llm.Message{Role: llm.RoleUser, Content: goal})
			return state, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	if agent.memory != nil {
		state := agent.memory
		state.Goal = strings.TrimSpace(goal)
		state.Phase = PhaseThinking
		state.UpdatedAt = now
		state.Messages = dropEmptyAssistantMessages(state.Messages)
		state.Messages = append(state.Messages, llm.Message{Role: llm.RoleUser, Content: goal})
		return state, nil
	}
	systemPrompt := agent.systemPrompt
	if !agent.provider.Capabilities().Tools {
		systemPrompt = buildReactSystemPrompt(agent.tools)
	}
	return &State{
		Goal:      strings.TrimSpace(goal),
		Phase:     PhaseThinking,
		StartedAt: now,
		UpdatedAt: now,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: systemPrompt},
			{Role: llm.RoleUser, Content: goal},
		},
		ActionCounts: make(map[string]int),
	}, nil
}

// runFunctionCalling 对应课件 4.5 的 Function Calling 循环。
// 循环结构保持“思考 -> 工具调用 -> 观察 -> 再思考”。
func (agent *Agent) runFunctionCalling(
	ctx context.Context,
	state *State,
	emit func(AgentEvent) bool,
) {
	for {
		if stop, reason := agent.budget.Exceeded(state); stop {
			agent.finishError(ctx, state, emit, "提前终止："+reason)
			return
		}

		resp, err := agent.provider.Chat(ctx, llm.ChatRequest{
			Model:    agent.model,
			Messages: state.Messages,
			Tools:    agent.toolDefs(),
		})
		if err != nil {
			agent.finishError(ctx, state, emit, err.Error())
			return
		}
		state.Step++
		state.Usage.InputTokens += resp.InputTokens
		state.Usage.OutputTokens += resp.OutputTokens
		state.UpdatedAt = time.Now()

		if len(resp.ToolCalls) == 0 {
			answer := strings.TrimSpace(resp.Content)
			if answer == "" {
				agent.finishError(ctx, state, emit, "模型返回空响应：没有回答内容，也没有工具调用")
				return
			}
			state.Phase = PhaseDone
			state.Answer = answer
			state.Messages = append(state.Messages, llm.Message{
				Role:    llm.RoleAssistant,
				Content: answer,
			})
			agent.checkpoint(ctx, state)
			emit(AgentEvent{Type: EventAnswerDelta, Text: answer, Step: state.Step})
			emit(AgentEvent{Type: EventDone, Step: state.Step})
			return
		}

		state.Phase = PhaseActing
		if strings.TrimSpace(resp.Content) != "" {
			emit(AgentEvent{Type: EventThought, Text: resp.Content, Step: state.Step})
		}
		state.Messages = append(state.Messages, llm.Message{
			Role:      llm.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		for _, call := range resp.ToolCalls {
			if !agent.beforeToolCall(ctx, state, call.Name, call.Args, emit) {
				return
			}
			observation := agent.callTool(ctx, call.Name, call.Args)
			emit(AgentEvent{Type: EventToolResult, Tool: call.Name, Text: observation, Step: state.Step})
			state.Messages = append(state.Messages, llm.Message{
				Role:       llm.RoleTool,
				ToolCallID: call.ID,
				Content:    observation,
			})
		}
		state.Phase = PhaseThinking
		agent.checkpoint(ctx, state)
	}
}

// runReAct 对应课件 4.4 的 ReAct 兜底循环。
// 面对不支持原生工具调用的 Provider，模型通过文本格式表达 Action。
func (agent *Agent) runReAct(
	ctx context.Context,
	state *State,
	emit func(AgentEvent) bool,
) {
	healAttempts := 0
	for {
		if stop, reason := agent.budget.Exceeded(state); stop {
			agent.finishError(ctx, state, emit, "提前终止："+reason)
			return
		}
		resp, err := agent.provider.Chat(ctx, llm.ChatRequest{
			Model:    agent.model,
			Messages: state.Messages,
			Stop:     []string{"Observation:"},
		})
		if err != nil {
			agent.finishError(ctx, state, emit, err.Error())
			return
		}
		state.Step++
		state.Usage.InputTokens += resp.InputTokens
		state.Usage.OutputTokens += resp.OutputTokens
		state.UpdatedAt = time.Now()

		step, err := parseReact(resp.Content)
		if err != nil {
			healAttempts++
			if healAttempts > agent.maxHealAttempts() {
				agent.finishError(ctx, state, emit, err.Error())
				return
			}
			if strings.TrimSpace(resp.Content) != "" {
				state.Messages = append(state.Messages, llm.Message{Role: llm.RoleAssistant, Content: resp.Content})
			}
			state.Messages = append(state.Messages, llm.Message{Role: llm.RoleUser, Content: "Observation: 错误：" + err.Error()})
			agent.checkpoint(ctx, state)
			continue
		}
		healAttempts = 0
		if step.FinalAnswer != "" {
			state.Phase = PhaseDone
			state.Answer = step.FinalAnswer
			state.Messages = append(state.Messages, llm.Message{Role: llm.RoleAssistant, Content: resp.Content})
			agent.checkpoint(ctx, state)
			emit(AgentEvent{Type: EventAnswerDelta, Text: step.FinalAnswer, Step: state.Step})
			emit(AgentEvent{Type: EventDone, Step: state.Step})
			return
		}
		if step.Thought != "" {
			emit(AgentEvent{Type: EventThought, Text: step.Thought, Step: state.Step})
		}
		rawArgs := json.RawMessage(step.ActionInput)
		if !agent.beforeToolCall(ctx, state, step.Action, rawArgs, emit) {
			return
		}
		observation := agent.callTool(ctx, step.Action, rawArgs)
		emit(AgentEvent{Type: EventToolResult, Tool: step.Action, Text: observation, Step: state.Step})
		state.Messages = append(
			state.Messages,
			llm.Message{Role: llm.RoleAssistant, Content: resp.Content},
			llm.Message{Role: llm.RoleUser, Content: "Observation: " + observation},
		)
		agent.checkpoint(ctx, state)
	}
}

// toolDefs 对应课件 4.5 的工具定义转换函数。
// 它把本地 tool.Registry 转成模型请求中的 []llm.ToolDef。
func (agent *Agent) toolDefs() []llm.ToolDef {
	if agent.tools == nil {
		return nil
	}
	return agent.tools.ToolDefs()
}

// beforeToolCall 在真正执行工具前做取消检查、重复动作保护和事件发送。
func (agent *Agent) beforeToolCall(
	ctx context.Context,
	state *State,
	name string,
	args json.RawMessage,
	emit func(AgentEvent) bool,
) bool {
	if err := ctx.Err(); err != nil {
		agent.finishError(ctx, state, emit, err.Error())
		return false
	}
	signature := actionSignature(name, args)
	if state.ActionCounts == nil {
		state.ActionCounts = make(map[string]int)
	}
	state.ActionCounts[signature]++
	if agent.budget.MaxSameAction > 0 && state.ActionCounts[signature] > agent.budget.MaxSameAction {
		agent.finishError(ctx, state, emit, fmt.Sprintf("重复动作过多：%s", signature))
		return false
	}
	emit(AgentEvent{Type: EventToolCall, Tool: name, Args: string(args), Step: state.Step})
	return true
}

// callTool 对应课件 4.4 / 4.5 的工具执行逻辑。
// 可恢复错误会被转换为 Observation 文本，交给下一轮模型决策处理。
func (agent *Agent) callTool(ctx context.Context, name string, args json.RawMessage) string {
	item, ok := agent.tools.Get(name)
	if !ok {
		return fmt.Sprintf("错误：工具 %q 不存在", name)
	}
	out, err := item.Call(ctx, args)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "错误：工具执行被取消：" + err.Error()
		}
		return "错误：工具执行失败：" + err.Error()
	}
	return out
}

// finishError 收束错误状态，保存快照，并向调用方发送错误事件。
func (agent *Agent) finishError(
	ctx context.Context,
	state *State,
	emit func(AgentEvent) bool,
	text string,
) {
	state.Phase = PhaseError
	state.UpdatedAt = time.Now()
	agent.checkpoint(ctx, state)
	emit(AgentEvent{Type: EventError, Text: text, Step: state.Step})
}

// checkpoint 对应课件 4.10 的“每轮保存状态”。
func (agent *Agent) checkpoint(ctx context.Context, state *State) {
	state.Messages = dropEmptyAssistantMessages(state.Messages)
	agent.memory = state
	if agent.store == nil || agent.sessionID == "" {
		return
	}
	_ = agent.store.Save(ctx, agent.sessionID, state)
}

// maxHealAttempts 返回 ReAct 输出格式自愈的最大尝试次数。
func (agent *Agent) maxHealAttempts() int {
	if agent.budget.MaxHealAttempts <= 0 {
		return 3
	}
	return agent.budget.MaxHealAttempts
}

// dropEmptyAssistantMessages 清理历史中没有任何有效载荷的 assistant 消息。
// 这类消息无法被 OpenAI 兼容协议接受，常见来源是上游模型返回空响应。
func dropEmptyAssistantMessages(messages []llm.Message) []llm.Message {
	if len(messages) == 0 {
		return messages
	}
	out := messages[:0]
	for _, message := range messages {
		if message.Role == llm.RoleAssistant &&
			strings.TrimSpace(message.Content) == "" &&
			len(message.ToolCalls) == 0 {
			continue
		}
		out = append(out, message)
	}
	return out
}
