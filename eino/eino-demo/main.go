package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/cloudwego/eino-ext/components/model/ark" // eino 组件
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
	// "github.com/cloudwego/eino-ext/components/model/deepseek" // eino 组件
	// "github.com/cloudwego/eino-ext/components/model/ark" // eino 组件
)

// eino 框架 模型调用示例

func main() {
	ctx := context.Background()

	// 1. 创建大模型 client  ---> 大模型厂商
	key := os.Getenv("DEEPSEEK_API_KEY")
	modelName := os.Getenv("DEEPSEEK_MODEL")
	baseURL := os.Getenv("DEEPSEEK_BASE_URL")
	// chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
	// chatModel, err := deepseek.NewChatModel(ctx, &deepseek.ChatModelConfig{
	chatModel, err := ark.NewChatModel(ctx, &ark.ChatModelConfig{
		BaseURL: baseURL,
		Model:   modelName,
		APIKey:  key,
	})
	if err != nil {
		log.Fatalf("create chat model failed,err:%v", err)
	}
	// 2. 准备发送的消息 ， prompt
	chatTemplate := prompt.FromMessages(
		schema.FString,
		schema.SystemMessage("你是一个Go语言编程助手，擅长回答Go语言相关技术问题。"),
		schema.UserMessage("{question}"),
	)
	msg, err := chatTemplate.Format(ctx, map[string]any{"question": "李文周老师是谁？"})
	if err != nil {
		log.Fatalf("format msg failed, err:%v", err)
	}
	// 3. 发送请求
	// reply, err := chatModel.Generate(ctx, msg, model.WithTemperature(0.1))
	reply, err := chatModel.Stream(ctx, msg, model.WithTemperature(0.1))
	if err != nil {
		log.Fatalf("generate failed, err:%v", err)
	}
	// 4. 接收消息
	// fmt.Printf("reply:%#v\n", reply)  // 同步调用的返回结果
	// 流式输出
	for {
		msg, err := reply.Recv()
		if errors.Is(err, io.EOF) { // err == io.EOF
			return
		}
		if err != nil {
			log.Fatalf("recv failed, err:%v", err)
		}
		fmt.Print(msg.Content)
		os.Stdout.Sync() // 强制刷新标准输出
	}
}
