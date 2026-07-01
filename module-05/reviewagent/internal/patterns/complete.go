package patterns

import (
	"context"

	"github.com/q1mi/reviewagent/internal/llm"
)

// Complete 是一次最朴素的模型调用：给 system 和 user，返回文本。
//
// M05 的多个模式都建立在这个轻量组件之上。需要工具循环时再换成 M04 的 Agent。
func Complete(ctx context.Context, provider llm.Provider, model, system, user string) (string, error) {
	resp, err := provider.Chat(ctx, llm.ChatRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: system},
			{Role: llm.RoleUser, Content: user},
		},
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}
