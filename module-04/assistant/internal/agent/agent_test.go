package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/q1mi/assistant/internal/llm"
	"github.com/q1mi/assistant/internal/schema"
	"github.com/q1mi/assistant/internal/tool"
)

type fakeProvider struct {
	calls []llm.ChatResponse
	idx   int
}

func (provider *fakeProvider) Name() string { return "fake" }
func (provider *fakeProvider) Capabilities() llm.Capability {
	return llm.Capability{Tools: true}
}
func (provider *fakeProvider) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if err := llm.ValidateRequest(req); err != nil {
		return nil, err
	}
	if provider.idx >= len(provider.calls) {
		return &llm.ChatResponse{Content: "done"}, nil
	}
	resp := provider.calls[provider.idx]
	provider.idx++
	return &resp, nil
}
func (provider *fakeProvider) ChatStream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, nil
}

func TestRunStreamFunctionCalling(t *testing.T) {
	provider := &fakeProvider{calls: []llm.ChatResponse{
		{
			Content: "需要调用工具",
			ToolCalls: []llm.ToolCall{{
				ID:   "call_1",
				Name: "echo",
				Args: json.RawMessage(`{}`),
			}},
			InputTokens:  10,
			OutputTokens: 5,
		},
		{
			Content:      "完成",
			InputTokens:  8,
			OutputTokens: 2,
		},
	}}
	registry := tool.NewRegistry(testTool{})
	agent := New(provider, "fake", registry)

	var events []AgentEvent
	for event := range agent.RunStream(context.Background(), "run") {
		events = append(events, event)
	}
	if len(events) == 0 || events[len(events)-1].Type != EventDone {
		t.Fatalf("events = %+v", events)
	}
}

func TestRunStreamEmptyModelResponseDoesNotPoisonHistory(t *testing.T) {
	provider := &fakeProvider{calls: []llm.ChatResponse{
		{},
		{Content: "第二轮正常回答"},
	}}
	agent := New(provider, "fake", tool.NewRegistry())

	first := collectEvents(agent.RunStream(context.Background(), "你执行工具使用的是什么模式?"))
	if len(first) == 0 || first[len(first)-1].Type != EventError {
		t.Fatalf("first events = %+v, want final error event", first)
	}

	second := collectEvents(agent.RunStream(context.Background(), "为什么不回答我了"))
	if len(second) == 0 || second[len(second)-1].Type != EventDone {
		t.Fatalf("second events = %+v, want final done event", second)
	}
	if got := second[len(second)-2]; got.Type != EventAnswerDelta || got.Text != "第二轮正常回答" {
		t.Fatalf("answer event = %+v", got)
	}
}

func collectEvents(ch <-chan AgentEvent) []AgentEvent {
	var events []AgentEvent
	for event := range ch {
		events = append(events, event)
	}
	return events
}

type testTool struct{}

func (testTool) Name() string        { return "echo" }
func (testTool) Description() string { return "echo" }
func (testTool) Parameters() json.RawMessage {
	parameters, err := schema.Generate(struct{}{})
	if err != nil {
		panic(err)
	}
	return schema.MustJSON(parameters)
}
func (testTool) Call(context.Context, json.RawMessage) (string, error) {
	return "ok", nil
}
