package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino-ext/components/document/loader/file"
	urlloader "github.com/cloudwego/eino-ext/components/document/loader/url"
	"github.com/cloudwego/eino/components/document"
)

// eino components: Document Loader

/*
用户的问题 + 知识库（资料）         --> LLM  --> 满意的回答

用户的问题 + 跟问题相关的知识（资料） --> LLM  --> 满意的回答

RAG
准备数据  --> 检索数据  --> 使用数据

Loader --> transformer --> embedding --> indexer --> retriever
*/

func main() {
	// loadFile(context.Background(), "docs/Aurora品牌数码产品售后服务手册.md")
	loadURL(context.Background(), "https://www.cloudwego.io/zh/docs/eino/core_modules/components/document_loader_guide/document_parser_interface_guide/")
}

func loadFile(ctx context.Context, src string) {
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
}

func loadURL(ctx context.Context, src string) {
	// 从url加载数据
	loader, err := urlloader.NewLoader(ctx, &urlloader.LoaderConfig{})
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
}
