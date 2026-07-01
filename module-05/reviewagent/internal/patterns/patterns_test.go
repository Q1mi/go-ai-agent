package patterns

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/q1mi/reviewagent/internal/llm"
)

type fakeProvider struct {
	mu sync.Mutex
	fn func(llm.ChatRequest) (*llm.ChatResponse, error)
}

func (provider *fakeProvider) Name() string { return "fake" }
func (provider *fakeProvider) DefaultModel() string {
	return "fake-model"
}
func (provider *fakeProvider) Capabilities() llm.Capability {
	return llm.Capability{}
}
func (provider *fakeProvider) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if err := llm.ValidateRequest(req); err != nil {
		return nil, err
	}
	provider.mu.Lock()
	defer provider.mu.Unlock()
	return provider.fn(req)
}
func (provider *fakeProvider) ChatStream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, errors.New("unused")
}

func TestRunChain(t *testing.T) {
	provider := &fakeProvider{fn: func(req llm.ChatRequest) (*llm.ChatResponse, error) {
		user := req.Messages[len(req.Messages)-1].Content
		return &llm.ChatResponse{Content: user + "!"}, nil
	}}
	out, err := RunChain(context.Background(), provider, "fake", "a", []ChainStep{
		{Name: "first", System: "s", Build: func(prev string) string { return prev + "b" }},
		{Name: "second", System: "s", Build: func(prev string) string { return prev + "c" }},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != "ab!c!" {
		t.Fatalf("out = %q", out)
	}
}

func TestIntentRouterDispatch(t *testing.T) {
	provider := &fakeProvider{fn: func(req llm.ChatRequest) (*llm.ChatResponse, error) {
		return &llm.ChatResponse{Content: `{"route":"code_review"}`}, nil
	}}
	router := IntentRouter{
		Provider: provider,
		Routes: []Route{{
			Name:        "code_review",
			Description: "审查代码",
			Handle: func(context.Context, string) (string, error) {
				return "reviewed", nil
			},
		}},
	}
	out, err := router.Dispatch(context.Background(), "func main(){}")
	if err != nil {
		t.Fatal(err)
	}
	if out != "reviewed" {
		t.Fatalf("out = %q", out)
	}
}

func TestSectioningPreservesOrder(t *testing.T) {
	results, err := Sectioning(context.Background(), []func(context.Context) (string, error){
		func(context.Context) (string, error) { return "a", nil },
		func(context.Context) (string, error) { return "b", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(results, "") != "ab" {
		t.Fatalf("results = %#v", results)
	}
}

func TestMajority(t *testing.T) {
	got := Majority([]string{"yes", "no", "yes"})
	if got != "yes" {
		t.Fatalf("Majority = %q", got)
	}
}

func TestEvaluatorOptimizer(t *testing.T) {
	round := 0
	out, ev, err := EvaluatorOptimizer(
		context.Background(),
		func(context.Context, string) (string, error) {
			round++
			return "draft", nil
		},
		func(context.Context, string) (Evaluation, error) {
			return Evaluation{Pass: round == 2, Score: 80, Feedback: "补充细节"}, nil
		},
		2,
	)
	if err != nil {
		t.Fatal(err)
	}
	if out != "draft" || !ev.Pass || round != 2 {
		t.Fatalf("out=%q ev=%+v round=%d", out, ev, round)
	}
}
