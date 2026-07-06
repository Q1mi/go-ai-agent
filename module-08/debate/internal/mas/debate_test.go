package mas

import (
	"context"
	"strings"
	"testing"

	"github.com/q1mi/debate/internal/llm"
	"github.com/q1mi/debate/internal/providers/offline"
)

func TestDebateWithTranscript(t *testing.T) {
	ctx := context.Background()
	meter := llm.NewMeter(offline.New())
	debaters := []Debater{
		{Name: "务实派", Provider: meter, Persona: "你务实，关注落地速度。"},
		{Name: "谨慎派", Provider: meter, Persona: "你谨慎，关注长期风险。"},
		{Name: "数据派", Provider: meter, Persona: "你重证据，关注指标。"},
	}

	transcript, err := DebateWithTranscript(ctx, debaters, "是否要先做单体架构？", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(transcript.Rounds) != 2 {
		t.Fatalf("rounds=%d, want 2", len(transcript.Rounds))
	}
	for _, round := range transcript.Rounds {
		if len(round.Answers) != len(debaters) {
			t.Fatalf("round %d answers=%d, want %d", round.Number, len(round.Answers), len(debaters))
		}
	}
	if len(transcript.FinalAnswers) != len(debaters) {
		t.Fatalf("final answers=%d, want %d", len(transcript.FinalAnswers), len(debaters))
	}
	calls, usage := meter.Snapshot()
	if calls != 6 {
		t.Fatalf("calls=%d, want 6", calls)
	}
	if usage.TotalTokens == 0 {
		t.Fatalf("usage should be recorded")
	}
}

func TestJudgeAndBaseline(t *testing.T) {
	ctx := context.Background()
	provider := llm.NewMeter(offline.New())
	answers := map[string]string{
		"务实派": "关注交付速度。",
		"谨慎派": "关注长期维护。",
		"数据派": "关注指标验证。",
	}
	final, err := Judge(ctx, provider, "", "是否要先做单体架构？", answers)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(final, "评审定稿") {
		t.Fatalf("final answer should contain judge marker, got %q", final)
	}
	base, err := Baseline(ctx, provider, "", "是否要先做单体架构？")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(base, "单 Agent") {
		t.Fatalf("baseline answer should contain marker, got %q", base)
	}
}

func TestDebateValidation(t *testing.T) {
	_, err := Debate(context.Background(), nil, "问题", 1)
	if err == nil {
		t.Fatalf("expected validation error")
	}
}
