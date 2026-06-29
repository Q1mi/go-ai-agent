package contextpack

import (
	"strings"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{text: "", want: 0},
		{text: "Hello world", want: 3},
		{text: "你好世界", want: 3},
	}
	for _, tt := range tests {
		if got := EstimateTokens(tt.text); got != tt.want {
			t.Fatalf("EstimateTokens(%q) = %d, want %d", tt.text, got, tt.want)
		}
	}
}

func TestBuildDemoPlan(t *testing.T) {
	plan, err := BuildDemoPlan("示例网关", "如何修改默认超时？", "2026-06-26T10:30:00+08:00")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan.SystemPrompt, "你是 示例网关 的文档问答助手") {
		t.Fatalf("SystemPrompt = %s", plan.SystemPrompt)
	}
	if !strings.Contains(plan.SystemPrompt, `"level"`) ||
		!strings.Contains(plan.SystemPrompt, `"reason"`) {
		t.Fatalf("schema 未写入 system prompt: %s", plan.SystemPrompt)
	}
	if len(plan.Usages) != 5 {
		t.Fatalf("usages = %+v", plan.Usages)
	}
	if !strings.Contains(plan.Messages[len(plan.Messages)-1].Content, "当前时间") {
		t.Fatalf("last message = %+v", plan.Messages[len(plan.Messages)-1])
	}
}

func TestRenderSystemPromptMissingDataWouldFail(t *testing.T) {
	_, err := RenderSystemPrompt("", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
}
