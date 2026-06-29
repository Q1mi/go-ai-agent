package agent

// EventType 表示 Agent 运行过程中对外发送的事件类型。
type EventType string

const (
	// EventThought 表示模型给出的中间思考文本。
	EventThought EventType = "thought"
	// EventToolCall 表示 Agent 即将调用工具。
	EventToolCall EventType = "tool_call"
	// EventToolResult 表示工具调用完成并返回观察结果。
	EventToolResult EventType = "tool_result"
	// EventAnswerDelta 表示 Agent 产出给用户的回答文本。
	EventAnswerDelta EventType = "answer_delta"
	// EventError 表示本轮执行失败。
	EventError EventType = "error"
	// EventDone 表示本轮执行完成。
	EventDone EventType = "done"
)

// AgentEvent 对应课件 4.8 的事件流载体。
// CLI、WebSocket、SSE 都可以消费同一组事件。
type AgentEvent struct {
	Type EventType `json:"type"`
	Text string    `json:"text,omitempty"`
	Tool string    `json:"tool,omitempty"`
	Args string    `json:"args,omitempty"`
	Step int       `json:"step,omitempty"`
}
