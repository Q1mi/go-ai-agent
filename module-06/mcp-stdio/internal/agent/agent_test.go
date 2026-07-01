package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/q1mi/mcptools/internal/llm"
	"github.com/q1mi/mcptools/internal/tool"
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
	resp := provider.calls[provider.idx]
	provider.idx++
	return &resp, nil
}
func (provider *fakeProvider) ChatStream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, nil
}

func TestAgentRunUsesTool(t *testing.T) {
	provider := &fakeProvider{calls: []llm.ChatResponse{
		{ToolCalls: []llm.ToolCall{{ID: "call_1", Name: "echo", Args: json.RawMessage(`{"text":"hi"}`)}}},
		{Content: "完成"},
	}}
	registry := tool.NewRegistry(tool.NewTypedTool("echo", "回显", func(_ context.Context, args struct {
		Text string `json:"text"`
	}) (string, error) {
		return args.Text, nil
	}))
	answer, err := New(provider, "fake", registry).Run(context.Background(), "run")
	if err != nil {
		t.Fatal(err)
	}
	if answer != "完成" {
		t.Fatalf("answer = %q", answer)
	}
}
