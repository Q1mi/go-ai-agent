package llm

import (
	"strings"
	"testing"
)

func TestDecodeChatResponse(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantContent string
		wantInput   int
		wantOutput  int
		wantErr     bool
	}{
		{
			name:        "正常响应",
			body:        `{"choices":[{"message":{"role":"assistant","content":"你好"}}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`,
			wantContent: "你好",
			wantInput:   3,
			wantOutput:  2,
		},
		{
			name:    "缺少 choices",
			body:    `{"choices":[],"usage":{}}`,
			wantErr: true,
		},
		{
			name:    "非法 JSON",
			body:    `{"choices":`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeChatResponse(strings.NewReader(tt.body))
			if tt.wantErr {
				if err == nil {
					t.Fatal("期望返回错误")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Content != tt.wantContent {
				t.Fatalf("Content = %q, want %q", got.Content, tt.wantContent)
			}
			if got.InputTokens != tt.wantInput || got.OutputTokens != tt.wantOutput {
				t.Fatalf(
					"tokens = (%d, %d), want (%d, %d)",
					got.InputTokens,
					got.OutputTokens,
					tt.wantInput,
					tt.wantOutput,
				)
			}
		})
	}
}
