package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino-ext/components/indexer/milvus2"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

const (
	milvusIDField       = "id"
	milvusContentField  = "content"
	milvusMetadataField = "metadata"
)

// indexDocuments 使用 Eino Milvus Indexer 将已向量化的文档写入本地 Milvus。
// 文档 ID 稳定，重复执行会通过 Upsert 更新已有数据。
func indexDocuments(ctx context.Context, cfg appConfig, docs []*schema.Document) ([]string, error) {
	client, err := milvusclient.New(ctx, &milvusclient.ClientConfig{Address: cfg.MilvusAddress})
	if err != nil {
		return nil, fmt.Errorf("连接 Milvus %q: %w", cfg.MilvusAddress, err)
	}
	defer client.Close(ctx)

	indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
		Client:           client,
		Collection:       cfg.Collection,
		ConsistencyLevel: milvus2.ConsistencyLevelStrong,
		Vector: &milvus2.VectorConfig{
			Dimension:    int64(cfg.EmbeddingDimension),
			MetricType:   milvus2.COSINE,
			VectorField:  cfg.VectorField,
			IndexBuilder: milvus2.NewAutoIndexBuilder(),
		},
		// embedding.go 已生成向量，这里采用 BYOV 写入方式。
		Embedding:         nil,
		DocumentConverter: milvusDocumentConverter(cfg.VectorField, cfg.EmbeddingDimension),
	})
	if err != nil {
		return nil, fmt.Errorf("初始化 Milvus Indexer: %w", err)
	}

	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		return nil, fmt.Errorf("写入 Milvus collection %q: %w", cfg.Collection, err)
	}
	return ids, nil
}

func milvusDocumentConverter(vectorField string, dimension int) func(context.Context, []*schema.Document, [][]float64) ([]column.Column, error) {
	return func(_ context.Context, docs []*schema.Document, _ [][]float64) ([]column.Column, error) {
		ids := make([]string, 0, len(docs))
		contents := make([]string, 0, len(docs))
		metadata := make([][]byte, 0, len(docs))
		vectors := make([][]float32, 0, len(docs))

		for i, doc := range docs {
			if doc == nil {
				return nil, fmt.Errorf("第 %d 个文档为空", i)
			}
			if doc.ID == "" {
				return nil, fmt.Errorf("第 %d 个文档缺少 ID", i)
			}
			vector := doc.DenseVector()
			if len(vector) != dimension {
				return nil, fmt.Errorf("文档 %q 向量维度为 %d，期望 %d", doc.ID, len(vector), dimension)
			}

			vector32 := make([]float32, len(vector))
			for j, value := range vector {
				vector32[j] = float32(value)
			}
			metaJSON, err := json.Marshal(publicMetadata(doc.MetaData))
			if err != nil {
				return nil, fmt.Errorf("序列化文档 %q 元数据: %w", doc.ID, err)
			}

			ids = append(ids, doc.ID)
			contents = append(contents, doc.Content)
			metadata = append(metadata, metaJSON)
			vectors = append(vectors, vector32)
		}

		return []column.Column{
			column.NewColumnVarChar(milvusIDField, ids),
			column.NewColumnVarChar(milvusContentField, contents),
			column.NewColumnJSONBytes(milvusMetadataField, metadata),
			column.NewColumnFloatVector(vectorField, dimension, vectors),
		}, nil
	}
}

func publicMetadata(metadata map[string]any) map[string]any {
	clean := make(map[string]any, len(metadata))
	for key, value := range metadata {
		switch key {
		case "_dense_vector", "_sparse_vector", "_score":
			continue
		default:
			clean[key] = value
		}
	}
	return clean
}
