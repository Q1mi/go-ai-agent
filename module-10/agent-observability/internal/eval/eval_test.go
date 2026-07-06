package eval

import (
	"context"
	"testing"

	"github.com/q1mi/traceagent/internal/agent"
	"github.com/q1mi/traceagent/internal/obs"
)

func TestContainsAll(t *testing.T) {
	score, err := ContainsAll{}.Evaluate(context.Background(),
		Sample{Keywords: []string{"北京", "带伞"}},
		"北京今天小雨，建议带伞。",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !score.Pass || score.Value != 1 {
		t.Fatalf("score=%+v", score)
	}
}

func TestJudgeTrajectory(t *testing.T) {
	result := agent.Result{
		Answer: "北京小雨，建议带伞。",
		Steps:  3,
		ToolCalls: []agent.ToolCall{
			{Name: "get_weather"},
		},
	}
	spans := []obs.SpanRecord{
		{Name: "chat test-model"},
		{Name: "execute_tool get_weather"},
	}
	score := JudgeTrajectory(Sample{
		Keywords:      []string{"北京", "带伞"},
		RequiredTools: []string{"get_weather"},
	}, result, spans)
	if score.Average() != 1 {
		t.Fatalf("trajectory score=%+v", score)
	}
}
