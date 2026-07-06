package ctxeng

import "testing"

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{name: "empty", text: "", want: 0},
		{name: "ascii", text: "abcdefgh", want: 3},
		{name: "cjk", text: "你好世界", want: 3},
		{name: "mixed", text: "Go语言Agent", want: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens(tt.text)
			if got != tt.want {
				t.Fatalf("EstimateTokens(%q)=%d, want %d", tt.text, got, tt.want)
			}
		})
	}
}

func TestBudgetOver(t *testing.T) {
	budget := Budget{Total: 12, SystemPrompt: 2, Tools: 2, History: 2, Retrieved: 2, OutputReserve: 2}
	over := budget.Over("系统提示很多", "工具很多", "历史很多", "检索很多")
	for _, key := range []string{"system", "tools", "history", "retrieved", "total"} {
		if _, ok := over[key]; !ok {
			t.Fatalf("expected %s over budget, got %#v", key, over)
		}
	}
}
