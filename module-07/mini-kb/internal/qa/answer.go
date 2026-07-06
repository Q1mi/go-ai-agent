package qa

import (
	"context"
	"fmt"
	"strings"

	"github.com/q1mi/minikb/internal/llm"
	"github.com/q1mi/minikb/internal/rag"
)

// AnswerWithSources 基于检索结果生成答案。provider 为空时返回可离线运行的资料摘要。
func AnswerWithSources(ctx context.Context, provider llm.Provider, model string, question string, docs []rag.Document) (string, error) {
	contextText := rag.FormatDocuments(docs)
	if len(docs) == 0 || strings.Contains(contextText, "未找到相关资料") {
		return "知识库中未找到足够资料，无法给出有依据的回答。", nil
	}
	if provider == nil {
		return extractiveAnswer(question, docs), nil
	}
	system := "你是知识库问答助手。只能依据给定资料回答。回答必须标注来源，格式使用 [来源 N]。资料不足时明确说明。"
	user := "用户问题：\n" + question + "\n\n检索资料：\n" + contextText
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
	return strings.TrimSpace(resp.Content), nil
}

func extractiveAnswer(question string, docs []rag.Document) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "问题：%s\n\n", strings.TrimSpace(question))
	sb.WriteString("根据知识库检索结果，最相关资料如下：\n\n")
	for i, doc := range docs {
		fmt.Fprintf(&sb, "[来源 %d] %s\n%s\n\n", i+1, doc.SourceLabel(), strings.TrimSpace(doc.Content))
	}
	sb.WriteString("如需更自然的综合回答，请配置 LLM_* 环境变量后使用 `ask --llm`。")
	return strings.TrimSpace(sb.String())
}
