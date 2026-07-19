package main

import (
	"context"
	"encoding/json"
	"fmt"

	einembedding "github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// retrieveDocuments 将问题向量化，并在 Milvus 中执行余弦相似度检索。
func retrieveDocuments(ctx context.Context, cfg appConfig, embedder einembedding.Embedder, query string, topK int) ([]*schema.Document, error) {
	if topK <= 0 {
		return nil, fmt.Errorf("topK 必须大于 0")
	}
	queryVector, err := embedQuery(ctx, embedder, query, cfg.EmbeddingDimension)
	if err != nil {
		return nil, err
	}
	vector32 := make([]float32, len(queryVector))
	for i, value := range queryVector {
		vector32[i] = float32(value)
	}

	client, err := milvusclient.New(ctx, &milvusclient.ClientConfig{Address: cfg.MilvusAddress})
	if err != nil {
		return nil, fmt.Errorf("连接 Milvus %q: %w", cfg.MilvusAddress, err)
	}
	defer client.Close(ctx)

	resultSets, err := client.Search(ctx, milvusclient.NewSearchOption(
		cfg.Collection,
		topK,
		[]entity.Vector{entity.FloatVector(vector32)},
	).
		WithANNSField(cfg.VectorField).
		WithOutputFields(milvusContentField, milvusMetadataField).
		WithConsistencyLevel(entity.ClStrong))
	if err != nil {
		return nil, fmt.Errorf("检索 Milvus collection %q: %w", cfg.Collection, err)
	}
	if len(resultSets) == 0 {
		return nil, nil
	}

	resultSet := resultSets[0]
	contentColumn := resultSet.GetColumn(milvusContentField)
	metadataColumn := resultSet.GetColumn(milvusMetadataField)
	if resultSet.IDs == nil || contentColumn == nil || metadataColumn == nil {
		return nil, fmt.Errorf("Milvus 检索结果缺少 id、content 或 metadata 字段")
	}

	docs := make([]*schema.Document, 0, resultSet.Len())
	for i := 0; i < resultSet.Len(); i++ {
		id, err := columnString(resultSet.IDs, i)
		if err != nil {
			return nil, fmt.Errorf("读取第 %d 条结果 ID: %w", i, err)
		}
		content, err := columnString(contentColumn, i)
		if err != nil {
			return nil, fmt.Errorf("读取第 %d 条结果内容: %w", i, err)
		}
		metadata, err := columnMetadata(metadataColumn, i)
		if err != nil {
			return nil, fmt.Errorf("读取第 %d 条结果元数据: %w", i, err)
		}

		doc := &schema.Document{ID: id, Content: content, MetaData: metadata}
		if i < len(resultSet.Scores) {
			doc.WithScore(float64(resultSet.Scores[i]))
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func columnString(col column.Column, index int) (string, error) {
	value, err := col.Get(index)
	if err != nil {
		return "", err
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("字段类型为 %T", value)
	}
	return text, nil
}

func columnMetadata(col column.Column, index int) (map[string]any, error) {
	value, err := col.Get(index)
	if err != nil {
		return nil, err
	}
	var raw []byte
	switch typed := value.(type) {
	case []byte:
		raw = typed
	case string:
		raw = []byte(typed)
	default:
		return nil, fmt.Errorf("metadata 字段类型为 %T", value)
	}

	metadata := make(map[string]any)
	if len(raw) == 0 {
		return metadata, nil
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return nil, err
	}
	return metadata, nil
}
