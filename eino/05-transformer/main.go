package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino-ext/components/document/loader/file"
	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

// eino components: Document Transformer

func main() {
	ctx := context.Background()
	docs := loadFile(ctx, "docs/Aurora品牌数码产品售后服务手册.md")

	transformerDemo(ctx, docs)

}

func transformerDemo(ctx context.Context, docs []*schema.Document) []*schema.Document {
	// 初始化
	transformer, err := markdown.NewHeaderSplitter(ctx, &markdown.HeaderConfig{
		Headers: map[string]string{
			"##":  "h2",
			"###": "h3",
		},
	})
	if err != nil {
		log.Fatalf("NewHeaderSplitter failed,err:%v", err)
	}

	// 转换文档
	transformedDocs, err := transformer.Transform(ctx, docs)
	if err != nil {
		log.Fatalf("transform failed, err:%v", err)
	}

	// 打印下看看效果
	for _, doc := range transformedDocs {
		fmt.Printf("doc:%#v\n", doc)
	}
	return transformedDocs
}

func loadFile(ctx context.Context, src string) []*schema.Document {
	loader, err := file.NewFileLoader(ctx, &file.FileLoaderConfig{})
	if err != nil {
		log.Fatalf("NewFileLoader failed, err:%v", err)
	}
	// 加载文档
	docs, err := loader.Load(ctx, document.Source{
		URI: src,
	})
	if err != nil {
		log.Fatalf("load failed, err:%v", err)
	}
	// docs
	for _, doc := range docs {
		fmt.Printf("doc:%#v\n", doc)
	}
	return docs
}
