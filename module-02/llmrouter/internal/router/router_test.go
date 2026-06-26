package router

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/q1mi/llmrouter/internal/cost"
	"github.com/q1mi/llmrouter/internal/llm"
)

type fakeProvider struct {
	name         string
	defaultModel string
	err          error
	delay        time.Duration
	seenModel    string
}

func (provider *fakeProvider) Name() string {
	return provider.name
}

func (provider *fakeProvider) DefaultModel() string {
	return provider.defaultModel
}

func (provider *fakeProvider) Capabilities() llm.Capability {
	return llm.Capability{Streaming: true}
}

func (provider *fakeProvider) Chat(
	ctx context.Context,
	request llm.ChatRequest,
) (*llm.ChatResponse, error) {
	provider.seenModel = request.Model
	if provider.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(provider.delay):
		}
	}
	if provider.err != nil {
		return nil, provider.err
	}
	model, err := llm.EffectiveModel(request.Model, provider.defaultModel)
	if err != nil {
		return nil, err
	}
	return &llm.ChatResponse{
		Content: "ok",
		Model:   model,
		Usage:   llm.Usage{InputTokens: 1, OutputTokens: 1},
	}, nil
}

func (provider *fakeProvider) ChatStream(
	context.Context,
	llm.ChatRequest,
) (<-chan llm.StreamChunk, error) {
	if provider.err != nil {
		return nil, provider.err
	}
	output := make(chan llm.StreamChunk, 2)
	output <- llm.StreamChunk{Content: "ok"}
	output <- llm.StreamChunk{Done: true}
	close(output)
	return output, nil
}

func TestRouterFailoverUsesProviderDefaultModel(t *testing.T) {
	first := &fakeProvider{name: "first", defaultModel: "model-a", err: errors.New("down")}
	second := &fakeProvider{name: "second", defaultModel: "model-b"}
	router, err := New(
		Priority{},
		Candidate{Provider: first},
		Candidate{Provider: second},
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := router.Chat(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Provider != "second" || result.Response.Model != "model-b" {
		t.Fatalf("result = %+v", result)
	}
	if first.seenModel != "" || second.seenModel != "" {
		t.Fatalf("router 不应写入统一模型名: first=%q second=%q", first.seenModel, second.seenModel)
	}
}

func TestCheapestFirst(t *testing.T) {
	expensive := &fakeProvider{name: "expensive", defaultModel: "a"}
	cheap := &fakeProvider{name: "cheap", defaultModel: "b"}
	candidates := []Candidate{
		{
			Provider: expensive,
			Pricing:  cost.Pricing{InputPer1M: 10, OutputPer1M: 20},
		},
		{
			Provider: cheap,
			Pricing:  cost.Pricing{InputPer1M: 1, OutputPer1M: 2},
		},
	}
	ordered := (CheapestFirst{}).Order(candidates, nil)
	if ordered[0].Provider.Name() != "cheap" {
		t.Fatalf("first = %s", ordered[0].Provider.Name())
	}
	if candidates[0].Provider.Name() != "expensive" {
		t.Fatal("策略不应修改原 slice")
	}
}

func TestRouterStreamFallsBackBeforeOutput(t *testing.T) {
	first := &fakeProvider{name: "first", defaultModel: "a", err: errors.New("down")}
	second := &fakeProvider{name: "second", defaultModel: "b"}
	modelRouter, err := New(
		Priority{},
		Candidate{Provider: first},
		Candidate{Provider: second},
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := modelRouter.ChatStream(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Provider != "second" {
		t.Fatalf("Provider = %q", result.Provider)
	}
	var content string
	for chunk := range result.Chunks {
		if chunk.Err != nil {
			t.Fatal(chunk.Err)
		}
		content += chunk.Content
	}
	if content != "ok" {
		t.Fatalf("content = %q", content)
	}
}

func TestLowestLatencyUsesStats(t *testing.T) {
	slow := &fakeProvider{name: "slow", defaultModel: "a"}
	fast := &fakeProvider{name: "fast", defaultModel: "b"}
	ordered := (LowestLatency{}).Order(
		[]Candidate{{Provider: slow}, {Provider: fast}},
		map[string]Stats{
			"slow": {Count: 3, P50: 200 * time.Millisecond},
			"fast": {Count: 3, P50: 20 * time.Millisecond},
		},
	)
	if ordered[0].Provider.Name() != "fast" {
		t.Fatalf("first = %s", ordered[0].Provider.Name())
	}
}

func TestCalculateStats(t *testing.T) {
	stats := calculateStats([]time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		100 * time.Millisecond,
	})
	if stats.P50 != 30*time.Millisecond || stats.P95 != 100*time.Millisecond {
		t.Fatalf("stats = %+v", stats)
	}
}
