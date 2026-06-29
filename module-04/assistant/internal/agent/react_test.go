package agent

import "testing"

func TestParseReact(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		wantAction string
		wantFinal  string
		wantErr    bool
	}{
		{
			name:       "tool call",
			text:       "Thought: 需要计算\nAction: calculator\nAction Input: {\"expr\":\"1+2\"}",
			wantAction: "calculator",
		},
		{
			name:      "final answer",
			text:      "Thought: 已完成\nFinal Answer: 答案是 3",
			wantFinal: "答案是 3",
		},
		{
			name:    "invalid",
			text:    "我要调用 calculator",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseReact(tt.text)
			if tt.wantErr {
				if err == nil {
					t.Fatal("期望错误")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Action != tt.wantAction {
				t.Fatalf("Action = %q, want %q", got.Action, tt.wantAction)
			}
			if got.FinalAnswer != tt.wantFinal {
				t.Fatalf("FinalAnswer = %q, want %q", got.FinalAnswer, tt.wantFinal)
			}
		})
	}
}
