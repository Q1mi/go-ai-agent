package review

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/q1mi/reviewagent/internal/llm"
)

type fakeProvider struct {
	mu    sync.Mutex
	calls []string
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
	system := req.Messages[0].Content
	user := req.Messages[len(req.Messages)-1].Content
	provider.mu.Lock()
	provider.calls = append(provider.calls, system)
	provider.mu.Unlock()

	switch {
	case strings.Contains(system, "意图分类器"):
		if !LooksLikeGoCode(user) {
			return &llm.ChatResponse{Content: `{"route":"general"}`}, nil
		}
		return &llm.ChatResponse{Content: `{"route":"code_review"}`}, nil
	case strings.Contains(system, "Planner"):
		return &llm.ChatResponse{Content: `{"dimensions":["正确性","错误处理"]}`}, nil
	case strings.Contains(system, "只审查维度：正确性"):
		return &llm.ChatResponse{Content: `{"dimension":"正确性","severity":"medium","location":"divide","problem":"整数除法可能除以 0","evidence":"return a / b 未检查 b","suggestion":"在除法前检查 b == 0 并返回错误"}`}, nil
	case strings.Contains(system, "只审查维度：错误处理"):
		return &llm.ChatResponse{Content: `{"dimension":"错误处理","severity":"medium","location":"divide","problem":"函数无法表达失败","evidence":"签名只返回 int","suggestion":"改为 (int, error)"}`}, nil
	case strings.Contains(system, "Evaluator"):
		return &llm.ChatResponse{Content: `{"pass":true,"score":92,"feedback":"覆盖充分"}`}, nil
	case strings.Contains(system, "课程助手"):
		return &llm.ChatResponse{Content: "普通回答"}, nil
	default:
		return nil, errors.New("unexpected prompt")
	}
}
func (provider *fakeProvider) ChatStream(context.Context, llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, errors.New("unused")
}

func TestReview(t *testing.T) {
	provider := &fakeProvider{}
	reviewer := Reviewer{Provider: provider, Model: "fake", MaxRounds: 2}
	report, err := reviewer.Review(context.Background(), `package main

func divide(a, b int) int {
	return a / b
}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Plan.Dimensions) != 2 {
		t.Fatalf("dimensions = %#v", report.Plan.Dimensions)
	}
	if len(report.Findings) != 2 {
		t.Fatalf("findings = %#v", report.Findings)
	}
	if !report.Evaluation.Pass || report.Rounds != 1 {
		t.Fatalf("evaluation=%+v rounds=%d", report.Evaluation, report.Rounds)
	}
}

func TestAnswerOrReviewRoutesCode(t *testing.T) {
	provider := &fakeProvider{}
	reviewer := Reviewer{Provider: provider, Model: "fake", OutputFormat: "json"}
	out, err := reviewer.AnswerOrReview(context.Background(), `func divide(a, b int) int { return a / b }`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"findings"`) {
		t.Fatalf("out = %s", out)
	}
}

func TestAnswerOrReviewRoutesGeneral(t *testing.T) {
	provider := &fakeProvider{}
	reviewer := Reviewer{Provider: provider, Model: "fake"}
	out, err := reviewer.AnswerOrReview(context.Background(), "请解释三 Agent 范式")
	if err != nil {
		t.Fatal(err)
	}
	if out != "普通回答" {
		t.Fatalf("out = %q", out)
	}
}

func TestLooksLikeGoCode(t *testing.T) {
	if !LooksLikeGoCode("func main() {}") {
		t.Fatal("expected Go code")
	}
	if LooksLikeGoCode("请解释一下三 Agent 范式") {
		t.Fatal("expected non-code")
	}
}
