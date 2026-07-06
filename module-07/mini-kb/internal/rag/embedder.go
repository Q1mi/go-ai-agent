package rag

import "context"

// Embedder 把文本批量转换为向量。
type Embedder interface {
	Dim() int
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}
