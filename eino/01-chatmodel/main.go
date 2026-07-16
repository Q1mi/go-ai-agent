package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/ark" // 火山
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// eino components: chatModel

func main() {
	chatModelDemo(context.Background())
}

func chatModelDemo(ctx context.Context) {
	// chatModel

	// model.BaseChatModel  --> ark.ChatModel (具体厂商实现)
	// model.BaseChatModel
	cm, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		APIKey:  os.Getenv("DOUBAO_API_KEY"),
		BaseURL: os.Getenv("DOUBAO_BASE_URL"),
		Model:   os.Getenv("DOUBAO_MODEL"),
	})
	if err != nil {
		log.Fatalf("create chat model failed, err:%v", err)
	}

	// schema.Message
	messages := []*schema.Message{
		schema.SystemMessage("你是一个Go语言的技术讲师"),
		schema.UserMessage("请用两句话介绍下eino框架"),
	}
	// 发送消息
	// Generate / Stream
	outMsg, err := cm.Generate(
		ctx,
		messages,
		model.WithTemperature(0.2), // 设置温度
	)
	if err != nil {
		log.Fatalf("generate failed, err:%v", err)
	}
	fmt.Printf("outMsg:%#v\n", outMsg)
}
