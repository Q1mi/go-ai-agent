package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	einembedding "github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
)

func newEmbedder(ctx context.Context, cfg appConfig) (einembedding.Embedder, error) {
	if cfg.EmbeddingAPIKey == "" {
		return nil, fmt.Errorf("缺少 Ark API Key，请设置 ARK_API_KEY 或 DOUBAO_API_KEY")
	}
	if cfg.EmbeddingModel == "" {
		return nil, fmt.Errorf("缺少 Embedding 模型，请设置 ARK_EMBEDDING_MODEL")
	}

	dimension := cfg.EmbeddingDimension
	timeout := 60 * time.Second
	apiType, err := arkAPIType(cfg.EmbeddingAPIType)
	if err != nil {
		return nil, err
	}
	arkConfig := &ark.EmbeddingConfig{
		APIKey:     cfg.EmbeddingAPIKey,
		Model:      cfg.EmbeddingModel,
		Dimensions: &dimension,
		Timeout:    &timeout,
		APIType:    &apiType,
	}
	if cfg.EmbeddingBaseURL != "" {
		arkConfig.BaseURL = cfg.EmbeddingBaseURL
	}

	embedder, err := ark.NewEmbedder(ctx, arkConfig)
	if err != nil {
		return nil, fmt.Errorf("初始化 Ark Embedder: %w", err)
	}
	return embedder, nil
}

func arkAPIType(value string) (ark.APIType, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "text", "text_api":
		return ark.APITypeText, nil
	case "multimodal", "multi_modal", "multi_modal_api":
		return ark.APITypeMultiModal, nil
	default:
		return "", fmt.Errorf("ARK_EMBEDDING_API_TYPE 仅支持 text 或 multimodal，当前值为 %q", value)
	}
}

// embedDocuments 分批向量化文档，并把向量写入 Eino Document。
func embedDocuments(ctx context.Context, embedder einembedding.Embedder, docs []*schema.Document, dimension, batchSize int) error {
	if embedder == nil {
		return fmt.Errorf("embedder 不能为空")
	}
	if batchSize <= 0 {
		return fmt.Errorf("batchSize 必须大于 0")
	}

	for start := 0; start < len(docs); start += batchSize {
		end := min(start+batchSize, len(docs))
		texts := make([]string, 0, end-start)
		for i := start; i < end; i++ {
			if docs[i] == nil || strings.TrimSpace(docs[i].Content) == "" {
				return fmt.Errorf("第 %d 个文档为空", i)
			}
			texts = append(texts, docs[i].Content)
		}

		vectors, err := embedder.EmbedStrings(ctx, texts)
		if err != nil {
			return fmt.Errorf("向量化第 %d-%d 个文档: %w", start, end-1, err)
		}
		if len(vectors) != len(texts) {
			return fmt.Errorf("Embedding 返回数量异常: 期望 %d，实际 %d", len(texts), len(vectors))
		}
		for i, vector := range vectors {
			if len(vector) != dimension {
				return fmt.Errorf("文档 %q 的向量维度为 %d，配置维度为 %d，请调整 EMBEDDING_DIMENSION", docs[start+i].ID, len(vector), dimension)
			}
			docs[start+i].WithDenseVector(vector)
		}
	}
	return nil
}

func embedQuery(ctx context.Context, embedder einembedding.Embedder, query string, dimension int) ([]float64, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("检索问题不能为空")
	}
	vectors, err := embedder.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("向量化检索问题: %w", err)
	}
	if len(vectors) != 1 {
		return nil, fmt.Errorf("Embedding 应返回 1 个查询向量，实际返回 %d 个", len(vectors))
	}
	if len(vectors[0]) != dimension {
		return nil, fmt.Errorf("查询向量维度为 %d，配置维度为 %d，请调整 EMBEDDING_DIMENSION", len(vectors[0]), dimension)
	}
	return vectors[0], nil
}
