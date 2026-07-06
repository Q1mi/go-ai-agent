package ctxeng

import (
	"context"
	"strings"
	"testing"

	"github.com/q1mi/ctxagent/internal/llm"
)

func TestCompact(t *testing.T) {
	tests := []struct {
		name       string
		keepRecent int
		messages   []llm.Message
		wantLen    int
	}{
		{
			name:       "compact older messages",
			keepRecent: 2,
			messages: []llm.Message{
				{Role: llm.RoleSystem, Content: "system"},
				{Role: llm.RoleUser, Content: "old user"},
				{Role: llm.RoleAssistant, Content: "old assistant"},
				{Role: llm.RoleUser, Content: "recent user"},
				{Role: llm.RoleAssistant, Content: "recent assistant"},
			},
			wantLen: 4,
		},
		{
			name:       "short history stays unchanged",
			keepRecent: 4,
			messages: []llm.Message{
				{Role: llm.RoleSystem, Content: "system"},
				{Role: llm.RoleUser, Content: "recent user"},
			},
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Compact(context.Background(), tt.messages, tt.keepRecent, func(ctx context.Context, older []llm.Message) (string, error) {
				var parts []string
				for _, msg := range older {
					parts = append(parts, msg.Content)
				}
				return strings.Join(parts, " | "), nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tt.wantLen {
				t.Fatalf("len(Compact())=%d, want %d", len(got), tt.wantLen)
			}
			if got[0].Role != llm.RoleSystem {
				t.Fatalf("system message should be preserved")
			}
			if len(tt.messages) > tt.keepRecent+1 && !strings.Contains(got[1].Content, "早前对话摘要") {
				t.Fatalf("summary message missing, got %#v", got)
			}
		})
	}
}

func TestAssembleWithReport(t *testing.T) {
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: "system"},
		{Role: llm.RoleUser, Content: strings.Repeat("历史", 100)},
		{Role: llm.RoleAssistant, Content: strings.Repeat("回答", 100)},
		{Role: llm.RoleUser, Content: "最近问题"},
		{Role: llm.RoleAssistant, Content: "最近回答"},
	}
	got, report, err := AssembleWithReport(context.Background(), messages, AssembleConfig{
		Budget:     Budget{History: 80},
		KeepRecent: 2,
		Summarize: func(ctx context.Context, older []llm.Message) (string, error) {
			return "摘要：保留关键事实", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Compactions == 0 {
		t.Fatalf("expected compaction report, got %#v", report)
	}
	if len(got) >= len(messages) {
		t.Fatalf("messages should be compacted")
	}
}
