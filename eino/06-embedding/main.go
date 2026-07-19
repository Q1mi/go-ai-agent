package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/ark"
)

// eio components: Embedding

// 注意：eino框架的版本，不要用 v0.6.0，至少 v0.9.12
// go get github.com/cloudwego/eino@latest

// 使用 doubao-embedding-vision-250615 模型

var (
	defaultDim     = 2048
	defaultTimeout = time.Second * 30
)

func main() {
	embeddingDemo(context.Background())
}

func embeddingDemo(ctx context.Context) {
	// 初始化
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

	// 向量化
	vectorIDs, err := embedder.EmbedStrings(ctx, []string{"拆封后不支持退货！", "不支持价格保护！"})
	if err != nil {
		log.Fatalf("embedding failed,err:%v", err)
	}
	for _, v := range vectorIDs {
		fmt.Printf("v:%#v\n", v)
	}

}
