package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino-ext/components/indexer/milvus2"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// eino components: indexder
// 注意：eino框架的版本，不要用 v0.6.0，至少 v0.9.12
// go get github.com/cloudwego/eino@latest

var (
	defaultDim     = 2048
	defaultTimeout = time.Second * 30
)

func main() {
	indexerDemo(context.Background())
}

func indexerDemo(ctx context.Context) {

	// embedder
	embedder, err := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
		APIKey:     os.Getenv("DOUBAO_API_KEY"),
		Model:      "doubao-embedding-vision-250615",
		BaseURL:    "https://ark.cn-beijing.volces.com/api/v3",
		Dimensions: &defaultDim,
		Timeout:    &defaultTimeout,
		APIType:    new(ark.APITypeMultiModal), // 注意这里要根据使用的模型填写！！！
	})
	if err != nil {
		log.Fatalf("new embedder failed,err:%v", err)
	}

	// 创建索引器
	indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
		ClientConfig: &milvusclient.ClientConfig{
			Address: "localhost:19530",
		},
		Collection: "my_collection",
		Vector: &milvus2.VectorConfig{
			Dimension:  int64(defaultDim), // 与 embedding 模型维度匹配
			MetricType: milvus2.COSINE,
			// IndexBuilder: milvus2.NewHNSWIndexBuilder().WithM(16).WithEfConstruction(200),
			IndexBuilder: milvus2.NewAutoIndexBuilder(),
		},
		Embedding: embedder,
	})
	if err != nil {
		log.Fatalf("new indexer failed, err:%v", err)
	}

	// 索引文档
	docs := []*schema.Document{
		{
			ID:      "doc1",
			Content: "EINO 是一个用于构建人工智能应用的框架！",
			MetaData: map[string]any{
				"year": 2021,
			},
		},
		{
			ID:      "doc2",
			Content: "Aurora品牌耳机拆封后概不退货",
			MetaData: map[string]any{
				"source": "Aurora品牌售后服务手册",
			},
		},
		{
			ID:      "doc3",
			Content: "《宠物厌食症状识别与干预》 —— 当宠物连续 24 小时拒食或显著减食，常见原因有应激反应、消化系统疾病、口腔问题……",
		},
	}
	ids, err := indexer.Store(ctx, docs)
	if err != nil {
		log.Fatalf("store failde, err:%v", err)
	}

	fmt.Printf("ids:%#v\n", ids)
}
