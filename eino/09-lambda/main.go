package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// eino components: lambda
// compose.Lambda

func main() {
	lambdaDemo(context.Background())

}

// formatDocuments 业务逻辑函数
func formatDocuments(ctx context.Context, docs []*schema.Document) (string, error) {
	var b strings.Builder
	for i, doc := range docs {
		fmt.Fprintf(&b, "[%d] %s\n", i+1, doc.Content)
	}
	return b.String(), nil
}

func lambdaDemo(ctx context.Context) {
	// 把业务逻辑函数转为 Lambda
	formatDocs := compose.InvokableLambda(formatDocuments)

	// 编排 Lambda 节点
	chain := compose.NewChain[[]*schema.Document, string]()
	chain.AppendLambda(formatDocs)

	runnable, err := chain.Compile(ctx)
	if err != nil {
		log.Fatalf("compile failed, err:%v", err)
	}

	// mock documents 数据
	docs := []*schema.Document{
		{ID: "doc-1", Content: "Eino 框架是一个大模型应用开发框架"},
		{ID: "doc-2", Content: "Lambda 可以将自定义的业务逻辑函数转为可编排的节点"},
	}
	result, err := runnable.Invoke(ctx, docs)
	if err != nil {
		log.Fatalf("Invoke failed, err:%v", err)
	}
	fmt.Printf("result:%#v\n", result)
}
