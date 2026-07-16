package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

// eino components: chatTemplate

func main() {

	chatTemplateDemo(context.Background())

}

func chatTemplateDemo(ctx context.Context) {

	// 准备chatModel
	cm, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		APIKey:  os.Getenv("DOUBAO_API_KEY"),
		BaseURL: os.Getenv("DOUBAO_BASE_URL"),
		Model:   os.Getenv("DOUBAO_MODEL"),
	})
	if err != nil {
		log.Fatalf("create chat model failed, err:%v", err)
	}

	// 准备消息
	// msgs := []*schema.Message{
	// 	schema.SystemMessage("你是一个Go语言学习助手"),
	// 	schema.UserMessage("两句话介绍下eino框架"),
	// }

	// prompt.ChatTemplate

	tpl := prompt.FromMessages(
		schema.FString,
		schema.SystemMessage(
			"你是{brand}客服。只依据已知资料回答。",
		),
		schema.MessagesPlaceholder("history", true), // 可以不传
		schema.UserMessage(
			"资料：\n{context}\n\n问题：{question}",
		),
	)

	retrievedContext := `
	《Aurora 品牌售后手册》
	## 退货须知
	1. 拆封后不允许退货。
	`
	userQuestion := "拆开试了一下不好用，我要退货！"

	messages, err := tpl.Format(ctx, map[string]any{
		"brand": "Aurora",
		// "history":  history,  // []*schema.Message{}
		"context":  retrievedContext,
		"question": userQuestion,
	})
	if err != nil {
		log.Fatalf("tpl format failed, err:%v", err)
	}

	outMsg, err := cm.Generate(ctx, messages)
	if err != nil {
		log.Fatalf("generate failed, err:%v", err)
	}

	fmt.Printf("outMsg:%#v\n", outMsg)

}
