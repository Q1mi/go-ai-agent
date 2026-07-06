package rag

import (
	"context"
	"fmt"
	"strings"

	"github.com/q1mi/minikb/internal/tool"
)

// SearchArgs 是知识库检索工具参数。
type SearchArgs struct {
	Query string `json:"query" desc:"要在知识库中检索的查询，请提炼出精准关键词或问题"`
	TopK  int    `json:"top_k,omitempty" desc:"返回片段数量，默认 5"`
}

// SearchTool 把检索器包装成 Agent 工具。
func SearchTool(retriever *Retriever) tool.Tool {
	return tool.NewTypedTool("search_knowledge_base",
		"在课程知识库中检索资料。回答课程、产品、发布说明或规范问题时调用；可以用不同查询多次调用。",
		func(ctx context.Context, args SearchArgs) (string, error) {
			query := strings.TrimSpace(args.Query)
			if query == "" {
				return "", fmt.Errorf("query 不能为空")
			}
			topK := args.TopK
			if topK <= 0 {
				topK = 5
			}
			if topK > 10 {
				topK = 10
			}
			docs, err := retriever.Retrieve(ctx, query, topK)
			if err != nil {
				return "", err
			}
			return FormatDocuments(docs), nil
		})
}

// FormatDocuments 把检索结果格式化为带来源的文本。
func FormatDocuments(docs []Document) string {
	if len(docs) == 0 {
		return "知识库中未找到相关资料。"
	}
	var sb strings.Builder
	for i, doc := range docs {
		fmt.Fprintf(&sb, "[来源 %d | doc=%s | chunk=%d | score=%.3f]\n%s\n\n",
			i+1, doc.SourceLabel(), doc.ChunkIndex, doc.Score, strings.TrimSpace(doc.Content))
	}
	return strings.TrimSpace(sb.String())
}
