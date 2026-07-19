package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
	"github.com/cloudwego/eino-ext/components/retriever/milvus2"
	"github.com/cloudwego/eino-ext/components/retriever/milvus2/search_mode"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// eino components: retriever
// go get github.com/cloudwego/eino@latest

var (
	defaultDim     = 2048
	defaultTimeout = time.Second * 30
)

func main() {
	retrieverDemo(context.Background())
}

func retrieverDemo(ctx context.Context) {

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

	// 创建 retriever
	retriever, err := milvus2.NewRetriever(ctx, &milvus2.RetrieverConfig{
		ClientConfig: &milvusclient.ClientConfig{
			Address: "localhost:19530",
		},
		Collection: "my_collection",
		TopK:       5,
		SearchMode: search_mode.NewApproximate(milvus2.COSINE),
		Embedding:  embedder,
	})

	// 检索文档
	documents, err := retriever.Retrieve(ctx, "耳机拆封后试了一下不合适能退吗？")
	if err != nil {
		log.Fatalf("retrive failed, err:%v", err)
	}

	for _, doc := range documents {
		fmt.Printf("docID:%v content:%v score:%v\n", doc.ID, doc.Content, doc.Score())
	}
}
