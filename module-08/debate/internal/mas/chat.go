package mas

import (
	"context"
	"strings"

	"github.com/q1mi/debate/internal/llm"
)

// chat 是本章多个拓扑共享的基础模型调用助手。
func chat(ctx context.Context, provider llm.Provider, model, system, user string) (string, error) {
	resp, err := provider.Chat(ctx, llm.ChatRequest{
		Model: strings.TrimSpace(model),
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: strings.TrimSpace(system)},
			{Role: llm.RoleUser, Content: strings.TrimSpace(user)},
		},
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Content), nil
}
